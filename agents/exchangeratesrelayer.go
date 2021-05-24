package agents

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/sacOO7/gowebsocket"
	"math"
	"math/big"
	"os"
	"os/signal"
	"portalfeeders/entities"
	"portalfeeders/utils"
	"sort"
	"strconv"
)

type ExchangeRatesRelayer struct {
	AgentAbs
	RestfulClient     *utils.RestfulClient
	IsProcessingStack bool
	StackOrder        map[string]Price
	WSTokens          []WSSPrices
}

type Price struct {
	total float64
	count int
}

type WSSPrices struct {
	StreamName string
	TokenId    string // this is id of token in incognito chain
}

type PriceItem struct {
	Symbol string
	Price  string
}

type WSData struct {
	E  string `json:"e"`
	EE int    `json:"E"`
	S  string `json:"s"`
	A  int    `json:"a"`
	P  string `json:"p"`
	Q  string `json:"q"`
	F  int    `json:"f"`
	L  int    `json:"l"`
	T  int    `json:"t"`
	M  bool   `json:"m"`
	MM bool   `json:"M"`
}

type Stream struct {
	StreamName string `json:"stream"`
	Data       WSData `json:"data"`
}

func (b *ExchangeRatesRelayer) getPublicTokenRates(symbol string) (PriceItem, error) {
	symbolFilter := map[string]string{
		"symbol": symbol,
	}

	result, err := b.RestfulClient.Get("/ticker/price", nil, symbolFilter)
	if err != nil {
		return PriceItem{}, err
	}

	var priceItems PriceItem
	err = json.Unmarshal(result, &priceItems)
	if err != nil {
		b.Logger.Errorf("ExchangeRatesRelayer: has a error when unmarshal, %v\n", err)
		return PriceItem{}, errors.New("ExchangeRatesRelayer: has a error when unmarshal")
	}

	return priceItems, nil
}

func (b *ExchangeRatesRelayer) getLatestBeaconHeight() (uint64, error) {
	params := []interface{}{}
	var beaconBestStateRes entities.BeaconBestStateRes
	err := b.RPCClient.RPCCall("getbeaconbeststate", params, &beaconBestStateRes)
	if err != nil {
		return 0, err
	}

	if beaconBestStateRes.RPCError != nil {
		b.Logger.Errorf("getLatestBeaconHeight: call RPC error, %v\n", beaconBestStateRes.RPCError.StackTrace)
		return 0, errors.New(beaconBestStateRes.RPCError.Message)
	}
	return beaconBestStateRes.Result.BeaconHeight, nil
}

func (b *ExchangeRatesRelayer) getPDEState(beaconHeight uint64) (*entities.PDEState, error) {
	params := []interface{}{
		map[string]uint64{
			"BeaconHeight": beaconHeight,
		},
	}
	var pdeStateRes entities.PDEStateRes
	err := b.RPCClient.RPCCall("getpdestate", params, &pdeStateRes)
	if err != nil {
		return nil, err
	}

	if pdeStateRes.RPCError != nil {
		b.Logger.Errorf("getPDEState: call RPC error, %v\n", pdeStateRes.RPCError.StackTrace)
		return nil, errors.New(pdeStateRes.RPCError.Message)
	}
	return pdeStateRes.Result, nil
}

func (b *ExchangeRatesRelayer) getPRVRate() (uint64, error) {
	latestBeaconHeight, err := b.getLatestBeaconHeight()
	if err != nil {
		return 0, err
	}
	pdeState, err := b.getPDEState(latestBeaconHeight)
	if err != nil {
		return 0, err
	}
	poolPairs := pdeState.PDEPoolPairs
	var prices []float64
	for _, v := range ListToken {
		prvPustPairKey := fmt.Sprintf("pdepool-%d-%s-%s", latestBeaconHeight, PRVID, v.tokenID)
		prvPustPair, found := poolPairs[prvPustPairKey]
		if !found || prvPustPair.Token1PoolValue == 0 || prvPustPair.Token2PoolValue == 0 {
			return 0, nil
		}

		tokenPoolValueToBuy := prvPustPair.Token1PoolValue
		tokenPoolValueToSell := prvPustPair.Token2PoolValue
		if prvPustPair.Token1IDStr == PRVID {
			tokenPoolValueToSell = prvPustPair.Token1PoolValue
			tokenPoolValueToBuy = prvPustPair.Token2PoolValue
		}

		invariant := big.NewInt(0)
		invariant.Mul(big.NewInt(int64(tokenPoolValueToSell)), big.NewInt(int64(tokenPoolValueToBuy)))
		newTokenPoolValueToSell := big.NewInt(0)
		newTokenPoolValueToSell.Add(big.NewInt(int64(tokenPoolValueToSell)), big.NewInt(int64(1e9)))

		newTokenPoolValueToBuy := big.NewInt(0).Div(invariant, newTokenPoolValueToSell).Uint64()
		modValue := big.NewInt(0).Mod(invariant, newTokenPoolValueToSell)
		if modValue.Cmp(big.NewInt(0)) != 0 {
			newTokenPoolValueToBuy++
		}
		if tokenPoolValueToBuy <= newTokenPoolValueToBuy {
			return 0, nil
		}
		prices = append(prices, float64(tokenPoolValueToBuy-newTokenPoolValueToBuy)/math.Pow(10, float64(v.decimals)))
	}
	sort.Slice(prices, func(i, j int) bool { return prices[i] < prices[j] })
	// sort then get median value
	return uint64(prices[len(prices)/2] * USDdecimals), nil
}

