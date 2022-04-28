package core

import (
	"time"
)

type Blockchain interface {
	FetchData(filepath string, start, end uint64) error
	ParseBlock(rawLine []byte) (Block, error)
	EmptyBlock() Block
}

type Block interface {
	Number() uint64
	EmptyBlocksCount() int
	TransactionsCountByAddress(string, string) int
	ZeroTxnBlocksCount() int
	TransactionsCount() int
	GetTxnP2Plist() []string
	SCCount() int
	Time() time.Time
	GetMiner() string
	ListActions() []Action
}

type Action interface {
	Sender() string
	Receiver() string
	Name() string
}
