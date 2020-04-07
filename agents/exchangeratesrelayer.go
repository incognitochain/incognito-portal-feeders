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

type Price struct {
	Price       float64
	LastUpdated string
}

type CoinMarketCapQuotesLatestItem struct {
	Id    int64
	Name  string
	Quote map[string]*Price
}

type CoinMarketCapQuotesLatest struct {
	Data map[string]*CoinMarketCapQuotesLatestItem
}

func (b *ExchangeRatesRelayer) getPublicTokenRates() (CoinMarketCapQuotesLatest, error) {
	//get price from CoinMarketCap
	header := map[string]string{
		"X-CMC_PRO_API_KEY": os.Getenv("COINMARKETCAP_KEY"),
	}

	filter := map[string]string{
		"id": CrytoFilterID,
	}

	result, err := b.RestfulClient.Get("cryptocurrency/quotes/latest", header, filter)
	if err != nil {
		return CoinMarketCapQuotesLatest{}, err
	}

	var coinMarketCapQuotesLatest CoinMarketCapQuotesLatest
	err = json.Unmarshal(result, &coinMarketCapQuotesLatest)
	if err != nil {
		fmt.Printf("ExchangeRatesRelayer: has a error when unmarshal CoinMarketCapQuotesLatest, %v\n", err)
		return CoinMarketCapQuotesLatest{}, errors.New("ExchangeRatesRelayer: has a error when unmarshal CoinMarketCapQuotesLatest")
	}

	return coinMarketCapQuotesLatest, nil
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

func convertPublicTokenPriceToPToken(price float64) uint64 {
	result := price * math.Pow10(6)
	roundUp := uint64(math.Ceil(result))
	fmt.Printf("ExchangeRatesRelayer: Convert public token to pToken, price: %+v, result %+v, round up: %+v\n", price, result, roundUp)
	return roundUp
}

func (b *ExchangeRatesRelayer) pushExchangeRates(
	coinMarketCapQuotesLatest CoinMarketCapQuotesLatest,
	prvRate uint64,
) error {
	rates := make(map[string]uint64)
	if prvRate > 0 {
		rates[PRVID] = prvRate
	}

	for _, value := range coinMarketCapQuotesLatest.Data {
		if value.Quote["USD"] == nil {
			continue
		}

		if converted := convertPublicTokenPriceToPToken(value.Quote["USD"].Price); converted > 0 {
			if value.Id == BTCCoinMarketCapID {
				rates[BTCID] = converted
			}

			if value.Id == BNBCoinMarketCapID {
				rates[BNBID] = converted
			}
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
	coinMarketCapQuotesLatest, err := b.getPublicTokenRates()
	if err != nil {
		fmt.Printf("ExchangeRatesRelayer: has a error, %v\n", err)
	}

	fmt.Println(coinMarketCapQuotesLatest)

	prvRate, err := b.getPRVRate()
	if err != nil {
		fmt.Printf("ExchangeRatesRelayer: has a error, %v\n", err)
	}

	err = b.pushExchangeRates(coinMarketCapQuotesLatest, prvRate)
	if err != nil {
		fmt.Printf("ExchangeRatesRelayer: has a error, %v\n", err)
	}
}
