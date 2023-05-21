package core

import (
	"encoding/json"
	"fmt"
	"sort"
	"time"
	"strings"

	"github.com/danhper/structomap"
)

type ActionProperty int

const (
	ActionName ActionProperty = iota
	ActionSender
	ActionReceiver
)

const (
	maxTopLevelResults = 1000
	maxNestedResults   = 50
)

func GetActionProperty(name string) (ActionProperty, error) {
	switch name {
	case "name":
		return ActionName, nil
	case "sender":
		return ActionSender, nil
	case "receiver":
		return ActionReceiver, nil
	default:
		return ActionName, fmt.Errorf("no property %s for actions", name)
	}
}

func (p ActionProperty) String() string {
	switch p {
	case ActionName:
		return "name"
	case ActionSender:
		return "sender"
	case ActionReceiver:
		return "receiver"
	default:
		panic(fmt.Errorf("no such action property"))
	}
}

func (c *ActionProperty) UnmarshalJSON(data []byte) (err error) {
	var rawProperty string
	if err = json.Unmarshal(data, &rawProperty); err != nil {
		return err
	}
	*c, err = GetActionProperty(rawProperty)
	return err
}

type Duration struct {
	time.Duration
}

func (d *Duration) UnmarshalJSON(b []byte) (err error) {
	var rawDuration string
	if err = json.Unmarshal(b, &rawDuration); err != nil {
		return err
	}
	d.Duration, err = time.ParseDuration(rawDuration)
	return err
}

type ActionsCount struct {
	Actions     map[string]uint64
	UniqueCount uint64
	TotalCount  uint64
}

type OneToOneStatsData struct {
	TotalOps			uint64
	ValueTransferCount	uint64
	SCTxnsCount			uint64
}

func NewOneToOneDataStats() *OneToOneStatsData {
	return &OneToOneStatsData{
		TotalOps: 0,
		ValueTransferCount: 0,
		SCTxnsCount: 0,
	}
}

type OneToOneTxnMap struct {
	TxnOneToOne		map[string]*OneToOneStatsData
}

func NewOneToOneMap() *OneToOneTxnMap {
	return &OneToOneTxnMap{
		TxnOneToOne: make(map[string]*OneToOneStatsData),
	}
}


func (o2o *OneToOneTxnMap) AddBlock(block Block) {
	txnLen := block.TransactionsCount()
	txnData := make([]string, 2*txnLen)
	txnData = block.GetTxnP2Plist()
	key := ""
	txnKind := ""
	i := 0

	for  ; i < 2 *txnLen; {
		key = txnData[i]
		txnKind = txnData[i+1]
		// fmt.Println(txnKind)
		oneToOneStat, ok := o2o.TxnOneToOne[key]

		if !ok {
			oneToOneStat = NewOneToOneDataStats()
			o2o.TxnOneToOne[key] = oneToOneStat
		}

		o2o.TxnOneToOne[key].TotalOps += 1
		if txnKind == "Contract" {
			o2o.TxnOneToOne[key].SCTxnsCount += 1
		} else {
			o2o.TxnOneToOne[key].ValueTransferCount += 1
		}
		i += 2
	}
}

func NewActionsCount() *ActionsCount {
	return &ActionsCount{
		Actions: make(map[string]uint64),
	}
}

func (a *ActionsCount) Increment(key string) {
	a.TotalCount++
	if _, ok := a.Actions[key]; !ok {
		a.UniqueCount++
	}
	a.Actions[key] += 1
}

func (a *ActionsCount) Get(key string) uint64 {
	return a.Actions[key]
}

func (a *ActionsCount) Merge(other *ActionsCount) {
	for key, value := range other.Actions {
		a.Actions[key] += value
	}
}

type NamedCount struct {
	Name  string
	Count uint64
}

var actionsCountSerializer = structomap.New().
	PickFunc(func(actions interface{}) interface{} {
		var results []NamedCount
		for name, count := range actions.(map[string]uint64) {
			results = append(results, NamedCount{Name: name, Count: count})
		}
		sort.Slice(results, func(i, j int) bool {
			return results[i].Count > results[j].Count
		})
		if len(results) > maxNestedResults {
			results = results[:maxNestedResults]
		}
		return results
	}, "Actions").
	Pick("UniqueCount", "TotalCount")

