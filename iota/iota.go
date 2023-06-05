package iota

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/binary"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"sync"
	"time"
	"strconv"

	"github.com/danhper/blockchain-analyzer/core"
	"github.com/danhper/blockchain-analyzer/fetcher"
	"github.com/iotaledger/inx-api-core-v1/pkg/hornet"
	"github.com/iotaledger/inx-api-core-v1/pkg/milestone"
	"github.com/iotaledger/hive.go/serializer"

	iotago "github.com/iotaledger/iota.go/v2"
	jsoniter "github.com/json-iterator/go"
	"github.com/linxGnu/grocksdb"
	//"github.com/iotaledger/hive.go/serializer/v2/marshalutil"
)

// SafeSet struct allows for safe concurrent access to a map m, avoiding race conditions.
type SafeSet struct {
    m map[string]bool
    mu sync.Mutex
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

var globalSet *SafeSet
var json = jsoniter.ConfigCompatibleWithStandardLibrary
const defaultNodeEndpoint string = "https://chrysalis-nodes.iota.cafe:443"
const defaultDatabasePath string = "/mnt/HC_Volume_32053780/hornet-2021-05_19979_283822/db/tangle"
const defaultUseDB bool = true

type Iota struct {
	UseDB 			bool
	RocksDB 		*grocksdb.DB
	NodeEndpoint 	string
	DatabasePath 	string
}

func New() *Iota {
	nodeEndpoint := os.Getenv("Iota_NODE_ENDPOINT")
	if nodeEndpoint == "" {
		nodeEndpoint = defaultNodeEndpoint
	}

	dbPath := os.Getenv("Iota_ROCKSDB_PATH")
	if dbPath == "" {
		dbPath = defaultDatabasePath
	}

	useDB := os.Getenv("Iota_USE_DATABASE")
	valueUseDB, err := strconv.ParseBool(useDB)
	if err != nil {
		valueUseDB = defaultUseDB
	}

	iotaObj := &Iota{
		NodeEndpoint: nodeEndpoint,
		DatabasePath: dbPath,
		UseDB: valueUseDB,
	}

	if valueUseDB { // Create instance of RocksDB
		bbto := grocksdb.NewDefaultBlockBasedTableOptions()
		bbto.SetBlockCache(grocksdb.NewLRUCache(3 << 30))

		opts := grocksdb.NewDefaultOptions()
		opts.SetBlockBasedTableFactory(bbto)
		opts.SetCreateIfMissing(true)

		rocksDB, err := grocksdb.OpenDb(opts, dbPath)
		if err != nil {
			fmt.Println("error opnening rocksdb: ", err)
			return nil
		}
		if rocksDB == nil {
			fmt.Println("Error opening rocksdb: it is nil")
			return nil
		}
		iotaObj.RocksDB = rocksDB
		//defer iotaObj.RocksDB.Close()
	}
	return iotaObj
}

type Milestone struct {
	Index     milestone.Index
	MessageID hornet.MessageID
	Timestamp time.Time
}

// Computes the RocksDB key for a milestone index
func databaseKeyForMilestoneIndex(milestoneIndex uint32) []byte {
	bytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(bytes, milestoneIndex)
	return bytes
}

// Computes a milestone index from the RocksDB key
func milestoneIndexFromDatabaseKey(key []byte) milestone.Index {
	return milestone.Index(binary.LittleEndian.Uint32(key))
}

// Creates a milestone from the bytes obtained from the database
func milestoneFactory(key []byte, data []byte) *Milestone {
	return &Milestone{
		Index:     milestoneIndexFromDatabaseKey(key),
		MessageID: hornet.MessageIDFromSlice(data[:iotago.MessageIDLength]),
		Timestamp: time.Unix(int64(binary.LittleEndian.Uint64(data[iotago.MessageIDLength:iotago.MessageIDLength+serializer.UInt64ByteSize])), 0),
	}
}

func (i *Iota) getBlock(blockNumber uint64) (*http.Response, error) {

	resp := &http.Response{
		StatusCode: 200,
		Proto:      "HTTP/1.1",
		ProtoMajor: 1,
		ProtoMinor: 1,
		Header:     make(http.Header, 0),
	}

	globalSet = NewSafeSet()
	ctx := context.Background()
	milestoneIndex := uint32(blockNumber)
	block := &Block{}
	var pruningIndex uint64
	var milestoneTimestamp int64
	var milestoneParentsLength int
	var messageIDmilestone iotago.MessageID
	// create a new node API client 
	nodeHTTPAPIClient := iotago.NewNodeHTTPAPIClient(i.NodeEndpoint)
	if nodeHTTPAPIClient == nil {
		resp.StatusCode = 500
		return resp, fmt.Errorf("Node HTTP Client is nil")
	}

	if i.UseDB { // Fetching from a locally stored RocksDB

		kyD := databaseKeyForMilestoneIndex(uint32(milestoneIndex))
		valueMil, err := i.RocksDB.Get(grocksdb.NewDefaultReadOptions(), ConcatBytes([]byte{3}, kyD))
		if err != nil {
			resp.StatusCode = 500
			return resp, fmt.Errorf("Error getting milestone Index from database: ", err)
		}
		defer valueMil.Free()
		if valueMil.Data() == nil {
			resp.StatusCode = 500
			return resp, fmt.Errorf("Key not found in databse for milestone index %s", milestoneIndex)
		}
	
		valObjMIL := valueMil.Data()
		storedMilestone := milestoneFactory(kyD, valObjMIL)

		var byteArray [32]byte
		copy(byteArray[:], storedMilestone.MessageID)

		messageStoredMilestone, err := getMessage(byteArray, i.RocksDB)
		if err != nil {
			resp.StatusCode = 500
			return resp, fmt.Errorf("Error getting the milestone message from its ID:  %s", err)
		}
		if messageStoredMilestone == nil {
			resp.StatusCode = 500
			return resp, fmt.Errorf("Error getting the milestone message from its ID, it is nil")
		}

		messageIDmilestone = byteArray
		milestoneTimestamp = storedMilestone.Timestamp.Unix()
		milestoneParentsLength = len(messageStoredMilestone.Parents)

	} else { // Fetching via a node
		
		// fetch the node's info to know the pruning index
		info, err := nodeHTTPAPIClient.Info(ctx)
		if err != nil {
			fmt.Println("Error getting info from node: ", err)
			resp.StatusCode = 500
			return resp, err
		}
		
		pruningIndex = uint64(info.PruningIndex)

		if blockNumber < pruningIndex {
			resp.StatusCode = 500
			return resp, fmt.Errorf("ERROR, cannot fetch block with milestone index: %v, pruning index is: %v, block number has to be bigger than pruning index ", blockNumber, pruningIndex)
		} else { 
			milestoneResponse, err := nodeHTTPAPIClient.MilestoneByIndex(ctx, milestoneIndex)
			if err != nil {
				resp.StatusCode = 500
				return resp, fmt.Errorf("Error getting milestone: ", err)
			}

			messageIDmilestone, err = iotago.MessageIDFromHexString(milestoneResponse.MessageID)
			if err != nil {
				resp.StatusCode = 500
				return resp, fmt.Errorf("Error getting messageID from hex string", err)
			}

			milestoneMessage, err := nodeHTTPAPIClient.MessageByMessageID(ctx, messageIDmilestone)
			if err != nil {
				resp.StatusCode = 500
				return resp, fmt.Errorf("Error getting milestone messgae from message id", err)
			}

			milestoneTimestamp = milestoneResponse.Time
			milestoneParentsLength = len(milestoneMessage.Parents)
		}
	}

	block.BlockNumber = uint64(milestoneIndex)
	block.MilestoneID = hex.EncodeToString(messageIDmilestone[:])
	block.MilestoneIndex = milestoneIndex
	block.MilestomeTimestamp = milestoneTimestamp

	ch := make(chan error, milestoneParentsLength)
	wg := sync.WaitGroup{}
	wg.Add(1)

	if i.UseDB {
		
		if i.RocksDB == nil {
			resp.StatusCode = 500
			return resp, fmt.Errorf("Cannot fetch from RocksBD, IOTA RocksDB instance variable is nil")
		}
		go getMessagesFromMilestoneParallelDB(true, block, messageIDmilestone, i.RocksDB, ch, &wg)

	} else {
		go getMessagesFromMilestoneParallel(true, block, messageIDmilestone, nodeHTTPAPIClient, ctx, ch, &wg)
	}

	wg.Wait()
	close(ch)

	for err := range ch {
		if err != nil {
			resp.StatusCode = 500
			return resp, err
		}
	}
	
	fmt.Println("Done fetching all messages for block with milestone index: ", milestoneIndex)
	// Removing possible duplicated messages in a block
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
		return resp, fmt.Errorf("Error marhsaling block: ", err)
	}
	if content == nil {
		resp.StatusCode = 500
		return resp, fmt.Errorf("Content of marshalled block is nil")
	}

	resp.Body = ioutil.NopCloser(bytes.NewReader(content))
	resp.ContentLength = int64(len(content))
	return resp, nil
}



