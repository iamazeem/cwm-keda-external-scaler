#!/usr/bin/env bash

set -e

echo "Deleting minikube cluster"
minikube delete --all

echo "--- [DONE] ---"