func (a *ActionsCount) MarshalJSON() ([]byte, error) {
	return json.Marshal(actionsCountSerializer.Transform(a))
}

func Persist(entity interface{}, outputFile string) error {
	file, err := CreateFile(outputFile)
	if err != nil {
		return err
	}
	defer file.Close()
	encoder := json.NewEncoder(file)
	return encoder.Encode(entity)
}

type TimeGroupedActions struct {
	Actions   map[time.Time]*GroupedActions
	Duration  time.Duration
	GroupedBy ActionProperty
}

func NewTimeGroupedActions(duration time.Duration, by ActionProperty) *TimeGroupedActions {
	return &TimeGroupedActions{
		Actions:   make(map[time.Time]*GroupedActions),
		Duration:  duration,
		GroupedBy: by,
	}
}

func (g *TimeGroupedActions) AddBlock(block Block) {
	group := block.Time().Truncate(g.Duration)
	if _, ok := g.Actions[group]; !ok {
		g.Actions[group] = NewGroupedActions(g.GroupedBy, false)
	}
	g.Actions[group].AddBlock(block)
}

func (g *TimeGroupedActions) Result() interface{} {
	return g
}

type MiningHistoryCount struct {
	MiningHistoryCounts map[string]int
}

func NewMiningHistoryCount() *MiningHistoryCount {
	return &MiningHistoryCount{
		MiningHistoryCounts: make(map[string]int),
	}
}

func (mh *MiningHistoryCount) AddBlock(block Block) {
	miner := block.GetMiner()
	mh.MiningHistoryCounts[miner] += 1
}

func (mh *MiningHistoryCount) Result() interface{} {
	return mh
}

type TimeGroupedMiningHistoryCount struct {
	MinerGroupByTime   map[time.Time]*MiningHistoryCount
	Duration  		   time.Duration
}

func NewMiningHistoryCountOverTime(duration time.Duration) *TimeGroupedMiningHistoryCount {
	return &TimeGroupedMiningHistoryCount{
		MinerGroupByTime: 	make(map[time.Time]*MiningHistoryCount),
		Duration:			duration,
	}
}

func (tmh *TimeGroupedMiningHistoryCount) AddBlock(block Block) {
	group := block.Time().Truncate(tmh.Duration)
	if _, ok := tmh.MinerGroupByTime[group]; !ok {
		tmh.MinerGroupByTime[group] = NewMiningHistoryCount()
	}
	tmh.MinerGroupByTime[group].AddBlock(block)
}

func (tmh *TimeGroupedMiningHistoryCount) Result() interface{} {
	return tmh
}


type TimeGroupedEmptyBlocks struct {
	EmptyBlocks 	  map[time.Time]int
	GroupedBy         time.Duration
}

func NewTimeGroupedEmptyBlocks(duration time.Duration) *TimeGroupedEmptyBlocks {
	return &TimeGroupedEmptyBlocks{
		EmptyBlocks: make(map[time.Time]int),
		GroupedBy:         duration,
	}
}

func (g *TimeGroupedEmptyBlocks) AddBlock(block Block) {
	group := block.Time().Truncate(g.GroupedBy)
	if _, ok := g.EmptyBlocks[group]; !ok {
		g.EmptyBlocks[group] = 0
	}
	g.EmptyBlocks[group] += block.EmptyBlocksCount()
}

func (g *TimeGroupedEmptyBlocks) Result() interface{} {
	return g
}

type TimeGroupedZeroTxnBlocks struct {
	ZeroTxnBlocks 	  map[time.Time]int
	GroupedBy         time.Duration
}

func NewTimeGroupedZeroTxnBlocks(duration time.Duration) *TimeGroupedZeroTxnBlocks {
	return &TimeGroupedZeroTxnBlocks{
		ZeroTxnBlocks: make(map[time.Time]int),
		GroupedBy:         duration,
	}
}

func (g *TimeGroupedZeroTxnBlocks) AddBlock(block Block) {
	group := block.Time().Truncate(g.GroupedBy)
	if _, ok := g.ZeroTxnBlocks[group]; !ok {
		g.ZeroTxnBlocks[group] = 0
	}
	g.ZeroTxnBlocks[group] += block.ZeroTxnBlocksCount()
}

