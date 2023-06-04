package processor

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/danhper/blockchain-analyzer/core"

	"github.com/ugorji/go/codec"
)

const logInterval int = 10000

type FileFormat int

const (
	JSONFormat FileFormat = iota
	MsgpackFormat
)

var (
	msgpackHandle = &codec.MsgpackHandle{}
)

func InferFormat(filepath string) (FileFormat, error) {
	if strings.Contains(filepath, ".jsonl") {
		return JSONFormat, nil
	}
	if strings.Contains(filepath, ".dat") {
		return MsgpackFormat, nil
	}
	return JSONFormat, fmt.Errorf("invalid filename %s", filepath)
}

func YieldBlocks(reader io.Reader, blockchain core.Blockchain, format FileFormat) <-chan core.Block {
	stream := bufio.NewReader(reader)
	blocks := make(chan core.Block)

	var decoder *codec.Decoder
	if format == MsgpackFormat {
		decoder = codec.NewDecoder(stream, msgpackHandle)
	}

	go func() {
		defer close(blocks)

		for i := 0; ; i++ {
			if i%logInterval == 0 {
				log.Printf("processed: %d", i)
			}
			block := blockchain.EmptyBlock()
			var err error
			switch format {
			case JSONFormat:
				rawLine, err := stream.ReadBytes('\n')
				if err == io.EOF {
					return
				}
				if err != nil {
					log.Printf("failed to read line %s\n", err.Error())
					return
				}
				rawLine = bytes.ToValidUTF8(rawLine, []byte{})
				block, err = blockchain.ParseBlock(rawLine)
			case MsgpackFormat:
				err = decoder.Decode(&block)
			}

			if err == io.EOF {
				break
			} else if err != nil {
				log.Printf("could not parse: %s", err.Error())
				continue
			}
			if block != nil {
				blocks <- block
			}
		}
	}()

	return blocks
}

func YieldAllBlocks(
	globPattern string,
	blockchain core.Blockchain,
	start, end uint64) (<-chan core.Block, error) {
	fmt.Println("patter: ", globPattern)
	files, err := filepath.Glob(globPattern)
	if err != nil {
		return nil, err
	}

	log.Printf("starting for %d files", len(files))
	blocks := make(chan core.Block)
	uniqueBlocks := make(chan core.Block)

	processed := 0
	fileDone := make(chan bool)

	var wg sync.WaitGroup
	run := core.MakeFileProcessor(func(filename string) error {
		defer wg.Done()
		fileFormat, err := InferFormat(filename)
		if err != nil {
			return err
		}
		reader, err := core.OpenFile(filename)
		if err != nil {
			return err
		}
		defer reader.Close()
		for block := range YieldBlocks(reader, blockchain, fileFormat) {
			if (start == 0 || block.Number() >= start) &&
				(end == 0 || block.Number() <= end) {
				blocks <- block
			}
		}
		fileDone <- true
		return err
	})

	seen := make(map[uint64]bool)
	go func() {
		for block := range blocks {
			if _, ok := seen[block.Number()]; !ok {
				uniqueBlocks <- block
				seen[block.Number()] = true
			}
		}
		close(uniqueBlocks)
	}()

	for _, filename := range files {
		wg.Add(1)
		go run(filename)
	}

	go func() {
		for range fileDone {
			processed++
			log.Printf("files processed: %d/%d", processed, len(files))
		}
	}()

	go func() {
		wg.Wait()
		close(blocks)
		close(fileDone)
	}()

	return uniqueBlocks, nil
}

func ComputeMissingBlockNumbers(blockNumbers map[uint64]bool, start, end uint64) []uint64 {
	missing := make([]uint64, 0)
	for blockNumber := start; blockNumber <= end; blockNumber++ {
		if _, ok := blockNumbers[blockNumber]; !ok {
			missing = append(missing, blockNumber)
		}
	}

	return missing
}

func OutputAllMissingBlockNumbers(
	blockchain core.Blockchain, globPattern string,
	outputPath string, start, end uint64) error {

	blocks, err := YieldAllBlocks(globPattern, blockchain, start, 0)
	if err != nil {
		return err
	}

	outputFile, err := core.CreateFile(outputPath)
	defer outputFile.Close()

	missingBlockNumbers := core.NewMissingBlocks(start, end)
	for block := range blocks {
		missingBlockNumbers.AddBlock(block)
	}

	missing := missingBlockNumbers.Compute()
	for _, number := range missing {
		fmt.Fprintf(outputFile, "{\"block\": %d}\n", number)
	}

	if len(missing) > 0 {
		return fmt.Errorf("%d missing blocks written to %s", len(missing), outputPath)
	}

	os.Remove(outputPath)

	return nil
}

