package iota

import (
	"encoding/hex"
	"fmt"

	"bytes"

	"github.com/iotaledger/hive.go/ds/bitmask"
	"github.com/iotaledger/hive.go/serializer"
	"github.com/iotaledger/hive.go/serializer/v2/marshalutil"
	"github.com/iotaledger/inx-api-core-v1/pkg/hornet"
	"github.com/iotaledger/inx-api-core-v1/pkg/milestone"
	iotago "github.com/iotaledger/iota.go/v2"
	"github.com/linxGnu/grocksdb"
)


// Conflict defines the reason why a message is marked as conflicting.
type Conflict uint8

const (
	// ConflictNone the message has no conflict.
	ConflictNone Conflict = iota

	// ConflictInputUTXOAlreadySpent the referenced UTXO was already spent.
	ConflictInputUTXOAlreadySpent = 1

	// ConflictInputUTXOAlreadySpentInThisMilestone the referenced UTXO was already spent while confirming this milestone.
	ConflictInputUTXOAlreadySpentInThisMilestone = 2

	// ConflictInputUTXONotFound the referenced UTXO cannot be found.
	ConflictInputUTXONotFound = 3

	// ConflictInputOutputSumMismatch the sum of the inputs and output values does not match.
	ConflictInputOutputSumMismatch = 4

	// ConflictInvalidSignature the unlock block signature is invalid.
	ConflictInvalidSignature = 5

	// ConflictInvalidDustAllowance the dust allowance for the address is invalid.
	ConflictInvalidDustAllowance = 6

	// ConflictSemanticValidationFailed the semantic validation failed.
	ConflictSemanticValidationFailed = 255
)

const (
	MessageMetadataSolid         = 0
	MessageMetadataReferenced    = 1
	MessageMetadataNoTx          = 2
	MessageMetadataConflictingTx = 3
	MessageMetadataMilestone     = 4
)

const (
	MetadataPrefix byte = 2
	MessagePrefix byte = 1
)

type MetadataReturn struct {
	Solid bool 
	ReferencedByMilestoneIndex *uint32 
	LedgerInclusionState *string 
	ShouldPromote *bool
	ShouldReattach *bool
	ConflictReason uint8
}

type MessageMetadata struct {
	messageID hornet.MessageID

	// Metadata
	metadata bitmask.BitMask

	// Unix time when the Tx became solid (needed for local modifiers for tipselection)
	solidificationTimestamp int32

	// The index of the milestone which referenced this msg
	referencedIndex milestone.Index

	conflict Conflict

	// youngestConeRootIndex is the highest referenced index of the past cone of this message
	youngestConeRootIndex milestone.Index

	// oldestConeRootIndex is the lowest referenced index of the past cone of this message
	oldestConeRootIndex milestone.Index

	// coneRootCalculationIndex is the confirmed milestone index ycri and ocri were calculated at
	coneRootCalculationIndex milestone.Index

	// parents are the parents of the message
	parents hornet.MessageIDs
}

func (m *MessageMetadata) MessageID() hornet.MessageID {
	return m.messageID
}


func (m *MessageMetadata) IsSolid() bool {
	return m.metadata.HasBit(MessageMetadataSolid)
}

func (m *MessageMetadata) IsIncludedTxInLedger() bool {
	return m.metadata.HasBit(MessageMetadataReferenced) && !m.metadata.HasBit(MessageMetadataNoTx) && !m.metadata.HasBit(MessageMetadataConflictingTx)
}

func (m *MessageMetadata) IsReferenced() bool {
	return m.metadata.HasBit(MessageMetadataReferenced)
}

func (m *MessageMetadata) ReferencedWithIndex() (bool, milestone.Index) {
	return m.metadata.HasBit(MessageMetadataReferenced), m.referencedIndex
}

func (m *MessageMetadata) IsNoTransaction() bool {
	return m.metadata.HasBit(MessageMetadataNoTx)
}

func (m *MessageMetadata) IsConflictingTx() bool {
	return m.metadata.HasBit(MessageMetadataConflictingTx)
}

func (m *MessageMetadata) Conflict() Conflict {
	return m.conflict
}

func (m *MessageMetadata) IsMilestone() bool {
	return m.metadata.HasBit(MessageMetadataMilestone)
}

func (m *MessageMetadata) Metadata() byte {
	return byte(m.metadata)
}

