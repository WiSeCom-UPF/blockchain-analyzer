package helium

import (
	"fmt"
	"net/http"
	"io/ioutil"
	"strings"
	// "io"
	"os"
	// "reflect"
	// "encoding/json"
	"time"

	jsoniter "github.com/json-iterator/go"

	"github.com/danhper/blockchain-analyzer/core"
	"github.com/danhper/blockchain-analyzer/fetcher"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

// const defaultRPCEndpoint string = "http://127.0.0.1:8080"
const defaultRPCEndpoint string = "https://api.helium.io"

type Helium struct {
	RPCEndpoint string
}


func (h *Helium) makeRequestWithCursor(client *http.Client, blockNumber uint64, cursor string) (*http.Response, error) {
	url := fmt.Sprintf("%s/v1/blocks/%d/transactions", h.RPCEndpoint, blockNumber)
	
	req, _ := http.NewRequest("GET", url, nil)
	// if err != nil {
	// 	return 429, "TO-DO"
	// }
	// User agent added, for smooth Helium API access
	req.Header.Set("User-Agent", "My User-Agent")
	resp, err := client.Do(req)

	// pr, pw := io.Pipe()
	// defer pw.Close()
	result_string := ""
	if err == nil {
		raw_bytes, _ := ioutil.ReadAll(resp.Body)
		result_string += string(raw_bytes)
		resp.Body = ioutil.NopCloser(strings.NewReader(string(raw_bytes)))
		raw_bytes1, _ := ioutil.ReadAll(resp.Body)
		if len(raw_bytes1) < 20	 {
			payloadstr := fmt.Sprintf(`{"height": %d, "isEmpty" : "true"}`, blockNumber)
			resp.Body = ioutil.NopCloser(strings.NewReader(string(payloadstr)))
			return resp, nil
		} 

		cursor_present, cursor_value := h.IsCursorPresent(string(raw_bytes))
		for cursor_present {

			url = fmt.Sprintf("%s/v1/blocks/%d/transactions?cursor=%s", h.RPCEndpoint, blockNumber, cursor_value)
			time.Sleep(time.Second)
			req, _ = http.NewRequest("GET", url, nil)
			req.Header.Set("User-Agent", "My User-Agent")
			resp, err = client.Do(req)
			if err != nil {
				return resp, err
			}
			
			raw_bytes, _ := ioutil.ReadAll(resp.Body)			
			result_string += string(raw_bytes)
			cursor_present, cursor_value = h.IsCursorPresent(string(raw_bytes))
		}
		resp.Body = ioutil.NopCloser(strings.NewReader(string(result_string)))
	}
	return resp, err
}

func (h *Helium) makeRequest(client *http.Client, blockNumber uint64) (*http.Response, error) {
	return h.makeRequestWithCursor(client, blockNumber, "")
}

func (h *Helium) IsCursorPresent(raw_stream string) (IsPresent bool, cursor string) {
	var data map[string]interface{}
	json.Unmarshal([]byte(raw_stream), &data)

	// check if cursor is found
	if data["cursor"] != nil {
    	return true, data["cursor"].(string)
	}

	return false, ""
}


func (h *Helium) FetchData(filepath string, start, end uint64) error {
	context := fetcher.NewHTTPContext(start, end, h.makeRequest)
	return fetcher.FetchHTTPData(filepath, context)
}

type Content struct {
	Kind        string
	Source      string
	Destination string
	Amount      string
}

type Operation struct {
	Hash     string
	Contents []Content
}

type BlockHeader struct {
	Level           uint64
	Timestamp       string
	ParsedTimestamp time.Time
}

type Block struct {
	Header     BlockHeader
	Operations [][]Operation
	actions    []core.Action
}

func New() *Helium {
	rpcEndpoint := os.Getenv("Helium_RPC_ENDPOINT")
	if rpcEndpoint == "" {
		rpcEndpoint = defaultRPCEndpoint
	}

	return &Helium{
		RPCEndpoint: rpcEndpoint,
	}
}

func (h *Helium) ParseBlock(rawLine []byte) (core.Block, error) {
	var block Block
	if err := json.Unmarshal(rawLine, &block); err != nil {
		return nil, err
	}
	parsedTime, err := time.Parse(time.RFC3339, block.Header.Timestamp)
	if err != nil {
		return nil, err
	}
	block.Header.ParsedTimestamp = parsedTime
	return &block, nil
}

func (h *Helium) EmptyBlock() core.Block {
	return &Block{}
}

func (b *Block) Number() uint64 {
	return b.Header.Level
}

func (b *Block) Time() time.Time {
	return b.Header.ParsedTimestamp
}

func (b *Block) TransactionsCount() int {
	total := 0
	for _, operations := range b.Operations {
		total += len(operations)
	}
	return total
}

// TO-DO
func (b *Block) GetTxnP2Plist() []string {
	// not yet implemented
	return nil
}

// TO-DO
func (b *Block) SCCount() int {
	// not yet implemented
	return 0
}

// TO-DO
func (b *Block) TransactionsCountByAddress(address string, by string) int {
	// not yet implemented
	return 0
}

// TO-DO
func (b *Block) EmptyBlocksCount() int {
	// not yet implemented
	return 0
}

// TO-DO
func (b *Block) ZeroTxnBlocksCount() int {
	// // not yet implemented
	return 0
}

func (b *Block) GetMiner() string {
	// // not yet implemented
	return ""
}

func (b *Block) ListActions() []core.Action {
	if len(b.actions) > 0 {
		return b.actions
	}
	var result []core.Action
	for _, operations := range b.Operations {
		for _, operation := range operations {
			for _, content := range operation.Contents {
				result = append(result, content)
			}
		}
	}
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