func CountTransactions(blockchain core.Blockchain, globPattern string, start, end uint64) (int, error) {
	blocks, err := YieldAllBlocks(globPattern, blockchain, start, end)
	if err != nil {
		return 0, err
	}
	txCounter := core.NewTransactionCounter()
	for block := range blocks {
		txCounter.AddBlock(block)
	}
	return (int)(*txCounter), nil
}

func CountMaxTransactionsInBlock(blockchain core.Blockchain, globPattern string, start, end uint64) (int, int, error) {
	blocks, err := YieldAllBlocks(globPattern, blockchain, start, end)
	if err != nil {
		return 0, 0, err
	}
	txCounter := core.NewMaxTransactionBlockCounter()
	max_counter := 0
	var blk_num uint64
	for block := range blocks {
		temp := txCounter.AddBlock(block)
		if max_counter < temp {
			max_counter = temp
			blk_num = block.Number()
		}
	}
	return (int)(max_counter), (int)(blk_num), nil
}

func CountGovTransactions(blockchain core.Blockchain, globPattern string, start, end uint64) (int, error) {
	blocks, err := YieldAllBlocks(globPattern, blockchain, start, end)
	if err != nil {
		return 0, err
	}
	govTxCounter := core.NewGovernanceCounter()
	for block := range blocks {
		govTxCounter.AddBlock(block)
	}
	return (int)(*govTxCounter), nil
}

func CountSCSign(blockchain core.Blockchain, globPattern string, start, end uint64, by string) (*core.SCCounter, error) {
	blocks, err := YieldAllBlocks(globPattern, blockchain, start, end)
	if err != nil {
		return nil, err
	}
	scCounter := core.NewSCCounter()
	for block := range blocks {
		scCounter.AddBlock(block, by)
	}
	return scCounter, nil
}

func CountTransactionsByAddress(blockchain core.Blockchain, globPattern string, address string, by string, start, end uint64) (int, error) {
	blocks, err := YieldAllBlocks(globPattern, blockchain, start, end)
	if err != nil {
		return 0, err
	}
	txCounter := core.NewTransactionCounterByAddress()
	for block := range blocks {
		txCounter.AddBlock(block, address, by)
	}
	return (int)(*txCounter), nil
}

func CountEmptyBlocks(blockchain core.Blockchain, globPattern string, start, end uint64) (int, error) {
	blocks, err := YieldAllBlocks(globPattern, blockchain, start, end)
	if err != nil {
		return 0, err
	}
	txCounter := core.NewEmptyBlockCounter()
	for block := range blocks {
		txCounter.AddBlock(block)
	}
	return (int)(*txCounter), nil
}

func CountEmptyBlocksOverTime(blockchain core.Blockchain, globPattern string,
	start, end uint64, duration time.Duration) (*core.TimeGroupedEmptyBlocks, error) {
	blocks, err := YieldAllBlocks(globPattern, blockchain, start, end)
	if err != nil {
		return nil, err
	}
	result := core.NewTimeGroupedEmptyBlocks(duration)
	for block := range blocks {
		result.AddBlock(block)
	}
	return result, nil
}

func CountZeroTxnBlocksOverTime(blockchain core.Blockchain, globPattern string,
	start, end uint64, duration time.Duration,
) (*core.TimeGroupedZeroTxnBlocks, error) {
	blocks, err := YieldAllBlocks(globPattern, blockchain, start, end)
	if err != nil {
		return nil, err
	}
	result := core.NewTimeGroupedZeroTxnBlocks(duration)
	for block := range blocks {
		result.AddBlock(block)
	}
	return result, nil
}

func CountZeroTxnBlocks(blockchain core.Blockchain, globPattern string, start, end uint64) (int, error) {
	blocks, err := YieldAllBlocks(globPattern, blockchain, start, end)
	if err != nil {
		return 0, err
	}
	txCounter := core.NewZeroTxnBlockCounter()
	for block := range blocks {
		txCounter.AddBlock(block)
	}
	return (int)(*txCounter), nil
}

