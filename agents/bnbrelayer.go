package agents

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/tendermint/tendermint/rpc/client"
	"github.com/tendermint/tendermint/types"
	"portalfeeders/entities"
)

type BNBRelayer struct {
	AgentAbs
}

func (b *BNBRelayer) getLatestBNBBlockHeightFromIncognito() (int64, error) {
	// get latest bnb block height from Incognito
	var relayingBlockRes entities.RelayingBlockRes
	err := b.RPCClient.RPCCall("getlatestbnbheaderblockheight", nil, &relayingBlockRes)
	if err != nil {
		return int64(0), err
	}
	if relayingBlockRes.Error != nil {
		return int64(0), errors.New(relayingBlockRes.Error.Message)
	}

	res := relayingBlockRes.Result.(map[string]interface{})
	latestBNBHeaderBlockHeight, ok := res["LatestBNBHeaderBlockHeight"]
	if !ok {
		return int64(0), errors.New("Can not get LatestBNBHeaderBlockHeight in response")
	}
	latestBNBHeaderBlockHeightInt64, ok := latestBNBHeaderBlockHeight.(int64)
	if !ok {
		return int64(0), errors.New("Can not get latestBNBHeaderBlockHeightInt64 in response")
	}

	return latestBNBHeaderBlockHeightInt64, nil
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
	meta := map[string]interface{}{
		"SenderAddress": "",
		"Header":        headerBlockStr,
		"BlockHeight":   bnbBlockHeight,
	}
	privateKey := "112t8rnjeorQyyy36Vz5cqtfQNoXuM7M2H92eEvLWimiAtnQCSZiP2HXpMW7mECSRXeRrP8yPwxKGuziBvGVfmxhQJSt2KqHAPZvYmM1ZKwR" // TODO: figure out to make it secret
	params := []interface{}{
		privateKey, nil, -1, 0, meta,
	}
	var relayingBlockRes entities.RelayingBlockRes
	err := b.RPCClient.RPCCall("createandsendtxwithrelayingbnbheader", params, &relayingBlockRes)
	if err != nil {
		return err
	}
	if relayingBlockRes.Error != nil {
		return errors.New(relayingBlockRes.Error.Message)
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
		return
	}
	nextBlockHeight := latestBNBBlockHeight + 1

	for {
		// get next BNB block from BNB chain
		block, err := b.getBNBBlockFromBNBChain(nextBlockHeight)
		if err != nil {
			break
		}
		headerBlockStr, err := buildBNBHeaderStr(block)
		if err != nil {
			break
		}

		// relay next BNB block to Incognito
		err = b.relayBNBBlockToIncognito(nextBlockHeight, headerBlockStr)
		if err != nil {
			break
		}

		nextBlockHeight++
	}
}
