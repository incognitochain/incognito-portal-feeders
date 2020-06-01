package agents

import (
	"errors"
	"fmt"

	"portalfeeders/entities"
	"portalfeeders/utils"
)

type BTCRelayingAlerter struct {
	AgentAbs
	RPCBTCRelayingReader *utils.HttpClient
}

func (b *BTCRelayingAlerter) getLatestBTCBlockHashFromIncog() (int32, string, error) {
	params := []interface{}{}
	var btcRelayingBestStateRes entities.BTCRelayingBestStateRes
	err := b.RPCBTCRelayingReader.RPCCall("getbtcrelayingbeststate", params, &btcRelayingBestStateRes)
	if err != nil {
		return 0, "", err
	}
	if btcRelayingBestStateRes.RPCError != nil {
		return 0, "", errors.New(btcRelayingBestStateRes.RPCError.Message)
	}

	// check whether there was a fork happened or not
	btcBestState := btcRelayingBestStateRes.Result
	if btcBestState == nil {
		return 0, "", errors.New("BTC relaying best state is nil")
	}
	currentBTCBlkHash := btcBestState.Hash.String()
	currentBTCBlkHeight := btcBestState.Height
	return currentBTCBlkHeight, currentBTCBlkHash, nil
}

func (b *BTCRelayingAlerter) Execute() {
	b.Logger.Info("BTCRelayingAlerter agent is executing...")
	bc := getBlockCypherAPI(b.GetNetwork())

	btcCypherChain, err := bc.GetChain()
	if err != nil {
		msg := fmt.Sprintf("Could not get btc chain info from cypher api - with err: %v", err)
		b.Logger.Error(msg)
		utils.SendSlackNotification(msg)
		return
	}

	latestBTCBlkHeight, latestBTCBlkHash, err := b.getLatestBTCBlockHashFromIncog()
	if err != nil {
		msg := fmt.Sprintf("Could not get the latest btc block height from incognito chain - with err: %v", err)
		b.Logger.Error(msg)
		utils.SendSlackNotification(msg)
		return
	}

	alertMsg := fmt.Sprintf(
		`
		The latest block info from Cypher: Height (%d), Hash (%s)
		The latest block info from Incognito: Height (%d), Hash (%s)
		Incognito's is behind %d blocks against Cypher's
		`,
		btcCypherChain.Height, btcCypherChain.Hash,
		latestBTCBlkHeight, latestBTCBlkHash,
		btcCypherChain.Height-int(latestBTCBlkHeight),
	)
	utils.SendSlackNotification(alertMsg)
}