func CountActionsOverTime(
	blockchain core.Blockchain,
	globPattern string,
	start, end uint64,
	duration time.Duration,
	actionProperty core.ActionProperty) (*core.TimeGroupedActions, error) {
	blocks, err := YieldAllBlocks(globPattern, blockchain, start, end)
	if err != nil {
		return nil, err
	}
	result := core.NewTimeGroupedActions(duration, actionProperty)
	for block := range blocks {
		result.AddBlock(block)
	}
	return result, nil
}

func CountTransactionsOverTime(blockchain core.Blockchain, globPattern string,
	start, end uint64, duration time.Duration,
) (*core.TimeGroupedTransactionCount, error) {
	blocks, err := YieldAllBlocks(globPattern, blockchain, start, end)
	if err != nil {
		return nil, err
	}
	result := core.NewTimeGroupedTransactionCount(duration)
	for block := range blocks {
		result.AddBlock(block)
	}
	return result, nil
}

func CountGovTransactionsOverTime(blockchain core.Blockchain, globPattern string, start, end uint64, duration time.Duration) (*core.TimeGroupedGovTransactionCount, error) {
	blocks, err := YieldAllBlocks(globPattern, blockchain, start, end)
	if err != nil {
		return nil, err
	}
	result := core.NewTimeGroupedGovTransactionCount(duration)
	for block := range blocks {
		result.AddBlock(block)
	}
	return result, nil
}

func CountSCCreatedOverTime(blockchain core.Blockchain, globPattern string,
	start, end uint64, duration time.Duration,
) (*core.TimeGroupedSCCount, error) {
	blocks, err := YieldAllBlocks(globPattern, blockchain, start, end)
	if err != nil {
		return nil, err
	}
	result := core.NewTimeGroupedSCCount(duration)
	for block := range blocks {
		result.AddBlock(block)
	}
	return result, nil
}

func CountMiningHistory(blockchain core.Blockchain, globPattern string, start, end uint64) (*core.MiningHistoryCount, error) {
	blocks, err := YieldAllBlocks(globPattern, blockchain, start, end)
	if err != nil {
		return nil, err
	}
	result := core.NewMiningHistoryCount()
	for block := range blocks {
		result.AddBlock(block)
	}
	return result, nil
}

func CountMiningHistoryOverTime(blockchain core.Blockchain, globPattern string, start, end uint64, duration time.Duration) (*core.TimeGroupedMiningHistoryCount, error) {
	blocks, err := YieldAllBlocks(globPattern, blockchain, start, end)
	if err != nil {
		return nil, err
	}
	result := core.NewMiningHistoryCountOverTime(duration)
	for block := range blocks {
		result.AddBlock(block)
	}
	return result, nil
}

func CountTransactionsByAddressOverTime(blockchain core.Blockchain, globPattern string, address string, by string, start, end uint64, duration time.Duration) (*core.TimeGroupedTransactionCountByAddress, error) {
	blocks, err := YieldAllBlocks(globPattern, blockchain, start, end)
	if err != nil {
		return nil, err
	}
	result := core.NewTimeGroupedTransactionCountByAddress(duration, address)
	for block := range blocks {
		result.AddBlock(block, address, by)
	}
	return result, nil
}

func GroupActions(blockchain core.Blockchain, globPattern string,
	start, end uint64, by core.ActionProperty, detailed bool,
) (*core.GroupedActions, error) {
	blocks, err := YieldAllBlocks(globPattern, blockchain, start, end)
	if err != nil {
		return nil, err
	}
	groupedActions := core.NewGroupedActions(by, detailed)
	for block := range blocks {
		groupedActions.AddBlock(block)
	}
	return groupedActions, nil
}

func SCGroupActions(blockchain core.Blockchain, globPattern string,
	start, end uint64, by core.ActionProperty, detailed bool,
) (*core.SCGroupedActions, error) {
	blocks, err := YieldAllBlocks(globPattern, blockchain, start, end)
	if err != nil {
		return nil, err
	}

	var verifiedTokensMap = map[string]string{}
	var XRC20Map = map[string]string{}
	var NFTTokensMap = map[string]string{}

	verifiedTokensMap = core.SliceToMapStrStr("VerifiedTokens")
	XRC20Map = core.SliceToMapStrStr("XRCTokens")
	NFTTokensMap = core.SliceToMapStrStr("NFTTokens")

	groupedActions := core.NewSCGroupedActions(by, detailed)
	for block := range blocks {
		groupedActions.AddBlock(block, verifiedTokensMap, XRC20Map, NFTTokensMap)
	}
	return groupedActions, nil
}

