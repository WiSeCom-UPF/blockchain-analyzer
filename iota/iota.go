package iota

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"os"
	"sync"
	"time"

	"github.com/danhper/blockchain-analyzer/core"
	"github.com/danhper/blockchain-analyzer/fetcher"

	iotago "github.com/iotaledger/iota.go/v2"
	jsoniter "github.com/json-iterator/go"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

const defaultNodeEndpoint string = "https://chrysalis-nodes.iota.cafe:443" //"https://iota-node.tanglebay.com" //

type Iota struct {
	NodeEndpoint string
}

func New() *Iota {
	nodeEndpoint := os.Getenv("Iota_NODE_ENDPOINT")
	if nodeEndpoint == "" {
		nodeEndpoint = defaultNodeEndpoint
	}

	return &Iota{
		NodeEndpoint: nodeEndpoint,
	}
}

func (i *Iota) getBlock(blockNumber uint64) (*http.Response, error) {

	path, err := os.Getwd()
	if err != nil {
		log.Panic(err)
	}

	LOG_FILE := path + "/tmp/logs/iota_fetcher_logs.log"
	// open log file
    logFile, err := os.OpenFile(LOG_FILE, os.O_APPEND|os.O_RDWR|os.O_CREATE, 0644)
    if err != nil {
		fmt.Println("Error opnening log file")
        log.Panic(err)
    }
    defer logFile.Close()
    log.SetOutput(logFile)
    // optional: log date-time, filename, and line number
    log.SetFlags(log.Lshortfile | log.LstdFlags)

	resp := &http.Response{
		StatusCode: 200,
		Proto:      "HTTP/1.1",
		ProtoMajor: 1,
		ProtoMinor: 1,
		Header:     make(http.Header, 0),
	}

	// create a new node API client
	nodeHTTPAPIClient := iotago.NewNodeHTTPAPIClient(i.NodeEndpoint)
	if nodeHTTPAPIClient == nil {
		resp.StatusCode = 500
		return resp, fmt.Errorf("Node HTTP Client is nil")
	}

	ctx := context.Background()

	// fetch the node's info to know the pruning index
	info, err := nodeHTTPAPIClient.Info(ctx)
	if err != nil {
		log.Println("Error getting info from node: ", err)
		fmt.Println("Error getting info from node: ", err)
		resp.StatusCode = 500
		return resp, err
	}
	block := &Block{}
	pruningIndex := info.PruningIndex
	milestoneIndex := uint32(blockNumber) // The blockNumber passed must be a Milestone index, the messages confirmed by it will be stored

	if blockNumber < uint64(pruningIndex) { // If the message we are trying to fetch has already been pruned, we query from the IOTA explorer
		//err = fmt.Errorf("ERROR, cannot fetch block with milestone index: %v, pruning index is: %v, block number has to be bigger than pruning index ", blockNumber, pruningIndex)
		//resp.StatusCode = 500
		//return resp, err

		milestoneURL := fmt.Sprint("https://explorer-api.iota.org/milestone/mainnet/", milestoneIndex)
		milestoneResponseUrl, _, _, err := getDataExplorer(milestoneURL, true, false, false)
		if err != nil {
			fmt.Println("Error getting milestone from explorer: ", err)
			log.Println("Error getting milestone from explorer: ", err)
			resp.StatusCode = 500
			return resp, err
		}
		if milestoneResponseUrl.MessageID == "" {
			fmt.Println("Error getting milestone from explorer, milestone reponse message id is empty for url: ", milestoneURL)
			log.Println("Error getting milestone from explorer, milestone reponse message id is empty for url: ", milestoneURL)
			resp.StatusCode = 500
			return resp, fmt.Errorf("Error getting milestone from explorer, milestone reponse message id is empty")
		}

		messageMilestoneURL := fmt.Sprint("https://explorer-api.iota.org/search/mainnet/", milestoneResponseUrl.MessageID)
		_, msgResponseUrl, _, err := getDataExplorer(messageMilestoneURL, false, true, false)
		if err != nil {
			fmt.Println("Error getting message from explorer: ", err)
			log.Println("Error getting message from explorer: ", err)
			resp.StatusCode = 500
			return resp, err
		}
		if msgResponseUrl.Parents == nil {
			fmt.Println("Error getting message's parents from explorer, parents are nil, url: ", messageMilestoneURL)
			log.Println("Error getting message's parents from explorer, parents are nil, url: ", messageMilestoneURL)
			resp.StatusCode = 500
			return resp, fmt.Errorf("Error getting message's parents from explorer, parents are nil, url: %s", messageMilestoneURL)
		}

		block.BlockNumber = uint64(milestoneIndex)
		block.MilestoneID = milestoneResponseUrl.MessageID
		block.MilestoneIndex = milestoneIndex
		block.MilestomeTimestamp = milestoneResponseUrl.Time

		ch := make(chan error, len(msgResponseUrl.Parents))
		wg := sync.WaitGroup{}
		wg.Add(1)

		go getMessagesFromExplorerParallel(true, block, milestoneResponseUrl.MessageID, "https://explorer-api.iota.org/", ch, &wg)

		wg.Wait()
		close(ch)

		for err := range ch {
			if err != nil {
				resp.StatusCode = 500
				fmt.Println("Error getting the block: ", err)
				log.Println("Error getting the block: ", err)
				return resp, err
			}
		}

	} else {

		milestoneResponse, err := nodeHTTPAPIClient.MilestoneByIndex(ctx, milestoneIndex)
		if err != nil {
			resp.StatusCode = 500
			fmt.Println("Error getting milestone: ", err)
			log.Println("Error getting milestone: ", err)
			return resp, err
		}

		messageIDmilestone, err := iotago.MessageIDFromHexString(milestoneResponse.MessageID)
		if err != nil {
			resp.StatusCode = 500
			fmt.Println("Error getting messageID from hex string", err)
			log.Println("Error getting messageID from hex string", err)
			return resp, err
		}

		miletsoneMessage, err := nodeHTTPAPIClient.MessageByMessageID(ctx, messageIDmilestone)
		if err != nil {
			resp.StatusCode = 500
			fmt.Println("Error getting milestone messgae from message id", err)
			log.Println("Error getting milestone messgae from message id", err)
			return resp, err
		}

		block.BlockNumber = uint64(milestoneIndex)
		block.MilestoneID = hex.EncodeToString(messageIDmilestone[:])
		block.MilestoneIndex = milestoneIndex
		block.MilestomeTimestamp = milestoneResponse.Time

		ch := make(chan error, len(miletsoneMessage.Parents))
		wg := sync.WaitGroup{}
		wg.Add(1)

		go getMessagesFromMilestoneParallel(true, block, messageIDmilestone, nodeHTTPAPIClient, ctx, ch, &wg)

		wg.Wait()
		close(ch)

		for err := range ch {
			if err != nil {
				resp.StatusCode = 500
				fmt.Println("Error getting the block: ", err)
				log.Println("Error getting the block: ", err)
				return resp, err
			}
		}
	}

	fmt.Println("Done fetching all messages for block with milestone index: ", milestoneIndex)
	log.Println("Done fetching all messages for block with milestone index: ", milestoneIndex)

	block.MessagesCount = int(len(block.Messages))
	if block.MessagesCount > 0 {
		block.IsEmptyBlock = false
	} else {
		block.IsEmptyBlock = true
	}

	content, err := json.Marshal(block)
	if err != nil {
		resp.StatusCode = 500
		fmt.Println("Error marhsaling block: ", err)
		log.Println("Error marhsaling block: ", err)
		return resp, err
	}
	if content == nil {
		resp.StatusCode = 500
		err := fmt.Errorf("Content of marshalled block is nil")
		log.Println(err.Error())
		return resp, err
	}

	resp.Body = ioutil.NopCloser(bytes.NewReader(content))
	resp.ContentLength = int64(len(content))
	return resp, nil
}

