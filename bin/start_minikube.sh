#!/usr/bin/env bash

set -e

echo "Starting minikube cluster"
minikube start --driver=docker --kubernetes-version=v1.16.14
minikube status
