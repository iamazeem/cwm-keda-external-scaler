#!/usr/bin/env bash

set -e

echo "minikube: download, install and set up"

cd "$(mktemp -d)"
curl -Lo minikube https://storage.googleapis.com/minikube/releases/v1.12.3/minikube-linux-amd64
chmod +x minikube
mv minikube /usr/local/bin/minikube
minikube config set WantUpdateNotification false

echo "SUCCESS: minikube is ready!"
