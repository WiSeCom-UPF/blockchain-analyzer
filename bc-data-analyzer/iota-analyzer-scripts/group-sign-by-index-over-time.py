import pandas as pd
import json
import matplotlib.pyplot as plt
import numpy as np
from matplotlib.ticker import MultipleLocator

with open('results_iota/group-by-index-over-time.json') as file:
    data = json.load(file)
    
# Create a DataFrame from the JSON data
df = pd.DataFrame(data)

# Rename the columns
df = df.rename(columns={'2021-03-29T22:00:00Z': 'March 2021', '2021-04-29T08:00:00Z': 'April 2021', '2021-05-29T18:00:00Z': 'May 2021',
                        '2021-06-29T04:00:00Z': 'June 2021', '2021-07-29T14:00:00Z': 'July 2021', '2021-08-29T00:00:00Z': 'August 2021',
                        '2021-09-28T10:00:00Z': 'September 2021', '2021-10-28T20:00:00Z': 'October 2021', '2021-11-28T06:00:00Z': 'November 2021',
                        '2021-12-28T16:00:00Z': 'December 2021', '2022-01-28T02:00:00Z': 'January 2022', '2022-02-27T12:00:00Z': 'February 2022',
                        '2022-03-29T22:00:00Z': 'March 2022', '2022-04-29T08:00:00Z': 'April 2022', '2022-05-29T18:00:00Z': 'May 2022',
                        '2022-06-29T04:00:00Z': 'June 2022', '2022-07-29T14:00:00Z': 'July 2022', '2022-08-29T00:00:00Z': 'August 2022',
                        '2022-09-28T10:00:00Z': 'September 2022', '2022-10-28T20:00:00Z': 'October 2022', '2022-11-28T06:00:00Z': 'November 2022',
                        '2022-12-28T16:00:00Z': 'December 2022', '2023-01-28T02:00:00Z': 'January 2023', '2023-02-27T12:00:00Z': 'February 2023',
                        '2023-03-29T22:00:00Z': 'March 2023', '2023-04-29T08:00:00Z': 'April 2023'})

df = df.drop('March 2021', axis=1)


cols = ['April 2021', 'May 2021', 'June 2021', 'July 2021', 'August 2021', 'September 2021', 'October 2021', 'November 2021', 'December 2021', 'January 2022', 'February 2022',
                  'March 2022', 'April 2022', 'May 2022', 'June 2022', 'July 2022', 'August 2022', 'September 2022', 'October 2022', 'November 2022', 'December 2022', 'January 2023', 'February 2023',
                  'March 2023', 'April 2023']

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
                     "INDEXINDEXINDEX", "Hello", "Powered By BiiLabs Alfred"]

df_counts_spam = pd.DataFrame(columns=cols)
new_row = pd.DataFrame({col: 0 for col in df_counts_spam.columns}, index=[len(df_counts_spam)])
df_counts_spam = df_counts_spam.append(new_row)
df_counts_spam = df_counts_spam.reset_index(drop=True)

df_counts_NONspam = pd.DataFrame(columns=cols)
new_row_n = pd.DataFrame({col: 0 for col in df_counts_NONspam.columns}, index=[len(df_counts_NONspam)])
df_counts_NONspam = df_counts_NONspam.append(new_row_n)
df_counts_NONspam = df_counts_NONspam.reset_index(drop=True)

df_destination = pd.DataFrame(columns=cols)

for index, row in df.iterrows():
    if "spam" not in index.lower():
        if index not in list_spam_indexes:
            df_counts_NONspam.iloc[0] = df_counts_NONspam.iloc[0] + df.loc[index].fillna(0).values
            row_to_copy = df.loc[index]
            df_destination = df_destination.append(row_to_copy)
    else:
        df_counts_spam.iloc[0] = df_counts_spam.iloc[0] + df.loc[index].fillna(0).values

for indx in list_spam_indexes:
    df_counts_spam.iloc[0] = df_counts_spam.iloc[0] + df.loc[indx].fillna(0).value
        
print(df_counts_spam)  
print(df_counts_NONspam)   

fig, ax = plt.subplots(figsize=(6, 15))

# Plot Scatter Plot 1
ax.scatter(df_counts_spam.columns, df_counts_spam.values.flatten(), color='red',  marker='o', linestyle='-', label='Plot 1')

# Plot Scatter Plot 2
ax.scatter(df_counts_NONspam.columns, df_counts_NONspam.values.flatten(), color='blue',  marker='o', linestyle='-', label='Plot 2')


ax.plot(df_counts_spam.columns, df_counts_spam.values.flatten(), color='red', linestyle='-', marker='o')
ax.plot(df_counts_NONspam.columns, df_counts_NONspam.values.flatten(), color='blue', linestyle='-', marker='o')



# Add labels and title
ax.set_xlabel('X-axis')
ax.set_ylabel('Y-axis')
ax.set_title('Scatter Plots')

# Add a legend
ax.legend()

# Display the plot
plt.show()  
  

top_indexes_odered = []

# # Select the top 5 indexes for each column
top_indexes_df = pd.DataFrame(index=range(5), columns=df_destination.columns)
for column in df_destination.columns:
    print("Month: ", column)
    top_indexes_df[column] = df_destination[column].nlargest(5).index
    top_indexes_odered.extend(df_destination[column].nlargest(5).index)
    for ind in df_destination[column].nlargest(5).index:
        print("Index: " + ind + ", val: ", df_destination.loc[ind, column])
