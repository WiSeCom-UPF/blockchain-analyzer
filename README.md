# blockchain-analyzer

[![CircleCI](https://circleci.com/gh/danhper/blockchain-analyzer.svg?style=svg)](https://circleci.com/gh/danhper/blockchain-analyzer)

CLI tool to fetch and analyze transactions data from several blockchains.

Currently supported blockchains:

- [Tezos](https://tezos.com/)
- [EOS](https://eos.io/)
- [XRP](https://ripple.com/xrp/)
- [IOTEX](https://iotex.io/)

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
$ go run main.go iotex fetch -o iotex-blocks.jsonl.gz --start 1 --end 16600000  
```

## Usage

### Fetching data

The `fetch` command can be used to fetch the data:

```
blockchain-analyzer BLOCKCHAIN fetch -o OUTPUT_FILE --start START_BLOCK --end END_BLOCK

# for example from 500,000 to 699,999 inclusive:
blockchain-analyzer eos fetch -o eos-blocks.jsonl.gz --start 500000 --end 699999
```

### Data format

The data has the following format

- One block per line, including transactions, formatted in JSON. Documentation of block format can be found in each chain documentation
  - [EOS](https://developers.eos.io/manuals/eos/latest/nodeos/plugins/chain_api_plugin/api-reference/index#operation/get_block)
  - [Tezos](https://tezos.gitlab.io/api/rpc.html#get-block-id)
  - [XRP](https://xrpl.org/ledger.html)
  - [IoTeX](https://docs.iotex.io/basic-concepts/blockchain-actions)
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

Configuration files used for [our paper](https://arxiv.org/abs/2003.02693) can be found in the [config](./config) directory.

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

OPTIONS:
   --help, -h  show help (default: false)
```

### Interpreting results

We provide Python scripts to plot and generate table out of the data from the analysis.
Please check the [bc-data-analyzer](./bc-data-analyzer) directory for more information.

## Dataset

All the data used in our paper mentioned below can be downloaded from the following link:

https://imperialcollegelondon.box.com/s/jijwo76e2pxlbkuzzt1yjz0z3niqz7yy

This includes data from October 1, 2019 to April 30, 2020 for EOS, Tezos and XRP, which corresponds to the following blocks:

| Blockchain | Start block | End block |
| ---------- | ----------: | --------: |
| EOS        |    82152667 | 118286375 |
| XRP        |    50399027 |  55152991 |
| Tezos      |      630709 |    932530 |
| IoTeX      |      1      |  16600000 |

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
