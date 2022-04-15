package iotex

import (
	"os"
	"fmt"
	"time"
	"io/ioutil"
	"strings"
	"net/http"

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
	payloadRaw := fmt.Sprintf(`{"jsonrpc":"2.0","method":"eth_getBlockByNumber","params":["%s", false],"id":1}`, blockNumberHex)
	payload := strings.NewReader(payloadRaw)
	req, _ := http.NewRequest("POST", url, payload)

	res, err := http.DefaultClient.Do(req)
	if err != nil {
	return res, err
	}

	apiRes, err := ix.makeRequestWebAPI(client, blockNumber)
	if err != nil {
		return apiRes, err
	}

	bodyRPCAPI, _ := ioutil.ReadAll(res.Body)
	if err != nil {
		return res, err
	}
	bodyRPCAPI_str := string(bodyRPCAPI)
	bodyRPCAPI_str = strings.TrimSuffix(bodyRPCAPI_str, "}")

	bodyWebAPI, _ := ioutil.ReadAll(apiRes.Body)
	if err != nil {
		return apiRes, err
	}
	bodyWebAPI_str := string(bodyWebAPI)

	index := strings.Index(bodyWebAPI_str,`"error":null,"meta":`)
	apiData := `,"txns":` + bodyWebAPI_str[21:index-1]

	appendedStr := bodyRPCAPI_str + apiData

	res.Body = ioutil.NopCloser(strings.NewReader(appendedStr))

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

type Content struct {
	Kind        string
	Source      string `json:"from"`
	Destination string `json:"to"`
	Amount      string
}

type Transaction struct {
	Hash     string
	Contents []Content
}

type BlockData struct {
	Number     uint64
	Timestamp       string
	ParsedTimestamp time.Time
	Transactions []Transaction
}

type Block struct {
	Result     BlockData
	// Transactions [][]Transaction
	actions    []core.Action
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
	if err := json.Unmarshal(rawLine, &block); err != nil {
		// fmt.Println(err)
		return nil, err
	}
	// convert hex string to decimal
	timestamp, err := core.HexStringToDecimal(block.Result.Timestamp)
	unixTimeUTC :=time.Unix(int64(timestamp), 0)
	parsedTimeString := unixTimeUTC.Format(time.RFC3339)
	// convert from string to time.Time format
	parsedTime, err := time.Parse(time.RFC3339, parsedTimeString)
	if err != nil {
		return nil, err
	}

	block.Result.ParsedTimestamp = parsedTime
	return &block, nil
}

func (ix *Iotex) EmptyBlock() core.Block {
	return &Block{}
}

func (b *Block) Number() uint64 {
	return b.Result.Number
}

func (b *Block) Time() time.Time {
	return b.Result.ParsedTimestamp
}

func (b *Block) TransactionsCount() int {
	total := 0
	// for _, operations := range b.Transactions {
	// 	total += len(operations)
	// }
	// fmt.Println(total)
	return total
}

func (b *Block) ListActions() []core.Action {
	if len(b.actions) > 0 {
		return b.actions
	}
	var result []core.Action
	// for _, operations := range b.Transactions {
	// 	for _, operation := range operations {
	// 		for _, content := range operation.Contents {
	// 			result = append(result, content)
	// 		}
	// 	}
	// }
	b.actions = result
	return result
}

func (c Content) Name() string {
	return c.Kind
}

func (c Content) Receiver() string {
	return c.Destination
}

func (c Content) Sender() string {
	return c.Source
}
