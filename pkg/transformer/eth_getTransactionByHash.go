package transformer

import (
	"encoding/json"
	"strings"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/pkg/errors"
	"github.com/qtumproject/janus/pkg/eth"
	"github.com/qtumproject/janus/pkg/qtum"
	"github.com/qtumproject/janus/pkg/utils"
)

// ProxyETHGetTransactionByHash implements ETHProxy
type ProxyETHGetTransactionByHash struct {
	*qtum.Qtum
}

func (p *ProxyETHGetTransactionByHash) Method() string {
	return "eth_getTransactionByHash"
}

func (p *ProxyETHGetTransactionByHash) Request(req *eth.JSONRPCRequest) (interface{}, error) {
	var txHash eth.GetTransactionByHashRequest
	if err := json.Unmarshal(req.Params, &txHash); err != nil {
		return nil, errors.Wrap(err, "couldn't unmarshal request")
	}
	if txHash == "" {
		return nil, errors.New("transaction hash is empty")
	}

	qtumReq := &qtum.GetTransactionRequest{
		TxID: utils.RemoveHexPrefix(string(txHash)),
	}
	return p.request(qtumReq)
}

func (p *ProxyETHGetTransactionByHash) request(req *qtum.GetTransactionRequest) (*eth.GetTransactionByHashResponse, error) {
	ethTx, err := getTransactionByHash(p.Qtum, req.TxID)
	if err != nil {
		return nil, err
	}
	return ethTx, nil
}

// TODO: think of returning flag if it's a rewrad transaction for miner
func getTransactionByHash(p *qtum.Qtum, hash string) (*eth.GetTransactionByHashResponse, error) {
	qtumTx, err := p.GetTransaction(hash)
	if err != nil {
		// TODO: implement typed error at Qtum pkg
		// * Filter "transaction not found" error
		//
		// TODO: researching
		// ?! Is the case only for reward transactions
		cause := errors.Cause(err)
		if !strings.Contains(cause.Error(), "[code: -5]") {
			return nil, err
		}

		ethTx, err := getRewardTransactionByHash(p, hash)
		if err != nil {
			return nil, errors.WithMessage(err, "couldn't get reward transaction by hash")
		}
		return ethTx, nil
	}

	qtumDecodedRawTx, err := p.DecodeRawTransaction(qtumTx.Hex)
	if err != nil {
		return nil, errors.WithMessage(err, "couldn't get raw transaction")
	}

	ethTx := &eth.GetTransactionByHashResponse{
		Hash: utils.AddHexPrefix(qtumDecodedRawTx.ID),
		// TODO: researching
		// ? May be don't need this
		// ! Probably, needs huge quantity requests to calc by hands
		Nonce: "0x0",

		// TODO: researching
		// ? Do we need those values
		V: "",
		R: "",
		S: "",
	}

	if !qtumTx.IsPending() { // otherwise, the following values must be nulls
		blockNumber, err := getBlockNumberByHash(p, qtumTx.BlockHash)
		if err != nil {
			return nil, errors.WithMessage(err, "couldn't get block number by hash")
		}
		ethTx.BlockNumber = hexutil.EncodeUint64(blockNumber)
		ethTx.BlockHash = utils.AddHexPrefix(qtumTx.BlockHash)
		ethTx.TransactionIndex = hexutil.EncodeUint64(uint64(qtumTx.BlockIndex))
	}

	amount, err := formatQtumAmount(qtumDecodedRawTx.CalcAmount())
	if err != nil {
		return nil, errors.WithMessage(err, "couldn't format amount")
	}
	ethTx.Value = amount

	qtumTxContractInfo, isContractTx, err := qtumDecodedRawTx.ExtractContractInfo()
	if err != nil {
		return nil, errors.WithMessage(err, "couldn't extract contract info")
	}
	if isContractTx {
		ethTx.Input = qtumTxContractInfo.UserInput
		ethTx.From = utils.AddHexPrefix(qtumTxContractInfo.From)
		ethTx.To = utils.AddHexPrefix(qtumTxContractInfo.To)
		ethTx.Gas = utils.AddHexPrefix(qtumTxContractInfo.GasUsed)
		// TODO: discuss, consider values
		// - gas limit is needed for `eth_getBlockByHash` request (return as a func result?)
		// ? ethTx.GasPrice
		ethTx.GasPrice = "0x0" // temporary solution

		return ethTx, nil
	}

	if qtumTx.Generated {
		ethTx.From = "0x0000000000000000000000000000000000"
	} else {
		ethTx.From, err = getNonContractTxSenderAddress(p, qtumDecodedRawTx.Vins)
		if err != nil {
			return nil, errors.WithMessage(err, "couldn't get non contract transaction sender address")
		}
	}
	ethTx.To, err = findNonContractTxReceiverAddress(qtumDecodedRawTx.Vouts)
	if err != nil {
		return nil, errors.WithMessage(err, "couldn't get non contract transaction receiver address")
	}

	for _, detail := range qtumTx.Details {
		// TODO: researching
		// ! Temporary solution
		ethTx.Input = detail.Label
		break
	}

	// TODO: researching
	// ? Is it correct for non contract transaction
	ethTx.Gas = "0x0"
	ethTx.GasPrice = "0x0"

	return ethTx, nil
}

// TODO: discuss
// ?! There are transactions, that is not acquireable nither via `gettransaction`, nor `getrawtransaction`
func getRewardTransactionByHash(p *qtum.Qtum, hash string) (*eth.GetTransactionByHashResponse, error) {
	rawQtumTx, err := p.GetRawTransaction(hash, false)
	if err != nil {
		return nil, errors.WithMessage(err, "couldn't get raw transaction")
	}

	ethTx := &eth.GetTransactionByHashResponse{
		Hash: utils.AddHexPrefix(hash),
		// TODO: researching
		// ? May be don't need this
		// ! Probably, needs huge quantity requests to calc by hands
		Nonce: "0x0",

		// TODO: discuss
		// ? Are zero values applicable
		Gas:      "0x0",
		GasPrice: "0x0",

		// TODO: researching
		// ? Do we need those values
		V: "",
		R: "",
		S: "",
	}

	if !rawQtumTx.IsPending() {
		blockIndex, err := getTransactionIndexInBlock(p, hash, rawQtumTx.BlockHash)
		if err != nil {
			return nil, errors.WithMessage(err, "couldn't get transaction index in block")
		}
		ethTx.TransactionIndex = hexutil.EncodeUint64(uint64(blockIndex))

		blockNumber, err := getBlockNumberByHash(p, rawQtumTx.BlockHash)
		if err != nil {
			return nil, errors.WithMessage(err, "couldn't get block number by hash")
		}
		ethTx.BlockNumber = hexutil.EncodeUint64(blockNumber)

		ethTx.BlockHash = utils.AddHexPrefix(rawQtumTx.BlockHash)
	}

	for i := range rawQtumTx.Vouts {
		_, err := p.GetTransactionOut(hash, i, rawQtumTx.IsPending())
		if err != nil {
			return nil, errors.WithMessage(err, "couldn't get transaction out")
		}
		// TODO: discuss, researching
		// ? Where is a reward amount
	}

	// TODO: discuss
	// ? Do we have to set `from` == `0x00..00`
	ethTx.From = "0x0000000000000000000000000000000000"
	// TODO: discuss
	// ? Where is a `to`
	ethTx.To = "0x0000000000000000000000000000000000"

	return ethTx, nil
}
