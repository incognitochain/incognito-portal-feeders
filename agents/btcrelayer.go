package agents

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"portalfeeders/entities"
	"portalfeeders/utils"

	"github.com/blockcypher/gobcy"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
)

const BTCBlockBatchSize = 1

type btcBlockRes struct {
	msgBlock    *wire.MsgBlock
	blockHeight int64
	err         error
}

type BTCRelayer struct {
	AgentAbs
}

func getBlockCypherAPI(networkName string) gobcy.API {
	//explicitly
	bc := gobcy.API{}
	bc.Token = "029727206f7e4c8fb19301e4629c5793"
	bc.Coin = "btc"        //options: "btc","bcy","ltc","doge"
	bc.Chain = networkName //depending on coin: "main","test3","test"
	return bc
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
	b.Logger.Infof("relayBTCBlockToIncognito success with TxID: %v\n", txID)
	return nil
}

func (b *BTCRelayer) getLatestBTCBlockHashFromIncog() (string, error) {
	params := []interface{}{}
	var btcRelayingBestStateRes entities.BTCRelayingBestStateRes
	err := b.RPCClient.RPCCall("getbtcrelayingbeststate", params, &btcRelayingBestStateRes)
	if err != nil {
		return "", err
	}
	if btcRelayingBestStateRes.RPCError != nil {
		return "", errors.New(btcRelayingBestStateRes.RPCError.Message)
	}
	return btcRelayingBestStateRes.Result.Hash.String(), nil
}

func (b *BTCRelayer) Execute() {
	b.Logger.Info("BTCRelayer agent is executing...")

	// get latest BNB block from Incognito
	latestBTCBlkHash, err := b.getLatestBTCBlockHashFromIncog()
	if err != nil {
		msg := fmt.Sprintf("Could not get latest btc block hash from incognito chain - with err: %v", err)
		b.Logger.Errorf(msg)
		utils.SendSlackNotification(msg)
		return
	}
	b.Logger.Infof("latestBTCBlkHash: %s", latestBTCBlkHash)

	bc := getBlockCypherAPI(b.GetNetwork())
	cypherBlock, err := bc.GetBlock(0, latestBTCBlkHash, nil)
	if err != nil {
		msg := fmt.Sprintf("Get cypher block err: %v", err)
		b.Logger.Infof(msg)
		utils.SendSlackNotification(msg)
		return
	}
	nextBlkHeight := cypherBlock.Height + 1

	blockQueue := make(chan btcBlockRes, BTCBlockBatchSize)
	relayingResQueue := make(chan error, BTCBlockBatchSize)
	lastCheckpoint := time.Now().UnixNano()
	lastCheckedBlockHash := latestBTCBlkHash
	for {
		for i := nextBlkHeight; i < nextBlkHeight+BTCBlockBatchSize; i++ {
			i := i // create locals for closure below
			go func() {
				block, err := bc.GetBlock(i, "", nil)
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
					//relay next BNB block to Incognito
					err := b.relayBTCBlockToIncognito(btcBlkRes.blockHeight, btcBlkRes.msgBlock)
					relayingResQueue <- err
				}
				<-sem
			}()
		}

		for i := nextBlkHeight; i < nextBlkHeight+BTCBlockBatchSize; i++ {
			relayingErr := <-relayingResQueue
			if relayingErr != nil {
				msg := fmt.Sprintf("BTC relaying error: %v\n", relayingErr)
				b.Logger.Infof(msg)
				utils.SendSlackNotification(msg)
				return
			}
		}

		if time.Now().UnixNano() >= lastCheckpoint+time.Duration(180*time.Second).Nanoseconds() {
			b.Logger.Info("Starting checking latest block height...")
			latestBlockHash, err := b.getLatestBTCBlockHashFromIncog()
			if err != nil {
				msg := fmt.Sprintf("getLatestBTCBlockHashFromIncog error: %v\n", err)
				b.Logger.Errorf(msg)
				utils.SendSlackNotification(msg)
				return
			}
			if latestBlockHash == lastCheckedBlockHash {
				msg := fmt.Sprintf("Latest btc block height on incognito chain has not increased for long time, still %s\n", latestBlockHash)
				b.Logger.Warnf(msg)
				utils.SendSlackNotification(msg)
				return
			}
			lastCheckpoint = time.Now().UnixNano()
			lastCheckedBlockHash = latestBlockHash
			b.Logger.Info("Finished checking latest block height.")
		}

		nextBlkHeight += BTCBlockBatchSize

		time.Sleep(30 * time.Second)
	}
}
