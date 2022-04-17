package core

import (
	"compress/gzip"
	"fmt"
	"io"
	"log"
	"os"
	"sort"
	"strconv"
	"os/exec"
	"strings"
	"bytes"
)

const (
	BatchSize uint64 = 100000
)

func MakeFilename(filePath string, first, last uint64) string {
	splitted := strings.SplitN(filePath, ".", 2)
	return fmt.Sprintf("%s-%d--%d.%s", splitted[0], first, last, splitted[1])
}

func MakeErrFilename(filePath string, first, last uint64) string {
	splitted := strings.SplitN(filePath, ".", 2)
	return fmt.Sprintf("%s-%d--%d-errors.%s", splitted[0], first, last, splitted[1])
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
		fmt.Println(err)
		return 0, err
	}
	//return by converting int64 to uint64
	return uint64(output), err  
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
