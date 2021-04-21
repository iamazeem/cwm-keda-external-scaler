#!/usr/bin/env bash

set -e

echo "kubectl: download, install and set up"

cd "$(mktemp -d)"
curl -LO "https://storage.googleapis.com/kubernetes-release/release/v1.16.14/bin/linux/amd64/kubectl"
chmod +x ./kubectl
mv ./kubectl /usr/local/bin/kubectl

echo "SUCCESS: kubectl is ready!"
