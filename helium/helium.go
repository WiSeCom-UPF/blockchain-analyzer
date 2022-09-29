package helium

import (
	"fmt"
	"net/http"
	"io/ioutil"
	"strings"
	"os"
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

	// Get the block specific data such as time, hash, height etc.
	url := fmt.Sprintf("%s/v1/blocks/%d", h.RPCEndpoint, blockNumber)
	req, _ := http.NewRequest("GET", url, nil)
	// User agent added, for smooth Helium API access
	req.Header.Set("User-Agent", "My User-Agent")
	resp, err := client.Do(req)

	temp_res_data, _ := ioutil.ReadAll(resp.Body)
	temp_res_data_str := string(temp_res_data)
	var data interface{}
	json.Unmarshal([]byte(temp_res_data_str), &data)
	m := data.(map[string]interface{})
	temp := m["data"]
	v := temp.(map[string]interface{})
	// Data to be put in a block
	transaction_count := v["transaction_count"]
	hash := v["hash"]
	prev_hash := v["prev_hash"]
	block_time := int(v["time"].(float64))

	// Get the transactions inside the block
	url = fmt.Sprintf("%s/v1/blocks/%d/transactions", h.RPCEndpoint, blockNumber)
	req, _ = http.NewRequest("GET", url, nil)
	// User agent added, for smooth Helium API access
	req.Header.Set("User-Agent", "My User-Agent")
	resp, err = client.Do(req)

	var result_string string
	if err == nil {
		raw_bytes, _ := ioutil.ReadAll(resp.Body)
		cursor_present, cursor_value := h.IsCursorPresent(string(raw_bytes))
		if cursor_present {
			result_string = string(h.RemoveCursor(string(raw_bytes)))
		} else {
			result_string += string(raw_bytes)
		}

		for cursor_present {
			// make a new API request with the cursor
			url = fmt.Sprintf("%s/v1/blocks/%d/transactions?cursor=%s", h.RPCEndpoint, blockNumber, cursor_value)
			time.Sleep(time.Second)
			req, _ = http.NewRequest("GET", url, nil)
			req.Header.Set("User-Agent", "My User-Agent")
			resp, err = client.Do(req)
			if err != nil {
				return resp, err
			}
			
			// post process the response of the API request
			raw_bytes, _ := ioutil.ReadAll(resp.Body)	
			// check and get the cursor first		
			cursor_present, cursor_value = h.IsCursorPresent(string(raw_bytes))
			temp_var := string(raw_bytes)[10:]
			// if ther cursor is present then remove it
			if cursor_present{
				result_string += string(h.RemoveCursor(temp_var))
			} else {
				result_string += temp_var
			}
		}

		len_result_string := len(result_string)
		// append the attributes captured earlier in each block
		if string(result_string[0]) == "{"  && string(result_string[len_result_string-1]) == "}" {
			result_string = strings.TrimSuffix(result_string, "}")
			payloadstr := fmt.Sprintf(`,"height": %d, transaction_count %v, "time": %d, "prev_hash": %s, "hash" %s: }`, blockNumber, transaction_count, block_time, prev_hash, hash)
			result_string += payloadstr
		} 
		// append the final block structure with response body
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

func (h *Helium) RemoveCursor(raw_stream string) string {
	// get cursor index, usually the last element, safe to remove
	cursor_index := strings.Index(raw_stream, `,"cursor":"`)
	// return the new string skip -2 elements
	return raw_stream[0:cursor_index-2]
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
func (b *Block) GovernanceTransactionsCount() int {
	// not yet implemented
	return 0
}

// TO-DO
func (b *Block) GetTxnP2Plist() []string {
	// not yet implemented
	return nil
}

// TO-DO
func (b *Block) SCCount(by string) (int, []string) {
	// not yet implemented
	return 0, nil
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
