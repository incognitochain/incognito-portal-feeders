package agents

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/big"
	"os"
	"portalfeeders/entities"
	"portalfeeders/utils"
	"strconv"
)

type ExchangePlatform int

const (
	Init ExchangePlatform = iota
	Binance
	P2pb2b
)

type ExchangeRatesRelayer struct {
	AgentAbs
	RestfulClient *utils.RestfulClient
	ListPlatforms []PriceItemOnPlatform
}

type ExchangeSymbol struct {
	Symbol string
	Price  string
	TokenID string
}

type PriceItemOnPlatform struct {
	SymbolBasedPlatForm []ExchangeSymbol
	Url string
	ExchangeType ExchangePlatform
}

type priceResult struct {
	total float64
	count int
}

// Binance prices
type BinanceData struct {
	SymBol string
	Price string
}

type BinanceTokens []BinanceData

//func (b *ExchangeRatesRelayer) getPublicTokenRates(symbol string) (PriceItem, error) {
//	symbolFilter := map[string]string {
//		"symbol": symbol,
//	}
//
//	result, err := b.RestfulClient.Get("/ticker/price", nil, symbolFilter)
//	if err != nil {
//		return PriceItem{}, err
//	}
//
//	var priceItems PriceItem
//	err = json.Unmarshal(result, &priceItems)
//	if err != nil {
//		b.Logger.Errorf("ExchangeRatesRelayer: has a error when unmarshal, %v\n", err)
//		return PriceItem{}, errors.New("ExchangeRatesRelayer: has a error when unmarshal")
//	}
//
//	return priceItems, nil
//}

// reduce num of get request to public exchange domain
func (b *ExchangeRatesRelayer) getTokenRatesByExPlatform(platforms []PriceItemOnPlatform) (map[string]priceResult, error) {
	priceSum :=  make(map[string]priceResult)
	for _, platform := range platforms {
		result, err := b.RestfulClient.Get(platform.Url, nil, nil)
		if err != nil {
			b.Logger.Info("Get exchange rate: %v error", err)
			continue
		}
		// add more exchange platform here
		switch platform.ExchangeType {
		case Binance:
			err = b.extractBinance(platform.SymbolBasedPlatForm, result, priceSum)
		case P2pb2b:
			err = b.extractP2P(platform.SymbolBasedPlatForm, result, priceSum)
		default:
			continue
		}
		if err != nil {
			continue
		}
	}
	return priceSum, nil
}

func (b *ExchangeRatesRelayer) extractP2P(symbolBasedPlatForm []ExchangeSymbol, result []byte, priceSum map[string]priceResult) (error) {
	var temp map[string]interface{}
	err := json.Unmarshal(result, &temp)
	if err != nil {
		return err
	}
	tickers, ok := temp["result"].(map[string]interface{})
	if !ok {
		b.Logger.Info("Value of key result is missing: %v error", temp)
		return errors.New("missing value")
	}
	for k, v := range tickers{
		for _, v2 := range symbolBasedPlatForm {
			if k == v2.Symbol {
				ticker, ok := v.(map[string]interface{})["ticker"]
				if !ok {
					b.Logger.Info("Field ticker is missing: %v", v)
					return errors.New("missing value")
				}
				priceString, ok := ticker.(map[string]interface{})["last"]
				if !ok {
					b.Logger.Info("Index last is missing: %v error", priceString)
					return errors.New("missing value")
				}

				s, err := strconv.ParseFloat(priceString.(string), 32)
				if err != nil {
					b.Logger.Info("Parse Float: %v error", priceString)
					return err
				}
				getCoinPrice, ok := priceSum[v2.TokenID]
				if ok {
					getCoinPrice.total += s
					getCoinPrice.count++
					priceSum[v2.TokenID] = getCoinPrice
				} else {
					priceSum[v2.TokenID] = priceResult{total: s, count: 1}
				}
			}
		}
	}
	return nil
}

func (b *ExchangeRatesRelayer) extractBinance(symbolBasedPlatForm []ExchangeSymbol, result []byte, priceSum map[string]priceResult) (error) {
	var temp BinanceTokens
	err := json.Unmarshal(result, &temp)
	if err != nil {
		return err
	}
	for _, v := range temp{
		for _, v2 := range symbolBasedPlatForm {
			if v.SymBol == v2.Symbol {
				s, err := strconv.ParseFloat(v.Price, 32)
				if err != nil {
					b.Logger.Info("Parse Float: %v error", v.Price)
					return err
				}
				getCoinPrice, ok := priceSum[v2.TokenID]
				if ok {
					getCoinPrice.total += s
					getCoinPrice.count++
					priceSum[v2.TokenID] = getCoinPrice
				} else {
					priceSum[v2.TokenID] = priceResult{total: s, count: 1}
				}
			}
		}
	}
	return nil
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
	prvPustPairKey := fmt.Sprintf("pdepool-%d-%s-%s", latestBeaconHeight, PRVID, PUSDTID)
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
	return tokenPoolValueToBuy - newTokenPoolValueToBuy, nil
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
	prices map[string]priceResult,
	prvRate uint64,
) error {
	rates := make(map[string]uint64)

	//fill data
	if prvRate > 0 {
		rates[PRVID] = prvRate
	}

	for tokenId, priceItem := range prices {
		if priceItem.count == 0 {
			b.Logger.Info("No response on token: %v", tokenId)
		}

		if converted := b.convertPublicTokenPriceToPToken(big.NewFloat(priceItem.total / float64(priceItem.count))); converted > 0 {
			rates[tokenId] = converted
		}
	}

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

func (b *ExchangeRatesRelayer) Execute() {
	b.Logger.Info("ExchangeRatesRelayer agent is executing...")
	prvRate, err := b.getPRVRate()
	if err != nil {
		msg := fmt.Sprintf("ExchangeRatesRelayer: has a error, %v\n", err)
		b.Logger.Errorf(msg)
		utils.SendSlackNotification(msg)
	}

	mapPrice, err := b.getTokenRatesByExPlatform(b.ListPlatforms)
	if err != nil {
		msg := fmt.Sprintf("ExchangeRatesRelayer: has a error, %v\n", err)
		b.Logger.Errorf(msg)
		utils.SendSlackNotification(msg)
	}

	err = b.pushExchangeRates(mapPrice, prvRate)
	if err != nil {
		msg := fmt.Sprintf("ExchangeRatesRelayer: has a error, %v\n", err)
		b.Logger.Errorf(msg)
		utils.SendSlackNotification(msg)
	}

	b.Logger.Info("ExchangeRatesRelayer agent finished...")
}