func (g *TimeGroupedZeroTxnBlocks) Result() interface{} {
	return g
}

type TimeGroupedTransactionCount struct {
	TransactionCounts map[time.Time]int
	GroupedBy         time.Duration
}

type TimeGroupedGovTransactionCount struct {
	GovTransactionCounts map[time.Time]int
	GroupedBy         time.Duration
}

type TimeGroupedSCCount struct {
	SCCounts 		  map[time.Time]int
	GroupedBy         time.Duration
}

type TimeGroupedTransactionCountByAddress struct {
	TransactionCounts map[time.Time]int
	GroupedBy         time.Duration
}

func NewTimeGroupedTransactionCountByAddress(duration time.Duration, address string) *TimeGroupedTransactionCountByAddress {
	return &TimeGroupedTransactionCountByAddress{
		TransactionCounts: make(map[time.Time]int),
		GroupedBy:         duration,
	}
}

func (g *TimeGroupedTransactionCountByAddress) AddBlock(block Block, address string, by string) {
	group := block.Time().Truncate(g.GroupedBy)
	if _, ok := g.TransactionCounts[group]; !ok {
		g.TransactionCounts[group] = 0
	}
	g.TransactionCounts[group] += block.TransactionsCountByAddress(address, by)
}

func (g *TimeGroupedTransactionCountByAddress) Result() interface{} {
	return g
}

func NewTimeGroupedTransactionCount(duration time.Duration) *TimeGroupedTransactionCount {
	return &TimeGroupedTransactionCount{
		TransactionCounts: make(map[time.Time]int),
		GroupedBy:         duration,
	}
}

func NewTimeGroupedGovTransactionCount(duration time.Duration) *TimeGroupedGovTransactionCount {
	return &TimeGroupedGovTransactionCount{
		GovTransactionCounts: make(map[time.Time]int),
		GroupedBy:         duration,
	}
}

func (g *TimeGroupedTransactionCount) AddBlock(block Block) {
	group := block.Time().Truncate(g.GroupedBy)
	if _, ok := g.TransactionCounts[group]; !ok {
		g.TransactionCounts[group] = 0
	}
	g.TransactionCounts[group] += block.TransactionsCount()
}

func (g *TimeGroupedTransactionCount) Result() interface{} {
	return g
}

func (gtx *TimeGroupedGovTransactionCount) AddBlock(block Block) {
	group := block.Time().Truncate(gtx.GroupedBy)
	if _, ok := gtx.GovTransactionCounts[group]; !ok {
		gtx.GovTransactionCounts[group] = 0
	}
	gtx.GovTransactionCounts[group] += block.GovernanceTransactionsCount()
}

func (gtx *TimeGroupedGovTransactionCount) Result() interface{} {
	return gtx
}

func NewTimeGroupedSCCount(duration time.Duration) *TimeGroupedSCCount {
	return &TimeGroupedSCCount{
		SCCounts: 		   make(map[time.Time]int),
		GroupedBy:         duration,
	}
}

func (sc *TimeGroupedSCCount) AddBlock(block Block) {
	group := block.Time().Truncate(sc.GroupedBy)
	if _, ok := sc.SCCounts[group]; !ok {
		sc.SCCounts[group] = 0
	}
	// if we pass the empty string in SSCount then 
	// it calculates SC created count
	tempValue, _ := block.SCCount("")
	sc.SCCounts[group] += tempValue
}

func (sc *TimeGroupedSCCount) Result() interface{} {
	return sc
}

type TimeGroupedIndexationCount struct {
	IndexationCounts map[time.Time]int
	GroupedBy         time.Duration
}

func NewTimeGroupedIndexationCount(duration time.Duration) *TimeGroupedIndexationCount {
	return &TimeGroupedIndexationCount{
		IndexationCounts: make(map[time.Time]int),
		GroupedBy:         duration,
	}
}

func (g *TimeGroupedIndexationCount) AddBlock(block Block) {
	group := block.Time().Truncate(g.GroupedBy)
	if _, ok := g.IndexationCounts[group]; !ok {
		g.IndexationCounts[group] = 0
	}
	g.IndexationCounts[group] += block.IndexationPayloadCount()
}