type Type struct {
	Type *int `json:"type"`
	//X map[string]interface{} `json:"-"`
}

func getType(msg *iotago.Message) (int, error) {
	if msg.Payload == nil {
		return -1, nil
	}
	jsonMsg, err := msg.Payload.MarshalJSON()
	if err != nil {
		return -1, fmt.Errorf("Error marshaling payload of message to get type: %s", err)
	}

	t := Type{}
	if err := json.Unmarshal(jsonMsg, &t); err != nil {
		fmt.Println(err)
		return -1, err
	}

	if t.Type == nil {
		return -1, fmt.Errorf("Type was not set in payload")
	}
	return *t.Type, nil
}

type MilestoneResponseExplorer struct {
	Milestone iotago.MilestoneResponse `json:"milestone"`
}
type MessageResponseExplorer struct {
	Message iotago.Message `json:"message"`
}
type MetadataResponseExplorer struct {
	Metadata iotago.MessageMetadataResponse `json:"metadata"`
	Children []string                       `json:"childrenMessageIds"`
}

func getDataExplorer(url string, isMilestone bool, isMsg bool, isMetadata bool) (*iotago.MilestoneResponse, *iotago.Message, *iotago.MessageMetadataResponse, error) {

	//fmt.Println("Getting data from explorer with GET req: " + url)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		err := fmt.Errorf("ERROR creating http request: " + err.Error())
		return nil, nil, nil, err
	}
	req.Close = true

	t := &http.Transport{
		MaxIdleConns:       20,
        MaxIdleConnsPerHost:  20,
		MaxConnsPerHost: 50,
		Dial: (&net.Dialer{
			Timeout:   60 * time.Second,
			KeepAlive: 30 * time.Second,
		}).Dial,
		// We use ABSURDLY large keys, and should probably not.
		TLSHandshakeTimeout: 60 * time.Second,
	}
	client := &http.Client{
		Transport: t,
	}
	

	req.Header.Add("accept", "*/*")
	//req.Header.Add("accept-encoding", "gzip, deflate, br")
	req.Header.Add("content-type", "application/json")
	req.Header.Add("origin", " https://explorer.iota.org")
	req.Header.Add("referer", "https://explorer.iota.org/")
	req.Host = "explorer-api.iota.org"
	req.Header.Add("user-agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/16.1 Safari/605.1.15")

	//resp, err := http.DefaultClient.Do(req)
	resp, err := client.Do(req)

	if err != nil {
		err := fmt.Errorf("ERROR executing http request to the explorer, for url: %s, error: %s ", url, err.Error())
		return nil, nil, nil, err
	}
	if resp.Status != "200 OK" {
		err := fmt.Errorf("ERROR executing http request to the explorer, got status: %d, for url: %s", resp.StatusCode, url)
		respD, _:= httputil.DumpRequest(req, true)
		fmt.Println("Status: " +  resp.Status + ", request:" +  string(respD)) 
		log.Println("Status: " +  resp.Status + ", request:" +  string(respD))
		return nil, nil, nil, err
	}
	
	defer resp.Body.Close()
	
	//respDump, err := httputil.DumpResponse(resp, true)
	// if err != nil {
	// 	err := fmt.Errorf("ERROR dumping http response to the explorer: " + err.Error())
	// 	return nil, nil, nil, err
	// }

	// fmt.Printf("RESPONSE:\n%s", string(respDump))
	// fmt.Println(" ------------------ ")
	// fmt.Println(" ------------------ ")
	//fmt.Println("Got message: " + url)

	// var jsonRes map[string]interface{}

	// resBody, err := ioutil.ReadAll(resp.Body)
	// if err != nil {
	// 	err := fmt.Errorf("ERROR reading http response to the explorer: " + err.Error())
	// 	return nil, nil, err
	// }
	// err = json.Unmarshal(resBody, &jsonRes)
	// if err != nil {
	// 	err := fmt.Errorf("ERROR unmarshaling http response to the explorer: " + err.Error())
	// 	return nil, nil, err
	// }

	// fmt.Println("JSON RES : ", jsonRes)

	if isMilestone {
		var milestoneRespExplorer MilestoneResponseExplorer
		err = json.NewDecoder(resp.Body).Decode(&milestoneRespExplorer)
		if err != nil {
			err := fmt.Errorf("ERROR decoding JSON milestone http response for url: %s, error: %s", url, err.Error())
			return nil, nil, nil, err
		}
		return &milestoneRespExplorer.Milestone, nil, nil, nil
	}
	if isMsg {
		var messageRespExplorer MessageResponseExplorer
		err = json.NewDecoder(resp.Body).Decode(&messageRespExplorer)
		if err != nil {
			err := fmt.Errorf("ERROR decoding JSON message http response for url: %s, error: %s", url, err.Error())
			return nil, nil, nil, err
		}
		return nil, &messageRespExplorer.Message, nil, nil
	}
	if isMetadata {
		var metadataRespExplorer MetadataResponseExplorer
		err = json.NewDecoder(resp.Body).Decode(&metadataRespExplorer)
		if err != nil {
			err := fmt.Errorf("ERROR decoding JSON metadata http response for query url: %s, error: %s", url, err.Error())
			return nil, nil, nil, err
		}
		return nil, nil, &metadataRespExplorer.Metadata, nil
	}

	return nil, nil, nil, fmt.Errorf("Error getting data from explorer, is not metadata, message or milestone")

}

