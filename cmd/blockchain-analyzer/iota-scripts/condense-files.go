package main

// Run this command to group all the files and their paths inside the file_list.txt file later used by this program to condense the data.
// find  /Users/someuser/Documents/ALL_DATA/blocks-6000000-6200000 -type f -name "*.gz" | sort > file_list.txt

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"log"
	"math"
	"os"
	"regexp"
)

func unzip(source string) ([]byte, error) {
	reader, err := os.Open(source)
	if err != nil {
		return nil, fmt.Errorf("Error opening block file %s: %s", source, err)
	}
	defer reader.Close()
	archive, err := gzip.NewReader(reader)
	if err != nil {
		return nil, fmt.Errorf("Error opening gzip reader for block file %s: %s", source, err)
	}
	defer archive.Close()
	buf := new(bytes.Buffer)
	io.Copy(buf, archive)
	// fmt.Println("buf read from gz file :", buf)
	return buf.Bytes(), nil
}

func chunkSlice(slice []string, chunkSize int) [][]string {
	var chunks [][]string
	for i := 0; i < len(slice); i += chunkSize {
		end := i + chunkSize

		if end > len(slice) {
			end = len(slice)
		}

		chunks = append(chunks, slice[i:end])
	}

	return chunks
}

func extractNumber(input string, which string) (string, error) {
	var re *regexp.Regexp
	if which == "second" {
		re = regexp.MustCompile(`iota-blocks-\d+--(\d+)\.jsonl\.gz`)
	} else if which == "first" {
		re = regexp.MustCompile(`iota-blocks-(\d+)--\d+\.jsonl\.gz`)
	} else {
		return "", fmt.Errorf("You must indicate whether to extract the first of last number from regex")
	}

	match := re.FindStringSubmatch(input)
	if len(match) != 2 {
		return "", fmt.Errorf("no match found")
	}
	return match[1], nil
}

func concatenateBlocks(fileNames []string, resFileName string) error {

	fileOut, err := os.Create("blocks-done/" + resFileName)
	defer fileOut.Close()
	if err != nil {
		return fmt.Errorf("Error creating result file %s: %s", resFileName, err)
	}

	gzWriter := gzip.NewWriter(fileOut)
	defer gzWriter.Close()

	//fmt.Println("Combining files: ", fileNames)

	for i := 0; i < len(fileNames); i++ {

		//fmt.Println("unzipping: ", fileNames[i])
		by, err := unzip(fileNames[i])
		if err != nil {
			log.Printf("Error with file %s in destination file %s: %s\n", fileNames[i], resFileName, err)
			errR := os.Remove("blocks-done/" + resFileName)
			if errR != nil {
				return fmt.Errorf("ERROR REMOVING FILE: %s: %s", resFileName, errR)
			}
			return fmt.Errorf("Unziping file %s: %s", fileNames[i], err)
		}
		_, err = gzWriter.Write(by)
		if err != nil {
			log.Printf("Error with file %s in destination file %s: %s\n", fileNames[i], resFileName, err)
			errR := os.Remove("blocks-done/" + resFileName)
			if errR != nil {
				return fmt.Errorf("ERROR REMOVING FILE: %s: %s", resFileName, errR)
			}
			return fmt.Errorf("Error writing bytes for file %s: %s", fileNames, err)
		}

	}

	return nil

}

func main() {

	path, err := os.Getwd()
	if err != nil {
		log.Panic(err)
	}
	LOG_FILE := path + "/logs.log"
	logFile, err := os.OpenFile(LOG_FILE, os.O_APPEND|os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		fmt.Println("Error opening log file")
		log.Panic(err)
	}
	defer logFile.Close()
	log.SetOutput(logFile)
	log.SetFlags(log.Lshortfile | log.LstdFlags)


	fileList, err := os.Open("file_list.txt")
    if err != nil {
        panic(err)
    }
    defer fileList.Close()
	allFiles := []string{}
    scanner := bufio.NewScanner(fileList)
    for scanner.Scan() { 
		fileName := scanner.Text()
		allFiles = append(allFiles, fileName)
	}

	
	numberFiles := len(allFiles)
	filesPerGroup := 1000

	resultingFiles := int(math.Ceil(float64(numberFiles) / float64(filesPerGroup)))
	fmt.Printf("There are %d files to be processed, resulting in %d files combined\n", numberFiles, resultingFiles)

	chunks := chunkSlice(allFiles, filesPerGroup)
	results := make(chan error, len(chunks))

	for _, someFiles := range chunks {

		go func(x []string) {

			firstFile := x[0]
			lastFile := x[len(x)-1]

			n1, err := extractNumber(firstFile, "first")
			if err != nil {
				results <- err
			} else {
				n2, err := extractNumber(lastFile, "second")
				if err != nil {
					results <- err
				} else {
					resFileName := "iota-blocks-" + n1 + "--" + n2 + ".jsonl.gz"
					results <- concatenateBlocks(x, resFileName)
				}
			}
		}(someFiles)
	}
	// wait for goroutines to finish
	for j := 0; j < len(chunks); j++ {
		errF := <-results
		if errF != nil {
			fmt.Println("Error occured: ", errF)
			//return
		}
	}

}

