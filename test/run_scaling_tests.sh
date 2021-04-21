#!/usr/bin/env bash

set -e

echo "Running scaling tests..."

IMAGE_NAME="cwm-keda-external-scaler:latest"
TEST_DEPLOYMENT="./test/deploy.yaml"
NAMESPACE="cwm-keda-external-scaler-ns"

TZ="UTC"
FMT_DATETIME="%Y-%m-%dT%H:%M:%S.%8NZ"

METRIC_KEY="deploymentid:minio-metrics:bytes_out"
LAST_ACTION_KEY="deploymentid:last_action"
PREFIX_TEST_APP="test-app"

# Start minikube cluster and install keda
./bin/start_minikube.sh
./bin/install_keda.sh

KUBECTL="minikube kubectl --"

eval "$(minikube docker-env)"
$KUBECTL get pods

# Build docker image
docker build -t "$IMAGE_NAME" .
docker images

# Deploy
echo "Deploying test deployment [$TEST_DEPLOYMENT] with ScaledObject"
$KUBECTL apply -f $TEST_DEPLOYMENT
sleep 60s
echo "Listing all in all namespaces"
$KUBECTL get all --all-namespaces
POD_NAME_SCALER=$($KUBECTL get pods --no-headers -o custom-columns=":metadata.name" -n $NAMESPACE)
echo "Waiting for pod/$POD_NAME_SCALER to be ready"
$KUBECTL wait --for=condition=ready --timeout=600s "pod/$POD_NAME_SCALER" -n $NAMESPACE
echo "SUCCESS: pod [$POD_NAME_SCALER] is ready"

echo "Pinging Redis server"
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
    exit 1
fi

# Test
echo
echo "TEST # 1: Zero-to-one scaling [0-to-1]"
echo "Setting $METRIC_KEY in Redis server"
$KUBECTL exec -n $NAMESPACE "$POD_NAME_SCALER" -c redis -- redis-cli SET "$METRIC_KEY" "10"
echo "Setting $LAST_ACTION_KEY in Redis server"
$KUBECTL exec -n $NAMESPACE "$POD_NAME_SCALER" -c redis -- redis-cli SET "$LAST_ACTION_KEY" "$(date +"$FMT_DATETIME")"
sleep 2m
echo "Listing all in all namespaces"
$KUBECTL get all --all-namespaces
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
sleep 2m
echo "Listing all in all namespaces"
$KUBECTL get all --all-namespaces
POD_NAMES_TEST_APP=$($KUBECTL get pods --no-headers -o custom-columns=":metadata.name" -n $NAMESPACE | grep "$PREFIX_TEST_APP")
POD_NAMES_ARRAY=($POD_NAMES_TEST_APP)
echo "Verifying pods' readiness"
for pod in "${POD_NAMES_ARRAY[@]}"; do
    echo "Waiting for pod/$pod to be ready"
    $KUBECTL wait --for=condition=ready --timeout=600s "pod/$pod" -n $NAMESPACE
done
echo "SUCCESS: Multiple pods scaling [1-to-4] completed"

echo "Deleting namespace [$NAMESPACE]"
$KUBECTL delete ns $NAMESPACE

minikube delete --all

echo "SUCCESS: Scaling tests completed successfully!"
