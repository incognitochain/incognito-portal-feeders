package agents

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"portalfeeders/entities"
	"portalfeeders/utils"

	"github.com/btcsuite/btcd/rpcclient"
	"github.com/btcsuite/btcd/wire"
)

// BTCBlockBatchSize is BTC block batch size
const BTCBlockBatchSize = 1

// BlockStepBacks is number of blocks that the job needs to step back to solve fork situation
const BlockStepBacks = 8

type btcBlockRes struct {
	msgBlock    *wire.MsgBlock
	blockHeight int64
	err         error
}

type BTCRelayerV2 struct {
	AgentAbs
	RPCBTCRelayingReaders[] *utils.HttpClient
	BTCClient *rpcclient.Client
}

func (b *BTCRelayerV2) relayBTCBlockToIncognito(
	btcBlockHeight int64,
	msgBlk *wire.MsgBlock,
) error {
	msgBlkBytes, err := json.Marshal(msgBlk)
	if err != nil {
		return err
	}
	headerBlockStr := base64.StdEncoding.EncodeToString(msgBlkBytes)
	incognitoPrivateKey := os.Getenv("INCOGNITO_PRIVATE_KEY")
	txID, err := CreateAndSendTxRelayBTCHeader(b.RPCClient, incognitoPrivateKey, headerBlockStr, btcBlockHeight)
	if err != nil {
		return err
	}
	b.Logger.Infof("relayBTCBlockToIncognito success (%d) with TxID: %v\n", btcBlockHeight, txID)
	return nil
}

func (b *BTCRelayerV2) getLatestBTCBlockHashFromIncog(btcClient *rpcclient.Client) (int32, error) {
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
		return 0, errors.New("Can not get height from all beacon and fullnode")
	}
	blkHash, err := btcClient.GetBlockHash(int64(lowestHeight))
	if err != nil {
		return 0, err
	}

	if blkHash.String() != lowestBlockHeightHash { // fork detected
		msg := fmt.Sprintf("There was a fork happened at block %d, stepping back %d blocks now...", lowestHeight, BlockStepBacks)
		b.Logger.Warnf(msg)
		utils.SendSlackNotification(msg)
		return lowestHeight - BlockStepBacks, nil
	}
	return lowestHeight, nil
}

func (b *BTCRelayerV2) Execute() {
	btcClient := b.BTCClient
	// get latest BTC block from Incognito
	latestBTCBlkHeight, err := b.getLatestBTCBlockHashFromIncog(btcClient)
	if err != nil {
		msg := fmt.Sprintf("Could not get latest btc block height from incognito chain - with err: %v", err)
		b.Logger.Error(msg)
		utils.SendSlackNotification(msg)
		return
	}
	b.Logger.Infof("Latest BTC block height: %d", latestBTCBlkHeight)

	nextBlkHeight := latestBTCBlkHeight + 1
	blockQueue := make(chan btcBlockRes, BTCBlockBatchSize)
	relayingResQueue := make(chan error, BTCBlockBatchSize)
	for {
		for i := nextBlkHeight; i < nextBlkHeight+BTCBlockBatchSize; i++ {
			i := i // create locals for closure below
			go func() {
				blkHash, err := btcClient.GetBlockHash(int64(i))
				if err != nil {
					res := btcBlockRes{msgBlock: nil, blockHeight: int64(0), err: err}
					blockQueue <- res
					return
				}

				btcMsgBlock, err := btcClient.GetBlock(blkHash)
				if err != nil {
					res := btcBlockRes{msgBlock: nil, blockHeight: int64(0), err: err}
					blockQueue <- res
					return
				}

				btcMsgBlock.Transactions = []*wire.MsgTx{}
				res := btcBlockRes{msgBlock: btcMsgBlock, blockHeight: int64(i), err: err}
				blockQueue <- res
			}()
		}

		sem := make(chan struct{}, BTCBlockBatchSize)
		for i := nextBlkHeight; i < nextBlkHeight+BTCBlockBatchSize; i++ {
			// i := i // create locals for closure below
			sem <- struct{}{}
			go func() {
				btcBlkRes := <-blockQueue
				if btcBlkRes.err != nil {
					relayingResQueue <- btcBlkRes.err
				} else {
					//relay next BTC block to Incognito
					err := b.relayBTCBlockToIncognito(btcBlkRes.blockHeight, btcBlkRes.msgBlock)
					relayingResQueue <- err
				}
				<-sem
			}()
		}

		for i := nextBlkHeight; i < nextBlkHeight+BTCBlockBatchSize; i++ {
			relayingErr := <-relayingResQueue

			if relayingErr != nil {
				if !strings.Contains(relayingErr.Error(), "Block height out of range") {
					msg := fmt.Sprintf("BTC relaying error: %v\n", relayingErr)
					b.Logger.Error(msg)
					utils.SendSlackNotification(msg)
				}
				return
			}
		}

		nextBlkHeight += BTCBlockBatchSize
		time.Sleep(2 * time.Second)
	}
}