func (g *TimeGroupedIndexationCount) Result() interface{} {
	return g
}

type TimeGroupedSignedTransactionCount struct {
	SgnTransactionCounts map[time.Time]int
	GroupedBy         time.Duration
}

func NewTimeGroupedSignedTransactionCount(duration time.Duration) *TimeGroupedSignedTransactionCount {
	return &TimeGroupedSignedTransactionCount{
		SgnTransactionCounts: make(map[time.Time]int),
		GroupedBy:         duration,
	}
}

func (g *TimeGroupedSignedTransactionCount) AddBlock(block Block) {
	group := block.Time().Truncate(g.GroupedBy)
	if _, ok := g.SgnTransactionCounts[group]; !ok {
		g.SgnTransactionCounts[group] = 0
	}
	g.SgnTransactionCounts[group] += block.SignedTransactionPayloadCount()
}

func (g *TimeGroupedSignedTransactionCount) Result() interface{} {
	return g
}

type TimeGroupedNoPayloadCount struct {
	NoPayloadCounts map[time.Time]int
	GroupedBy         time.Duration
}

func NewTimeGroupedNoPayloadCount(duration time.Duration) *TimeGroupedNoPayloadCount {
	return &TimeGroupedNoPayloadCount{
		NoPayloadCounts: make(map[time.Time]int),
		GroupedBy:         duration,
	}
}

func (g *TimeGroupedNoPayloadCount) AddBlock(block Block) {
	group := block.Time().Truncate(g.GroupedBy)
	if _, ok := g.NoPayloadCounts[group]; !ok {
		g.NoPayloadCounts[group] = 0
	}
	g.NoPayloadCounts[group] += block.NoPayloadCount()
}

func (g *TimeGroupedNoPayloadCount) Result() interface{} {
	return g
}

type TimeGroupedConflictsCount struct {
	ConflictsCounts map[time.Time]int
	GroupedBy         time.Duration
}

func NewTimeGroupedConflictsCount(duration time.Duration) *TimeGroupedConflictsCount {
	return &TimeGroupedConflictsCount{
		ConflictsCounts: make(map[time.Time]int),
		GroupedBy:         duration,
	}
}

func (g *TimeGroupedConflictsCount) AddBlock(block Block) {
	group := block.Time().Truncate(g.GroupedBy)
	if _, ok := g.ConflictsCounts[group]; !ok {
		g.ConflictsCounts[group] = 0
	}
	g.ConflictsCounts[group] += block.ConflictsCount()
}

func (g *TimeGroupedConflictsCount) Result() interface{} {
	return g
}

type GroupedConflictsCount struct {
	GroupedConflicts map[int]int
}

func NewGroupedConflictsCount() *GroupedConflictsCount {
	return &GroupedConflictsCount{
		GroupedConflicts: make(map[int]int),
	}
}

func (g *GroupedConflictsCount) AddBlock(block Block) {

	gConflicts := block.GetGroupedConflicts()
	if gConflicts == nil {
		return
	}

	for key, value := range *gConflicts {
        g.GroupedConflicts[key] += value
    }
}

func (g *GroupedConflictsCount) Result() interface{} {
	return g
}

type GroupedByIndexCount struct {
	GroupedIndexes map[string]int
}

func NewGroupedByIndexCount() *GroupedByIndexCount {
	return &GroupedByIndexCount{
		GroupedIndexes: make(map[string]int),
	}
}

func (g *GroupedByIndexCount) AddBlock(block Block) {

	gIndexes := block.GetGroupedIndexes()
	if gIndexes == nil {
		return
	}

	for key, value := range *gIndexes {
        g.GroupedIndexes[key] += value
    }
}

func (g *GroupedByIndexCount) Result() interface{} {
	return g
}

type GroupedSgnTransactionsByIndexCount struct {
	GroupedIndexes map[string]int
}

func NewGroupedSgnTransactionsByIndexCount() *GroupedSgnTransactionsByIndexCount {
	return &GroupedSgnTransactionsByIndexCount{
		GroupedIndexes: make(map[string]int),
	}
}

func (g *GroupedSgnTransactionsByIndexCount) AddBlock(block Block) {

	gIndexes := block.GetGroupedIndexesTransactions()
	if gIndexes == nil {
		return
	}

	for key, value := range *gIndexes {
        g.GroupedIndexes[key] += value
    }
}