func getMessagesFromExplorerParallel(isStart bool, block *Block, msgID string, explorerURL string, ch chan error, wg *sync.WaitGroup) {

	messageURL := fmt.Sprint(explorerURL, "search/mainnet/")
	messageURL = fmt.Sprint(messageURL, msgID)
	_, message, _, err := getDataExplorer(messageURL, false, true, false)
	if err != nil {
		fmt.Println("Error getting the message data: ", err)
		ch <- err
		wg.Done()
		return
	}
	if message == nil {
		ch <- fmt.Errorf("Message was nil")
		wg.Done()
		return
	}
	if isStart == false {
		ty, err := getType(message) // For the moment, if a message payload is nil I won't process it or store it
		if err == nil && ty == 1 {  // Type = 1 is a milestone
			//fmt.Println("Found a new milestone! Stopping here...")
			ch <- nil
			wg.Done()
			return
		}
		if err != nil {
			ch <- err
			wg.Done()
			return
		}

		messageMetadataURL := fmt.Sprint(explorerURL, "message/mainnet/")
		messageMetadataURL = fmt.Sprint(messageMetadataURL, msgID)
		_, _, msgMetadata, err := getDataExplorer(messageMetadataURL, false, false, true)
		if err != nil {
			fmt.Println("Error getting the message metadata: ", err)
			ch <- err
			wg.Done()
			return
		}
		if msgMetadata == nil {
			fmt.Println("Error getting the message metadata, it is nil, url: ", messageMetadataURL)
			ch <- fmt.Errorf("Error getting the message metadata, it is nil, url: %s ", messageMetadataURL)
			wg.Done()
			return
		}

		referencedMilestoneIndex := msgMetadata.ReferencedByMilestoneIndex
		if referencedMilestoneIndex == nil {
			ch <- fmt.Errorf("Referenced milestone index of message is nil")
			wg.Done()
			return
		}
		//fmt.Println("Referenced milestone index: ", *referencedMilestoneIndex)
		//fmt.Println("Block milestone index: ", block.MilestoneIndex)
		if *referencedMilestoneIndex != block.MilestoneIndex {
			ch <- nil
			wg.Done()
			return
		}
		msgData := MessageData{MessageID: msgID, Message: *message}
		block.Messages = append(block.Messages, msgData) // The message corresponding to the initial milestone won't be stored, just it's id and index and part of the block
	}

	ch2 := make(chan error, len(message.Parents))
	wg2 := sync.WaitGroup{}

	for _, parentMessageID := range message.Parents { // Recursively look in parallel into the parents of the message
		wg2.Add(1)
		//fmt.Println("Looking into parent: ", parentMessageID)
		go getMessagesFromExplorerParallel(false, block, hex.EncodeToString(parentMessageID[:]), explorerURL, ch2, &wg2)
	}

	wg2.Wait()
	close(ch2)

	for err := range ch2 {
		if err != nil {
			fmt.Println("received err from channel child: " +  err.Error())
			log.Println("received err from channel child: " +  err.Error())
			wg.Done()
			return
			//ch <- err // TODO: test if this cause recursion to not stop when errors happen
		}
	}
	wg.Done()
	return
}

