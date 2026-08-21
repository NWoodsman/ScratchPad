# Use a lightweight Python base image
FROM python:3.12-slim

# Set the working directory inside the container
WORKDIR /app

# Install the required websockets package
RUN pip install --no-cache-dir websockets

# Copy your application files into the container
COPY . .

# Expose both the websocket and HTTP server ports
EXPOSE 63000 63001

# Execute the startup command, ensuring the container waits for the background processes
CMD ["bash", "-c", "python3 server.py & python3 -m http.server 63001 & trap 'kill 0' EXIT; wait"]