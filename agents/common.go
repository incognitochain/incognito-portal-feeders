package agents

import (
	"fmt"
	"github.com/0xkraken/incognito-sdk-golang/rpcclient"
	"github.com/0xkraken/incognito-sdk-golang/transaction"
	"portalfeeders/utils"
)

func CreateAndSendNormalTx(client *utils.HttpClient, privateKeyStr string, paymentInfoParams map[string]uint64, isPrivacy bool) (string, error) {
	rpcClient := rpcclient.NewHttpClient(client.GetURL(), "", "", 0)
	txID, err := transaction.CreateAndSendNormalTx(rpcClient, privateKeyStr, paymentInfoParams, DefaultFee, isPrivacy)
	if err != nil {
		fmt.Printf("Error when create and send normal tx %v\n", err)
		return "", fmt.Errorf("Error when create and send normal tx %v\n", err)
	}

	return txID, nil
}

func CreateAndSendTxRelayBNBHeader(client *utils.HttpClient, privateKeyStr string, bnbHeaderStr string, bnbBlockHeight int64) (string, error) {
	rpcClient := rpcclient.NewHttpClient(client.GetURL(), "", "", 0)

	txID, err := transaction.CreateAndSendTxRelayBNBHeader(rpcClient, privateKeyStr, bnbHeaderStr, bnbBlockHeight, DefaultFee)
	if err != nil {
		fmt.Printf("Error when create and send tx relay bnb block %v\n", err)
		return "", fmt.Errorf("Error when create and send tx relay bnb block %v\n", err)
	}

	return txID, nil
}

func CreateAndSendTxRelayBTCHeader(client *utils.HttpClient, privateKeyStr string, btcHeaderStr string, btcBlockHeight int64) (string, error) {
	rpcClient := rpcclient.NewHttpClient(client.GetURL(), "", "", 0)

	txID, err := transaction.CreateAndSendTxRelayBTCHeader(rpcClient, privateKeyStr, btcHeaderStr, btcBlockHeight, DefaultFee)
	if err != nil {
		fmt.Printf("Error when create and send tx relay btc block %v\n", err)
		return "", fmt.Errorf("Error when create and send tx relay btc block %v\n", err)
	}

	return txID, nil
}

func CreateAndSendTxPortalExchangeRate(client *utils.HttpClient, privateKeyStr string, exchangeRateParam map[string]uint64) (string, error) {
	rpcClient := rpcclient.NewHttpClient(client.GetURL(), "", "", 0)

	txID, err := transaction.CreateAndSendTxPortalExchangeRate(rpcClient, privateKeyStr, exchangeRateParam, DefaultFee)
	if err != nil {
		fmt.Printf("Error when create and send tx exchange rate %v\n", err)
		return "", fmt.Errorf("Error when create and send tx exchange rate %v\n", err)
	}

	return txID, nil
}

// number of blocks per year: 3*60*60*24*365 = 94608000
// PRV fee for BNB relayer in 10 years: 94608000 * 10 * 20 = 18921600000 ~ 20 PRV + 10 PRV (extra)
func SplitUTXOs(client *utils.HttpClient, privateKeyStr string, numUTXOs int) error {
	rpcClient := rpcclient.NewHttpClient(client.GetURL(), "", "", 0)

	err := transaction.SplitUTXOs(rpcClient, privateKeyStr, numUTXOs)
	if err != nil {
		fmt.Printf("Error when split utxos %v\n", err)
		return err
	}

	return nil
}