func (b *ExchangeRatesRelayer) convertPublicTokenPriceToPToken(price *big.Float) uint64 {
	var e6 = new(big.Float).SetFloat64(math.Pow10(6))
	mul := new(big.Float).Mul(price, e6)

	result := new(big.Int)
	mul.Int(result)

	b.Logger.Infof("ExchangeRatesRelayer: Convert public token to pToken, price: %+v, result %+v\n", price.String(), result.Uint64())

	return result.Uint64()
}

func (b *ExchangeRatesRelayer) pushExchangeRates(
	prvRate uint64,
) error {
	rates := make(map[string]uint64)

	//fill data
	if prvRate > 0 {
		rates[PRVID] = prvRate
	}

	for _, v := range b.WSTokens {
		price, ok := b.StackOrder[v.StreamName]
		if !ok || price.count == 0 {
			msg := fmt.Sprintf("ExchangeRatesRelayer: can not get price on tokenid %v\n", v.TokenId)
			b.Logger.Errorf(msg)
			utils.SendSlackNotification(msg)
			continue
		}
		if converted := b.convertPublicTokenPriceToPToken(big.NewFloat(price.total / float64(price.count))); converted > 0 {
			rates[v.TokenId] = converted * ConvertDecimals
		}
		delete(b.StackOrder, v.StreamName)
	}

	b.IsProcessingStack = false
	if len(rates) == 0 {
		return errors.New("ExchangeRatesRelayer: Exchange rates is empty")
	}
	incognitoPrivateKey := os.Getenv("INCOGNITO_PRIVATE_KEY")
	txID, err := CreateAndSendTxPortalExchangeRate(b.RPCClient, incognitoPrivateKey, rates)
	if err != nil {
		return err
	}
	b.Logger.Infof("pushExchangeRates success with TxID: %v\n", txID)
	return nil
}

func (b *ExchangeRatesRelayer) Listen(wssEndpoint string) {
	go func() {
		interrupt := make(chan os.Signal, 1)
		signal.Notify(interrupt, os.Interrupt)

		socket := gowebsocket.New(wssEndpoint)

		socket.OnConnectError = func(err error, socket gowebsocket.Socket) {
			b.Logger.Info("Received connect error - ", err)
		}

		socket.OnConnected = func(socket gowebsocket.Socket) {
			b.Logger.Info("Connected to server")
		}

		socket.OnTextMessage = func(message string, socket gowebsocket.Socket) {
			if !b.IsProcessingStack {
				data := Stream{}
				err := json.Unmarshal([]byte(message), &data)
				if err != nil {
					b.Logger.Info("ExchangeRatesRelayer: Can not unmashal data from binance: %v", err)
					return
				}

				// stack data here
				price, ok := b.StackOrder[data.StreamName]
				s, err := strconv.ParseFloat(data.Data.P, 64)
				if err != nil {
					b.Logger.Info("ExchangeRatesRelayer: Can not parse data from binance: %v", err)
					return
				}
				if ok {
					price.count++
					price.total += s
					b.StackOrder[data.StreamName] = price
				} else {
					b.StackOrder[data.StreamName] = Price{total: s, count: 1}
				}
			}
		}

		socket.OnDisconnected = func(err error, socket gowebsocket.Socket) {
			b.Logger.Info("Disconnected from server ")
			return
		}
		socket.Connect()

		for {
			select {
			case <-interrupt:
				b.Logger.Info("interrupt")
				socket.Close()
				return
			}
		}
	}()
}
func (b *ExchangeRatesRelayer) Execute() {
	b.Logger.Info("ExchangeRatesRelayer agent is executing...")
	b.IsProcessingStack = true
	prvRate, err := b.getPRVRate()
	if err != nil {
		msg := fmt.Sprintf("ExchangeRatesRelayer: has a error, %v\n", err)
		b.Logger.Errorf(msg)
		utils.SendSlackNotification(msg)
	}

	err = b.pushExchangeRates(prvRate)
	if err != nil {
		msg := fmt.Sprintf("ExchangeRatesRelayer: has a error, %v\n", err)
		b.Logger.Errorf(msg)
		utils.SendSlackNotification(msg)
	}

	b.Logger.Info("ExchangeRatesRelayer agent finished...")
}
