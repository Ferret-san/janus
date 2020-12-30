package transformer

import (
	"strings"

	"github.com/ethereum/go-ethereum/common/hexutil"
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
	var req eth.GetBlockByHashRequest
	if err := unmarshalRequest(rawreq.Params, &req); err != nil {
		return nil, err
	}

	qtumreq := p.ToRequest(&req)

	return p.request(qtumreq)
}
func (p *ProxyETHGetBlockByHash) request(req *eth.GetBlockByHashRequest) (*eth.GetBlockByHashResponse, error) {
	blockHeaderResp, err := p.GetBlockHeader(req.BlockHash)
	if err != nil {
		return nil, err
	}

	// TODO: Correct to normal values
	if blockHeaderResp.Previousblockhash == "" {
		blockHeaderResp.Previousblockhash = "0000000000000000000000000000000000000000000000000000000000000000"
	}

	nonce := hexutil.EncodeUint64(uint64(blockHeaderResp.Nonce))

	// TODO: Correct translation into hex
	if len(strings.TrimLeft(nonce, "0x")) < 16 {
		res := strings.TrimLeft(nonce, "0x")
		for i := 0; i < 16-len(res); {
			res = "0" + res
		}
		nonce = res
	}

	blockResp, err := p.GetBlock(string(req.BlockHash))
	if err != nil {
		return nil, err
	}

	txsString := make([]string, 0, len(blockResp.Tx))
	for _, tx := range blockResp.Tx {
		txsString = append(txsString, utils.AddHexPrefix(tx))
	}

	txsObj := make([]eth.GetTransactionByHashResponse, 0, len(blockResp.Tx))
	for i, tx := range blockResp.Tx {
		if blockHeaderResp.Height == 0 {
			break
		}

		ethTx, err := GetTransactionByHash(p.Qtum, tx, blockHeaderResp.Height, i)
		if err != nil {
			return nil, err
		}

		txsObj = append(txsObj, *ethTx)
	}

	// TODO: Correct to normal values
	result := &eth.GetBlockByHashResponse{
		Hash:             utils.AddHexPrefix(blockHeaderResp.Hash),
		Nonce:            utils.AddHexPrefix(nonce),
		Number:           hexutil.EncodeUint64(uint64(blockHeaderResp.Height)),
		ParentHash:       utils.AddHexPrefix(blockHeaderResp.Previousblockhash),
		Difficulty:       hexutil.EncodeUint64(uint64(blockHeaderResp.Difficulty)),
		Timestamp:        hexutil.EncodeUint64(blockHeaderResp.Time),
		StateRoot:        utils.AddHexPrefix(blockHeaderResp.HashStateRoot),
		Size:             hexutil.EncodeUint64(uint64(blockResp.Size)),
		Transactions:     make([]string, 0),
		TransactionsRoot: utils.AddHexPrefix(blockResp.Merkleroot),
		ReceiptsRoot:     utils.AddHexPrefix(blockResp.Merkleroot),

		ExtraData:       "0x00",
		Miner:           "0x0000000000000000000000000000000000000000",
		TotalDifficulty: "0x00",
		GasLimit:        "0x00",
		GasUsed:         "0x00",
		LogsBloom:       "0x00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000",

		Sha3Uncles: "0x1dcc4de8dec75d7aab85b567b6ccd41ad312451b948a7413f0a142fd40d49347",
		Uncles:     []string{},
	}

	if !req.FullTransaction {
		result.Transactions = txsString
	} else {
		result.Transactions = txsObj
	}

	return result, nil
}

func (p *ProxyETHGetBlockByHash) ToRequest(ethreq *eth.GetBlockByHashRequest) *eth.GetBlockByHashRequest {
	return &eth.GetBlockByHashRequest{
		BlockHash:       utils.RemoveHexPrefix(strings.Trim(ethreq.BlockHash, "\"")),
		FullTransaction: ethreq.FullTransaction,
	}
}
