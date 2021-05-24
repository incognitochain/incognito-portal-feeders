package agents

import (
	"fmt"
	"github.com/pkg/errors"

	"portalfeeders/entities"
	"portalfeeders/utils"
)

type BTCRelayingAlerter struct {
	AgentAbs
	RPCBTCRelayingReaders []*utils.HttpClient
}

func (b *BTCRelayingAlerter) getLatestBTCBlockHashFromIncog() (int32, string, error) {
	params := []interface{}{}
	var btcRelayingBestStateRes entities.BTCRelayingBestStateRes
	var lowestHeight int32
	var lowestBlockHeightHash string

	for _, btcRelayingHeader := range b.RPCBTCRelayingReaders {
		err := btcRelayingHeader.RPCCall("getbtcrelayingbeststate", params, &btcRelayingBestStateRes)
		if err != nil {
			b.Logger.Error(err)
			return 0, "", errors.Errorf("Can not get height from beacon: %v", err.Error())
		}
		btcBestState := btcRelayingBestStateRes.Result
		if btcBestState == nil {
			b.Logger.Error("BTC relaying best state is nil")
			return 0, "", errors.New("BTC relaying best state is nil")
		}

		if lowestHeight > btcBestState.Height || lowestHeight == 0 {
			lowestHeight = btcBestState.Height
			lowestBlockHeightHash = btcBestState.Hash.String()
		}
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