func (g *GroupedSgnTransactionsByIndexCount) Result() interface{} {
	return g
}

type TimeGroupedByIndexCount struct {
	TimeIndexesCounts map[time.Time]map[string]int
	GroupedBy         time.Duration
}

func NewTimeGroupedByIndexCount(duration time.Duration) *TimeGroupedByIndexCount {
	return &TimeGroupedByIndexCount{
		TimeIndexesCounts: make(map[time.Time]map[string]int),
		GroupedBy:         duration,
	}
}

func (g *TimeGroupedByIndexCount) AddBlock(block Block) {

	gIndexes := block.GetGroupedIndexes()
	if gIndexes == nil {
		return
	}

	group := block.Time().Truncate(g.GroupedBy)
	if _, ok := g.TimeIndexesCounts[group]; !ok {
		g.TimeIndexesCounts[group] = make(map[string]int)
	}

	for key, value := range *gIndexes {
        g.TimeIndexesCounts[group][key] += value
    }
}

func (g *TimeGroupedByIndexCount) Result() interface{} {
	return g
}


type GroupedByAddressCount struct {
	GroupedAddresses map[string]int
}

func NewGroupedByAddressCount() *GroupedByAddressCount {
	return &GroupedByAddressCount{
		GroupedAddresses: make(map[string]int),
	}
}

func (g *GroupedByAddressCount) AddBlock(block Block) {

	gAddrs := block.GetGroupedAddresses()
	if gAddrs == nil {
		return
	}

	for key, value := range *gAddrs {
        g.GroupedAddresses[key] += value
    }
}

func (g *GroupedByAddressCount) Result() interface{} {
	return g
}

type Stats struct {
	Max			int	
	Min 		int
	Median 		float64
	Average 	float64
}

type TimeAverageValuesCount struct {
	AverageValues 		map[time.Time][]int
	FinalAverages		map[time.Time]Stats
	GroupedBy         	time.Duration
}

func NewTimeAverageValuesCount(duration time.Duration) *TimeAverageValuesCount {
	return &TimeAverageValuesCount{
		AverageValues: make(map[time.Time][]int),
		GroupedBy:         duration,
	}
}

func (g *TimeAverageValuesCount) AddBlock(block Block) {
	group := block.Time().Truncate(g.GroupedBy)
	if _, ok := g.AverageValues[group]; !ok {
		g.AverageValues[group] = []int{}
	}
	g.AverageValues[group] = append(g.AverageValues[group], *block.GetValuesSpent()...)
}

func (g *TimeAverageValuesCount) Result() interface{} {
	return g
}

type TimeAverageMilestonesTimeCount struct {
	AverageValues 		map[time.Time][]int64
	FinalAverages		map[time.Time]int64
	GroupedBy         	time.Duration
}

func NewTimeAverageMilestonesTimeCount(duration time.Duration) *TimeAverageMilestonesTimeCount {
	return &TimeAverageMilestonesTimeCount{
		AverageValues: 		make(map[time.Time][]int64),
		GroupedBy:         	duration,
	}
}

func (g *TimeAverageMilestonesTimeCount) AddBlock(block Block) {
	group := block.Time().Truncate(g.GroupedBy)
	if _, ok := g.AverageValues[group]; !ok {
		g.AverageValues[group] = []int64{}
	}
	g.AverageValues[group] = append(g.AverageValues[group], block.Time().Unix())
}

func (g *TimeAverageMilestonesTimeCount) Result() interface{} {
	return g
}

type ActionGroup struct {
	Name      string
	Count     uint64
	Names     *ActionsCount
	Senders   *ActionsCount
	Receivers *ActionsCount
}

var actionGroupSerializer = structomap.New().
	Pick("Name", "Count").
	PickIf(func(a interface{}) bool {
		return a.(*ActionGroup).Names.TotalCount > 0
	}, "Names", "Senders", "Receivers")

func (a *ActionGroup) MarshalJSON() ([]byte, error) {
	return json.Marshal(actionGroupSerializer.Transform(a))
}

