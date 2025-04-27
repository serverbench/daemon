# Check if Docker is installed, if not, install it
if ! docker --version &> /dev/null
then
    echo "Docker not found, installing..."
    curl -fsSL https://get.docker.com | sh
else
    echo "Docker is already installed."
fi

# Remove any existing container, ignore errors
docker rm -f serverbench || true

# Run the Docker container
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
