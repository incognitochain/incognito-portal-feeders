package agents

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"portalfeeders/entities"
	"portalfeeders/utils"
)

type ExchangeRatesRelayer struct {
	AgentAbs
	RestfulClient *utils.RestfulClient
}

type Price struct {
	Price float64
	LastUpdated string
}

type CoinMarketCapQuotesLatestItem struct {
	Id int64
	Name string
	Quote map[string]*Price
}

type CoinMarketCapQuotesLatest struct {
	Data map[string]*CoinMarketCapQuotesLatestItem

}

func (b *ExchangeRatesRelayer) getPublicTokenRates() (CoinMarketCapQuotesLatest, error) {
	//get prv from CoinMarketCap
	header := map[string]string{
		"X-CMC_PRO_API_KEY": CoinMarketCapKey,
	}

	filter := map[string]string{
		"id": IdCrytoFilter,
	}

	result, err := b.RestfulClient.Get("cryptocurrency/quotes/latest", header, filter)
	if err != nil {
		return CoinMarketCapQuotesLatest{}, err
	}

	var coinMarketCapQuotesLatest CoinMarketCapQuotesLatest
	err = json.Unmarshal(result, &coinMarketCapQuotesLatest)

	if err != nil {
		fmt.Printf("ExchangeRatesRelayer: has a error when unmarshal CoinMarketCapQuotesLatest, %v", err)
		fmt.Println()
		return CoinMarketCapQuotesLatest{}, errors.New("ExchangeRatesRelayer: has a error when unmarshal CoinMarketCapQuotesLatest")
	}

	return coinMarketCapQuotesLatest, nil
}

func (b *ExchangeRatesRelayer) getPRVRates() (float64, error) {
	//todo: get prv
	//get prv from pde
	return  0.5, nil
}

func convertPublicTokenPriceToPToken(price float64) uint64  {
	result := price * math.Pow10(9)
	roundUp := uint64(math.Ceil(result))
	fmt.Printf("ExchangeRatesRelayer: Convert public token to pToken, price: %+v, result %+v, round up: %+v", price, result, roundUp)
	fmt.Println()
	return roundUp
}

func (b *ExchangeRatesRelayer) pushExchangeRates(coinMarketCapQuotesLatest CoinMarketCapQuotesLatest, prvRates float64)  error {
	rates := make(map[string]uint64)

	if converted := convertPublicTokenPriceToPToken(prvRates); converted > 0 {
		rates[PRVId] = converted
	}

	for _, value := range coinMarketCapQuotesLatest.Data {
		if value.Quote["USD"] == nil {
			continue
		}

		if converted := convertPublicTokenPriceToPToken(value.Quote["USD"].Price); converted > 0 {
			if value.Id == BTCCoinMarketCapId {
				rates[BTCId] = converted
			}

			if value.Id == BNBCoinMarketCapId {
				rates[BNBId] = converted
			}
		}
	}


	if len(rates) < 0 {
		return errors.New("ExchangeRatesRelayer: Exchange rates is empty")
	}

	meta := map[string]interface{}{
		"SenderAddress": SenderAddressExchangeRates,
		"Rates": rates,
	}

	params := []interface{}{
		SenderPrivateKeyExchangeRates,
		nil,
		-1,
		0,
		meta,
	}

	var relayingBlockRes entities.RelayingBlockRes
	err := b.RPCClient.RPCCall("createandsendportalexchangerates", params, &relayingBlockRes)

	if err != nil {
		return err
	}

	if relayingBlockRes.RPCError != nil {
		fmt.Printf("ExchangeRatesRelayer: call RPC error, %v", relayingBlockRes.RPCError.StackTrace)
		fmt.Println()
		return errors.New(relayingBlockRes.RPCError.Message)
	}

	fmt.Println("ExchangeRatesRelayer: Call RPC successfully!")
	return nil
}

func (b *ExchangeRatesRelayer) Execute() {
	fmt.Println("ExchangeRatesRelayer agent is executing...")
	coinMarketCapQuotesLatest, err := b.getPublicTokenRates()
	if err != nil {
		fmt.Printf("ExchangeRatesRelayer: has a error, %v", err)
		fmt.Println()
	}

	fmt.Println(coinMarketCapQuotesLatest)

	prvRates, err := b.getPRVRates()
	if err != nil {
		fmt.Printf("ExchangeRatesRelayer: has a error, %v", err)
		fmt.Println()
	}

	err = b.pushExchangeRates(coinMarketCapQuotesLatest, prvRates)
	if err != nil {
		fmt.Printf("ExchangeRatesRelayer: has a error, %v", err)
		fmt.Println()
	}
}