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

# Start minikube
echo
echo "Starting minikube"
minikube start --driver=docker --kubernetes-version=v1.16.14
minikube addons list
sleep 30s

eval "$(minikube -p minikube docker-env)"

KUBECTL="minikube kubectl --"

echo
echo "kubectl version:"
$KUBECTL version

# Set up keda
echo
echo "keda: download, install and set up"

helm repo add kedacore https://kedacore.github.io/charts
helm repo update

KEDA_NAMESPACE="keda"
$KUBECTL create namespace $KEDA_NAMESPACE
helm install keda kedacore/keda --version 2.1.0 --namespace $KEDA_NAMESPACE

KEDA_COMPONENT="keda-operator"
echo "Waiting for $KEDA_COMPONENT to be ready"
$KUBECTL wait --for=condition=ready --timeout=600s pod -l app=$KEDA_COMPONENT -n $KEDA_NAMESPACE

KEDA_COMPONENT="keda-operator-metrics-apiserver"
echo "Waiting for $KEDA_COMPONENT to be ready"
$KUBECTL wait --for=condition=ready --timeout=600s pod -l app=$KEDA_COMPONENT -n $KEDA_NAMESPACE

echo "SUCCESS: keda is ready!"

# Build docker image
echo
echo "Building docker image [$IMAGE_NAME]"
docker build -t "$IMAGE_NAME" .
docker images

# Deploy
echo
echo "Deploying test deployment [$TEST_DEPLOYMENT] with ScaledObject"
$KUBECTL apply -f $TEST_DEPLOYMENT
sleep 60s
echo "Listing all in all namespaces"
$KUBECTL get all --all-namespaces
echo "Describing HPA from namespace $NAMESPACE"
$KUBECTL describe hpa -n $NAMESPACE
POD_NAME_SCALER=$($KUBECTL get pods --no-headers -o custom-columns=":metadata.name" -n $NAMESPACE)
echo "Waiting for pod/$POD_NAME_SCALER to be ready"
$KUBECTL wait --for=condition=ready --timeout=600s "pod/$POD_NAME_SCALER" -n $NAMESPACE
echo "SUCCESS: pod [$POD_NAME_SCALER] is ready"

# Ping Redis server before proceeding with tests
echo
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
sleep 30s
echo "Listing all in all namespaces"
$KUBECTL get all --all-namespaces
echo "Describing HPA from namespace $NAMESPACE"
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
$KUBECTL exec -n $NAMESPACE "$POD_NAME_SCALER" -c redis -- redis-cli SET "$METRIC_KEY" "90"
echo "Setting $LAST_ACTION_KEY in Redis server"
$KUBECTL exec -n $NAMESPACE "$POD_NAME_SCALER" -c redis -- redis-cli SET "$LAST_ACTION_KEY" "$(date +"$FMT_DATETIME")"
sleep 60s
echo "Listing all in all namespaces"
$KUBECTL get all --all-namespaces
echo "Describing HPA from namespace $NAMESPACE"
$KUBECTL describe hpa -n $NAMESPACE
POD_NAMES_TEST_APP=$($KUBECTL get pods --no-headers -o custom-columns=":metadata.name" -n $NAMESPACE | grep "$PREFIX_TEST_APP")
POD_NAMES_ARRAY=($POD_NAMES_TEST_APP)
echo "Verifying pods' readiness"
for pod in "${POD_NAMES_ARRAY[@]}"; do
    echo "Waiting for pod/$pod to be ready"
    $KUBECTL wait --for=condition=ready --timeout=600s "pod/$pod" -n $NAMESPACE
done
echo "SUCCESS: Multiple pods scaling [1-to-4] completed"

echo
echo "Deleting namespace [$NAMESPACE]"
$KUBECTL delete ns $NAMESPACE

echo "SUCCESS: Scaling tests completed successfully!"
