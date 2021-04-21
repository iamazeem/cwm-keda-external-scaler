#!/usr/bin/env bash

set -e

echo "Setting up test environment"

cd ./bin
pwd

./install_helm.sh && helm version
./install_kubectl.sh && kubectl version --client
./install_minikube.sh && minikube version

cd ..
pwd

echo "SUCCESS: Test environment is ready!"
