package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"reflect"
	"strings"
	"time"
)

func linesStringCount(s string) int {
	n := strings.Count(s, "\n")
	if len(s) > 0 && !strings.HasSuffix(s, "\n") {
		n++
	}
	return n
}

func main() {

	path, err := os.Getwd()
	if err != nil {
		log.Panic(err)
	}

	LOG_FILE := path + "/blockchain-analyzer/cmd/blockchain-analyzer/tmp/logs/script_logs.log"
	logFile, err := os.OpenFile(LOG_FILE, os.O_APPEND|os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		fmt.Println("Error opnening log file")
		log.Panic(err)
	}
	defer logFile.Close()
	log.SetOutput(logFile)
	log.SetFlags(log.Lshortfile | log.LstdFlags)


	num_blocks := 200

	starting_index := 20080
	latest_index := 283822 

	for i := starting_index; i < latest_index; i = i + num_blocks {
		command := fmt.Sprintf("cd blockchain-analyzer/cmd/blockchain-analyzer/; go run main.go iota fetch -o tmp/data-F/iota-blocks.jsonl.gz --start %d --end %d;", i, i+(num_blocks-1))
		fmt.Println("Executing command: " + command)
		cmd := exec.Command("/bin/sh", "-c", command)
		stdout, err := cmd.Output()
		if err != nil {
			fmt.Println("Error ocurred: " + string(err.Error()))
			fmt.Println(reflect.ValueOf(err).Type())
			//if errors.Is(err)
			if string(err.Error()) == "exit status 1" {

				log.Println(command)

				time.Sleep(2 * time.Second)
				continue
			} else {
				return
			}
		}
		if linesStringCount(string(stdout)) > num_blocks {
			fmt.Println("Some error occured")
			i = i - num_blocks
			time.Sleep(2 * time.Second) 
			continue
		}
		fmt.Println("Success: " + string(stdout))
	}
}