func NewActionGroup(name string) *ActionGroup {
	return &ActionGroup{
		Name:      name,
		Count:     0,
		Names:     NewActionsCount(),
		Senders:   NewActionsCount(),
		Receivers: NewActionsCount(),
	}
}

type GroupedActions struct {
	Actions        map[string]*ActionGroup
	GroupedBy      string
	BlocksCount    uint64
	ActionsCount   uint64
	actionProperty ActionProperty
	detailed       bool
}

var groupedActionsSerializer = structomap.New().
	PickFunc(func(actions interface{}) interface{} {
		var results []*ActionGroup
		for _, action := range actions.(map[string]*ActionGroup) {
			results = append(results, action)
		}
		sort.Slice(results, func(i, j int) bool {
			return results[i].Count > results[j].Count
		})
		if len(results) > maxTopLevelResults {
			results = results[:maxTopLevelResults]
		}
		return results
	}, "Actions").
	Pick("GroupedBy", "BlocksCount", "ActionsCount")

var SCgroupedActionsSerializer = structomap.New().
	PickFunc(func(actions interface{}) interface{} {
		var results []*ActionGroup
		for _, action := range actions.(map[string]*ActionGroup) {
			results = append(results, action)
		}
		sort.Slice(results, func(i, j int) bool {
			return results[i].Count > results[j].Count
		})
		if len(results) > maxTopLevelResults {
			results = results[:maxTopLevelResults]
		}
		return results
	}, "Actions").
	Pick("VerifiedSC", "UnVerifiedSC", "XRC20Tokens", "NFTTokens","GroupedBy", "BlocksCount", "ActionsCount")

func (g *GroupedActions) MarshalJSON() ([]byte, error) {
	return json.Marshal(groupedActionsSerializer.Transform(g))
}

func (g *GroupedActions) Get(key string) *ActionGroup {
	return g.Actions[key]
}

func (g *GroupedActions) GetCount(key string) uint64 {
	group := g.Get(key)
	if group == nil {
		return 0
	}
	return group.Count
}

func NewGroupedActions(by ActionProperty, detailed bool) *GroupedActions {
	actions := make(map[string]*ActionGroup)
	return &GroupedActions{
		Actions:        actions,
		GroupedBy:      by.String(),
		BlocksCount:    0,
		ActionsCount:   0,
		actionProperty: by,
		detailed:       detailed,
	}
}

func (g *GroupedActions) getActionKey(action Action) string {
	switch g.actionProperty {
	case ActionName:
		return action.Name()
	case ActionSender:
		return action.Sender()
	case ActionReceiver:
		return action.Receiver()
	default:
		panic(fmt.Errorf("no such property %d", g.actionProperty))
	}
}

func (g *GroupedActions) AddBlock(block Block) {
	g.BlocksCount += 1
	for _, action := range block.ListActions() {
		g.ActionsCount += 1
		key := g.getActionKey(action)
		if key == "" {
			continue
		}
		actionGroup, ok := g.Actions[key]
		if !ok {
			actionGroup = NewActionGroup(key)
			g.Actions[key] = actionGroup
		}
		actionGroup.Count += 1
		if g.detailed {
			actionGroup.Names.Increment(action.Name())
			actionGroup.Senders.Increment(action.Sender())
			actionGroup.Receivers.Increment(action.Receiver())
		}
	}
}

func (g *GroupedActions) Result() interface{} {
	return g
}

type SCGroupedActions struct {
	Actions        map[string]*ActionGroup
	GroupedBy      string
	BlocksCount    uint64
	VerifiedSC     uint64
	UnVerifiedSC   uint64
	XRC20Tokens	   uint64
	NFTTokens      uint64
	ActionsCount   uint64
	actionProperty ActionProperty
	detailed       bool
}

func NewSCGroupedActions(by ActionProperty, detailed bool) *SCGroupedActions {
	actions := make(map[string]*ActionGroup)
	return &SCGroupedActions{
		Actions:        actions,
		GroupedBy:      by.String(),
		BlocksCount:    0,
		VerifiedSC:     0,
		UnVerifiedSC:   0,
		XRC20Tokens:	0,
		NFTTokens:      0,
		ActionsCount:   0,
		actionProperty: by,
		detailed:       detailed,
	}
}

