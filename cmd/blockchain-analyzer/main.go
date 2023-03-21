package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"runtime/pprof"
	"time"

	"github.com/danhper/blockchain-analyzer/core"
	"github.com/danhper/blockchain-analyzer/eos"
	"github.com/danhper/blockchain-analyzer/processor"
	"github.com/danhper/blockchain-analyzer/tezos"
	"github.com/danhper/blockchain-analyzer/xrp"
	"github.com/danhper/blockchain-analyzer/helium"
	"github.com/danhper/blockchain-analyzer/iotex"
	"github.com/danhper/blockchain-analyzer/iota"
	"github.com/urfave/cli/v2"
)

func addStartFlag(flags []cli.Flag, required bool) []cli.Flag {
	return append(flags, &cli.IntFlag{
		Name:     "start",
		Aliases:  []string{"s"},
		Required: required,
		Value:    0,
		Usage:    "Start block/ledger index",
	})
}

func addEndFlag(flags []cli.Flag, required bool) []cli.Flag {
	return append(flags, &cli.IntFlag{
		Name:     "end",
		Aliases:  []string{"e"},
		Required: required,
		Value:    0,
		Usage:    "End block/ledger index",
	})
}

func addOutputFlag(flags []cli.Flag) []cli.Flag {
	return append(flags, &cli.StringFlag{
		Name:     "output",
		Aliases:  []string{"o"},
		Usage:    "Base output filepath",
		Required: true,
	})
}

func addConfigFlag(flags []cli.Flag) []cli.Flag {
	return append(flags, &cli.StringFlag{
		Name:     "config",
		Aliases:  []string{"c"},
		Usage:    "Configuration file",
		Required: true,
	})
}

func addActionPropertyFlag(flags []cli.Flag) []cli.Flag {
	return append(flags, &cli.StringFlag{
		Name:  "by",
		Value: "name",
		Usage: "Property to group the actions by",
	})
}

func addRangeFlags(flags []cli.Flag, required bool) []cli.Flag {
	return addStartFlag(addEndFlag(flags, required), required)
}

func addFetchFlags(flags []cli.Flag) []cli.Flag {
	return addRangeFlags(addOutputFlag(flags), true)
}

func addPatternFlag(flags []cli.Flag) []cli.Flag {
	return append(flags, &cli.StringFlag{
		Name:     "pattern",
		Aliases:  []string{"p"},
		Value:    "",
		Usage:    "Patterns of files to check",
		Required: true,
	})
}

func addAddressFlag(flags []cli.Flag) []cli.Flag {
	return append(flags, &cli.StringFlag{
		Name:     "address",
		Value:    "",
		Usage:    "address of EOA or a contract",
		Required: false,
	})
}

func addGroupDurationFlag(flags []cli.Flag) []cli.Flag {
	return append(flags, &cli.StringFlag{
		Name:    "duration",
		Aliases: []string{"d"},
		Value:   "6h",
		Usage:   "Duration to group by when counting",
	})
}

func addDetailedFlag(flags []cli.Flag) []cli.Flag {
	return append(flags, &cli.BoolFlag{
		Name:     "detailed",
		Usage:    "Whether to add the details about sender/receivers etc",
		Value:    false,
		Required: false,
	})
}

func addCpuProfileFlag(flags []cli.Flag) []cli.Flag {
	return append(flags, &cli.StringFlag{
		Name:     "cpu-profile",
		Usage:    "Path where to store the CPU profile",
		Value:    "",
		Required: false,
	})
}

func makeAction(f func(*cli.Context) error) func(*cli.Context) error {
	return func(c *cli.Context) error {
		cpuProfile := c.String("cpu-profile")
		if cpuProfile != "" {
			f, err := os.Create(cpuProfile)
			if err != nil {
				return fmt.Errorf("could not create CPU profile: %s", err.Error())
			}
			defer f.Close()
			if err := pprof.StartCPUProfile(f); err != nil {
				return fmt.Errorf("could not start CPU profile: %s", err.Error())
			}
			defer pprof.StopCPUProfile()
		}

		return f(c)
	}
}

