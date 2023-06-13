import pandas as pd


pd_object = pd.read_json('results_iota/group-by-index.json', typ='series')
df = pd.DataFrame(pd_object)

df.columns = ["Count"]

print("Number of rows: ", df.shape[0])

rslt_df = df.sort_values(by = 'Count', ascending = False)
print(rslt_df.head(10))

# Create the destination DataFrame
df_destination = pd.DataFrame(columns=['Count'])

number_spam_messages = 0
list_spam = []
print("SSTART HERE-----------------------------------------")

for index, row in df.iterrows():
    if "spam" in index.lower():
        list_spam.append(index)
        number_spam_messages += row["Count"]
    else:
        row_to_copy = df.loc[index]
        df_destination = df_destination.append(row_to_copy)
    
print(list_spam)
print("FINISH HERE---------------------------------------")

print("Number of rows without spam: ", df_destination.shape[0])
print("Number of messages that were spam: ", number_spam_messages)
rslt_df_no_spam = df_destination.sort_values(by = 'Count', ascending = False)
print(rslt_df_no_spam.head(10))


list_spam_indexes = ["al_scot#0690", "github.com/cerus", 
                     "Hornet Basse Bam Junge", "HORNET Msg www.itconsulting-wolfinger.de (index)", "Rajiv", "HORNET autopeering", "GEILE HORNISSE",
                     "Vote for DAO", "iota.in.net", "SCHRÖDINGERS_CAT_WAS_HERE", "OLIVER", "Moon",
                     "tangle.wtf", "Michaelis", "ebsi cef", "HORNET do as hornets do", "DOMICIDE 2021",
                     "Network support", "MIOTAX", "CARLOS MATOS", "C3P0 WAS HERE .-=o=-.", "dom is a fraud",
                     "Free Tacos: youtu.be/dQw4w9WgXcQ", "WEN", "internationaltradingyh_5-0", "internationaltradingyh_0-5",
                     "internationaltradingyh_6-1", "internationaltradingyh_7-2", "internationaltradingyh_2-7", "internationaltradingyh_1-6",
                     "internationaltradingyh_8-3", "internationaltradingyh_3-8", "internationaltradingyh_4-9", "internationaltradingyh_9-4",
                     "HORNET IOTA MOON", "Sporting CP", "HORNET IOTA TO THE MOON", "hello-world-index",
                     "Test_And_More_Test", "MY-DATA-INDEX", "HeyHo", "hello world", "hello-iota-index",
                     "Test_And_More_Test_", "THANKYOU", "iota-message-test", "CFB suxxx", "Teleconsys Permanode",
                     "Jokes on Tangle (JokesAPI)", "TESTING", "DEMO12-kevin-Green-energy", "Foo", "iota.c🦋",
                     "INDEXINDEXINDEX", "Hello"]

for indx in list_spam_indexes:
    number_spam_messages +=  rslt_df_no_spam.loc[indx, "Count"]
    rslt_df_no_spam = rslt_df_no_spam.drop(indx)

print("Number of rows without spam: ", rslt_df_no_spam.shape[0])
print("Number of messages that were spam: ", number_spam_messages)
rslt_df_no_spam = rslt_df_no_spam.sort_values(by = 'Count', ascending = False)
print(rslt_df_no_spam.head(50))

num_messages_r360 = 0
num_messages_THST = 0
for index, row in rslt_df_no_spam.iterrows():
    if "R360" in index:
        num_messages_r360 += row["Count"]
    if "THST" in index: 
        num_messages_THST += row["Count"]
        
print("R30- MESSAGES: ", num_messages_r360)
print("THST MESSAGES: ", num_messages_THST)