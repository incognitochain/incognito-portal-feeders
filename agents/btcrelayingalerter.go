package agents

import (
	"errors"
	"fmt"

	"portalfeeders/entities"
	"portalfeeders/utils"
)

type BTCRelayingAlerter struct {
	AgentAbs
	RPCBTCRelayingReaders[] *utils.HttpClient
}

func (b *BTCRelayingAlerter) getLatestBTCBlockHashFromIncog() (int32, string, error) {
	params := []interface{}{}
	var btcRelayingBestStateRes entities.BTCRelayingBestStateRes
	var lowestHeight, errsCount int32
	var lowestBlockHeightHash string

	for _, btcRelayingHeader := range b.RPCBTCRelayingReaders {
		err := btcRelayingHeader.RPCCall("getbtcrelayingbeststate", params, &btcRelayingBestStateRes)
		if err != nil {
			b.Logger.Error(err)
			errsCount++
			continue
		}
		btcBestState := btcRelayingBestStateRes.Result
		if btcBestState == nil {
			b.Logger.Error("BTC relaying best state is nil")
			errsCount++
			continue
		}

		if  lowestHeight > btcBestState.Height || lowestHeight == 0 {
			lowestHeight = btcBestState.Height
			lowestBlockHeightHash = btcBestState.Hash.String()
		}
	}
	if errsCount >= int32(len(b.RPCBTCRelayingReaders)) {
		return 0, "", errors.New("Can not get height from all beacon and fullnode")
	}

	return lowestHeight, lowestBlockHeightHash, nil
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
		The latest block info from Cypher: height (%d), hash (%s)
		The latest block info from Incognito: height (%d), hash (%s)
		Incognito's is behind %d blocks against Cypher's
		`,
		btcCypherChain.Height, btcCypherChain.Hash,
		latestBTCBlkHeight, latestBTCBlkHash,
		btcCypherChain.Height-int(latestBTCBlkHeight),
	)
	utils.SendSlackNotification(alertMsg)
}