func addCommonCommands(blockchain core.Blockchain, commands []*cli.Command) []*cli.Command {
	return append(commands, []*cli.Command{
		{
			Name:  "fetch",
			Flags: addFetchFlags(nil),
			Usage: "Fetches blockchain data",
			Action: makeAction(func(c *cli.Context) error {
				return blockchain.FetchData(c.String("output"), c.Uint64("start"), c.Uint64("end"))
			}),
		},
		{
			Name:  "check",
			Flags: addPatternFlag(addFetchFlags(nil)),
			Usage: "Checks for missing blocks in data",
			Action: makeAction(func(c *cli.Context) error {
				return processor.OutputAllMissingBlockNumbers(
					blockchain, c.String("pattern"), c.String("output"),
					c.Uint64("start"), c.Uint64("end"))
			}),
		},
		{
			Name:  "count-transactions",
			Flags: addPatternFlag(addRangeFlags(nil, false)),
			Usage: "Count the number of transactions in the data",
			Action: makeAction(func(c *cli.Context) error {
				count, err := processor.CountTransactions(
					blockchain, c.String("pattern"),
					c.Uint64("start"), c.Uint64("end"))
				if err != nil {
					return err
				}
				fmt.Printf("found %d transactions\n", count)
				return nil
			}),
		},
		{
			Name:  "count-max-txn-a-block",
			Flags: addPatternFlag(addRangeFlags(nil, false)),
			Usage: "Count the maximum number of transactions in a block in the data",
			Action: makeAction(func(c *cli.Context) error {
				count, blk_num, err := processor.CountMaxTransactionsInBlock(
					blockchain, c.String("pattern"),
					c.Uint64("start"), c.Uint64("end"))
				if err != nil {
					return err
				}
				fmt.Printf("found %d max transactions in a block number %d \n", count, blk_num)
				return nil
			}),
		},
		{
			Name:  "count-gov-transactions",
			Flags: addPatternFlag(addRangeFlags(nil, false)),
			Usage: "iotex specific: Count the number of governance transactions in the data",
			Action: makeAction(func(c *cli.Context) error {
				count, err := processor.CountGovTransactions(
					blockchain, c.String("pattern"),
					c.Uint64("start"), c.Uint64("end"))
				if err != nil {
					return err
				}
				fmt.Printf("found %d governance transactions\n", count)
				return nil
			}),
		},
		{
			Name:  "count-sc-sign",
			Flags: addOutputFlag(addActionPropertyFlag(addPatternFlag(addRangeFlags(nil, false)))),
			Usage: "iotex specific: Count the function signature in smart contract transactions",
			Action: makeAction(func(c *cli.Context) error {
				count, err := processor.CountSCSign(
					blockchain, c.String("pattern"),
					c.Uint64("start"), c.Uint64("end"), c.String("by"))
				if err != nil {
					return err
				}
				fmt.Printf("found %d SC created and %d unique function Signatures \n", count.SCCreated, len(count.SCSignMap))
				return core.Persist(core.SortMapStringU64(count.SCSignMap), c.String("output"))
			}),
		},
		{
			Name:  "count-sc-created-over-time",
			Flags: addGroupDurationFlag(addPatternFlag(addOutputFlag(addRangeFlags(nil, false)))),
			Usage: "iotex specific: Count number of smart contracts created over time in the data",
			Action: makeAction(func(c *cli.Context) error {
				duration, err := time.ParseDuration(c.String("duration"))
				if err != nil {
					return err
				}
				counts, err := processor.CountSCCreatedOverTime(
					blockchain, c.String("pattern"),
					c.Uint64("start"), c.Uint64("end"), duration)
				if err != nil {
					return err
				}
				return core.Persist(counts, c.String("output"))
			}),
		},
		{
			Name:  "count-transactions-by-address",
			Flags: addActionPropertyFlag(addAddressFlag(addPatternFlag(addRangeFlags(nil, false)))),
			Usage: "iotex specific: Count the number of transactions in the data by address of either sender or recover",
			Action: makeAction(func(c *cli.Context) error {
				count, err := processor.CountTransactionsByAddress(
					blockchain, c.String("pattern"),
					c.String("address"), c.String("by"),
					c.Uint64("start"), c.Uint64("end"))
				if err != nil {
					return err
				}
				fmt.Printf("found %d transactions\n", count)
				return nil
			}),
		},
		{
			Name:  "count-transactions-by-address-over-time",
			Flags: addActionPropertyFlag(addGroupDurationFlag(addAddressFlag(addOutputFlag(addPatternFlag(addRangeFlags(nil, false)))))),
			Usage: "iotex specific: Count the number of transactions in the data by address of either sender or recover per fixed time interval",
			Action: makeAction(func(c *cli.Context) error {
				duration, err := time.ParseDuration(c.String("duration"))
				if err != nil {
					return err
				}
				counts, err := processor.CountTransactionsByAddressOverTime(
					blockchain, c.String("pattern"),
					c.String("address"), c.String("by"),
					c.Uint64("start"), c.Uint64("end"), duration)
				if err != nil {
					return err
				}
				return core.Persist(counts, c.String("output"))
			}),
		},
		{
			Name:  "count-empty-blocks",
			Flags: addPatternFlag(addRangeFlags(nil, false)),
			Usage: "iotex specific: Count the number of empty blocks in the data",
			Action: makeAction(func(c *cli.Context) error {
				count, err := processor.CountEmptyBlocks(
					blockchain, c.String("pattern"),
					c.Uint64("start"), c.Uint64("end"))
				if err != nil {
					return err
				}
				fmt.Printf("found %d empty blocks\n", count)
				return nil
			}),
		},
		{
			Name:  "count-empty-blocks-over-time",
			Flags: addPatternFlag(addGroupDurationFlag(addOutputFlag(addRangeFlags(nil, false)))),
			Usage: "iotex specific: Count the number of empty blocks per fixed time interval in the data",
			Action: makeAction(func(c *cli.Context) error {
				duration, err := time.ParseDuration(c.String("duration"))
				if err != nil {
					return err
				}
				counts, err := processor.CountEmptyBlocksOverTime(
					blockchain, c.String("pattern"),
					c.Uint64("start"), c.Uint64("end"), duration)
				if err != nil {
					return err
				}
				return core.Persist(counts, c.String("output"))
			}),
		},
		{
			Name:  "count-zero-txn-blocks-over-time",
			Flags: addPatternFlag(addGroupDurationFlag(addOutputFlag(addRangeFlags(nil, false)))),
			Usage: "iotex specific: Count the number of zero txn blocks per fixed time interval in the data",
			Action: makeAction(func(c *cli.Context) error {
				duration, err := time.ParseDuration(c.String("duration"))
				if err != nil {
					return err
				}
				counts, err := processor.CountZeroTxnBlocksOverTime(
					blockchain, c.String("pattern"),
					c.Uint64("start"), c.Uint64("end"), duration)
				if err != nil {
					return err
				}
				return core.Persist(counts, c.String("output"))
			}),
		},
		{
			Name:  "count-zero-txn-blocks",
			Flags: addPatternFlag(addRangeFlags(nil, false)),
			Usage: "iotex specific: Count the number of zero txn blocks in the data",
			Action: makeAction(func(c *cli.Context) error {
				count, err := processor.CountZeroTxnBlocks(
					blockchain, c.String("pattern"),
					c.Uint64("start"), c.Uint64("end"))
				if err != nil {
					return err
				}
				fmt.Printf("found %d zero transaction blocks\n", count)
				return nil
			}),
		},
		{
			Name: "group-actions",
			Flags: addDetailedFlag(addActionPropertyFlag(
				addPatternFlag(addOutputFlag(addRangeFlags(nil, false))))),
			Usage: "Count and groups the number of \"actions\" in the data",
			Action: makeAction(func(c *cli.Context) error {
				actionProperty, err := core.GetActionProperty(c.String("by"))
				if err != nil {
					return err
				}
				counts, err := processor.GroupActions(
					blockchain, c.String("pattern"),
					c.Uint64("start"), c.Uint64("end"),
					actionProperty, c.Bool("detailed"))
				if err != nil {
					return err
				}
				return core.Persist(counts, c.String("output"))
			}),
		},
		{
			Name: "sc-group-actions",
			Flags: addDetailedFlag(addActionPropertyFlag(
				addPatternFlag(addOutputFlag(addRangeFlags(nil, false))))),
			Usage: "iotex specific: Count and groups the number of \"actions\" in the data, along with type of SC i.e. verified, unverified, XRC20 or NFT",
			Action: makeAction(func(c *cli.Context) error {
				actionProperty, err := core.GetActionProperty(c.String("by"))
				if err != nil {
					return err
				}
				counts, err := processor.SCGroupActions(
					blockchain, c.String("pattern"),
					c.Uint64("start"), c.Uint64("end"),
					actionProperty, c.Bool("detailed"))
				if err != nil {
					return err
				}
				return core.Persist(counts, c.String("output"))
			}),
		},
		{
			Name: "group-actions-over-time",
			Flags: addActionPropertyFlag(addGroupDurationFlag(
				addPatternFlag(addOutputFlag(addRangeFlags(nil, false))))),
			Usage: "Count and groups per time the number of \"actions\" in the data",
			Action: makeAction(func(c *cli.Context) error {
				duration, err := time.ParseDuration(c.String("duration"))
				if err != nil {
					return err
				}
				actionProperty, err := core.GetActionProperty(c.String("by"))
				if err != nil {
					return err
				}
				counts, err := processor.CountActionsOverTime(
					blockchain, c.String("pattern"),
					c.Uint64("start"), c.Uint64("end"),
					duration, actionProperty)
				if err != nil {
					return err
				}
				return core.Persist(counts, c.String("output"))
			}),
		},
		{
			Name:  "count-transactions-over-time",
			Flags: addGroupDurationFlag(addPatternFlag(addOutputFlag(addRangeFlags(nil, false)))),
			Usage: "Count number of \"transactions\" over time in the data",
			Action: makeAction(func(c *cli.Context) error {
				duration, err := time.ParseDuration(c.String("duration"))
				if err != nil {
					return err
				}
				counts, err := processor.CountTransactionsOverTime(
					blockchain, c.String("pattern"),
					c.Uint64("start"), c.Uint64("end"), duration)
				if err != nil {
					return err
				}
				return core.Persist(counts, c.String("output"))
			}),
		},
		{
			Name:  "count-gov-transactions-over-time",
			Flags: addOutputFlag((addGroupDurationFlag(addPatternFlag(addRangeFlags(nil, false))))),
			Usage: "iotex specific: Count the number of governance transactions in the data over the fixed intervals of time",
			Action: makeAction(func(c *cli.Context) error {
				duration, err := time.ParseDuration(c.String("duration"))
				if err != nil {
					return err
				}
				count, err := processor.CountGovTransactionsOverTime(
					blockchain, c.String("pattern"),
					c.Uint64("start"), c.Uint64("end"), duration)
				if err != nil {
					return err
				}
				return core.Persist(count, c.String("output"))
			}),
		},
		{
			Name:  "count-mining-history",
			Flags: addGroupDurationFlag(addPatternFlag(addOutputFlag(addRangeFlags(nil, false)))),
			Usage: "iotex specific: Count the miner history as per number of blocks produced",
			Action: makeAction(func(c *cli.Context) error {
				counts, err := processor.CountMiningHistory(
					blockchain, c.String("pattern"),
					c.Uint64("start"), c.Uint64("end"))
				if err != nil {
					return err
				}
				return core.Persist(core.SortMapStringU64(counts.MiningHistoryCounts), c.String("output"))
			}),
		},
		{
			Name: "count-one-to-one-txns",
			Flags: addActionPropertyFlag(
				addPatternFlag(addOutputFlag(addRangeFlags(nil, false)))),
			Usage: "iotex specific: Count transactions between sender and receiver, put in a map",
			Action: makeAction(func(c *cli.Context) error {
				counts, err := processor.OneToOneCount(
					blockchain, c.String("pattern"),
					c.Uint64("start"), c.Uint64("end"))
				if err != nil {
					return err
				}
				return core.Persist(counts, c.String("output"))
			}),
		},
		{
			Name:  "count-mining-history-over-time",
			Flags: addGroupDurationFlag(addPatternFlag(addOutputFlag(addRangeFlags(nil, false)))),
			Usage: "iotex specific: Count the miner history as per number of blocks produced over time",
			Action: makeAction(func(c *cli.Context) error {
				duration, err := time.ParseDuration(c.String("duration"))
				if err != nil {
					return err
				}
				counts, err := processor.CountMiningHistoryOverTime(
					blockchain, c.String("pattern"),
					c.Uint64("start"), c.Uint64("end"), duration)
				if err != nil {
					return err
				}
				return core.Persist(counts, c.String("output"))
			}),
		},
		{
			Name:  "bulk-process",
			Flags: addConfigFlag(addOutputFlag(nil)),
			Usage: "Bulk process the data according to the given configuration file",
			Action: makeAction(func(c *cli.Context) error {
				file, err := os.Open(c.String("config"))
				if err != nil {
					return err
				}
				defer file.Close()

				var config processor.BulkConfig
				if err := json.NewDecoder(file).Decode(&config); err != nil {
					return err
				}
				fmt.Println(config.RawProcessors)
				result, err := processor.RunBulkActions(blockchain, config)
				if err != nil {
					return err
				}
				return core.Persist(result, c.String("output"))
			}),
		},
		{
			Name:  "export",
			Flags: addPatternFlag(addOutputFlag(addRangeFlags(nil, false))),
			Usage: "Export a subset of the fields to msgpack format for faster processing",
			Action: makeAction(func(c *cli.Context) error {
				return processor.ExportToMsgpack(blockchain, c.String("pattern"),
					c.Uint64("start"), c.Uint64("end"), c.String("output"))
			}),
		},
	}...)
}

