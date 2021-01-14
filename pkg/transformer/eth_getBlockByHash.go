package transformer

import (
	"strconv"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/pkg/errors"
	"github.com/qtumproject/janus/pkg/eth"
	"github.com/qtumproject/janus/pkg/qtum"
	"github.com/qtumproject/janus/pkg/utils"
)

// ProxyETHGetBlockByHash implements ETHProxy
type ProxyETHGetBlockByHash struct {
	*qtum.Qtum
}

func (p *ProxyETHGetBlockByHash) Method() string {
	return "eth_getBlockByHash"
}

func (p *ProxyETHGetBlockByHash) Request(rawreq *eth.JSONRPCRequest) (interface{}, error) {
	req := new(eth.GetBlockByHashRequest)
	if err := unmarshalRequest(rawreq.Params, req); err != nil {
		return nil, err
	}
	req.BlockHash = utils.RemoveHexPrefix(req.BlockHash)

	return p.request(req)
}

func (p *ProxyETHGetBlockByHash) request(req *eth.GetBlockByHashRequest) (*eth.GetBlockByHashResponse, error) {
	blockHeader, err := p.GetBlockHeader(req.BlockHash)
	if err != nil {
		return nil, errors.WithMessage(err, "couldn't get block header")
	}
	block, err := p.GetBlock(string(req.BlockHash))
	if err != nil {
		return nil, errors.WithMessage(err, "couldn't get block")
	}
	resp := &eth.GetBlockByHashResponse{
		// TODO: Mark is researching
		// * If ETH block has pending status, then the following values must be null
		// ? Is it possible case for Qtum
		Hash:   utils.AddHexPrefix(req.BlockHash),
		Number: hexutil.EncodeUint64(uint64(block.Height)),

		// TODO: Mark is researching
		// ! Not found
		// ! Incorrect value
		ReceiptsRoot: utils.AddHexPrefix(block.Merkleroot),
		// TODO: Mark is researching
		// ! Not found
		// ! Probably, may be calculated by many requests
		TotalDifficulty: hexutil.EncodeUint64(uint64(blockHeader.Difficulty)),

		// TODO: Mark is researching
		// ! Not found
		// ? Expect always null
		Uncles: []string{},
		// TODO: check value correctness
		Sha3Uncles: "0x1dcc4de8dec75d7aab85b567b6ccd41ad312451b948a7413f0a142fd40d49347",

		// TODO: backlog
		// ! Not found
		// - Temporary expect this value to be always zero, as Etherium logs are usually zeros
		LogsBloom: "0x00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000",

		// TODO: Mark is researching
		// ? What value to put
		// - Temporary set this value to be always zero
		ExtraData: "0x00",

		Nonce:            formatNonce(block.Nonce),
		Size:             hexutil.EncodeUint64(uint64(block.Size)),
		Difficulty:       hexutil.EncodeUint64(uint64(blockHeader.Difficulty)),
		StateRoot:        utils.AddHexPrefix(blockHeader.HashStateRoot),
		TransactionsRoot: utils.AddHexPrefix(block.Merkleroot),
		Transactions:     make([]interface{}, 0, len(block.Tx)),
		Timestamp:        hexutil.EncodeUint64(blockHeader.Time),
	}

	if blockHeader.IsGenesisBlock() {
		resp.ParentHash = "0x0000000000000000000000000000000000000000000000000000000000000000"
		resp.Miner = "0x0000000000000000000000000000000000000000"
	} else {
		resp.ParentHash = utils.AddHexPrefix(blockHeader.Previousblockhash)
		// ! Found only in txout method response
		//
		// NOTE:
		// 	In order to find a miner it seems, that we have to check
		// 	address field of the txout method response. Current
		// 	suggestion is to fill this field with zeros, not to
		// 	spend much time on requests execution
		//
		// TODO: check if it's value is acquirable via logs
		resp.Miner = "0x0000000000000000000000000000000000000000"
	}

	// TODO: suggestion
	// ? Maybe Qutum implements new RPC method to fetch all needed txs someday
	// * We already fetch tx info for each tx in a block
	// * Each call to GetTransactionByHash() func may call unpredictable additional requests inside
	if req.FullTransaction {
		// TODO: rethink someday
		// ! Found only for contracts transactions
		// As there is no gas values presented at common block info, we set
		// gas limit value equalling to default node gas limit for a block
		resp.GasLimit = utils.AddHexPrefix(qtum.DefaultBlockGasLimit)

		for i, txHash := range block.Tx {
			tx, err := GetTransactionByHash(p.Qtum, txHash, blockHeader.Height, i)
			if err != nil {
				return nil, errors.WithMessage(err, "couldn't get transaction by hash")
			}
			resp.Transactions = append(resp.Transactions, *tx)
			// TODO: fill gas used
			// resp.GasUsed +=  tx.Gas * 1e8
		}
	} else {
		for _, txHash := range block.Tx {
			// NOTE:
			// 	Etherium RPC API doc says, that tx hashes must be of [32]byte,
			// 	however it doesn't seem to be correct, 'cause Etherium tx hash
			// 	has [64]byte just like Qtum tx hash has. In this case we do no
			// 	additional convertations. Now everything works fine
			resp.Transactions = append(resp.Transactions, utils.AddHexPrefix(txHash))
		}
	}

	return resp, nil
}

// Formats Qtum nonce to Etherium like value. Length of the resulting string is 16+2 (0x) bytes
func formatNonce(nonce int) string {
	var (
		hexedNonce     = strconv.FormatInt(int64(nonce), 16)
		missedCharsNum = 16 - len(hexedNonce)
	)
	for i := 0; i < missedCharsNum; i++ {
		hexedNonce = "0" + hexedNonce
	}
	return "0x" + hexedNonce
}