func OneToOneCount(blockchain core.Blockchain, globPattern string,
	start, end uint64) (*core.OneToOneTxnMap, error) {
	blocks, err := YieldAllBlocks(globPattern, blockchain, start, end)
	if err != nil {
		return nil, err
	}
	oneToOneCount := core.NewOneToOneMap()
	for block := range blocks {
		oneToOneCount.AddBlock(block)
	}
	return oneToOneCount, nil
}

func CountIndexationPayload(blockchain core.Blockchain, globPattern string, start, end uint64) (int, error) {
	blocks, err := YieldAllBlocks(globPattern, blockchain, start, end)
	if err != nil {
		return 0, err
	}
	txCounter := core.NewIndexationPayloadCounter()
	for block := range blocks {
		txCounter.AddBlock(block)
	}
	return (int)(*txCounter), nil
}

func CountSignedTransactionPayload(blockchain core.Blockchain, globPattern string, start, end uint64) (int, error) {
	blocks, err := YieldAllBlocks(globPattern, blockchain, start, end)
	if err != nil {
		return 0, err
	}
	txCounter := core.NewSignedTransactionPayloadCounter()
	for block := range blocks {
		txCounter.AddBlock(block)
	}
	return (int)(*txCounter), nil
}

func CountNoPayload(blockchain core.Blockchain, globPattern string, start, end uint64) (int, error) {
	blocks, err := YieldAllBlocks(globPattern, blockchain, start, end)
	if err != nil {
		return 0, err
	}
	txCounter := core.NewNoPayloadCounter()
	for block := range blocks {
		txCounter.AddBlock(block)
	}
	return (int)(*txCounter), nil
}

func CountOtherPayload(blockchain core.Blockchain, globPattern string, start, end uint64) (int, error) {
	blocks, err := YieldAllBlocks(globPattern, blockchain, start, end)
	if err != nil {
		return 0, err
	}
	txCounter := core.NewOtherPayloadCounter()
	for block := range blocks {
		txCounter.AddBlock(block)
	}
	return (int)(*txCounter), nil
}

func CountNoSolid(blockchain core.Blockchain, globPattern string, start, end uint64) (int, error) {
	blocks, err := YieldAllBlocks(globPattern, blockchain, start, end)
	if err != nil {
		return 0, err
	}
	txCounter := core.NewNoSolidCounter()
	for block := range blocks {
		txCounter.AddBlock(block)
	}
	return (int)(*txCounter), nil
}

func GroupConflicting(blockchain core.Blockchain, globPattern string, start, end uint64) (*core.GroupedConflictsCount, error) {
	blocks, err := YieldAllBlocks(globPattern, blockchain, start, end)
	if err != nil {
		return nil, err
	}
	result := core.NewGroupedConflictsCount()
	for block := range blocks {
		result.AddBlock(block)
	}
	return result, nil
}

func CountConflicting(blockchain core.Blockchain, globPattern string, start, end uint64) (int, error) {
	blocks, err := YieldAllBlocks(globPattern, blockchain, start, end)
	if err != nil {
		return 0, err
	}
	txCounter := core.NewConflictsCounter()
	for block := range blocks {
		txCounter.AddBlock(block)
	}
	return (int)(*txCounter), nil
}

func CountConflictingOverTime(blockchain core.Blockchain, globPattern string,
	start, end uint64, duration time.Duration) (*core.TimeGroupedConflictsCount, error) {
	blocks, err := YieldAllBlocks(globPattern, blockchain, start, end)
	if err != nil {
		return nil, err
	}
	result := core.NewTimeGroupedConflictsCount(duration)
	for block := range blocks {
		result.AddBlock(block)
	}
	return result, nil
}

func CountIndexationPayloadOverTime(blockchain core.Blockchain, globPattern string,
	start, end uint64, duration time.Duration) (*core.TimeGroupedIndexationCount, error) {
	blocks, err := YieldAllBlocks(globPattern, blockchain, start, end)
	if err != nil {
		return nil, err
	}
	result := core.NewTimeGroupedIndexationCount(duration)
	for block := range blocks {
		result.AddBlock(block)
	}
	return result, nil
}

func CountSgnTransactionPayloadOverTime(blockchain core.Blockchain, globPattern string,
	start, end uint64, duration time.Duration) (*core.TimeGroupedSignedTransactionCount, error) {
	blocks, err := YieldAllBlocks(globPattern, blockchain, start, end)
	if err != nil {
		return nil, err
	}
	result := core.NewTimeGroupedSignedTransactionCount(duration)
	for block := range blocks {
		result.AddBlock(block)
	}
	return result, nil
}

