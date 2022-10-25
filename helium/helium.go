package helium

import (
	"fmt"
	"net/http"
	"io/ioutil"
	"strings"
	"os"
	"time"
	// "reflect"

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
	if err != nil {
		return resp, err
	}

	temp_res_data, _ := ioutil.ReadAll(resp.Body)
	temp_res_data_str := string(temp_res_data)
	if len(temp_res_data_str) < 165 {
		resp.StatusCode = 429
		return resp, err
	}

	var data map[string]map[string]interface{}
	// var height map[string]float64
	json.Unmarshal([]byte(temp_res_data_str), &data)
	// m := data.(map[string]interface{})
	// temp := data["data"]
	// height["height"] = data["data"]["height"].(float64)
	// fmt.Println(data, blockNumber)
	v := data["data"]
	// fmt.Println(v, string(temp_res_data))
	
	// Data to be put in a block
	transaction_count := v["transaction_count"]
	hash := v["hash"]
	prev_hash := v["prev_hash"]
	// block_time := v["time"]
	// var dd interface{} = data["data"]["time"]
	// var val1 int = dd.(int)
	// fmt.Println(reflect.TypeOf(dd))
	// block_time = int(block_time.(float64))

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
			temp_var := string(raw_bytes)[9:]
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
			payloadstr := ""
			// It is a special case where last data is type_witness
			if (string(result_string[len(result_string) -1]) != "]"){
				payloadstr = fmt.Sprintf(`}],"height": %d, "transaction_count": %v, "time": %d, "prev_hash": "%s", "hash": "%s"}`, blockNumber, transaction_count, int(data["data"]["time"].(float64)), prev_hash, hash)
			} else {
				payloadstr = fmt.Sprintf(`,"height": %d, "transaction_count": %v, "time": %d, "prev_hash": "%s", "hash": "%s"}`, blockNumber, transaction_count, int(data["data"]["time"].(float64)), prev_hash, hash)
			}
			result_string += payloadstr
		}
		// append the final block structure with response body
		resp.Body = ioutil.NopCloser(strings.NewReader(string(result_string)))
		// fmt.Println(string(result_string))
		// fmt.Println(v, string(temp_res_data))
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
	return raw_stream[0:cursor_index-2]+"},"
}


func (h *Helium) FetchData(filepath string, start, end uint64) error {
	context := fetcher.NewHTTPContext(start, end, h.makeRequest)
	return fetcher.FetchHTTPData(filepath, context)
}

// type Content struct {
// 	Kind        string
// 	Source      string
// 	Destination string
// 	Amount      string
// }

// type PoCRequest struct {
// 	Hash     string
// 	Contents []Content
// }

type TransactionData struct {
	Version         uint64  `json:"version"`
	Type            string
	Timestamp       uint64  `json:"time"`
	ParsedTimestamp time.Time
	Hash            string
}

type Block struct {
	BlockNumber      uint64  `json:"height"`
	TransactionCount uint64  `json:"transaction_count"`
	Timestamp        uint64  `json:"time"`
	BlockTimestamp   time.Time
	Hash             string
	PrevHash         string  `json:"prev_hash"`
	Transactions     []TransactionData `json:"data"`
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
		fmt.Println(err)
		return nil, err
	}

	parsedTime, err := h.ConvertDecimalTimestampToTime(block.Timestamp)
	if err != nil {
		return nil, err
	}
	block.BlockTimestamp = parsedTime
	return &block, nil
}

func (h *Helium) ConvertDecimalTimestampToTime(timeStamp uint64) (time.Time, error) {
	// convert UNIX decimal timestamp to 
	unixTimeUTC := time.Unix(int64(timeStamp), 0)
	parsedTimeString := unixTimeUTC.Format(time.RFC3339)
	// convert from string to time.Time format
	parsedTime, err := time.Parse(time.RFC3339, parsedTimeString)

	return parsedTime, err
}

func (h *Helium) EmptyBlock() core.Block {
	return &Block{}
}

func (b *Block) Number() uint64 {
	return b.BlockNumber
}

func (b *Block) Time() time.Time {
	return b.BlockTimestamp
}

func (b *Block) TransactionsCount() int {
	// total := 0
	// for _, operations := range b.Operations {
	// 	total += len(operations)
	// }
	return int(b.TransactionCount)
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
	// if len(b.actions) > 0 {
	// 	return b.actions
	// }
	// var result []core.Action
	// for _, operations := range b.Operations {
	// 	for _, operation := range operations {
	// 		for _, content := range operation.Contents {
	// 			result = append(result, content)
	// 		}
	// 	}
	// }
	// b.actions = result
	return nil
}

// func (c Content) Name() string {
// 	return c.Kind
// }

// func (c Content) Receiver() string {
// 	return c.Destination
// }

// func (c Content) Sender() string {
// 	return c.Source
// }
