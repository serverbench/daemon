#!/bin/bash

# Check if Docker is already installed
if ! command -v docker &> /dev/null; then
    echo "Docker not found, installing..."
    curl -fsSL https://get.docker.com | sh
else
    echo "Docker is already installed, skipping installation"
fi

# Run daemon - ignore errors from docker rm
echo "Setting up serverbench container..."
docker rm -f serverbench 2>/dev/null || true
docker run \
  --privileged \
  --name serverbench \
  --restart=always \
  --pull=always \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -v ./containers:/containers \
  -v ./keys:/keys \
  -e KEY="$1" \
  -e HOSTNAME="$(hostname)" \
  --pid=host \
  --network=host \
  serverbench/daemon