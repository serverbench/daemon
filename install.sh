#!/bin/sh

# Parse command line arguments
SAFE_MODE=false
ARGS=""

while [ $# -gt 0 ]; do
    case "$1" in
        --safe)
            SAFE_MODE=true
            shift
            ;;
        *)
            ARGS="$ARGS \"$1\""
            shift
            ;;
    esac
done

# Set positional parameters from remaining args
# Use eval to correctly handle quoted args stored in ARGS
eval set -- $ARGS

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
    if ! pgrep -x dockerd >/dev/null 2>&1; then
        echo "Starting Docker daemon..."
        if command -v systemctl >/dev/null 2>&1; then
            systemctl start docker 2>/dev/null
        elif command -v service >/dev/null 2>&1; then
            service docker start 2>/dev/null
        else
            dockerd &
        fi
        sleep 5
    fi
fi

# Now run the serverbench container
echo "Setting up serverbench container..."

# Detect iptables variant
if iptables --version 2>&1 | grep -q nf; then
  IPTABLES_BIN="iptables"
else
  IPTABLES_BIN="iptables-legacy"
fi

# Detect ip6tables variant
if ip6tables --version 2>&1 | grep -q nf; then
  IP6TABLES_BIN="ip6tables"
else
  IP6TABLES_BIN="ip6tables-legacy"
fi

docker rm -f serverbench 2>/dev/null || true

# Compose docker run command in a variable
DOCKER_CMD="docker run -d \
  --privileged \
  --cap-add=NET_ADMIN \
  --name serverbench \
  --restart=always \
  --pull=always \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -v ./containers:/containers \
  -v ./keys:/keys \
  -v serverbench-sshd:/etc \
  -v /proc/1/ns/net:/mnt/host_netns \
  -e IPTABLES_BIN=\"$IPTABLES_BIN\" \
  -e IP6TABLES_BIN=\"$IP6TABLES_BIN\" \
  -e KEY=\"$1\" \
  -e HOSTNAME=\"${2:-$(hostname)}\""

if [ "$SAFE_MODE" = "true" ]; then
    DOCKER_CMD="$DOCKER_CMD -e SKIP_CLEAN=true"
    echo "Running in safe mode (SKIP_CLEAN=true)"
fi

DOCKER_CMD="$DOCKER_CMD --pid=host --network=host serverbench/daemon"

# Execute the docker command
eval $DOCKER_CMD

echo "serverbench installed"