func getMessagesFromMilestoneParallel(isStart bool, block *Block, msgID iotago.MessageID, client *iotago.NodeHTTPAPIClient, ctx context.Context, ch chan error, wg *sync.WaitGroup) {

	message, err := client.MessageByMessageID(ctx, msgID)
	if err != nil {
		ch <- fmt.Errorf("Error getting the message from its ID: ", err.Error())
		wg.Done()
		return
	}
	if message == nil {
		ch <- fmt.Errorf("Message was nil")
		wg.Done()
		return
	}

	if isStart == false {
		ty, err := getType(message) // For the moment, if a message payload is nil I won't process it or store it
		if err == nil && ty == 1 {  // Type = 1 is a milestone
			//fmt.Println("Found a new milestone! Stopping here...")
			ch <- nil
			wg.Done()
			return
		}
		if err != nil {
			ch <- err
			wg.Done()
			return
		}

		msgMetadata, err := client.MessageMetadataByMessageID(ctx, msgID)
		if err != nil {
			ch <- fmt.Errorf("Error getting the message metadata: ", err.Error())
			wg.Done()
			return
		}

		referencedMilestoneIndex := msgMetadata.ReferencedByMilestoneIndex
		if referencedMilestoneIndex == nil {
			ch <- fmt.Errorf("Referenced milestone index of message is nil")
			wg.Done()
			return
		}
		//fmt.Println("Referenced milestone index: ", *referencedMilestoneIndex)
		//fmt.Println("Block milestone index: ", block.MilestoneIndex)
		if *referencedMilestoneIndex != block.MilestoneIndex {
			ch <- nil
			wg.Done()
			return
		}
		msgData := MessageData{MessageID: hex.EncodeToString(msgID[:]), Message: *message}
		block.Messages = append(block.Messages, msgData) // The message corresponding to the initial milestone won't be stored, just it's id and index and part of the block
	}

	ch2 := make(chan error, len(message.Parents))
	wg2 := sync.WaitGroup{}

	for _, parentMessageID := range message.Parents { // Recursively look in parallel into the parents of the message
		wg2.Add(1)
		//fmt.Println("Looking into parent: ", parentMessageID)
		go getMessagesFromMilestoneParallel(false, block, parentMessageID, client, ctx, ch2, &wg2)
	}

	wg2.Wait()
	close(ch2)

	for err := range ch2 {
		if err != nil {
			fmt.Println("received err from channel child: " +  err.Error())
			log.Println("received err from challe child: " + err.Error())
			//ch <- err // TODO: test if this cause recursion to not stop when errors happen
		}
	}

	wg.Done()
	return
}

