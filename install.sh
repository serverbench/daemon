# install docker
curl -fsSL https://get.docker.com | sh

# run daemon
docker run \
  --privileged \
  --name serverbench \
  --restart=always \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -v ./test/containers:/containers \
  -v ./test/keys:/keys \
  -e KEY="$1" \
  -e HOSTNAME="$(hostname)" \
  --pid=host \
  --network=host \
  serverbench/daemon
