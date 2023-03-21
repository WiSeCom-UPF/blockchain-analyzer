package iota

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/danhper/blockchain-analyzer/core"
	"github.com/danhper/blockchain-analyzer/fetcher"

	iotago "github.com/iotaledger/iota.go/v2"
	jsoniter "github.com/json-iterator/go"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

const defaultNodeEndpoint string = "https://chrysalis-nodes.iota.cafe:443"

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

	resp := &http.Response{
		StatusCode:    200,
		Proto:         "HTTP/1.1",
		ProtoMajor:    1,
		ProtoMinor:    1,
		Header:        make(http.Header, 0),
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
		fmt.Println("Error getting info from node: ", err)
		resp.StatusCode = 500
		fmt.Println(info)
		return resp, err
	}
	pruningIndex := info.PruningIndex

	if blockNumber < uint64(pruningIndex) { // TODO: not sure works well, not sure block number has to be bigger
		err = fmt.Errorf("ERROR, cannot fetch block with milestone index: %v, pruning index is: %v, block number has to be bigger than pruning index ", blockNumber, pruningIndex)
		//fmt.Println(err)
		resp.StatusCode = 500
		return resp, err
	}

	milestoneIndex := uint32(blockNumber) // The blockNumber passed must be a Milestone index, the messages confirmed by it will be stored
	          
	milestoneResponse, err := nodeHTTPAPIClient.MilestoneByIndex(ctx, milestoneIndex)
	if err != nil {
		resp.StatusCode = 500
		fmt.Println(err)
		return resp, err
	}

	messageIDmilestone, err := iotago.MessageIDFromHexString(milestoneResponse.MessageID)
	if err != nil {
		resp.StatusCode = 500
		fmt.Println(err)
		return resp, err
	}

	miletsoneMessage, err := nodeHTTPAPIClient.MessageByMessageID(ctx, messageIDmilestone)
	if err != nil {
		resp.StatusCode = 500
		fmt.Println(err)
		return resp, err
	}

	block := &Block{
		BlockNumber: uint64(milestoneIndex),
		MilestoneID: hex.EncodeToString(messageIDmilestone[:]),
		MilestoneIndex: milestoneIndex,
	}

	ch := make(chan error, len(miletsoneMessage.Parents))
	wg := sync.WaitGroup{}
	wg.Add(1)

	go getMessagesFromMilestoneParallel(true, block, messageIDmilestone, nodeHTTPAPIClient, ctx, ch, &wg)

	wg.Wait()
	close(ch)

	fmt.Println("Done fetching all messages for block with milestone index: ", milestoneIndex)

	for err := range ch {
		if err != nil {
			resp.StatusCode = 500
			fmt.Println("Error getting the block: ", err)
			return resp, err
		}
	}

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
		return resp, err
	}
	if content == nil {
		resp.StatusCode = 500
		err := fmt.Errorf("Content of marshalled block is nil")
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
		return -1, fmt.Errorf("Cannot get type as payload is nil")
	}
	jsonMsg, err := msg.Payload.MarshalJSON()
	if err != nil {
		fmt.Println("Error marshaling payload of message to get type", err)
		return -1, err
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

func getMessagesFromMilestoneParallel(isStart bool, block *Block, msgID iotago.MessageID, client *iotago.NodeHTTPAPIClient, ctx context.Context, ch chan error, wg *sync.WaitGroup) {

	message, err := client.MessageByMessageID(ctx, msgID)
	if err != nil {
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

		msgMetadata, err := client.MessageMetadataByMessageID(ctx, msgID)
		if err != nil {
			fmt.Println("Error getting the message metadata: ", err) 
			ch <- err
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
			fmt.Println("received err from channel child")
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
	BlockNumber        uint64           `json:"block_number"` 		// For now, block number corresponds to a milestone index, so it will be the same as the filed MilestoneIndex
	MilestoneID        string 			`json:"milestone_id"`
	MilestoneIndex     uint32           `json:"milestone_index"`     
	Messages           []MessageData    `json:"messages"`            // The messages part of this block, which are confirmed by this block's milestone
	MilestomeTimestamp uint64           `json:"milestone_timestamp"` // TODO: The time at which this milestone was issued ?
	BlockTimestamp     time.Time        `json:"block_timestamp"`     // TODO: should this be the milestone timestamp ?
	MessagesCount      int           	`json:"messages_count"`
	IsEmptyBlock       bool             `json:"is_empty"`
	actions            []core.Action
}

type MessageData struct {
	Message     iotago.Message 		`json:"message"`
	MessageID 	string				`json:"message_id"`
	NameMsg     string				// Adding these fields as required by the interface, but not used in IOTA
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
	} else {
		return 1
	}
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