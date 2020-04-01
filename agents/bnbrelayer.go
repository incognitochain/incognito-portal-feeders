package agents

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/tendermint/tendermint/rpc/client"
	"github.com/tendermint/tendermint/types"
	"portalfeeders/entities"
	"time"
)

type BNBRelayer struct {
	AgentAbs
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

	res := relayingBlockRes.Result.(map[string]interface{})
	latestBNBHeaderBlockHeight, ok := res["LatestBNBHeaderBlockHeight"]
	if !ok {
		fmt.Errorf("Can not get LatestBNBHeaderBlockHeight in response")
		return int64(0), errors.New("Can not get LatestBNBHeaderBlockHeight in response")
	}
	latestBNBHeaderBlockHeightFloat64, ok := latestBNBHeaderBlockHeight.(float64)
	if !ok {
		fmt.Errorf("Can not get latestBNBHeaderBlockHeightFloat64 in response")
		return int64(0), errors.New("Can not get latestBNBHeaderBlockHeightFloat64 in response")
	}

	return int64(latestBNBHeaderBlockHeightFloat64), nil
}

// getBNBBlockFromBNBChain calls RPC to get bnb block with blockHeight from BNB peers
func (b *BNBRelayer) getBNBBlockFromBNBChain(
	bnbBlockHeight int64,
) (*types.Block, error) {
	serverAddress := b.getServerAddress()
	client := client.NewHTTP(serverAddress, "/websocket")
	client.Start()
	defer client.Stop()
	block, err := client.Block(&bnbBlockHeight)
	if err != nil {
		return nil, err
	}
	if block == nil {
		return nil, errors.New("Can not get block from bnb chain")
	}

	return block.Block, nil
}

func buildBNBHeaderStr (block *types.Block) (string, error) {
	blockHeader := types.Block {
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
	//privateKey := "112t8rnjeorQyyy36Vz5cqtfQNoXuM7M2H92eEvLWimiAtnQCSZiP2HXpMW7mECSRXeRrP8yPwxKGuziBvGVfmxhQJSt2KqHAPZvYmM1ZKwR" // TODO: figure out to make it secret
	//
	//txID, err := CreateAndSendTxRelayBNBBlock(b.RPCClient, privateKey, headerBlockStr, bnbBlockHeight)
	//if err != nil {
	//	return err
	//}
	//fmt.Printf("relayBNBBlockToIncognito success with TxID: %v\n", txID)

	meta := map[string]interface{}{
		"SenderAddress": "12RuEdPjq4yxivzm8xPxRVHmkL74t4eAdUKPdKKhMEnpxPH3k8GEyULbwq4hjwHWmHQr7MmGBJsMpdCHsYAqNE18jipWQwciBf9yqvQ",
		"Header":        headerBlockStr,
		"BlockHeight":   bnbBlockHeight,
	}
	privateKey := "112t8rnjeorQyyy36Vz5cqtfQNoXuM7M2H92eEvLWimiAtnQCSZiP2HXpMW7mECSRXeRrP8yPwxKGuziBvGVfmxhQJSt2KqHAPZvYmM1ZKwR"
	params := []interface{}{
		privateKey, nil, -1, 0, meta,
	}
	var relayingBlockRes entities.RelayingBlockRes
	err := b.RPCClient.RPCCall("createandsendtxwithrelayingbnbheader", params, &relayingBlockRes)
	if err != nil {
		fmt.Errorf("Error when call RPC create and send relaying bnb header %v\n", err)
		return err
	}
	if relayingBlockRes.RPCError != nil {
		fmt.Errorf("%v\n", relayingBlockRes.RPCError.Message)
		return errors.New(relayingBlockRes.RPCError.Message)
	}

	return nil
}

func (b *BNBRelayer) getServerAddress() string {
	if b.GetNetwork() == "main" {
		return BNBMainnetAddress
	} else if b.GetNetwork() == "test" {
		return BNBTestnetAddress
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

	for {
		// get next BNB block from BNB chain
		block, err := b.getBNBBlockFromBNBChain(nextBlockHeight)
		if err != nil {
			fmt.Printf("getBNBBlockFromBNBChain error: %v\n", err)
			break
		}
		headerBlockStr, err := buildBNBHeaderStr(block)
		if err != nil {
			fmt.Printf("buildBNBHeaderStr error: %v\n", err)
			break
		}

		// relay next BNB block to Incognito
		err = b.relayBNBBlockToIncognito(nextBlockHeight, headerBlockStr)
		if err != nil {
			fmt.Printf("relayBNBBlockToIncognito error: %v\n", err)
			break
		}
		fmt.Printf("Relay bnb block header %v\n", nextBlockHeight)

		nextBlockHeight++
		time.Sleep(60000 * time.Millisecond)
	}
}