func (scg *SCGroupedActions) getActionKey(action Action) string {
	switch scg.actionProperty {
	case ActionName:
		return action.Name()
	case ActionSender:
		return action.Sender()
	case ActionReceiver:
		return action.Receiver()
	default:
		panic(fmt.Errorf("no such property %d", scg.actionProperty))
	}
}

func (scg *SCGroupedActions) AddBlock(block Block, verifiedTokensMap, XRC20Map, NFTTokensMap map[string]string) {
	scg.BlocksCount += 1

	for _, action := range block.ListActions() {
		scg.ActionsCount += 1
		key := scg.getActionKey(action)
		if key == "" {
			continue
		}

		if key == "Contract" {

			address := strings.ToUpper(action.Receiver())
			if _, ok := verifiedTokensMap[address]; ok{
				scg.VerifiedSC += 1
			} else {
				scg.UnVerifiedSC += 1
			}

			if _, ok := XRC20Map[address]; ok{
				scg.XRC20Tokens += 1
			}

			if _, ok := NFTTokensMap[address]; ok{
				scg.NFTTokens += 1
			}
		}

		actionGroup, ok := scg.Actions[key]
		if !ok {
			actionGroup = NewActionGroup(key)
			scg.Actions[key] = actionGroup
		}
		actionGroup.Count += 1
		if scg.detailed {
			actionGroup.Names.Increment(action.Name())
			actionGroup.Senders.Increment(action.Sender())
			actionGroup.Receivers.Increment(action.Receiver())
		}
	}
}

func (scg *SCGroupedActions) Result() interface{} {
	return scg
}

func (scg *SCGroupedActions) MarshalJSON() ([]byte, error) {
	return json.Marshal(SCgroupedActionsSerializer.Transform(scg))
}

func (scg *SCGroupedActions) Get(key string) *ActionGroup {
	return scg.Actions[key]
}

func (scg *SCGroupedActions) GetCount(key string) uint64 {
	group := scg.Get(key)
	if group == nil {
		return 0
	}
	return group.Count
}


type TransactionCounter int
type IndexationPayloadCounter int
type SignedTransactionPayloadCounter int
type NoPayloadCounter int
type OtherPayloadCounter int
type NoSolidCounter int
type ConflictsCounter int
type AverageValuesAppender []int
type TimeMilestonesCounter []int64
type MaxTransactionBlockCounter int
type GovernanceCounter int
type SCCounter struct {
	SCCreated    	int
	SCSignMap		map[string]int
}
type TransactionCounterByAddress int
type EmptyBlockCounter int
type ZeroTxnBlockCounter int

func NewTransactionCounter() *TransactionCounter {
	value := 0
	return (*TransactionCounter)(&value)
}

func NewMaxTransactionBlockCounter() *MaxTransactionBlockCounter {
	value := 0
	return (*MaxTransactionBlockCounter)(&value)
}

func NewGovernanceCounter() *GovernanceCounter {
	value := 0
	return (*GovernanceCounter)(&value)
}

func NewSCCounter() *SCCounter {
	return &SCCounter{
		SCCreated: 0,
		SCSignMap: make(map[string]int),
	}
}

func NewTransactionCounterByAddress() *TransactionCounterByAddress {
	value := 0
	return (*TransactionCounterByAddress)(&value)
}

func NewEmptyBlockCounter() *EmptyBlockCounter {
	value := 0
	return (*EmptyBlockCounter)(&value)
}

func NewZeroTxnBlockCounter() *ZeroTxnBlockCounter {
	value := 0
	return (*ZeroTxnBlockCounter)(&value)
}

func NewIndexationPayloadCounter() *IndexationPayloadCounter {
	value := 0
	return (*IndexationPayloadCounter)(&value)
}

func (t *IndexationPayloadCounter) AddBlock(block Block) {
	*t += (IndexationPayloadCounter)(block.IndexationPayloadCount())
}

func NewSignedTransactionPayloadCounter() *SignedTransactionPayloadCounter {
	value := 0
	return (*SignedTransactionPayloadCounter)(&value)
}

func (t *SignedTransactionPayloadCounter) AddBlock(block Block) {
	*t += (SignedTransactionPayloadCounter)(block.SignedTransactionPayloadCount())
}

