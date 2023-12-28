#!/usr/bin/env python

# based on https://github.com/python-websockets/websockets/blob/main/example/hello.py

import asyncio
import sys
from websockets.sync.client import connect

def hello():
    url = sys.argv[1]  # "ws://localhost:8765"
    with connect(url) as websocket:
        websocket.send("Hello world!")
        message = websocket.recv()
        print(f"Received: {message}")

hello()
