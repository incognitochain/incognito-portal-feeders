package agents

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"portalfeeders/entities"

	"github.com/blockcypher/gobcy"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
)

const BTCBlockBatchSize = 1

type btcBlockRes struct {
	msgBlock *wire.MsgBlock
	err      error
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
	fmt.Printf("relayBTCBlockToIncognito success with TxID: %v\n", txID)
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
	fmt.Println("BTCRelayer agent is executing...")

	// get latest BNB block from Incognito
	latestBTCBlkHash, err := b.getLatestBTCBlockHashFromIncog()
	if err != nil {
		fmt.Printf("Could not get latest btc block hash from incognito chain - with err: %v", err)
		return
	}
	fmt.Println("latestBTCBlkHash: ", latestBTCBlkHash)

	bc := getBlockCypherAPI(b.GetNetwork())
	cypherBlock, err := bc.GetBlock(0, latestBTCBlkHash, nil)
	if err != nil {
		fmt.Println("Get cypher block err: ", err)
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
					res := btcBlockRes{msgBlock: nil, err: err}
					blockQueue <- res
				} else {
					btcMsgBlock, err := buildBTCBlockFromCypher(&block)
					res := btcBlockRes{msgBlock: btcMsgBlock, err: err}
					blockQueue <- res
				}
			}()
		}

		sem := make(chan struct{}, BTCBlockBatchSize)
		for i := nextBlkHeight; i < nextBlkHeight+BTCBlockBatchSize; i++ {
			i := i // create locals for closure below
			sem <- struct{}{}
			go func() {
				btcBlkRes := <-blockQueue
				if btcBlkRes.err != nil {
					relayingResQueue <- btcBlkRes.err
				} else {
					//relay next BNB block to Incognito
					err := b.relayBTCBlockToIncognito(int64(i), btcBlkRes.msgBlock)
					relayingResQueue <- err
				}
				<-sem
			}()
		}

		for i := nextBlkHeight; i < nextBlkHeight+BTCBlockBatchSize; i++ {
			relayingErr := <-relayingResQueue
			if relayingErr != nil {
				fmt.Printf("BTC relaying error: %v\n", relayingErr)
				return
			}
		}

		if time.Now().UnixNano() >= lastCheckpoint+time.Duration(180*time.Second).Nanoseconds() {
			fmt.Println("Starting checking latest block height...")
			latestBlockHash, err := b.getLatestBTCBlockHashFromIncog()
			if err != nil {
				fmt.Printf("getLatestBTCBlockHashFromIncog error: %v\n", err)
				return
			}
			if latestBlockHash == lastCheckedBlockHash {
				fmt.Printf("Latest btc block height on incognito chain has not increased for long time, still %s\n", latestBlockHash)
				return
			}
			lastCheckpoint = time.Now().UnixNano()
			lastCheckedBlockHash = latestBlockHash
			fmt.Println("Finished checking latest block height.")
		}

		nextBlkHeight += BTCBlockBatchSize

		// TODO: uncomment this as having defragment account's money process
		//time.Sleep(30 * time.Second)
	}
}
