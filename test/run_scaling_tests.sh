#!/usr/bin/env bash

set -e

echo "Running scaling tests..."

KEDA_DEPLOYMENT="https://github.com/kedacore/keda/releases/download/v2.1.0/keda-2.1.0.yaml"

IMAGE_NAME="cwm-keda-external-scaler:latest"
TEST_DEPLOYMENT="./test/deploy.yaml"
NAMESPACE="cwm-keda-external-scaler-ns"

TZ="UTC"
FMT_DATETIME="%Y-%m-%dT%H:%M:%S.%8NZ"

METRIC_KEY="deploymentid:minio-metrics:bytes_out"
LAST_ACTION_KEY="deploymentid:last_action"
PREFIX_TEST_APP="test-app"

# Setup

minikube status

eval "$(minikube -p minikube docker-env)"

KUBECTL="minikube kubectl --"

echo
echo "kubectl version:"
$KUBECTL version

# Set up KEDA
echo
echo "Set up keda"
$KUBECTL apply -f $KEDA_DEPLOYMENT
sleep 1m

for pod in "keda-operator" "keda-metrics-apiserver"; do
    echo "Waiting for pod/$pod to be ready"
    $KUBECTL wait --for=condition=ready --timeout=600s pod -l app=$pod -n keda
done

echo "SUCCESS: keda is ready!"

# Build
echo
echo "Building docker image [$IMAGE_NAME]"
docker build -t "$IMAGE_NAME" .
docker images

# Deploy
echo
echo "Deploying test deployment [$TEST_DEPLOYMENT] with ScaledObject"
$KUBECTL apply -f $TEST_DEPLOYMENT
sleep 1m
echo "Listing all in all namespaces"
$KUBECTL get all -n $NAMESPACE
echo "Checking HPA in namespace $NAMESPACE"
$KUBECTL describe hpa -n $NAMESPACE
POD_NAME_SCALER=$($KUBECTL get pods --no-headers -o custom-columns=":metadata.name" -n $NAMESPACE)
echo "Waiting for pod/$POD_NAME_SCALER to be ready"
$KUBECTL wait --for=condition=ready --timeout=600s "pod/$POD_NAME_SCALER" -n $NAMESPACE
echo
echo "Waiting for HPA to be ready [No. of tries: 5]"
HPA_STATUS="down"
for i in {1..5}; do
    HPA_OUTPUT="$($KUBECTL describe hpa -n $NAMESPACE)"
    if [[ "$HPA_OUTPUT" != "" ]]; then
        echo "$HPA_OUTPUT"
        HPA_STATUS="up"
        break
    fi
    echo "HPA is not ready yet!"
    sleep 1m
done

if [[ "${HPA_STATUS}" == "down" ]]; then
    echo "ERROR: HPA is down! Exiting..."
    $KUBECTL cluster-info dump
    exit 1
fi

echo "SUCCESS: scaler [$POD_NAME_SCALER] is ready"

# Ping Redis server
echo
echo "Pinging Redis server [No. of tries: 5]"
REDIS_STATUS="down"
for i in {1..5}; do
    if $KUBECTL exec -n $NAMESPACE "$POD_NAME_SCALER" -c redis -- redis-cli PING; then
        REDIS_STATUS="up"
        break
    fi
    sleep 10s
done

if [[ "${REDIS_STATUS}" == "down" ]]; then
    echo "ERROR: Redis server is down! Exiting..."
    $KUBECTL cluster-info dump
    exit 1
fi

# Test
echo
echo "TEST # 1: Zero-to-one scaling [0-to-1]"
echo "Setting $METRIC_KEY in Redis server"
$KUBECTL exec -n $NAMESPACE "$POD_NAME_SCALER" -c redis -- redis-cli SET "$METRIC_KEY" "10"
echo "Setting $LAST_ACTION_KEY in Redis server"
$KUBECTL exec -n $NAMESPACE "$POD_NAME_SCALER" -c redis -- redis-cli SET "$LAST_ACTION_KEY" "$(date +"$FMT_DATETIME")"
sleep 30s
echo "Listing all in all namespaces"
$KUBECTL get all --all-namespaces
echo "Checking HPA in namespace $NAMESPACE"
$KUBECTL describe hpa -n $NAMESPACE
POD_NAME_TEST_APP=$($KUBECTL get pods --no-headers -o custom-columns=":metadata.name" -n $NAMESPACE | grep "$PREFIX_TEST_APP")
echo "Waiting for pod/$POD_NAME_TEST_APP to be ready"
$KUBECTL wait --for=condition=ready --timeout=600s "pod/$POD_NAME_TEST_APP" -n $NAMESPACE
echo "SUCCESS: pod/$POD_NAME_TEST_APP is ready"
echo "SUCCESS: Zero-to-one scaling [0-to-1] completed"

# Test
echo
echo "TEST # 2: Multiple pods scaling [1-to-4]"
echo "Setting $METRIC_KEY in Redis server"
$KUBECTL exec -n $NAMESPACE "$POD_NAME_SCALER" -c redis -- redis-cli SET "$METRIC_KEY" "50"
echo "Setting $LAST_ACTION_KEY in Redis server"
$KUBECTL exec -n $NAMESPACE "$POD_NAME_SCALER" -c redis -- redis-cli SET "$LAST_ACTION_KEY" "$(date +"$FMT_DATETIME")"
sleep 1m
echo "Listing all in all namespaces"
$KUBECTL get all --all-namespaces
echo "Checking HPA in namespace $NAMESPACE"
$KUBECTL describe hpa -n $NAMESPACE
POD_NAMES_TEST_APP=$($KUBECTL get pods --no-headers -o custom-columns=":metadata.name" -n $NAMESPACE | grep "$PREFIX_TEST_APP")
POD_NAMES_ARRAY=($POD_NAMES_TEST_APP)
for pod in "${POD_NAMES_ARRAY[@]}"; do
    echo "Waiting for pod/$pod to be ready"
    $KUBECTL wait --for=condition=ready --timeout=600s "pod/$pod" -n $NAMESPACE
done
echo "SUCCESS: Multiple pods scaling [1-to-4] completed"

# Teardown
echo
echo "Deleting namespace [$NAMESPACE]"
$KUBECTL delete ns $NAMESPACE

echo "Deleting keda deployment"
$KUBECTL delete -f $KEDA_DEPLOYMENT

echo "SUCCESS: Scaling tests completed successfully!"
