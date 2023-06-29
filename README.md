# blockchain-analyzer

[![CircleCI](https://circleci.com/gh/danhper/blockchain-analyzer.svg?style=svg)](https://circleci.com/gh/danhper/blockchain-analyzer)

CLI tool to fetch and analyze transactions data from several blockchains.

Currently supported blockchains:

- [Tezos](https://tezos.com/)
- [EOS](https://eos.io/)
- [XRP](https://ripple.com/xrp/)
- [IOTEX](https://iotex.io/)
- [IOTA](https://www.iota.org)
- [Helium](https://www.helium.com/)

## Installation

### Static binaries

We provide static binaries for Windows, macOS and Linux with each [release](https://github.com/danhper/blockchain-analyzer/releases)

### From source

Go needs to be installed. The tool can then be installed by running

```
go get github.com/danhper/blockchain-analyzer/cmd/blockchain-analyzer
```

To run IoTeX related command, run the following:
```
# from root of blockchain-analyzer, change directory into cmd/blockchain-analyzer
$ cd cmd/blockchain-analyzer

# from here run the normal command but using go run main.go
$ go run main.go iotex fetch -o iotex-blocks.jsonl.gz --start 1 --end 19500000  
```


To run IOTA related command, run the following:
```
# from root of blockchain-analyzer, change directory into cmd/blockchain-analyzer
$ cd cmd/blockchain-analyzer

# from here run the normal command but using go run main.go
$ go run main.go iota fetch -o iota-blocks.jsonl.gz --start 6000000 --end 6000100  
```

### Dependencies
All Golang dependencies will be installed automatically according to the go.mod file
#### IOTA
For  IOTA the external gRocksdb dependency is needed. To install it, follow the instructions [here](https://github.com/linxGnu/grocksdb).

## Usage

### Fetching data

The `fetch` command can be used to fetch the data:

```
blockchain-analyzer BLOCKCHAIN fetch -o OUTPUT_FILE --start START_BLOCK --end END_BLOCK

# for example from 500,000 to 699,999 inclusive:
blockchain-analyzer eos fetch -o eos-blocks.jsonl.gz --start 500000 --end 699999
```
#### IOTA Data Source
To donwload IOTA data you can either provide a URL corresponding to a node endpoint or use a RocksDB locally downloaded to fetch the data from it. 

Fetching from Rocks DBs provide better performance and faster downloads than node endpoints. IOTA Foundation RocksDbs can be found [here](https://chrysalis-dbfiles.iota.org/?prefix=dbs/)

To either download from a RocksDB or node endpoint, the following ENV vars have to be set
```
Iota_NODE_ENDPOINT      url of the node endpoint
Iota_ROCKSDB_PATH       local absolute path of the location of the donwloaded RocksDB
Iota_USE_DATABASE       set it to 'true' to use RocksDB, or 'false' to use a node endpoint
```

### Data format

The data has the following format

- One block per line, including transactions, formatted in JSON. Documentation of block format can be found in each chain documentation
  - [EOS](https://developers.eos.io/manuals/eos/latest/nodeos/plugins/chain_api_plugin/api-reference/index#operation/get_block)
  - [Tezos](https://tezos.gitlab.io/api/rpc.html#get-block-id)
  - [XRP](https://xrpl.org/ledger.html)
  - [IoTeX](https://docs.iotex.io/basic-concepts/blockchain-actions)
  - IOTA
    - For IOTA, there are no blocks per se in the Distributed Ledger as IOTA uses a Directed Acyclic Graph as the data structure for its network. Therefore, in this project to download an IOTA block means to download all the messages that were referenced and confirmed by a given milestone index. For example, the command `go run main.go iota fetch -o iota-blocks.jsonl.gz --start 6000000 --end 6000100` downloads all the messages that were referenced by milestones 6000000 to 6000100 inclusive.  
    - The data format of the downloaded data corresponds to blocks of data where the block number is the milestone index, and a list of messages. To see the block declaration see file iota/iota.go (line 477).
    - More info on IOTA'S [design](https://wiki.iota.org/learn/about-iota/tangle/) 
  - [Helium](https://docs.helium.com/api/blockchain/blocks)
  
- Grouped in files of 100,000 blocks each, suffixed by the block range (e.g. `eos-blocks-500000--599999.jsonl` and `eos-blocks-600000--699999.jsonl` for the above)
- Gziped if the `.gz` extension is added to the output file name (recommended)

### Checking data

The `check` command can then be used to check the fetched data. It will ensure that all the block from `--start` to `--end` exist in the given files, and output the missing blocks into `missing.jsonl` if any.

```
blockchain-analyzer eos check -p 'eos-blocks*.jsonl.gz' -o missing.jsonl --start 500000 --end 699999
```

### Analyzing data

The simplest way to analyze the data is to provide a configuration file about what to analyze and run the tool with the following command.

```
blockchain-analyzer <tezos|eos|xrp> bulk-process -c config.json -o tmp/results.json
```

The tool's help also contains information about what other commands can be used

```plain
$ ./build/blockchain-analyzer -h
NAME:
   blockchain-analyzer - Tool to fetch and analyze blockchain transactions

USAGE:
   blockchain-analyzer [global options] command [command options] [arguments...]

COMMANDS:
   eos      Analyze EOS data
   tezos    Analyze Tezos data
   xrp      Analyze XRP data
   iotex    Analyze IoTeX data
   helium   Analyze Helium data
   help, h  Shows a list of commands or help for one command

GLOBAL OPTIONS:
   --cpu-profile value  Path where to store the CPU profile
   --help, -h           show help (default: false)

# the following is also available for xrp and tezos
$ ./build/blockchain-analyzer eos -h
NAME:
   blockchain-analyzer eos - Analyze EOS data

USAGE:
   blockchain-analyzer eos command [command options] [arguments...]

COMMANDS:
   export-transfers              Export all the transfers to a CSV file
   fetch                         Fetches blockchain data
   check                         Checks for missing blocks in data
   count-transactions            Count the number of transactions in the data
   group-actions                 Count and groups the number of "actions" in the data
   group-actions-over-time       Count and groups per time the number of "actions" in the data
   count-transactions-over-time  Count number of "transactions" over time in the data
   bulk-process                  Bulk process the data according to the given configuration file
   export                        Export a subset of the fields to msgpack format for faster processing
   help, h                       Shows a list of commands or help for one command
IOTEX SPECIFIC ADDITIONAL COMMANDS
   count-gov-transactions        Count the number of governance transactions in the data
   count-sc-sign                 Count the function signature in smart contract transactions
   count-sc-created-over-time    Count number of smart contracts created over time in the data
   count-transactions-by-address Count the number of transactions in the data by address of either sender or recover
   count-transactions-by-address-over-time  Count the number of transactions in the data by address of either sender or recover per fixed time interval
   count-empty-blocks            Count the number of empty blocks in the data
   count-empty-blocks-over-time  Count the number of empty blocks per fixed time interval in the data
   count-zero-txn-blocks-over-time Count the number of zero txn blocks per fixed time interval in the data
   count-zero-txn-blocks         Count the number of zero txn blocks in the data
   sc-group-actions              Count and groups the number of \"actions\" in the data, along with type of SC i.e. verified, unverified, XRC20 or NFT
   count-gov-transactions-over-time Count the number of governance transactions in the data over the fixed intervals of time
   count-mining-history          Count the miner history as per number of blocks produced
   count-one-to-one-txns         Count transactions between sender and receiver, put in a map
   count-mining-history-over-time Count the miner history as per number of blocks produced over time

IOTA SPECIFIC COMMANDS
count-messages                   Count the number of IOTA messages in the data
count-messages-over-time         Count number of messages over time in the data
count-empty-blocks               Count the number of empty blocks in the data, i.e. blocks with no messages
count-empty-blocks-over-time     Count number of empty blocks (i.e. blocks with no messages) over time in the data
count-indexation-payload         Count the number of messages in the data which have an indexation payload
count-indexation-payload-over-time  Count number of messages with indexation payload over time in the data
count-signed-transaction-payload Count the number of messages in the data which have a signed transaction payload
count-signed-transaction-payload-over-time   Count number of messages with signed transaction payload over time in the data
count-no-payload                 Count the number of messages in the data which have a no payload
count-no-payload-over-time       Count number of messages with no payload over time in the data
count-other-payload              Count the number of messages in the data which have a payload other than 0,1,2
count-conflicts                  Count the number of messages in the data which have a transaction marked as conflicting
count-conflicts-over-time        Count number of messages which have a transaction marked as conflicting over time in the data
group-conflicts                  Count and group the number of messages in the data which have a transaction marked as conflicting per conflict type
group-by-index                   Count and group the number of messages in the data which have indexation payload per index
group-by-index-over-time         Count and group the number of messages in the data which have indexation payload per index over time
group-by-output-address          Count and group the number of messages in the data which have transaction payload per outputs address
average-value-spent-transaction  Returns the average value spent per transaction after computing the mean among all value transactions
average-value-spent-transaction-over-time Returns the average value spent per transaction after computing the mean among all value transactions over time
average-time-between-milestones Returns the average time between the issuing of two consecutive milestones
average-time-between-milestones-over-time Returns the average time between the issuing of two consecutive milestones over time
group-signed-transactions-by-index  Count and group the number of messages in the data with a signed transaction payload by index
average-number-messages-per-block Compute the average number of messages (i.e. the number of messages referenced by each milestone) per block in the data
average-number-messages-per-block-over-time Compute the average number of messages (i.e. the number of messages referenced by each milestone) per block in the data over time

OPTIONS:
   --help, -h  show help (default: false)
```

### Interpreting results

We provide Python scripts to plot and generate table out of the data from the analysis.
Please check the [bc-data-analyzer](./bc-data-analyzer) directory for more information.

## Dataset

All the data used in our paper mentioned below can be downloaded from the following link:

https://imperialcollegelondon.box.com/s/jijwo76e2pxlbkuzzt1yjz0z3niqz7yy

This includes data from October 1, 2019 to April 30, 2020 for EOS, Tezos and XRP, for IoT Blockchains such as IoTeX and Helium the blocks are regarded from the genesis until around September 2022, which corresponds to the following blocks:

| Blockchain | Start block | End block |
| ---------- | ----------: | --------: |
| EOS        |    82152667 | 118286375 |
| XRP        |    50399027 |  55152991 |
| Tezos      |      630709 |    932530 |
| IoTeX      |      1      |  16600000 |
| IOTA       |      4      |   6400000 |
| Helium     |      1      |   1531124 |

Please refer to the [Data format](https://github.com/danhper/blockchain-analyzer#data-format) section above for a description of the data format.

## Supporting other blockchains

Although the framework currently only supports EOS, Tezos and XRP, it has been designed to easily support other blockchains.
The three following interfaces need to be implemented in order to do so:

```go
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
        GovernanceTransactionsCount()  int
        GetTxnP2Plist() []string
        SCCount(string) (int, []string)
        Time() time.Time
        GetMiner() string
        ListActions() []Action
}

type Action interface {
        Sender() string
        Receiver() string
        Name() string
}
```

We also provide a utilities to make methods such as `FetchData` easier to implement.
[Existing implementations](https://github.com/danhper/blockchain-analyzer/blob/master/tezos/tezos.go) can be used as a point of reference for how a new blockchain can be supported.

## Academic work

This tool has originally been created to analyze data for the following paper: [Revisiting Transactional Statistics of High-scalability Blockchain](https://arxiv.org/abs/2003.02693), presented at [IMC'20](https://conferences.sigcomm.org/imc/2020/accepted/).  
If you are using this for academic work, we would be thankful if you could cite it.

```
@misc{perez2020revisiting,
    title={Revisiting Transactional Statistics of High-scalability Blockchain},
    author={Daniel Perez and Jiahua Xu and Benjamin Livshits},
    year={2020},
    eprint={2003.02693},
    archivePrefix={arXiv},
    primaryClass={cs.CR}
}
```
