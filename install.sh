#!/bin/bash

# Define a reusable function to check if a command exists
command_exists() {
    command -v "$1" >/dev/null 2>&1
}

# Check if Docker is installed
if command_exists docker; then
    echo "Docker is already installed and working, skipping installation"
else
    echo "Docker not found, installing..."
    curl -fsSL https://get.docker.com | sh

    # Wait briefly to ensure Docker is usable
    sleep 5

    # Optionally start Docker if it's not running
    if ! pgrep -x dockerd >/dev/null; then
        echo "Starting Docker daemon..."
        systemctl start docker 2>/dev/null || service docker start 2>/dev/null || dockerd &
        sleep 5
    fi
fi

# Now run the serverbench container
echo "Setting up serverbench container..."
docker rm -f serverbench 2>/dev/null || true
docker run -d \
  --privileged \
  --cap-add=NET_ADMIN \
  --name serverbench \
  --restart=always \
  --pull=always \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -v ./containers:/containers \
  -v ./keys:/keys \
  -v /proc/1/ns/net:/mnt/host_netns \
  -e KEY="$1" \
  -e HOSTNAME="$(hostname)" \
  --pid=host \
  --network=host \
  serverbench/daemon

echo "serverbench installed"
