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

	"github.com/blockcypher/gobcy"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
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

type BTCRelayer struct {
	AgentAbs
	RPCBTCRelayingReader *utils.HttpClient
}

func buildBTCBlockFromCypher(cypherBlock *gobcy.Block) (*wire.MsgBlock, error) {
	prevBlkHash, err := chainhash.NewHashFromStr(cypherBlock.PrevBlock)
	if err != nil {
		return nil, err
	}
	merkleRoot, err := chainhash.NewHashFromStr(cypherBlock.MerkleRoot)
	if err != nil {
		return nil, err
	}
	return &wire.MsgBlock{
		Header: wire.BlockHeader{
			Version:    int32(cypherBlock.Ver),
			PrevBlock:  *prevBlkHash,
			MerkleRoot: *merkleRoot,
			Timestamp:  cypherBlock.Time,
			Bits:       uint32(cypherBlock.Bits),
			Nonce:      uint32(cypherBlock.Nonce),
		},
		Transactions: []*wire.MsgTx{},
	}, nil
}

func (b *BTCRelayer) relayBTCBlockToIncognito(
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

func (b *BTCRelayer) getLatestBTCBlockHashFromIncog(bc gobcy.API) (int32, error) {
	params := []interface{}{}
	var btcRelayingBestStateRes entities.BTCRelayingBestStateRes
	err := b.RPCBTCRelayingReader.RPCCall("getbtcrelayingbeststate", params, &btcRelayingBestStateRes)
	if err != nil {
		return 0, err
	}
	if btcRelayingBestStateRes.RPCError != nil {
		return 0, errors.New(btcRelayingBestStateRes.RPCError.Message)
	}

	// check whether there was a fork happened or not
	btcBestState := btcRelayingBestStateRes.Result
	if btcBestState == nil {
		return 0, errors.New("BTC relaying best state is nil")
	}
	currentBTCBlkHash := btcBestState.Hash.String()
	currentBTCBlkHeight := btcBestState.Height
	cypherBlock, err := bc.GetBlock(int(currentBTCBlkHeight), "", nil)
	if cypherBlock.Hash != currentBTCBlkHash { // fork detected
		msg := fmt.Sprintf("There was a fork happened at block %d, stepping back %d blocks now...", currentBTCBlkHeight, BlockStepBacks)
		b.Logger.Warnf(msg)
		utils.SendSlackNotification(msg)
		return currentBTCBlkHeight - BlockStepBacks, nil
	}
	return currentBTCBlkHeight, nil
}

func (b *BTCRelayer) Execute() {
	b.Logger.Info("BTCRelayer agent is executing...")
	bc := getBlockCypherAPI(b.GetNetwork())

	// get latest BTC block from Incognito
	latestBTCBlkHeight, err := b.getLatestBTCBlockHashFromIncog(bc)
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
				block, err := bc.GetBlock(int(i), "", nil)
				if err != nil {
					res := btcBlockRes{msgBlock: nil, blockHeight: int64(0), err: err}
					blockQueue <- res
				} else {
					btcMsgBlock, err := buildBTCBlockFromCypher(&block)
					res := btcBlockRes{msgBlock: btcMsgBlock, blockHeight: int64(i), err: err}
					blockQueue <- res
				}
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
				if !strings.Contains(relayingErr.Error(), "HTTP 404 Not Found") {
					msg := fmt.Sprintf("BTC relaying error: %v\n", relayingErr)
					b.Logger.Error(msg)
					utils.SendSlackNotification(msg)
				}
				return
			}
		}

		nextBlkHeight += BTCBlockBatchSize
		time.Sleep(60 * time.Second)
	}
}
