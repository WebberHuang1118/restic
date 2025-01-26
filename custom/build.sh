#!/bin/sh

set -e

export DOCKER_BUILDKIT=${DOCKER_BUILDKIT-1}

echo "Build docker image webberhuang/restic:latest"
docker build \
  --rm \
  --pull \
  --file custom/Dockerfile \
  --tag webberhuang/restic:latest \
  .
