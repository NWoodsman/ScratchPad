import asyncio
from datetime import datetime
import os
from pathlib import Path
import platform
from websockets.asyncio.server import serve
from platformdirs import user_data_dir

connected_clients = set()
shared_content = ""
is_dirty = False


def get_data_file_path() -> Path:
    # 1. Systemd/NixOS override takes highest priority
    systemd_data_dir = os.environ.get("DATA_DIR")
    if systemd_data_dir:
        data_dir = Path(systemd_data_dir)
    else:
        # 2. Cross-platform fallback using standard OS conventions:
        # Windows: AppData\Local\ScratchPad
        # macOS:   ~/Library/Application Support/ScratchPad
        # Linux:   ~/.local/share/ScratchPad (respects XDG_DATA_HOME)
        data_dir = Path(user_data_dir(appname="ScratchPad"))

    # 3. Create the directory layout safely
    data_dir.mkdir(parents=True, exist_ok=True)
    
    return data_dir / "content.txt"



DATA_FILE = get_data_file_path()


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
            print("A client closed; flushed to disk before disconnect.")

async def main():
    asyncio.create_task(periodic_disk_writer())
    async with serve(editor_handler, "0.0.0.0", 63000) as server:
        print("ScratchPad socket server active on ws://localhost:63000")
        await server.serve_forever()

if __name__ == "__main__":
    asyncio.run(main())
