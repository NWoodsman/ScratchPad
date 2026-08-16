import asyncio
import os
from datetime import datetime
from websockets.asyncio.server import serve

connected_clients = set()
shared_content = ""
DATA_FILE = "content.txt"
is_dirty = False

# Load existing content from disk on startup if available
if os.path.exists(DATA_FILE):
    with open(DATA_FILE, "r", encoding="utf-8") as f:
        shared_content = f.read()

# Background task to write memory state to disk every 20 seconds if modified
async def periodic_disk_writer():
    global is_dirty, shared_content
    while True:
        await asyncio.sleep(20)
        if is_dirty:
            with open(DATA_FILE, "w", encoding="utf-8") as f:
                f.write(shared_content)
            is_dirty = False
            print(f"{datetime.now().isoformat()}: snapshot taken.")

async def editor_handler(websocket):
    global shared_content, is_dirty
    connected_clients.add(websocket)
    
    # Send current raw text immediately upon connection
    if shared_content:
        await websocket.send(shared_content)
        
    try:
        async for message in websocket:
            # Message is already a raw plain-text string
            shared_content = message
            is_dirty = True  
            
            # Real-time broadcast to all other connected clients
            for client in connected_clients:
                if client != websocket:
                    await client.send(shared_content)
    finally:
        connected_clients.remove(websocket)
        # Flush immediately when a browser window closes or disconnects
        if is_dirty:
            with open(DATA_FILE, "w", encoding="utf-8") as f:
                f.write(shared_content)
            is_dirty = False
            print("Final state flushed to disk on client close/disconnect.")

async def main():
    asyncio.create_task(periodic_disk_writer())
    async with serve(editor_handler, "0.0.0.0", 63000) as server:
        print("Pure text real-time sync active on ws://localhost:3000")
        await server.serve_forever()

if __name__ == "__main__":
    asyncio.run(main())