func CountNoPayloadOverTime(blockchain core.Blockchain, globPattern string,
	start, end uint64, duration time.Duration) (*core.TimeGroupedNoPayloadCount, error) {
	blocks, err := YieldAllBlocks(globPattern, blockchain, start, end)
	if err != nil {
		return nil, err
	}
	result := core.NewTimeGroupedNoPayloadCount(duration)
	for block := range blocks {
		result.AddBlock(block)
	}
	return result, nil
}

func GroupByIndex(blockchain core.Blockchain, globPattern string, start, end uint64) (*core.GroupedByIndexCount, error) {
	blocks, err := YieldAllBlocks(globPattern, blockchain, start, end)
	if err != nil {
		return nil, err
	}
	result := core.NewGroupedByIndexCount()
	for block := range blocks {
		result.AddBlock(block)
	}
	for k, v := range result.GroupedIndexes {
		if v < 50 { // Removing INDEXES that only appear a small number of times
			delete(result.GroupedIndexes, k)
		}
	}
	return result, nil
}

func GroupByIndexOverTime(blockchain core.Blockchain, globPattern string,
	start, end uint64, duration time.Duration) (*core.TimeGroupedByIndexCount, error) {
	blocks, err := YieldAllBlocks(globPattern, blockchain, start, end)
	if err != nil {
		return nil, err
	}
	result := core.NewTimeGroupedByIndexCount(duration)
	for block := range blocks {
		result.AddBlock(block)
	}
	for k, v := range result.TimeIndexesCounts {
		for k2, v2 := range v {
			if v2 < 50 { // Removing INDEXES that only appear a small number of times
				delete(result.TimeIndexesCounts[k], k2)
			}
		}
	}
	return result, nil
}

func GroupByAddress(blockchain core.Blockchain, globPattern string, start, end uint64) (*core.GroupedByAddressCount, error) {
	blocks, err := YieldAllBlocks(globPattern, blockchain, start, end)
	if err != nil {
		return nil, err
	}
	result := core.NewGroupedByAddressCount()
	for block := range blocks {
		result.AddBlock(block)
	}
	for k, v := range result.GroupedAddresses {
		if v < 10 { // Removing addresses that only appear once as those are not of importance,
			// TODO: Should I remove 2 too????
			delete(result.GroupedAddresses, k)
		}
	}
	return result, nil
}

func AvegareValueTransactions(blockchain core.Blockchain, globPattern string, start, end uint64) (float64, error) {
	blocks, err := YieldAllBlocks(globPattern, blockchain, start, end)
	if err != nil {
		return 0, err
	}
	avgValues := core.NewAverageValuesCounter()
	for block := range blocks {
		avgValues.AddBlock(block)
	}
	sum := 0
	for _, v := range *avgValues {
		sum += v
	}
	avg := float64(sum) / float64(len(*avgValues))

	median, min, max := computeMedian(*avgValues)

	fmt.Printf("Median is: %f, max is: %d, min is: %d\n", median, max, min)
	
	return avg, nil
}

func AvegareValueTransactionsOverTime(blockchain core.Blockchain, globPattern string, 
	start, end uint64, duration time.Duration) (*core.TimeAverageValuesCount, error) {
	blocks, err := YieldAllBlocks(globPattern, blockchain, start, end)
	if err != nil {
		return nil, err
	}

	avgValues := core.NewTimeAverageValuesCount(duration)
	for block := range blocks {
		avgValues.AddBlock(block)
	}

	retObj := &core.TimeAverageValuesCount{
		GroupedBy: duration,
		FinalAverages: make(map[time.Time]core.Stats),
	}

	for k, v := range avgValues.AverageValues {
		sum := 0
		for _, val := range v {
			sum += val
		}
		avgF := float64(sum) / float64(len(v))
		median, min, max := computeMedian(v)

		retObj.FinalAverages[k] = core.Stats{
			Average: avgF,
			Median: median,
			Max: max,
			Min: min,
		}
	}

	return retObj, nil
}

func computeMedian(arr []int) (float64, int, int) {
	sort.Ints(arr) // Sort the array in ascending order

	n := len(arr)
	if n%2 == 1 {
		// Length is odd, return the middle element
		return float64(arr[n/2]), arr[0], arr[len(arr)-1]
	} else {
		// Length is even, return the average of the two middle elements
		return float64(arr[n/2-1]+arr[n/2]) / 2.0, arr[0], arr[len(arr)-1]
	}
}

