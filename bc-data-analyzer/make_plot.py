# This is a modified file to quickly draw the plots

import json

from bc_data_analyzer.blockchain import Blockchain
from bc_data_analyzer import data_reader
from bc_data_analyzer import plot_utils
from bc_data_analyzer.eos import EOS
from bc_data_analyzer.iotex import IOTEX

def plot_actions_over_time(args):
    actions_over_time = data_reader.read_actions_over_time(args["input"])
    print(len(actions_over_time))
    # blockchain = Blockchain.create(args["blockchain"])
    blockchain = IOTEX()
    labels, dates, ys, colors = blockchain.transform_actions_over_time(actions_over_time)
    plot_utils.plot_chart_area(labels, dates, *ys, colors=colors, filename=args["output"])

def plot_block_activity_over_time(args):
    actions_over_time_EB = data_reader.read_block_data_over_time(args["input1"])
    actions_over_time_ZB = data_reader.read_block_data_over_time(args["input2"])
    # blockchain = Blockchain.create(args["blockchain"])
    blockchain = IOTEX()
    # labels, dates, ys, colors = blockchain.transform_block_activity_over_time(actions_over_time)
    plot_utils.demo(actions_over_time_EB, actions_over_time_ZB, args["output"])
    # plot_utils.plot_chart_area(labels, dates, *ys, colors=colors, filename=args["output"])

def plot_txn_activity_over_time(args):
    actions_over_time_Txc = data_reader.read_block_data_over_time(args["input1"])
    actions_over_time_Gtxc = data_reader.read_block_data_over_time(args["input2"])
    # blockchain = Blockchain.create(args["blockchain"])
    blockchain = IOTEX()
    # labels, dates, ys, colors = blockchain.transform_block_activity_over_time(actions_over_time)
    plot_utils.demo1(actions_over_time_Txc, actions_over_time_Gtxc, args["output"])
    # plot_utils.plot_chart_area(labels, dates, *ys, colors=colors, filename=args["output"])

def generate_table(args):
    with open(args["input"]) as f:
        data = json.load(f)
    # blockchain: Blockchain = Blockchain.create(args["blockchain"])
    blockchain = EOS()    
    table = blockchain.generate_table(args["name"], data)
    print(table)

def main(input_file1, input_file2, output_file):
    args = {"input1": input_file1,"input2": input_file2, "output": output_file}
    plot_block_activity_over_time(args)


if __name__ == '__main__':
    import plac
    plac.call(main)