func metadataFactory(key []byte, data []byte) (*MessageMetadata, error) {

	/*
		1 byte  metadata bitmask
		4 bytes uint32 solidificationTimestamp
		4 bytes uint32 referencedIndex
		1 byte  uint8 conflict
		4 bytes uint32 youngestConeRootIndex
		4 bytes uint32 oldestConeRootIndex
		4 bytes uint32 coneRootCalculationIndex
		1 byte  parents count
		parents count * 32 bytes parent id
	*/

	marshalUtil := marshalutil.New(data)

	metadataByte, err := marshalUtil.ReadByte()
	if err != nil {
		return nil, err
	}

	solidificationTimestamp, err := marshalUtil.ReadUint32()
	if err != nil {
		return nil, err
	}

	referencedIndex, err := marshalUtil.ReadUint32()
	if err != nil {
		return nil, err
	}

	conflict, err := marshalUtil.ReadByte()
	if err != nil {
		return nil, err
	}

	youngestConeRootIndex, err := marshalUtil.ReadUint32()
	if err != nil {
		return nil, err
	}

	oldestConeRootIndex, err := marshalUtil.ReadUint32()
	if err != nil {
		return nil, err
	}

	coneRootCalculationIndex, err := marshalUtil.ReadUint32()
	if err != nil {
		return nil, err
	}

	m := &MessageMetadata{
		messageID: hornet.MessageIDFromSlice(key[:32]),
	}

	m.metadata = bitmask.BitMask(metadataByte)
	m.solidificationTimestamp = int32(solidificationTimestamp)
	m.referencedIndex = milestone.Index(referencedIndex)
	m.conflict = Conflict(conflict)
	m.youngestConeRootIndex = milestone.Index(youngestConeRootIndex)
	m.oldestConeRootIndex = milestone.Index(oldestConeRootIndex)
	m.coneRootCalculationIndex = milestone.Index(coneRootCalculationIndex)

	parentsCount, err := marshalUtil.ReadByte()
	if err != nil {
		return nil, err
	}

	m.parents = make(hornet.MessageIDs, parentsCount)
	for i := 0; i < int(parentsCount); i++ {
		parentBytes, err := marshalUtil.ReadBytes(iotago.MessageIDLength)
		if err != nil {
			return nil, err
		}

		parent := hornet.MessageIDFromSlice(parentBytes)
		m.parents[i] = parent
	}

	return m, nil
}

func getMessage(msgID iotago.MessageID, db *grocksdb.DB) (*iotago.Message, error) {

	ro := grocksdb.NewDefaultReadOptions()
	ro.SetFillCache(false)
	defer ro.Destroy()

	valueMessage, err := db.Get(ro, ConcatBytes([]byte{MessagePrefix}, msgID[:]))
    if err != nil {
		return nil, fmt.Errorf("Error geting message from rocksdb: ", err)
    }
    defer valueMessage.Free()
	if valueMessage.Data() == nil {
		return nil, fmt.Errorf("Key not found in rocksdb for message ID: ", hex.EncodeToString(msgID[:]))
	}
	valObjMsg := valueMessage.Data()
	resBytes := make([]byte, len(valObjMsg))
	copy(resBytes, valObjMsg)
			
	msg := &iotago.Message{}
	_, err = msg.Deserialize(resBytes, serializer.DeSeriModePerformValidation)
	if err != nil {
		return nil, fmt.Errorf("Error deserializing rocksdb message with ID %s: %s",  hex.EncodeToString(msgID[:]), err)
	}
	return msg, nil	
}

func getMessageMetadata(msgID iotago.MessageID, db *grocksdb.DB) (*MetadataReturn, error) {

	ro := grocksdb.NewDefaultReadOptions()
	ro.SetFillCache(false)
	defer ro.Destroy()

	valueMetadata, err := db.Get(ro, ConcatBytes([]byte{MetadataPrefix}, msgID[:]))
    if err != nil {
		return nil, fmt.Errorf("Error getting metadata from rocksdb for messageID %s: %s", hex.EncodeToString(msgID[:]), err)
    }
    defer valueMetadata.Free()
	if valueMetadata.Data() == nil {
		return nil, fmt.Errorf("Key not found in rocksdb for metatdata of message ID: ", hex.EncodeToString(msgID[:]))
	}
	valObjMetadata := valueMetadata.Data()
	
	mm, err := metadataFactory(ConcatBytes([]byte{MetadataPrefix}, msgID[:]), valObjMetadata)
	if err != nil {
		return nil, fmt.Errorf("Error with metadata factor with messageID %s: %s", hex.EncodeToString(msgID[:]), err)
	}

	ledgerState := ""
	if mm.IsIncludedTxInLedger() {
		ledgerState = "included"
	} else if mm.IsNoTransaction() {
		ledgerState = "noTransaction"
	} else if mm.IsConflictingTx() {
		ledgerState = "conflicting"
	}

	var x uint32 = uint32(mm.referencedIndex)
    var ptrReferenced *uint32 = &x 

	var ptrLedger *string = &ledgerState

	metadataResponse := &MetadataReturn{
		Solid: mm.IsSolid(),
		ReferencedByMilestoneIndex: ptrReferenced,
		LedgerInclusionState: ptrLedger,
		ConflictReason: uint8(mm.Conflict()),
	}
	return metadataResponse, nil
}

func ConcatBytes(byteSlices ...[]byte) []byte {
	var b bytes.Buffer
	for _, byteSlice := range byteSlices {
		b.Write(byteSlice)
	}
	return b.Bytes()
}