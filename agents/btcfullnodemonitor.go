package agents

import (
	"fmt"
	"github.com/btcsuite/btcd/rpcclient"
	"github.com/prometheus/common/model"
	"portalfeeders/utils"
	"time"
)

type BTCMonitor struct {
	AgentAbs
	RPCBTCRelayingReader *utils.HttpClient
	BTCClient *rpcclient.Client
	TimeStart model.Time
	TimeBlockInterval int64
	AliveCheckInterval int
	RecentHeight int64
}

func (b *BTCMonitor) getLatestBTCBlockHeightFromBTCFullNode() (int64, error) {
	// get latest blockchain info from btc fullnode
	result, err := b.BTCClient.GetBlockChainInfo()
	if err != nil {
		fmt.Errorf("Error when call RPC to get latest blockchain infor from btc fullnode %v", err)
		return int64(0), err
	}

	return int64(result.Blocks), nil
}

func (b *BTCMonitor) Execute() {
	recentHeight, err := b.getLatestBTCBlockHeightFromBTCFullNode()
	if err != nil {
		msg := fmt.Sprintf("Could not get latest btc block height from btc fullnoed - with err: %v", err)
		b.Logger.Error(msg)
		utils.SendSlackNotification(msg)
		return
	}
	if recentHeight > b.RecentHeight {
		b.Logger.Infof("BTC fullnode still alive with latest block height: %d", b.RecentHeight)
		recentHeight = b.RecentHeight
		b.TimeStart = model.Now()
		b.TimeBlockInterval = 20
	} else {
		minutes := (model.Now().Unix() - b.TimeStart.Unix()) / 60
		if minutes > b.TimeBlockInterval {
			msg := fmt.Sprintf("No new block in %d minutes", b.TimeBlockInterval)
			b.TimeBlockInterval += 100000000
			utils.SendSlackNotification(msg)
		}
	}
	if b.AliveCheckInterval % 3 == 0 {
		fmt.Println("Something went wrong...3")
		msg := fmt.Sprintf("My name is BTC fullnode. I'm still alive.")
		b.AliveCheckInterval = 0
		utils.SendSlackNotification(msg)
	}
	b.AliveCheckInterval++
	time.Sleep(2 * time.Second)
}