func AvegareMessagesPerBlock(blockchain core.Blockchain, globPattern string, start, end uint64) (float64, error) {
	blocks, err := YieldAllBlocks(globPattern, blockchain, start, end)
	if err != nil {
		return 0, err
	}
	msgsCounter := core.NewAverageMessagePerBlock()
	for block := range blocks {
		msgsCounter.AddBlock(block)
	}

	sum := int64(0)
	for _, m := range *msgsCounter {
		sum += int64(m)
	}
	avgMsgs := float64(sum) / float64(len(*msgsCounter))

	return avgMsgs, nil
}

func AvegareMessagesPerBlockOverTime(blockchain core.Blockchain, globPattern string, 
	start, end uint64, duration time.Duration) (*core.TimeAverageMessagesPerBlockCount, error) {
	blocks, err := YieldAllBlocks(globPattern, blockchain, start, end)
	if err != nil {
		return nil, err
	}
	msgValues := core.NewTimeAverageMessagesPerBlockCount(duration)
	for block := range blocks {
		msgValues.AddBlock(block)
	}

	retObj := &core.TimeAverageMessagesPerBlockCount{
		GroupedBy: duration,
		FinalAverages: make(map[time.Time]int64),
	}

	for k, v := range msgValues.AverageValues {

		sum := int64(0)
		for _, m := range v {
			sum += m
		}
		avgVals := float64(sum) / float64(len(v))
		retObj.FinalAverages[k] = int64(avgVals)
	}

	return retObj, nil
}

func AvegareTimeMilestones(blockchain core.Blockchain, globPattern string, start, end uint64) (float64, error) {
	blocks, err := YieldAllBlocks(globPattern, blockchain, start, end)
	if err != nil {
		return 0, err
	}
	timeValues := core.NewTimeMilestonesCounter()
	for block := range blocks {
		timeValues.AddBlock(block)
	}

	var tV []int64
	for _, val := range *timeValues {
		tV = append(tV, val)
	}

	sort.Slice(tV, func(i, j int) bool {
		return tV[i] < tV[j]
	})

	differences := make([]int64, len(tV)-1)
	for i := 1; i < len(tV); i++ {
		differences[i-1] = tV[i] - tV[i-1]
	}

	sum := int64(0)
	for _, diff := range differences {
		sum += diff
	}
	avgTimes := float64(sum) / float64(len(differences))

	return avgTimes, nil
}

func AvegareTimeMilestonesOverTime(blockchain core.Blockchain, globPattern string, 
	start, end uint64, duration time.Duration) (*core.TimeAverageMilestonesTimeCount, error) {
	blocks, err := YieldAllBlocks(globPattern, blockchain, start, end)
	if err != nil {
		return nil, err
	}
	timeValues := core.NewTimeAverageMilestonesTimeCount(duration)
	for block := range blocks {
		timeValues.AddBlock(block)
	}

	retObj := &core.TimeAverageMilestonesTimeCount{
		GroupedBy: duration,
		FinalAverages: make(map[time.Time]int64),
	}

	for k, v := range timeValues.AverageValues {

		var tV []int64
		for _, val := range v {
			tV = append(tV, val)
		}

		sort.Slice(tV, func(i, j int) bool {
			return tV[i] < tV[j]
		})

		differences := make([]int64, len(tV)-1)
		for i := 1; i < len(tV); i++ {
			differences[i-1] = tV[i] - tV[i-1]
		}

		sum := int64(0)
		for _, diff := range differences {
			sum += diff
		}
		avgTimes := float64(sum) / float64(len(differences))
		retObj.FinalAverages[k] = int64(avgTimes)
	}

	return retObj, nil
}

func GroupSgndTransactionsByIndex(blockchain core.Blockchain, globPattern string, start, end uint64) (*core.GroupedSgnTransactionsByIndexCount, error) {
	blocks, err := YieldAllBlocks(globPattern, blockchain, start, end)
	if err != nil {
		return nil, err
	}
	result := core.NewGroupedSgnTransactionsByIndexCount()
	for block := range blocks {
		result.AddBlock(block)
	}

	for k, v := range result.GroupedIndexes {
		if v < 10 { // Removing INDEXES that only appear a small number of times
			// TODO: Should I remove more??
			delete(result.GroupedIndexes, k)
		}
	}

	return result, nil
}