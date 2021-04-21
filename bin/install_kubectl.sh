#!/usr/bin/env bash

set -e

cd "$(mktemp -d)"
curl -LO "https://storage.googleapis.com/kubernetes-release/release/v1.16.14/bin/linux/amd64/kubectl"
chmod +x ./kubectl
mv ./kubectl /usr/local/bin/kubectl
