#!/bin/bash

docker build . --tag serverbench
docker rm -f serverbench || true
docker run \
  --privileged \
  --name serverbench \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -v /tmp/containers:/containers \
  -v serverbench-sshd:/etc \
  -v /tmp/keys:/keys \
  -e KEY="$1" \
  -e HOSTNAME="$(hostname)" \
  -e SKIP_IPTABLES="true" \
  -e SKIP_UPDATE="true" \
  -e ENDPOINT="ws://localhost:3030" \
  -e TEST_ETH0="true" \
  --pid=host \
  --network=host \
  serverbench