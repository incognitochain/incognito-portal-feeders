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
)

type ExchangeRatesRelayer struct {
	AgentAbs
	RestfulClient *utils.RestfulClient
}

type PriceItem struct {
	Symbol string
	Price  string
}

func (b *ExchangeRatesRelayer) getPublicTokenRates(symbol string) (PriceItem, error) {
	symbolFilter := map[string]string {
		"symbol": symbol,
	}

	result, err := b.RestfulClient.Get("/ticker/price", nil, symbolFilter)
	if err != nil {
		return PriceItem{}, err
	}

	var priceItems PriceItem
	err = json.Unmarshal(result, &priceItems)
	if err != nil {
		fmt.Printf("ExchangeRatesRelayer: has a error when unmarshal, %v\n", err)
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
		fmt.Printf("getLatestBeaconHeight: call RPC error, %v\n", beaconBestStateRes.RPCError.StackTrace)
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
		fmt.Printf("getPDEState: call RPC error, %v\n", pdeStateRes.RPCError.StackTrace)
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

func convertPublicTokenPriceToPToken(price *big.Float) uint64 {
	var e6 = new(big.Float).SetFloat64(math.Pow10(6))
	mul := new(big.Float).Mul(price, e6)

	result := new(big.Int)
	mul.Int(result)

	fmt.Printf("ExchangeRatesRelayer: Convert public token to pToken, price: %+v, result %+v\n", price.String(), result.Uint64())

	return result.Uint64()
}

func (b *ExchangeRatesRelayer) pushExchangeRates(
	btcPrice PriceItem,
	bnbPrice PriceItem,
	prvRate uint64,
) error {
	rates := make(map[string]uint64)

	//fill data
	if prvRate > 0 {
		rates[PRVID] = prvRate
	}

	if len(btcPrice.Price) > 0 {
		price := new(big.Float)
		price, ok := price.SetString(btcPrice.Price)

		if !ok {
			fmt.Println("SetString: BTC error")
		}

		if converted := convertPublicTokenPriceToPToken(price); ok && converted > 0 {
			rates[BTCID] = converted
		}
	}

	if len(bnbPrice.Price) > 0 {
		price := new(big.Float)
		price, ok := price.SetString(bnbPrice.Price)

		if !ok {
			fmt.Println("SetString: BNB error")
		}

		if converted := convertPublicTokenPriceToPToken(price); converted > 0 {
			rates[BNBID] = converted
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

	fmt.Printf("pushExchangeRates success with TxID: %v\n", txID)
	return nil
}

func (b *ExchangeRatesRelayer) Execute() {
	fmt.Println("ExchangeRatesRelayer agent is executing...")
	prvRate, err := b.getPRVRate()
	if err != nil {
		fmt.Printf("ExchangeRatesRelayer: has a error, %v\n", err)
	}

	btcPrice, err := b.getPublicTokenRates(BTCSymbol)
	if err != nil {
		fmt.Printf("ExchangeRatesRelayer: has a error, %v\n", err)
	}

	bnbPrice, err := b.getPublicTokenRates(BNBSymbol)
	if err != nil {
		fmt.Printf("ExchangeRatesRelayer: has a error, %v\n", err)
	}

	err = b.pushExchangeRates(btcPrice, bnbPrice, prvRate)
	if err != nil {
		fmt.Printf("ExchangeRatesRelayer: has a error, %v\n", err)
	}

	fmt.Println("ExchangeRatesRelayer agent finished...")
}
