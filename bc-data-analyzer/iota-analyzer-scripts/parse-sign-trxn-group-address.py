import pandas as pd


pd_object = pd.read_json('results_iota/group-by-output-address.json', typ='series')
df = pd.DataFrame(pd_object)

df.columns = ["Count"]

print("Number of rows: ", df.shape[0])

rslt_df = df.sort_values(by = 'Count', ascending = False)
print(rslt_df.head(10))

cou = 0

for index, row in rslt_df.iterrows():
    print("Index: ", index)
    print("Count: ", row["Count"])
    cou += 1
    if cou == 10:
        break
    