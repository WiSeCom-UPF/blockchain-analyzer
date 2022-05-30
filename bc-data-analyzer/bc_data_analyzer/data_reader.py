import datetime as dt
import json


def read_actions_over_time(filename: str):
    with open(filename) as f:
        data = json.load(f)
    # if "Results" in data and "GroupedActionsOverTime" in data["Results"]:
    #     data = data["Results"]["GroupedActionsOverTime"]
    actions_over_time = []
    for key, value in data["Actions"].items():
        parsed_time = dt.datetime.fromisoformat(key.rstrip("Z"))
        actions_over_time.append((parsed_time, value))
    return sorted(actions_over_time, key=lambda a: a[0])

def read_block_data_over_time(filename: str):
    with open(filename) as f:
        data = json.load(f)
    # if "Results" in data and "GroupedActionsOverTime" in data["Results"]:
    #     data = data["Results"]["GroupedActionsOverTime"]
    actions_over_time = []
    keys = list(data.keys())
    index = keys[0]
    if keys[0] == "GroupedBy":
    	index = keys[1]

    for key, value in data[index].items():
        parsed_time = dt.datetime.fromisoformat(key.rstrip("Z"))
        actions_over_time.append((parsed_time, value))
    return sorted(actions_over_time, key=lambda a: a[0])
