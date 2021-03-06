package agents

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"portalfeeders/entities"
	"time"

	"github.com/tendermint/tendermint/rpc/client"
	"github.com/tendermint/tendermint/types"
)

const BNBBlockBatchSize = 10

type bnbBlockRes struct {
	blockStr    string
	blockHeight int64
	err         error
}

type BNBRelayer struct {
	AgentAbs
	BNBClient *client.HTTP
}

func (b *BNBRelayer) getLatestBNBBlockHeightFromIncognito() (int64, error) {
	// get latest bnb block height from Incognito
	var relayingBlockRes entities.RelayingBlockRes
	err := b.RPCClient.RPCCall("getlatestbnbheaderblockheight", nil, &relayingBlockRes)
	if err != nil {
		fmt.Errorf("Error when call RPC to get latest bnb header block height %v", err)
		return int64(0), err
	}

	if relayingBlockRes.RPCError != nil {
		fmt.Errorf(relayingBlockRes.RPCError.Message)
		return int64(0), errors.New(relayingBlockRes.RPCError.Message)
	}

	res := relayingBlockRes.Result.(float64)
	return int64(res), nil
}

// getBNBBlockFromBNBChain calls RPC to get bnb block with blockHeight from BNB peers
func (b *BNBRelayer) getBNBBlockFromBNBChain(
	bnbBlockHeight int64,
) (*types.Block, error) {
	block, err := b.BNBClient.Block(&bnbBlockHeight)
	if err != nil {
		return nil, err
	}
	if block == nil {
		return nil, errors.New("Can not get block from bnb chain")
	}

	return block.Block, nil
}

func buildBNBHeaderStr(block *types.Block) (string, error) {
	blockHeader := types.Block{
		Header:     block.Header,
		LastCommit: block.LastCommit,
	}
	bnbHeaderBytes, err := json.Marshal(blockHeader)
	if err != nil {
		return "", err
	}

	bnbHeaderStr := base64.StdEncoding.EncodeToString(bnbHeaderBytes)
	return bnbHeaderStr, nil
}

func (b *BNBRelayer) relayBNBBlockToIncognito(
	bnbBlockHeight int64,
	headerBlockStr string,
) error {
	incognitoPrivateKey := os.Getenv("INCOGNITO_PRIVATE_KEY")
	txID, err := CreateAndSendTxRelayBNBHeader(b.RPCClient, incognitoPrivateKey, headerBlockStr, bnbBlockHeight)
	if err != nil {
		return err
	}

	fmt.Printf("relayBNBBlockToIncognito success blockHeight %v with TxID: %v\n", bnbBlockHeight, txID)

	return nil
}

func (b *BNBRelayer) GetServerAddress() string {
	if b.GetNetwork() == "main" {
		return os.Getenv("BNB_MAINNET_ADDRESS")
	} else if b.GetNetwork() == "test" {
		return os.Getenv("BNB_TESTNET_ADDRESS")
	}
	return ""
}

func (b *BNBRelayer) Execute() {
	fmt.Println("BNBRelayer agent is executing...")

	// get latest BNB block from Incognito
	latestBNBBlockHeight, err := b.getLatestBNBBlockHeightFromIncognito()
	if err != nil {
		fmt.Printf("getLatestBNBBlockHeightFromIncognito error: %v\n", err)
		return
	}
	nextBlockHeight := latestBNBBlockHeight + 1

	//blockQueue := make(chan bnbBlockRes, BNBBlockBatchSize)
	relayingResQueue := make(chan error, BNBBlockBatchSize)
	lastCheckpoint := time.Now().UnixNano()
	lastCheckedBlockHeight := latestBNBBlockHeight
	for {
		//for i := nextBlockHeight; i < nextBlockHeight+BNBBlockBatchSize; i++ {
		//	i := i // create locals for closure below
		//	go func() {
		//		// get next BNB block from BNB chain
		//		block, err := b.getBNBBlockFromBNBChain(i)
		//		if err != nil {
		//			res := bnbBlockRes{blockStr: "", blockHeight: i, err: err}
		//			blockQueue <- res
		//		} else {
		//			headerBlockStr, err := buildBNBHeaderStr(block)
		//			res := bnbBlockRes{blockStr: headerBlockStr, blockHeight: i, err: err}
		//			blockQueue <- res
		//		}
		//	}()
		//}
		//
		//for i := nextBlockHeight; i < nextBlockHeight+BNBBlockBatchSize; i++ {
		//	_ = i // create locals for closure below
		//	go func() {
		//		bnbBlkRes := <-blockQueue
		//		if bnbBlkRes.err != nil {
		//			relayingResQueue <- bnbBlkRes.err
		//		} else {
		//			//relay next BNB block to Incognito
		//			err := b.relayBNBBlockToIncognito(bnbBlkRes.blockHeight, bnbBlkRes.blockStr)
		//			relayingResQueue <- err
		//		}
		//	}()
		//}
		//

		for i := nextBlockHeight; i < nextBlockHeight+BNBBlockBatchSize; i++ {
			i := i // create locals for closure below
			go func() {
				// get next BNB block from BNB chain
				block, err := b.getBNBBlockFromBNBChain(i)
				if err != nil {
					relayingResQueue <- err
				} else {
					headerBlockStr, err := buildBNBHeaderStr(block)
					if err != nil {
						relayingResQueue <- err
					} else {
						err = b.relayBNBBlockToIncognito(i, headerBlockStr)
						if err != nil {
							relayingResQueue <- err
						}
					}
				}
				relayingResQueue <- nil
			}()
		}

		for i := nextBlockHeight; i < nextBlockHeight+BNBBlockBatchSize; i++ {
			relayingErr := <-relayingResQueue
			if relayingErr != nil {
				fmt.Printf("BNB relaying error: %v\n", relayingErr)
				return
			}
		}

		if time.Now().UnixNano() >= lastCheckpoint+time.Duration(180*time.Second).Nanoseconds() {
			fmt.Println("Starting checking latest block height...")
			latestBNBBlkHeight, err := b.getLatestBNBBlockHeightFromIncognito()
			if err != nil {
				fmt.Printf("getLatestBNBBlockHeightFromIncognito error: %v\n", err)
				return
			}
			if latestBNBBlkHeight <= lastCheckedBlockHeight {
				fmt.Printf("Latest bnb block height on incognito chain has not increased for long time, still %d\n", latestBNBBlkHeight)
				return
			}
			lastCheckpoint = time.Now().UnixNano()
			// assign lastCheckedBlockHeight = nextBlockHeight, instead of latestBNBBlkHeight, help to detect wrong batch sooner.
			lastCheckedBlockHeight = nextBlockHeight
			fmt.Println("Finished checking latest block height.")
		}

		nextBlockHeight += BNBBlockBatchSize
		//time.Sleep(2*time.Second)
	}
}