var eosCommands []*cli.Command = []*cli.Command{
	{
		Name:  "export-transfers",
		Flags: addPatternFlag(addOutputFlag(addRangeFlags(nil, false))),
		Usage: "Export all the transfers to a CSV file",
		Action: makeAction(func(c *cli.Context) error {
			return eos.ExportTransfers(
				c.String("pattern"),
				c.Uint64("start"), c.Uint64("end"), c.String("output"))
		}),
	},
}

func main() {
	app := &cli.App{
		Usage: "Tool to fetch and analyze blockchain transactions",
		Flags: addCpuProfileFlag(nil),
		Commands: []*cli.Command{
			{
				Name:        "eos",
				Usage:       "Analyze EOS data",
				Subcommands: addCommonCommands(eos.New(), eosCommands),
			},
			{
				Name:        "tezos",
				Usage:       "Analyze Tezos data",
				Subcommands: addCommonCommands(tezos.New(), nil),
			},
			{
				Name:        "xrp",
				Usage:       "Analyze XRP data",
				Subcommands: addCommonCommands(xrp.New(), nil),
			},
			{
				Name:        "helium",
				Usage:       "Analyze Helium data",
				Subcommands: addCommonCommands(helium.New(), nil),
			},
			{
				Name:        "iotex",
				Usage:       "Analyze IoTeX data",
				Subcommands: addCommonCommands(iotex.New(), nil),
			},
			{
				Name:        "iota",
				Usage:       "Analyze IOTA data",
				Subcommands: addCommonCommands(iota.New(), nil),
			},
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}
