#!/bin/bash

set -e  # Optional: exit on any command failure

# Function to check if docker is usable
docker_available() {
  command -v docker &>/dev/null && docker info &>/dev/null
}

# Install Docker if not available
if docker_available; then
    echo "Docker is already installed and working, skipping installation"
else
    echo "Docker not found or not working, installing..."
    curl -fsSL https://get.docker.com | sh
    # Reload PATH and ensure Docker daemon is started
    export PATH=$PATH:/usr/bin:/usr/local/bin
    systemctl start docker || service docker start || dockerd &
    sleep 5
    if ! docker_available; then
        echo "Docker installation failed or not started correctly"
        exit 1
    fi
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
