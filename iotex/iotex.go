package iotex

import (
	"os"
	"fmt"
	"time"
	"strings"
	"net/http"
	"math/big"

	jsoniter "github.com/json-iterator/go"

	"github.com/danhper/blockchain-analyzer/core"
	"github.com/danhper/blockchain-analyzer/fetcher"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

// const defaultRPCEndpoint string = "https://babel-api.mainnet.iotex.one"
const defaultRPCEndpoint string = "https://rpc.ankr.com/iotex"

type Iotex struct {
	RPCEndpoint string
}

func (ix *Iotex) makeRequest(client *http.Client, blockNumber uint64) (*http.Response, error) {
	// not using client, not useful
	url := fmt.Sprintf("%s", ix.RPCEndpoint)
	blockNumberHex := fmt.Sprintf("%016x",blockNumber)
	payloadRaw := fmt.Sprintf(`{"jsonrpc":"2.0","method":"eth_getBlockByNumber","params":["%s", true],"id":1}`, blockNumberHex)
	payload := strings.NewReader(payloadRaw)
	req, _ := http.NewRequest("POST", url, payload)

	res, err := http.DefaultClient.Do(req)
	// if err != nil {
	// 	return res, err
	// }

	// apiRes, err := ix.makeRequestWebAPI(client, blockNumber)
	// if err != nil {
	// 	return apiRes, err
	// }

	// bodyRPCAPI, _ := ioutil.ReadAll(res.Body)
	// if err != nil {
	// 	return res, err
	// }
	// bodyRPCAPI_str := string(bodyRPCAPI)
	// bodyRPCAPI_str = strings.TrimSuffix(bodyRPCAPI_str, "}")

	// bodyWebAPI, _ := ioutil.ReadAll(apiRes.Body)
	// if err != nil {
	// 	return apiRes, err
	// }
	// bodyWebAPI_str := string(bodyWebAPI)

	// index := strings.Index(bodyWebAPI_str,`"error":null,"meta":`)
	// apiData := `,"txns":` + bodyWebAPI_str[21:index-1]

	// appendedStr := bodyRPCAPI_str + apiData

	// res.Body = ioutil.NopCloser(strings.NewReader(appendedStr))

	return res, err
}

func (ix *Iotex) makeRequestWebAPI(client *http.Client, blockNumber uint64) (*http.Response, error) {
	url := "https://iotexscan.io/api/services/action/queries/getActionsData"
	method := "POST"

	payloadstr := fmt.Sprintf(`{"params":{"height":%d,"page":1,"limit":10000,"address":"","types":null},"meta":{"params":{"values":{"types":["undefined"]}}}}`, blockNumber)
	payload := strings.NewReader(payloadstr)

	req, _ := http.NewRequest(method, url, payload)

	req.Header.Add("authority", "iotexscan.io")
	req.Header.Add("accept", "*/*")
	req.Header.Add("accept-language", "en-US,en;q=0.9,hi;q=0.8")
	req.Header.Add("content-type", "application/json")
	// req.Header.Add("cookie", "iotexscan-v2_sPublicDataToken=eyJ1c2VySWQiOm51bGx9; iotexscan-v2_sAnonymousSessionToken=eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJibGl0empzIjp7ImlzQW5vbnltb3VzIjp0cnVlLCJoYW5kbGUiOiJZQzl6c21ITTlqOU1kWnlwZlVBc2hHVU9pT2RtRndEVjphand0IiwicHVibGljRGF0YSI6eyJ1c2VySWQiOm51bGx9LCJhbnRpQ1NSRlRva2VuIjoiYk14bS1DTmg3VG4yM1g0blV5c0dfbE53c3didVRZQm0ifSwiaWF0IjoxNjM2OTM4MDU0LCJhdWQiOiJibGl0empzIiwiaXNzIjoiYmxpdHpqcyIsInN1YiI6ImFub255bW91cyJ9.AijzTGhE0yk6bn1VL0NzPumKDnbsCR58ihvh9MSqav0; iotexscan-v2_sAntiCsrfToken=bMxm-CNh7Tn23X4nUysG_lNwswbuTYBm")
	req.Header.Add("origin", "https://iotexscan.io")
	req.Header.Add("referer", "https://iotexscan.io/txs?block=16860135&format=0x")
	req.Header.Add("sec-ch-ua", "\" Not A;Brand\";v=\"99\", \"Chromium\";v=\"100\", \"Google Chrome\";v=\"100\"")
	req.Header.Add("sec-ch-ua-mobile", "?0")
	req.Header.Add("sec-ch-ua-platform", "\"Linux\"")
	req.Header.Add("sec-fetch-dest", "empty")
	req.Header.Add("sec-fetch-mode", "cors")
	req.Header.Add("sec-fetch-site", "same-origin")
	req.Header.Add("sentry-trace", "cdb700418f334ca6bcafa5c7bf374d13-a9ececeefe10d341-1")
	req.Header.Add("user-agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/100.0.4896.75 Safari/537.36")

	res, err := client.Do(req)
	if err != nil {
		fmt.Println(err)
		return res, err
	}

	return res, err
}


func (ix *Iotex) FetchData(filepath string, start, end uint64) error {
	context := fetcher.NewHTTPContext(start, end, ix.makeRequest)
	return fetcher.FetchHTTPData(filepath, context)
}

type Transaction struct {
	Hash     			string  // from JSON data
	Value				string  // from JSON data
	Amount      		*big.Int
	Gas					string  // from JSON data
	ParsedGasLimit 		uint64
	SCSign				string
	TokenCount			uint64
	Kind        		string `json:"input"`  // from JSON data
	Source      		string `json:"from"`  // from JSON data
	Destination 		string `json:"to"`  // from JSON data
	SmartContractCreated		string `json:"creates"`  // from JSON data
	}

type BlockData struct {
	Miner               string `json:"author"`  // from JSON data
	Size  				string 	// from JSON data
	TotalTxnCount		uint64
	Number     			string 	// from JSON data
	Timestamp       	string 	// from JSON data
	ParsedTimestamp 	time.Time
	ParsedNumber 		uint64  // block number in decimal
	Transactions 		[]Transaction  // from JSON data
	GovernanceTxns		uint64
	GasUsed         	string  // from JSON data
	GasUsedByBlock		uint64
}

type Block struct {
	BlockData	     	BlockData `json:"result"`  // from JSON data
	actions    			[]core.Action
	IsEmptyBlock		bool //  if block only contains 1 mandatory txn, i.e. block mining reward
	ZeroTxnBlock		bool //  if no transafer or contract txn present, may be governance
	Uncles				[]string
}

func New() *Iotex {
	rpcEndpoint := os.Getenv("IOTEX_RPC_ENDPOINT")
	if rpcEndpoint == "" {
		rpcEndpoint = defaultRPCEndpoint
	}

	return &Iotex{
		RPCEndpoint: rpcEndpoint,
	}
}

func (ix *Iotex) ParseBlock(rawLine []byte) (core.Block, error) {
	var block Block
	var err error
	if err = json.Unmarshal(rawLine, &block); err != nil {
		fmt.Println(err)
		return nil, err
	}

	// convert from string to time.Time format
	block.BlockData.ParsedTimestamp, err = ix.ConvertStringTimestampToTime(block)
	if err != nil {
		return nil, err
	}

	block.BlockData.ParsedNumber, err = core.HexStringToDecimal(block.BlockData.Number)
	if err != nil {
		return nil, err
	}

	// this count also includes governance transactions
	block.BlockData.TotalTxnCount, err = core.HexStringToDecimal(block.BlockData.Size)
	if err != nil {
		return nil, err
	}

	txnsInBlock := uint64(len(block.BlockData.Transactions))

	// calculates the total number of Governance transaction inside the block
	block.BlockData.GovernanceTxns = block.BlockData.TotalTxnCount - txnsInBlock - 1

	block.BlockData.GasUsedByBlock, err = core.HexStringToDecimal(block.BlockData.GasUsed)
	if err != nil {
		return nil, err
	}

	if 	txnsInBlock == 0 && block.BlockData.GovernanceTxns == 0 {
		block.IsEmptyBlock = true
	} else {
		block.IsEmptyBlock = false
	}

	if txnsInBlock == 0 && block.BlockData.GovernanceTxns != 0 {
		block.ZeroTxnBlock = true
	} else {
		block.ZeroTxnBlock = false
	}

	i := 0
	for _, txn := range block.BlockData.Transactions {
		block.BlockData.Transactions[i].Amount = core.HexStringToDecimalIntBig(txn.Value)

		if txn.Kind == "0x" {
			block.BlockData.Transactions[i].Kind = "Transfer"
		} else {
			if block.BlockData.Transactions[i].SmartContractCreated == "" {
				inputLen := len(block.BlockData.Transactions[i].Kind)
				block.BlockData.Transactions[i].SCSign = block.BlockData.Transactions[i].Kind[:10]
				if inputLen > 10 {
					block.BlockData.Transactions[i].TokenCount, err = core.HexStringToDecimal(block.BlockData.Transactions[i].Kind[inputLen-9:])
				}
				if err != nil {
					fmt.Println(txn.Hash)
				}
			// fmt.Println(block.BlockData.Transactions[i].TokenCount, block.BlockData.Transactions[i].SCSign, block.BlockData.Transactions[i].SmartContractCreated)
			}

			block.BlockData.Transactions[i].Kind = "Contract"
		}

		block.BlockData.Transactions[i].ParsedGasLimit, err = core.HexStringToDecimal(txn.Gas)
		if err != nil {
			return nil, err
		}
		i = i+1
	}

	return &block, err
}

func (ix *Iotex) ConvertStringTimestampToTime(marshalledBlock Block) (time.Time, error) {
	// convert hex string to decimal
	timestamp, err := core.HexStringToDecimal(marshalledBlock.BlockData.Timestamp)
	unixTimeUTC := time.Unix(int64(timestamp), 0)
	parsedTimeString := unixTimeUTC.Format(time.RFC3339)
	// convert from string to time.Time format
	parsedTime, err := time.Parse(time.RFC3339, parsedTimeString)

	return parsedTime, err
}

func (ix *Iotex) EmptyBlock() core.Block {
	return &Block{}
}

func (b *Block) Number() uint64 {
	return b.BlockData.ParsedNumber
}

func (b *Block) Time() time.Time {
	return b.BlockData.ParsedTimestamp
}

func (b *Block) TransactionsCount() int {
	return len(b.BlockData.Transactions)
}

func (b *Block) GetTxnP2Plist() []string {
	p2pTxnData := make([]string, 2 * len(b.BlockData.Transactions))
	txnKey 	:= ""
	txnKind := ""
	// counter to iterate over p2pTxnData array
	i := 0
	for _, txn := range b.BlockData.Transactions {
		txnKey = txn.Sender() + " TO " + txn.Receiver()
		txnKind = txn.Name()
		p2pTxnData[i] = txnKey
		i +=1
		p2pTxnData[i] = txnKind
		i +=1
	}
	return p2pTxnData
}

func (b *Block) SCCount(by string) (int, []string) {
	counter := 0
	SCSignCounter := []string{}
	if by == "" {
		for _, txn := range b.BlockData.Transactions {
			if txn.Destination == "" {
				counter += 1
			}
		}
	} else {
		for _, txn := range b.BlockData.Transactions {
			if txn.Destination == "" {
				counter += 1
			}
			if txn.Kind == "Contract" && txn.SmartContractCreated == "" {
				SCSignCounter = append(SCSignCounter, txn.SCSign )
			}
		}
	}
	return counter, SCSignCounter
}

func (b *Block) TransactionsCountByAddress(address string, by string) int {
	txCounter := 0
	for _, txn := range b.BlockData.Transactions {
			if by == "sender" || by == "Sender"{
				if txn.Source == address{
					txCounter = txCounter + 1
				}
			} else if by == "receiver" || by == "Receiver" {
				if txn.Destination == address{
					txCounter = txCounter + 1
			}
		}
	}
	return txCounter
}

func (b *Block) EmptyBlocksCount() int {
	if b.IsEmptyBlock{
		return 1	
	}
	return 0
}


func (b *Block) ZeroTxnBlocksCount() int {
	if b.ZeroTxnBlock{
		return 1
	}
	return 0
}

func (b *Block) GetMiner() string {
	return b.BlockData.Miner
}

func (b *Block) ListActions() []core.Action {
	if len(b.actions) > 0 {
		return b.actions
	}
	var result []core.Action
	for _, txn := range b.BlockData.Transactions {
		result = append(result, txn)
	}
	b.actions = result
	return result
}

func (t Transaction) Name() string {
	return t.Kind
}

func (t Transaction) Receiver() string {
	return t.Destination
}

func (t Transaction) Sender() string {
	return t.Source
}
