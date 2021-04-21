#!/usr/bin/env bash

set -e

echo "keda: download, install and set up"

helm repo add kedacore https://kedacore.github.io/charts
helm repo update
kubectl create namespace keda
helm install keda kedacore/keda --version 2.1.0  --namespace keda

echo "SUCCESS: keda is ready!"
