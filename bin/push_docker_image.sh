#!/usr/bin/env bash

set -e

eval "$(minikube -p minikube docker-env)"

# Required environment variables:
#  USERNAME: ${{ github.actor }}
#  PASSWORD: ${{ github.token }}
#  REPO: ${{ github.repository }}

PKGS_URL="docker.pkg.github.com"
SOURCE_IMAGE="cwm-keda-external-scaler"
TAG="latest"
TARGET_IMAGE="$PKGS_URL/$REPO/$SOURCE_IMAGE:$TAG"

echo "Logging in to docker registry"
echo "$PASSWORD" | docker login https://docker.pkg.github.com -u "$USERNAME" --password-stdin

echo "Retagging docker image"
docker tag "$SOURCE_IMAGE" "$TARGET_IMAGE"
docker images

echo "Pushing image [$TARGET_IMAGE]"
docker push "$TARGET_IMAGE"

echo "Docker image [$TARGET_IMAGE] pushed successfully!"
