#!/usr/bin/env bash

set -e

echo "Setting up test environment"

cd ./bin
pwd

./install_helm.sh && helm version
./install_kubectl.sh && kubectl version
./install_minikube.sh && minikube version
./start_minikube.sh
./install_keda.sh

cd ..
pwd

echo "SUCCESS: Test environment is ready!"
