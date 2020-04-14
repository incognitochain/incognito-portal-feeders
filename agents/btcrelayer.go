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

// func (b *BTCRelayer) relayBTCBlockToIncognito(
// 	btcBlockHeight int,
// 	msgBlk *wire.MsgBlock,
// ) error {
// 	msgBlkBytes, err := json.Marshal(msgBlk)
// 	if err != nil {
// 		return err
// 	}
// 	meta := map[string]interface{}{
// 		"SenderAddress": "12S5Lrs1XeQLbqN4ySyKtjAjd2d7sBP2tjFijzmp6avrrkQCNFMpkXm3FPzj2Wcu2ZNqJEmh9JriVuRErVwhuQnLmWSaggobEWsBEci",
// 		"Header":        base64.StdEncoding.EncodeToString(msgBlkBytes),
// 		"BlockHeight":   btcBlockHeight,
// 	}
// 	privateKey := "112t8roafGgHL1rhAP9632Yef3sx5k8xgp8cwK4MCJsCL1UWcxXvpzg97N4dwvcD735iKf31Q2ZgrAvKfVjeSUEvnzKJyyJD3GqqSZdxN4or" // TODO: figure out to make it secret
// 	params := []interface{}{
// 		privateKey, nil, 5, -1, meta,
// 	}
// 	var relayingBlockRes entities.RelayingBlockRes
// 	err = b.RPCClient.RPCCall("createandsendtxwithrelayingbtcheader", params, &relayingBlockRes)
// 	if err != nil {
// 		return err
// 	}
// 	if relayingBlockRes.RPCError != nil {
// 		return errors.New(relayingBlockRes.RPCError.Message)
// 	}
// 	return nil
// }

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
	for {
		nextCypherBlock, err := bc.GetBlock(nextBlkHeight, "", nil)
		if err != nil {
			fmt.Println("Get next cypher block err: ", err)
			break
		}

		btcMsgBlock, err := buildBTCBlockFromCypher(&nextCypherBlock)
		if err != nil {
			fmt.Println("Build btc block from cypher block error: ", err)
			break
		}
		err = b.relayBTCBlockToIncognito(int64(nextBlkHeight), btcMsgBlock)
		if err != nil {
			fmt.Println("Relay btc block to incognito error: ", err)
			break
		}
		fmt.Printf("Relaying suceeded, Finished process for block: %s\n", btcMsgBlock.BlockHash())
		nextBlkHeight++
		time.Sleep(30 * time.Second)
	}
}
