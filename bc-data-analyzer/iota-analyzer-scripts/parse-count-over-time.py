import pandas as pd
import matplotlib.pyplot as plt
import sys

args = sys.argv[1:]
file_name = args[0]

# pd_object2 = pd.read_json("results_iota/count-conflicts-over-time.json", typ='series')
# df2 = pd.DataFrame(pd_object2)
# df2.columns = ["Counts"]

pd_object = pd.read_json(file_name, typ='series')
df = pd.DataFrame(pd_object)
df.columns = ["Counts"]

# Plot the line connecting the dots
#df.plot(marker='o', linestyle='-', color='red')

# Define your custom index labels
custom_indexes = ['March 2021', 'April 2021', 'May 2021', 'June 2021', 'July 2021', 'August 2021', 'September 2021', 'October 2021', 'November 2021', 'December 2021', 'January 2022', 'February 2022',
                  'March 2022', 'April 2022', 'May 2022', 'June 2022', 'July 2022', 'August 2022', 'September 2022', 'October 2022', 'November 2022', 'December 2022', 'January 2023', 'February 2023',
                  'March 2023', 'April 2023']

#conflict_indexes = ['Conflict 0', 'Conflict 1', 'Conflict 2', 'Conflict 3','Conflict 4','Conflict 5', 'Conflict 6']

fig, ax = plt.subplots()

# Plot Scatter Plot 1
ax.scatter(df.index, df["Counts"], color='red',  marker='o', linestyle='-')

# Plot Scatter Plot 2
# ax.scatter(df2.index, df2["Counts"], color='blue',  marker='o', linestyle='-', label='Conflicting Signed Transactions')


ax.plot(df.index, df["Counts"], color='red', linestyle='-', marker='o')
# ax.plot(df2.index, df2["Counts"], color='blue', linestyle='-', marker='o')



# Add labels and title
ax.set_ylabel('Count')
ax.set_title('Average number of messages per block per month')

# Add a legend
ax.legend()

plt.xticks(df.index, custom_indexes)
plt.gcf().autofmt_xdate(rotation=45)

# Display the plot
plt.show()  
