package iota

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"log"
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
var globalSet *SafeSet // This variable is used to prevent duplicate messages from being stored
const defaultNodeEndpoint string = "https://chrysalis-nodes.iota.cafe:443" //"http://localhost:80"// "https://chrysalis-nodes.iota.cafe:443"//"https://chrysalis-nodes.iota.cafe:443" //"https://iota-node.tanglebay.com" //

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

type SafeSet struct {
	mu sync.Mutex
	m  map[string]bool
}

func NewSafeSet() *SafeSet {
	return &SafeSet{m: make(map[string]bool)}
}

func (s *SafeSet) Add(item string) {
	s.mu.Lock()
	s.m[item] = true
	s.mu.Unlock()
}

func (s *SafeSet) Contains(item string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.m[item]
	return ok
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

	globalSet = NewSafeSet()

	if blockNumber < uint64(pruningIndex) {
		err = fmt.Errorf("ERROR, cannot fetch block with milestone index: %v, pruning index is: %v, block number has to be bigger than pruning index ", blockNumber, pruningIndex)
		resp.StatusCode = 500
		return resp, err
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
		block.MilestoneTimestamp = milestoneResponse.Time

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

	removeDuplicates(block)

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

// Types used to deserialize data from extracted messages
type Type struct {
	Type *int `json:"type"`
}

type Index struct {
	Index *string `json:"index"`
}

type TransactionInput struct {
	Type 			*int 		`json:"type"`
	TransactionID 	*string 	`json:"transactionId"`
	TransactionOutputIndex *int	`json:"transactionOutputIndex"`
}

type TransactionAddress struct {
	Type 			*int 		`json:"type"`
	Address 		*string 	`json:"address"`
}

type TransactionOuput struct {
	Type 			*int 				`json:"type"`
	Address 		*TransactionAddress `json:"address"`
	Amount 			*int 				`json:"amount"`
}

type TransactionPayload struct {
	Type 			*int 				`json:"type"`
	Index			*string				`json:"index"`
	Data			*string				`json:"data"`
}

type TransactionEssence struct {
	Type 			*int 				`json:"type"`
	Inputs 			*[]TransactionInput `json:"inputs"`
	Outputs 		*[]TransactionOuput `json:"outputs"`
	Payload 		*TransactionPayload `json:"payload"`
}

type Transaction struct {
	Essence *TransactionEssence `json:"essence"`
}

func getType(msg *iotago.Message) (int, error) {
	if msg.Payload == nil {
		return -1, nil
	}
	jsonMsg, err := msg.Payload.MarshalJSON()
	if err != nil {
		return -10, fmt.Errorf("Error marshaling payload of message to get type: %s", err)
	}

	t := Type{}
	if err := json.Unmarshal(jsonMsg, &t); err != nil {
		return -10, fmt.Errorf("Error unmarsahling json message to get type: %s", err)
	}

	if t.Type == nil {
		return -10, fmt.Errorf("Type was not set in payload")
	}
	return *t.Type, nil
}

// Gets the index of a message with an indexation payload
func getIndex(msg *iotago.Message) (string, error) {
	if msg.Payload == nil {
		return "", nil
	}
	jsonMsg, err := msg.Payload.MarshalJSON()
	if err != nil {
		return "", fmt.Errorf("Error marshaling payload of message to get index: %s", err)
	}

	indx := Index{}
	if err := json.Unmarshal(jsonMsg, &indx); err != nil {
		return "", fmt.Errorf("Error unmarsahling json message to get index: %s", err)
	}

	if indx.Index == nil {
		return "", fmt.Errorf("Index was not set in payload")
	}

	bs, err := hex.DecodeString(*indx.Index)
	if err != nil {
		return "", fmt.Errorf("Error decoding hex string to string to get index from payload: %s", err)
	}
	return string(bs), nil
}

// Gets the output addresses, values and the index from a message containing a signed transaction payload
func getInfoTransaction(msg *iotago.Message) (*[]string, *[]int, *string, error) {

	if msg.Payload == nil {
		return nil, nil, nil, fmt.Errorf("Error getting output addresses, payload is nil")
	}
	jsonMsg, err := msg.Payload.MarshalJSON()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("Error marshaling payload of message to get addresses: %s", err)
	}

	trnx := Transaction{}
	if err := json.Unmarshal(jsonMsg, &trnx); err != nil {
		return nil, nil, nil, fmt.Errorf("Error unmarsahling json message to get outputs: %s", err)
	}

	if trnx.Essence == nil {
		return nil, nil, nil, fmt.Errorf("Transaction essence is nil")
	}
	if trnx.Essence.Outputs == nil {
		return nil, nil, nil, fmt.Errorf("Transaction essence outputs is nil")
	}

	listOutputAddresses := []string{}
	listValues := []int{}

	for _, out := range *trnx.Essence.Outputs {
		if out.Address != nil && out.Address.Address != nil {
			listOutputAddresses = append(listOutputAddresses, *out.Address.Address)
			if out.Amount != nil {
				listValues = append(listValues, *out.Amount)
			}
		}
	}

	var indxStr *string = nil
	if trnx.Essence.Payload != nil && trnx.Essence.Payload.Index != nil {
		indxStr = trnx.Essence.Payload.Index
	}

	return &listOutputAddresses, &listValues, indxStr, nil
}

func getMessagesFromMilestoneParallel(isStart bool, block *Block, msgID iotago.MessageID, client *iotago.NodeHTTPAPIClient, ctx context.Context, ch chan error, wg *sync.WaitGroup) {

	globalSet.Add(hex.EncodeToString(msgID[:]))

	message, err := client.MessageByMessageID(ctx, msgID)
	if err != nil {
		ch <- fmt.Errorf("Error getting the message from its ID: %s", err.Error())
		wg.Done()
		return
	}
	if message == nil {
		ch <- fmt.Errorf("Message was nil")
		wg.Done()
		return
	}

	if isStart == false {
		ty, err := getType(message)
		if err == nil && ty == 1 { // Type = 1 is a milestone
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
			ch <- fmt.Errorf("Error getting the message metadata: %s", err.Error())
			wg.Done()
			return
		}
		referencedMilestoneIndex := msgMetadata.ReferencedByMilestoneIndex
		if referencedMilestoneIndex == nil {
			ch <- fmt.Errorf("Referenced milestone index of message is nil")
			wg.Done()
			return
		}
		if *referencedMilestoneIndex != block.MilestoneIndex {
			ch <- nil
			wg.Done()
			return
		}

		msgData := MessageData{
			MessageID:            hex.EncodeToString(msgID[:]),
			Message:              *message,
			IsSolid:              msgMetadata.Solid,
			LedgerInclusionState: msgMetadata.LedgerInclusionState,
			ConflictReason:       msgMetadata.ConflictReason,
			ShouldPromote:        msgMetadata.ShouldPromote,
			ShouldReattach:       msgMetadata.ShouldReattach,
		}

		block.Messages = append(block.Messages, msgData) // The message corresponding to the initial milestone won't be stored, just it's id and index and part of the block
	}

	ch2 := make(chan error, len(message.Parents))
	wg2 := sync.WaitGroup{}

	for _, parentMessageID := range message.Parents { // Recursively look in parallel into the parents of the message

		if globalSet.Contains(hex.EncodeToString(parentMessageID[:])) {
			ch2 <- nil
			continue
		}
		wg2.Add(1)
		go getMessagesFromMilestoneParallel(false, block, parentMessageID, client, ctx, ch2, &wg2)
	}

	wg2.Wait()
	close(ch2)

	for err := range ch2 {
		if err != nil {
			fmt.Println("received err from channel child: " + err.Error())
			log.Println("received err from challe child: " + err.Error())
			//ch <- err // TODO: test if this cause recursion to not stop when errors happen
		}
	}

	wg.Done()
	return
}

func removeDuplicates(block *Block) {
	key := blockKey{}
	newArr := []MessageData{}
	seen := make(map[blockKey]bool, len(block.Messages))
	for i, p := range block.Messages {
		key.MessageID = p.MessageID
		if seen[key] {
			//block.Messages = append(block.Messages[:i], block.Messages[i+1:]...)
		} else {
			seen[key] = true
			newArr = append(newArr, block.Messages[i])
		}
	}
	block.Messages = newArr
}

func (i *Iota) makeRequest(client *http.Client, blockNumber uint64) (*http.Response, error) {
	return i.getBlock(blockNumber)
}

func (i *Iota) FetchData(filepath string, start, end uint64) error {
	context := fetcher.NewHTTPContext(start, end, i.makeRequest)
	return fetcher.FetchHTTPData(filepath, context)
}

type Block struct {
	IsEmptyBlock       	bool   `json:"is_empty"`
	MilestoneIndex     	uint32 `json:"milestone_index"`
	BlockNumber        	uint64 `json:"block_number"` // Block number corresponds to a milestone index
	MilestoneTimestamp 	int64  `json:"milestone_timestamp"`
	MessagesCount      	int    `json:"messages_count"`
	IndxPayldCnt       	int64
	SgnedTxnPayldCnt   	int64
	MilstnPayldCnt     	int64
	NoPaylodCnt        	int64
	OtherPayloadCnt		int64
	ConflictsCnt 		int64
	NoSolidCnt			int64
	AverageValuesSpent	float64
	ValuesSpent 		[]int
	MilestoneID        	string        `json:"milestone_id"`
	BlockTimestamp     	time.Time     `json:"block_timestamp"` // This field is not used, the MilestoneTimestamp is the relevant one
	Messages           	[]MessageData `json:"messages"` // The messages part of this block, which are confirmed by this block's milestone
	actions            	[]core.Action	// Not used in IOTA
	GroupedConflicts 	map[int]int
	GroupedIndexes 		map[string]int
	GroupedTransactionIndexes 		map[string]int
	GroupedAddresses 	map[string]int
}

type blockKey struct {
	MessageID string
}

type MessageData struct {
	ShouldPromote        *bool          `json:"shouldPromote,omitempty"`
	ShouldReattach       *bool          `json:"shouldReattach,omitempty"`
	LedgerInclusionState *string        `json:"ledgerInclusionState"`
	IsSolid              bool           `json:"isSolid"`
	ConflictReason       uint8          `json:"conflictReason,omitempty"`
	MessageID            string         `json:"message_id"`
	NameMsg              string         // Adding this field as required by the interface, but not used in IOTA
	ReceiverMsg          string			// Adding this field as required by the interface, but not used in IOTA
	SenderMsg            string			// Adding this field as required by the interface, but not used in IOTA
	Message              iotago.Message `json:"message"`
}

func (i *Iota) ParseBlock(rawLine []byte) (core.Block, error) {

	var block Block
	if err := json.Unmarshal(rawLine, &block); err != nil {
		fmt.Println("Error unmarshaling block inside ParseBlock for IOTA block: ", err.Error())
		return nil, err
	}

	block.SgnedTxnPayldCnt = 0
	block.MilstnPayldCnt = 0
	block.IndxPayldCnt = 0
	block.NoPaylodCnt = 0
	block.ConflictsCnt = 0
	block.OtherPayloadCnt = 0
	block.NoSolidCnt = 0
	block.BlockTimestamp = time.Unix(block.MilestoneTimestamp, 0)
	block.GroupedConflicts =  make(map[int]int)
	block.GroupedIndexes =  make(map[string]int)
	block.GroupedTransactionIndexes = make(map[string]int)
	block.GroupedAddresses = make(map[string]int)
	block.ValuesSpent = []int{}

	if len(block.Messages) > 0 {
		for _, message := range block.Messages {
			ty, err := getType(&message.Message)
			if err != nil {
				fmt.Println(err)
				return nil, err
			}
			switch ty {
			case 0:
				block.SgnedTxnPayldCnt += 1
				break
			case 1:
				block.MilstnPayldCnt += 1
				break
			case 2:
				block.IndxPayldCnt += 1
				break
			case -1:
				block.NoPaylodCnt += 1
				break
			default:
				block.OtherPayloadCnt += 1
			}
			if !message.IsSolid {
				block.NoSolidCnt += 1
			}

			if message.LedgerInclusionState != nil && *message.LedgerInclusionState == "conflicting" {
				block.ConflictsCnt += 1
				conflictReasn := int(message.ConflictReason)
				if _, ok := block.GroupedConflicts[conflictReasn]; !ok {
					block.GroupedConflicts[conflictReasn] = 0
				}
				block.GroupedConflicts[conflictReasn] += 1
			}
			if ty == 2 {
				indx, err := getIndex(&message.Message)
				if err != nil {
					fmt.Println("Error occured getting index: ", err)
					continue
				}

				if _, ok := block.GroupedIndexes[indx]; !ok {
					block.GroupedIndexes[indx] = 0
				}
				block.GroupedIndexes[indx] += 1

			} else if ty == 0 {
				if message.LedgerInclusionState != nil && *message.LedgerInclusionState == "included" {

					outputAddrs, values, indxStr, err := getInfoTransaction(&message.Message)
					if err != nil {
						fmt.Println("Error occured getting transaction info: ", err)
						continue
					}
					if indxStr != nil {

						bs, err := hex.DecodeString(*indxStr)
						if err != nil {
							fmt.Println("Error decoding hex string to string to get index from transaction payload: ", err)
						}
						decodedIndex := string(bs)

						if _, ok := block.GroupedTransactionIndexes[decodedIndex]; !ok {
							block.GroupedTransactionIndexes[decodedIndex] = 0
						}
						block.GroupedTransactionIndexes[decodedIndex] += 1
					}

					if values != nil && len(*values) > 0 {
						block.ValuesSpent = append(block.ValuesSpent, *values...)
					}
					for _, addr := range *outputAddrs {
						if _, ok := block.GroupedAddresses[addr]; !ok {
							block.GroupedAddresses[addr] = 0
						}
						block.GroupedAddresses[addr] += 1
					}
				}
			}
		
		}
		sum := 0
		for _, value := range block.ValuesSpent {
			sum += value
		}
		block.AverageValuesSpent = float64(sum) / float64(len(block.ValuesSpent))
	}

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

func (b *Block) IndexationPayloadCount() int {
	return int(b.IndxPayldCnt)
}

func (b *Block) SignedTransactionPayloadCount() int {
	return int(b.SgnedTxnPayldCnt)
}

func (b *Block) NoPayloadCount() int {
	return int(b.NoPaylodCnt)
}

func (b *Block) OtherPayloadCount() int {
	return int(b.OtherPayloadCnt)
}

func (b *Block) NoSolidCount() int {
	return int(b.NoSolidCnt)
}

func (b *Block) ConflictsCount() int {
	return int(b.ConflictsCnt)
}

func (b *Block) GetGroupedConflicts() *map[int]int {
	if len(b.GroupedConflicts) == 0 {
		return nil
	}
	return &b.GroupedConflicts
}

func (b *Block) GetGroupedIndexes() *map[string]int {
	if len(b.GroupedIndexes) == 0 {
		return nil
	}
	return &b.GroupedIndexes
}

func (b *Block) GetGroupedAddresses() *map[string]int {
	if len(b.GroupedAddresses) == 0 {
		return nil
	}
	return &b.GroupedAddresses
}

func (b *Block) GetValuesSpent() *[]int {
	return &b.ValuesSpent
}

func (b *Block) EmptyBlocksCount() int {
	if b.IsEmptyBlock {
		return 1
	}
	return 0
}

func (b *Block) GetGroupedIndexesTransactions() *map[string]int {
	if len(b.GroupedTransactionIndexes ) == 0 {
		return nil
	}
	return &b.GroupedTransactionIndexes 
}

func (i *Iota) EmptyBlock() core.Block {
	return &Block{}
}

func (b *Block) Number() uint64 {
	return b.BlockNumber
}

func (b *Block) Time() time.Time {
	tm := time.Unix(b.MilestoneTimestamp, 0)
	return tm
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
