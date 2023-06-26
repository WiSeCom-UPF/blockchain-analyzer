package core

import (
	"io"
	"os"
	"log"
	"fmt"
	"sort"
	"bytes"
	"strconv"
	"os/exec"
	"strings"
	"math/big"
	"compress/gzip"
)

type Pair struct {
	Key   string
	Value int
}

type PairList []Pair

func (p PairList) Len() int           { return len(p) }
func (p PairList) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }
func (p PairList) Less(i, j int) bool { return p[i].Value < p[j].Value }

func SortMapStringU64(p map[string]int) (PairList){
	p1 := make(PairList, len(p))
	valCounter := 0
	i := 0
	for k, v := range p {
		valCounter += v
		p1[i] = Pair{k, v}
		i++
	}

	sort.Sort(sort.Reverse(p1))

	return p1
}

const (
	BatchSize uint64 = 100 // Changed this
)

func MakeFilename(filePath string, first, last uint64) string {
	splitted := strings.SplitN(filePath, ".", 2)
	return fmt.Sprintf("%s-%d--%d.%s", splitted[0], first, last, splitted[1])
}

func MakeErrFilename(filePath string, first, last uint64) string {
	splitted := strings.SplitN(filePath, ".", 2)
	return fmt.Sprintf("%s-%d--%d-errors.%s", splitted[0], first, last, splitted[1])
}

func SplitItems2Spaces(itemsString string) ([]string){
	itemsSlice := strings.Split(itemsString, "  ")
	for i:= 0; i< len(itemsSlice); i = i +1{
		itemsSlice[i] = strings.TrimSpace(itemsSlice[i])
	}
	return itemsSlice
}

func SpliceToMapStrSTr(itemsSlice []string) (map[string]string){
	var itemsMap = map[string]string{}
	for j:= 0; j< len(itemsSlice); {
		itemsMap[strings.ToUpper(itemsSlice[j])] = itemsSlice[j+1]
		j = j + 2
	}
	return itemsMap
}

func SliceToMapStrStr(staticVarName string) (map[string]string){
	if staticVarName == "VerifiedTokens"{
		itemsSlice := SplitItems2Spaces(VerifiedSCIotex)
		return SpliceToMapStrSTr(itemsSlice)
	} else if staticVarName == "XRCTokens"{
		itemsSlice := SplitItems2Spaces(ERC20TokensIotex)
		return SpliceToMapStrSTr(itemsSlice)
	} else if staticVarName == "NFTTokens"{
		itemsSlice := SplitItems2Spaces(NFTTokenIotex)
		return SpliceToMapStrSTr(itemsSlice)
	} else {
		return nil
	}
}

// provided ioctl program with iotex config is installed
func ConvertEthAddrToIotexAddr(Ethaddr string) string {

    cmd := exec.Command("ioctl", "account", "ethaddr", Ethaddr)

    var out bytes.Buffer
    cmd.Stdout = &out

    err := cmd.Run()

    if err != nil {
        log.Fatal(err)
    }

    return (strings.Split(out.String(), ` -`)[0])
}

func CreateFile(name string) (io.WriteCloser, error) {
	file, err := os.OpenFile(name, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return nil, err
	}
	if strings.HasSuffix(name, ".gz") {
		return gzip.NewWriter(file), nil
	}
	return file, nil
}

func OpenFile(name string) (io.ReadCloser, error) {
	file, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	if strings.HasSuffix(name, ".gz") {
		return gzip.NewReader(file)
	}
	return file, nil
}

func SortU64Slice(values []uint64) {
	sort.Slice(values, func(i, j int) bool { return values[i] < values[j] })
}

// convert hex such as , "0x5cd567" to decimal(uint64)
func HexStringToDecimal(hexaString string) (uint64, error) {
	// replace 0x or 0X with empty String 
	numberStr := strings.Replace(hexaString, "0x", "", -1)
	numberStr = strings.Replace(numberStr, "0X", "", -1)
	// converting hex to decimal int64
	output, err := strconv.ParseInt(numberStr, 16, 64)

	if err != nil {
		fmt.Println(hexaString)
		fmt.Println(err)
		return 0, err
	}
	//return by converting int64 to uint64
	return uint64(output), err  
}

// convert hex such as , "0x5cd567" to decimal(uint64)
func HexStringToDecimalIntBig(hexaString string) (*big.Int) {
	// replace 0x or 0X with empty String 
	numberStr := strings.Replace(hexaString, "0x", "", -1)
	numberStr = strings.Replace(numberStr, "0X", "", -1)
	i := new(big.Int)
	i.SetString(numberStr, 16)
	//return by converting int64 to uint64
	return i  
} 

func MakeFileProcessor(f func(string) error) func(string) {
	return func(filename string) {
		log.Printf("processing %s", filename)
		if err := f(filename); err != nil {
			log.Printf("error while processing %s: %s", filename, err.Error())
		} else {
			log.Printf("done processing %s", filename)
		}
	}
}