func (i *Iota) makeRequest(client *http.Client, blockNumber uint64) (*http.Response, error) {
	return i.getBlock(blockNumber)
}

func (i *Iota) FetchData(filepath string, start, end uint64) error {
	context := fetcher.NewHTTPContext(start, end, i.makeRequest)
	return fetcher.FetchHTTPData(filepath, context)
}

type Block struct {
	BlockNumber        uint64        `json:"block_number"` // For now, block number corresponds to a milestone index, so it will be the same as the filed MilestoneIndex
	MilestoneID        string        `json:"milestone_id"`
	MilestoneIndex     uint32        `json:"milestone_index"`
	Messages           []MessageData `json:"messages"`            // The messages part of this block, which are confirmed by this block's milestone
	MilestomeTimestamp int64         `json:"milestone_timestamp"` 
	BlockTimestamp     time.Time     `json:"block_timestamp"`     // TODO: should this be the milestone timestamp ?
	MessagesCount      int           `json:"messages_count"`
	IsEmptyBlock       bool          `json:"is_empty"`
	actions            []core.Action
}

type MessageData struct {
	Message     iotago.Message `json:"message"`
	MessageID   string         `json:"message_id"`
	NameMsg     string         // Adding these fields as required by the interface, but not used in IOTA
	ReceiverMsg string
	SenderMsg   string
}

