#!/bin/bash

# Define a reusable function to check if a command exists
command_exists() {
    command -v "$1" >/dev/null 2>&1
}

if command_exists iptables; then
  echo "iptables is correctly installed"
else
  echo "missing requirements: iptables"
  exit 1
fi

if command_exists ip6tables; then
  echo "ip6tables is correctly installed"
else
  echo "missing requirements: ip6tables"
  exit 1
fi

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
# Detect iptables variant
if iptables --version | grep -q nft; then
  IPTABLES_BIN="iptables"
else
  IPTABLES_BIN="iptables-legacy"
fi

# Detect ip6tables variant
if ip6tables --version | grep -q nft; then
  IP6TABLES_BIN="ip6tables"
else
  IP6TABLES_BIN="ip6tables-legacy"
fi
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
  -e IPTABLES_BIN="$IPTABLES_BIN" \
  -e IP6TABLES_BIN="$IP6TABLES_BIN" \
  -e KEY="$1" \
  -e HOSTNAME="${2:-$(hostname)}" \
  --pid=host \
  --network=host \
  serverbench/daemon

echo "serverbench installed"
