---
title: "Python Data Science with Pandas"
lang: en
---

# Python Data Science with Pandas

Pandas is the most popular data manipulation library in Python, built on top of NumPy.

## DataFrames

The core data structure:

```python
import pandas as pd

df = pd.DataFrame({
    'name': ['Alice', 'Bob', 'Charlie'],
    'age': [25, 30, 35],
    'city': ['NYC', 'LA', 'Chicago']
})
```

## Reading Data

```python
df = pd.read_csv('data.csv')
df = pd.read_excel('data.xlsx')
df = pd.read_json('data.json')
```

## Data Selection

```python
df['name']           # Select column
df.loc[0]            # Select row by label
df.iloc[0:5]         # Select rows by position
df[df['age'] > 25]   # Boolean indexing
```

## Aggregation

```python
df.groupby('city')['age'].mean()
df.agg({'age': 'mean', 'salary': 'sum'})
```

## Merging

```python
pd.merge(df1, df2, on='key')
df1.join(df2)
```

## NumPy Integration

```python
import numpy as np

arr = np.array([1, 2, 3, 4, 5])
mean = np.mean(arr)
std = np.std(arr)
```
