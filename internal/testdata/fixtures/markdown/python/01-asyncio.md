---
title: "Python AsyncIO Complete Guide"
lang: en
---

# Python AsyncIO Complete Guide

AsyncIO is Python's standard library for writing concurrent code using the async/await syntax.

## Basic Concepts

```python
import asyncio

async def main():
    print("Hello")
    await asyncio.sleep(1)
    print("World")

asyncio.run(main())
```

## Coroutines

A coroutine is a function that can suspend its execution before reaching return:

```python
async def fetch_data():
    print("start fetching")
    await asyncio.sleep(2)
    print("done fetching")
    return {"data": 1}
```

## Tasks

Schedule coroutines concurrently:

```python
async def main():
    task1 = asyncio.create_task(fetch_data())
    task2 = asyncio.create_task(fetch_data())
    await task1
    await task2
```

## Gather

Run multiple coroutines and wait for all:

```python
results = await asyncio.gather(
    fetch_data(),
    fetch_data(),
    fetch_data(),
)
```

## Event Loop

The event loop is the core of AsyncIO. It runs asynchronous tasks and callbacks.

```python
loop = asyncio.get_event_loop()
loop.run_until_complete(main())
```

## Real-world Example

```python
import aiohttp

async def fetch_urls(urls):
    async with aiohttp.ClientSession() as session:
        tasks = [session.get(url) for url in urls]
        responses = await asyncio.gather(*tasks)
        return responses
```