func (i *Iota) ParseBlock(rawLine []byte) (core.Block, error) { // TODO: yet to implement

	var block Block
	// if err := json.Unmarshal(rawLine, &block); err != nil {
	// 	fmt.Println(err)
	// 	return nil, err
	// }

	// parsedTime, err := h.ConvertDecimalTimestampToTime(block.Timestamp)
	// if err != nil {
	// 	return nil, err
	// }
	// block.BlockTimestamp = parsedTime
	// // if len (block.Transactions) > 0 {
	// // 	for i :=0; i < len (block.Transactions); i++ {
	// // 		if block.Transactions[i].Type == "poc_receipts_v1" || block.Transactions[i].Type == "poc_receipts_v2" {
	// // 			if len(block.Transactions[i].WitnessDataPath[0].WitnessList) == 0 {
	// // 				fmt.Println(block.Transactions[i].WitnessDataPath[0].ChallengeeGeoInfo.Country)
	// // 			}
	// // 		}
	// // 	}
	// // }
	// if len(block.Transactions) == 0 {
	// 	block.IsEmptyBlock = true
	// }

	return &block, nil
}

func (i *Iota) ConvertDecimalTimestampToTime(timeStamp uint64) (time.Time, error) {
	// convert UNIX decimal timestamp to
	unixTimeUTC := time.Unix(int64(timeStamp), 0)
	parsedTimeString := unixTimeUTC.Format(time.RFC3339)
	// convert from string to time.Time format
	parsedTime, err := time.Parse(time.RFC3339, parsedTimeString)
	return parsedTime, err
}

func (i *Iota) EmptyBlock() core.Block {
	return &Block{}
}

func (b *Block) Number() uint64 {
	return b.BlockNumber
}

func (b *Block) Time() time.Time {
	return b.BlockTimestamp
}

func (b *Block) TransactionsCount() int {
	return int(b.MessagesCount)
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
	// txCounter := 0
	// for _, txn := range b.Transactions {
	// 	if by == txn.Kind {
	// 		txCounter = txCounter + 1
	// 	}
	// }
	// return txCounter
	return -1
}

func (b *Block) EmptyBlocksCount() int {
	if b.IsEmptyBlock {
		return 0
	}
	return 1
	
}

// TO-DO
func (b *Block) ZeroTxnBlocksCount() int {
	// // not yet implemented
	return 0
}

// TO-DO
func (b *Block) GetMiner() string {
	// // not yet implemented
	return ""
}

func (b *Block) ListActions() []core.Action {
	if len(b.actions) > 0 {
		return b.actions
	}
	var result []core.Action
	for _, msg := range b.Messages {
		result = append(result, msg)
	}
	b.actions = result
	return result
}

func (m MessageData) Name() string {
	return m.NameMsg
}

func (m MessageData) Receiver() string {
	return m.ReceiverMsg
}

func (m MessageData) Sender() string {
	return m.SenderMsg
}