func NewConflictsCounter() *ConflictsCounter {
	value := 0
	return (*ConflictsCounter)(&value)
}

func (t *ConflictsCounter) AddBlock(block Block) {
	*t += (ConflictsCounter)(block.ConflictsCount())
}

func NewAverageValuesCounter() *AverageValuesAppender {
	value := []int{}
	return (*AverageValuesAppender)(&value)
}

func (t *AverageValuesAppender) AddBlock(block Block) {
	if *block.GetValuesSpent() != nil && len(*block.GetValuesSpent()) > 0 {
		*t = append(*t, *block.GetValuesSpent()...)
	}
}

func NewTimeMilestonesCounter() *TimeMilestonesCounter {
	value := []int64{}
	return (*TimeMilestonesCounter)(&value)
}

func (t *TimeMilestonesCounter) AddBlock(block Block) {
	*t = append(*t, block.Time().Unix())
}

func NewOtherPayloadCounter() *OtherPayloadCounter {
	value := 0
	return (*OtherPayloadCounter)(&value)
}

func (t *OtherPayloadCounter) AddBlock(block Block) {
	*t += (OtherPayloadCounter)(block.OtherPayloadCount())
}

func NewNoSolidCounter() *NoSolidCounter {
	value := 0
	return (*NoSolidCounter)(&value)
}

func (t *NoSolidCounter) AddBlock(block Block) {
	*t += (NoSolidCounter)(block.NoSolidCount())
}

func NewNoPayloadCounter() *NoPayloadCounter {
	value := 0
	return (*NoPayloadCounter)(&value)
}

func (t *NoPayloadCounter) AddBlock(block Block) {
	*t += (NoPayloadCounter)(block.NoPayloadCount())
}

func (t *TransactionCounter) AddBlock(block Block) {
	*t += (TransactionCounter)(block.TransactionsCount())
}

func (t *MaxTransactionBlockCounter) AddBlock(block Block) (int){
	return (int)(block.TransactionsCount())
}

func (t *GovernanceCounter) AddBlock(block Block) {
	*t += (GovernanceCounter)(block.GovernanceTransactionsCount())
}

func (sc *SCCounter) AddBlock(block Block, by string) {
	if by == ""{
		tempCounter, _ := block.SCCount("")
		sc.SCCreated += tempCounter 
	} else {
		tempCounter, tempSlice := block.SCCount(by)
		sc.SCCreated += tempCounter
		if len(tempSlice) > 0 {
			for _, item := range tempSlice{
				sc.SCSignMap[item] += 1
			}
		}
	}
}

func (sc *SCCounter) Result() interface{} {
	return sc
}

func (t *TransactionCounterByAddress) AddBlock(block Block, address string, by string) {
	*t += (TransactionCounterByAddress)(block.TransactionsCountByAddress(address, by))
}

func (ebc *EmptyBlockCounter) AddBlock(block Block) {
	*ebc += (EmptyBlockCounter)(block.EmptyBlocksCount())
}

func (ztbc *ZeroTxnBlockCounter) AddBlock(block Block) {
	*ztbc += (ZeroTxnBlockCounter)(block.ZeroTxnBlocksCount())
}

func (t *TransactionCounter) Result() interface{} {
	return t
}

func (t *MaxTransactionBlockCounter) Result() interface{} {
	return t
}

func (eb *EmptyBlockCounter) Result() interface{} {
	return eb
}

func (zb *ZeroTxnBlockCounter) Result() interface{} {
	return zb
}

type MissingBlocks struct {
	Start uint64
	End   uint64
	Seen  map[uint64]bool
}

func NewMissingBlocks(start, end uint64) *MissingBlocks {
	return &MissingBlocks{
		Start: start,
		End:   end,
		Seen:  make(map[uint64]bool),
	}
}

func (t *MissingBlocks) AddBlock(block Block) {
	t.Seen[block.Number()] = true
}

func (t *MissingBlocks) Compute() []uint64 {
	missing := make([]uint64, 0)
	for blockNumber := t.Start; blockNumber <= t.End; blockNumber++ {
		if _, ok := t.Seen[blockNumber]; !ok {
			missing = append(missing, blockNumber)
		}
	}
	return missing
}

func (t *MissingBlocks) Result() interface{} {
	return t.Compute()
}
