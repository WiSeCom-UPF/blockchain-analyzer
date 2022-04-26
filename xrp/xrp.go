package xrp

import (
	"time"

	jsoniter "github.com/json-iterator/go"

	"github.com/danhper/blockchain-analyzer/core"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

const rippleEpochOffset int64 = 946684800

type XRP struct {
}

func New() *XRP {
	return &XRP{}
}

type Transaction struct {
	Account         string
	TransactionType string
	Destination     string
}

type Ledger struct {
	Index           uint64 `json:"-"`
	CloseTimestamp  int64  `json:"close_time"`
	parsedCloseTime time.Time
	Transactions    []Transaction
}

type xrpLedger struct {
	LedgerIndex uint64 `json:"ledger_index"`
	Ledger      Ledger
	Validated   bool
}

type xrpLedgerResponse struct {
	Result xrpLedger
}

func ParseRawLedger(rawLedger []byte) (*Ledger, error) {
	var response xrpLedgerResponse
	if err := json.Unmarshal(rawLedger, &response); err != nil {
		return nil, err
	}
	result := response.Result
	if result.LedgerIndex == uint64(0) && !result.Validated {
		if err := json.Unmarshal(rawLedger, &result); err != nil {
			return nil, err
		}
	}
	ledger := result.Ledger
	ledger.parsedCloseTime = time.Unix(ledger.CloseTimestamp+rippleEpochOffset, 0).UTC()
	ledger.Index = result.LedgerIndex
	return &ledger, nil
}

func (x *XRP) ParseBlock(rawLine []byte) (core.Block, error) {
	return ParseRawLedger(rawLine)
}

func (x *XRP) EmptyBlock() core.Block {
	return &Ledger{}
}

func (x *XRP) FetchData(filepath string, start, end uint64) error {
	return fetchXRPData(filepath, start, end)
}

func (l *Ledger) Number() uint64 {
	return l.Index
}

func (l *Ledger) Time() time.Time {
	return l.parsedCloseTime
}

func (l *Ledger) TransactionsCount() int {
	return len(l.Transactions)
}

// TO-DO
func (l *Ledger) TransactionsCountByAddress(address string, by string) int {
	// not yet implemented
	return 0
}

// TO-DO
func (l *Ledger) EmptyBlocksCount() int {
	// not yet implemented
	return 0
}

// TO-DO
func (l *Ledger) ZeroTxnBlocksCount() int {
	// // not yet implemented
	return 0
}

func (l *Ledger) GetMiner() string {
	// // not yet implemented
	return ""
}

func (l *Ledger) ListActions() []core.Action {
	var actions []core.Action
	for _, t := range l.Transactions {
		actions = append(actions, t)
	}
	return actions
}

func (t Transaction) Sender() string {
	return t.Account
}

func (t Transaction) Receiver() string {
	return t.Destination
}

func (t Transaction) Name() string {
	return t.TransactionType
}