func getMessagesFromMilestoneParallelDB(isStart bool, block *Block, msgID iotago.MessageID, db *grocksdb.DB,  ch chan error, wg *sync.WaitGroup) {

	globalSet.Add(hex.EncodeToString(msgID[:]))

	message, err := getMessage(msgID, db)
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

		msgMetadata, err := getMessageMetadata(msgID, db)
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
			MessageID: hex.EncodeToString(msgID[:]), 
			Message: *message,
			IsSolid: msgMetadata.Solid,
			LedgerInclusionState: msgMetadata.LedgerInclusionState,
			ConflictReason: msgMetadata.ConflictReason,
		}
		
		block.Messages = append(block.Messages, msgData) // The message corresponding to the initial milestone won't be stored, just it's id and index and part of the block
	}

	ch2 := make(chan error, len(message.Parents))
	wg2 := sync.WaitGroup{}

	for _, parentMessageID := range message.Parents { // Recursively look in parallel into the parents of the message

		if globalSet.Contains(hex.EncodeToString(parentMessageID[:])) { // Means this parent has already been viisted
			ch2 <- nil
			continue
		}
		wg2.Add(1)

		go getMessagesFromMilestoneParallelDB(false, block, parentMessageID, db, ch2, &wg2)
	}

	wg2.Wait()
	close(ch2)

	for err := range ch2 {
		if err != nil {
			fmt.Println("Received err from channel child: " +  err.Error())
			//ch <- err
		}
	}

	wg.Done()
	return
}

