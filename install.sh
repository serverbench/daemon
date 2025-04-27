#!/bin/bash

# More reliable check for Docker - actually try to use it
if docker info &>/dev/null; then
    echo "Docker is already installed and working, skipping installation"
else
    echo "Docker not found or not working, installing..."
    curl -fsSL https://get.docker.com | sh
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