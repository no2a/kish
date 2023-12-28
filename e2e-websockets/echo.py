#!/usr/bin/env python

# based on https://github.com/python-websockets/websockets/blob/main/example/echo.py

import asyncio
import sys
from websockets.server import serve

async def echo(websocket):
    async for message in websocket:
        await websocket.send(message)

async def main():
    port = int(sys.argv[1])  # 8765
    async with serve(echo, "localhost", port):
        await asyncio.Future()  # run forever

asyncio.run(main())
