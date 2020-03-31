package agents

import (
	"encoding/json"
	"errors"
	"fmt"

	"portalfeeders/entities"

	"github.com/blockcypher/gobcy"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
	"github.com/btcsuite/btcutil"
)

type BTCRelayer struct {
	AgentAbs
}

func (b *BTCRelayer) getLatestBTCBlockHashFromIncog() (string, error) {
	// TODO: hardcoded the result (block hash) for now and will update when implementing this rpc on Incogninto chain
	return "", nil
}

func getBlockCypherAPI(networkName string) gobcy.API {
	//explicitly
	bc := gobcy.API{}
	bc.Token = "029727206f7e4c8fb19301e4629c5793"
	bc.Coin = "btc"        //options: "btc","bcy","ltc","doge"
	bc.Chain = networkName //depending on coin: "main","test3","test"
	return bc
}

func buildBTCBlockFromCypher(cypherBlock *gobcy.Block) (*btcutil.Block, error) {
	prevBlkHash, err := chainhash.NewHashFromStr(cypherBlock.PrevBlock)
	if err != nil {
		return nil, err
	}
	merkleRoot, err := chainhash.NewHashFromStr(cypherBlock.MerkleRoot)
	if err != nil {
		return nil, err
	}
	msgBlk := wire.MsgBlock{
		Header: wire.BlockHeader{
			Version:    int32(cypherBlock.Ver),
			PrevBlock:  *prevBlkHash,
			MerkleRoot: *merkleRoot,
			Timestamp:  cypherBlock.Time,
			Bits:       uint32(cypherBlock.Bits),
			Nonce:      uint32(cypherBlock.Nonce),
		},
		Transactions: []*wire.MsgTx{},
	}
	blk := btcutil.NewBlock(&msgBlk)
	// blk.SetHeight(int32(blkHeight))
	return blk, nil
}

func (b *BTCRelayer) relayBTCBlockToIncognito(
	btcBlockHeight int,
	blk *btcutil.Block,
) error {
	blkBytes, err := json.Marshal(blk)
	if err != nil {
		return err
	}

	meta := map[string]interface{}{
		"SenderAddress": "",
		"Header":        string(blkBytes),
		"BlockHeight":   btcBlockHeight,
	}
	privateKey := "112t8rnjeorQyyy36Vz5cqtfQNoXuM7M2H92eEvLWimiAtnQCSZiP2HXpMW7mECSRXeRrP8yPwxKGuziBvGVfmxhQJSt2KqHAPZvYmM1ZKwR" // TODO: figure out to make it secret
	params := []interface{}{
		privateKey, nil, -1, 0, meta,
	}
	var relayingBlockRes entities.RelayingBlockRes
	err = b.RPCClient.RPCCall("createandsendtxwithrelayingbtcheader", params, &relayingBlockRes)
	if err != nil {
		return err
	}
	if relayingBlockRes.Error != nil {
		return errors.New(relayingBlockRes.Error.Message)
	}
	return nil
}

func (b *BTCRelayer) Execute() {
	fmt.Println("BTCRelayer agent is executing...")

	latestBTCBlkHash, err := b.getLatestBTCBlockHashFromIncog()
	if err != nil {
		return
	}

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
			break
		}

		btcBlock, err := buildBTCBlockFromCypher(&nextCypherBlock)
		if err != nil {
			break
		}
		err = b.relayBTCBlockToIncognito(nextBlkHeight, btcBlock)
		if err != nil {
			break
		}
		nextBlkHeight++
	}
}