func getMessagesFromMilestoneParallel(isStart bool, block *Block, msgID iotago.MessageID, client *iotago.NodeHTTPAPIClient, ctx context.Context, ch chan error, wg *sync.WaitGroup) {

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
			MessageID: hex.EncodeToString(msgID[:]), 
			Message: *message,
			IsSolid: msgMetadata.Solid,
			LedgerInclusionState: msgMetadata.LedgerInclusionState,
			ConflictReason: msgMetadata.ConflictReason,
			ShouldPromote: msgMetadata.ShouldPromote,
			ShouldReattach: msgMetadata.ShouldReattach,
		}
		
		block.Messages = append(block.Messages, msgData) // The message corresponding to the initial milestone won't be stored, just it's id and index and part of the block
	}

	ch2 := make(chan error, len(message.Parents))
	wg2 := sync.WaitGroup{}

	for _, parentMessageID := range message.Parents { // Recursively look in parallel into the parents of the message
		wg2.Add(1)
		go getMessagesFromMilestoneParallel(false, block, parentMessageID, client, ctx, ch2, &wg2)
	}

	wg2.Wait()
	close(ch2)

	for err := range ch2 {
		if err != nil {
			fmt.Println("received err from channel child: " +  err.Error())
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
	IsEmptyBlock       	bool          `json:"is_empty"`
	MessagesCount      	int           `json:"messages_count"`
	MilestoneIndex     	uint32        `json:"milestone_index"`
	BlockNumber        	uint64        `json:"block_number"` // block number corresponds to a milestone index
	MilestomeTimestamp 	int64         `json:"milestone_timestamp"` 
	IndxPayldCnt		int64
	SgnedTxnPayldCnt	int64
	MilstnPayldCnt		int64
	NoPaylodCnt			int64
	MilestoneID        	string        `json:"milestone_id"`
	Messages           	[]MessageData `json:"messages"`            // The messages part of this block, which are confirmed by this block's milestone
	BlockTimestamp     	time.Time     `json:"block_timestamp"`    
	actions            	[]core.Action
}

type MessageData struct {
	IsSolid 		bool			`json:"isSolid"`
	ShouldPromote 	*bool `json:"shouldPromote,omitempty"`
	ShouldReattach 	*bool `json:"shouldReattach,omitempty"`
	ConflictReason 	uint8 `json:"conflictReason,omitempty"`
	MessageID   	string         `json:"message_id"`
	LedgerInclusionState *string `json:"ledgerInclusionState"`
	NameMsg     string         // Adding these fields as required by the interface, but not used in IOTA
	ReceiverMsg string
	SenderMsg   string
	Message     	iotago.Message `json:"message"`
}

func (i *Iota) ParseBlock(rawLine []byte) (core.Block, error) { // TODO: yet to implement

	var block Block
	if err := json.Unmarshal(rawLine, &block); err != nil {
		fmt.Println("Error unmarshaling block inside ParseBlock for IOTA block: ", err.Error())
		return nil, err
	}

	block.SgnedTxnPayldCnt = 0
	block.MilstnPayldCnt = 0
	block.IndxPayldCnt = 0
	block.NoPaylodCnt = 0
	block.BlockTimestamp = time.Unix(block.MilestomeTimestamp, 0)

	if len(block.Messages) > 0 {
		for _, message := range block.Messages {
			ty, err := getType(&message.Message)
			if err != nil {
				fmt.Println(err)
				return nil, err
			}
			switch ty {
			case 0:
				block.SgnedTxnPayldCnt +=1
				break
			case 1:
				block.MilstnPayldCnt += 1
				break
			case 2:
				block.IndxPayldCnt += 1
				break
			case -1: 
				block.NoPaylodCnt += 1
			default:
				fmt.Printf("Found message with unsual payload type: %d, messageID: %s",  ty,  message.MessageID)
			}

		}
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

func (i *Iota) EmptyBlock() core.Block {
	return &Block{}
}

func (b *Block) Number() uint64 {
	return b.BlockNumber
}

func (b *Block) Time() time.Time {
	//return b.BlockTimestamp
	tm := time.Unix(b.MilestomeTimestamp, 0)
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

type blockKey struct {
	MessageID string
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