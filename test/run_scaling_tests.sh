#!/usr/bin/env bash

set -e

echo "Running scaling tests..."

KEDA_DEPLOYMENT="https://github.com/kedacore/keda/releases/download/v2.1.0/keda-2.1.0.yaml"

IMAGE_NAME="cwm-keda-external-scaler:latest"
TEST_DEPLOYMENT="./test/deploy.yaml"
NAMESPACE="cwm-keda-external-scaler-ns"

# Time constants

TZ="UTC"
FMT_DATETIME="%Y-%m-%dT%H:%M:%S.%8NZ"

# Test constants

DEPLOYMENT_ID_1="minio1"
LAST_ACTION_KEY_1="deploymentid:last_action:$DEPLOYMENT_ID_1"
METRIC_NAME_1="bytes_out"
METRIC_KEY_1="deploymentid:minio-metrics:$METRIC_NAME_1"
METRIC_NAME_1_NEW="bytes_in"
METRIC_KEY_1_NEW="deploymentid:minio-metrics:$METRIC_NAME_1_NEW"
PREFIX_TEST_APP_1="test-app1"

# Setup

minikube version
minikube status

eval "$(minikube -p minikube docker-env)"

KUBECTL="minikube kubectl --"

echo "kubectl version:"
$KUBECTL version

# Set up KEDA
echo
echo "Set up keda"
$KUBECTL apply -f $KEDA_DEPLOYMENT

KEDA_NAMESPACE="keda"
KEDA_DEPLOYMENTS=("keda-operator" "keda-metrics-apiserver")
for deployment in "${KEDA_DEPLOYMENTS[@]}"; do
    echo "Waiting for deployment/$deployment to be ready"
    if ! $KUBECTL -n $KEDA_NAMESPACE rollout status --timeout=3m deployment/$deployment; then
      $KUBECTL -n $KEDA_NAMESPACE get pods
      exit 1
    fi
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
echo "Listing all in namespace [$NAMESPACE]"
$KUBECTL get all -n $NAMESPACE
echo "Waiting for test deployments to be ready"
$KUBECTL -n $NAMESPACE rollout status --timeout=5m deployment/cwm-keda-external-scaler
$KUBECTL -n $NAMESPACE rollout status --timeout=5m deployment/test-app1
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

if [[ $HPA_STATUS == "down" ]]; then
    echo
    echo -e "ERROR: HPA is down!"
    echo
    $KUBECTL cluster-info dump
    exit 1
fi

echo "SUCCESS: scaler is ready"

# Ping Redis server
echo
echo "Pinging Redis server [No. of tries: 5]"
REDIS_STATUS="down"
for i in {1..5}; do
    if $KUBECTL exec -n $NAMESPACE deployment/cwm-keda-external-scaler -c redis -- redis-cli PING; then
        REDIS_STATUS="up"
        break
    fi
    sleep 10s
done

if [[ $REDIS_STATUS == "down" ]]; then
    echo
    echo "ERROR: Redis server is down!"
    echo
    $KUBECTL cluster-info dump
    exit 1
fi

# --- TESTS - START ---

# Test # 1
echo
echo "TEST # 1: Zero-to-one scaling [0-to-1]"
echo "Setting $METRIC_KEY_1 in Redis server"
$KUBECTL exec -n $NAMESPACE deployment/cwm-keda-external-scaler -c redis -- redis-cli SET "$METRIC_KEY_1" "10"
echo "Setting $LAST_ACTION_KEY_1 in Redis server"
$KUBECTL exec -n $NAMESPACE deployment/cwm-keda-external-scaler -c redis -- redis-cli SET "$LAST_ACTION_KEY_1" "$(date +"$FMT_DATETIME")"
sleep 30s
echo "Listing all in namespace [$NAMESPACE]"
$KUBECTL get all -n $NAMESPACE
echo "Checking HPA in namespace [$NAMESPACE]"
$KUBECTL describe hpa -n $NAMESPACE
POD_NAME_TEST_APP=$($KUBECTL get pods --no-headers -o custom-columns=":metadata.name" -n $NAMESPACE | grep "$PREFIX_TEST_APP_1")
echo "Waiting for pod/$POD_NAME_TEST_APP to be ready"
$KUBECTL wait --for=condition=ready --timeout=600s "pod/$POD_NAME_TEST_APP" -n $NAMESPACE
echo "SUCCESS: Test (zero-to-one scaling) completed successfully!"

# Test # 2
echo
echo "TEST # 2: Redeploy ScaledObject with scaleMetricName [$METRIC_NAME_1 => $METRIC_NAME_1_NEW]"
echo "Redeploying test deployment [$TEST_DEPLOYMENT] with scaleMetricName [$METRIC_NAME_1_NEW]"
sed -i "s#scaleMetricName: \"$METRIC_NAME_1\"#scaleMetricName: \"$METRIC_NAME_1_NEW\"#" $TEST_DEPLOYMENT
$KUBECTL apply -f $TEST_DEPLOYMENT
sleep 30s
echo "Setting $METRIC_KEY_1_NEW in Redis server"
$KUBECTL exec -n $NAMESPACE deployment/cwm-keda-external-scaler -c redis -- redis-cli SET "$METRIC_KEY_1_NEW" "10"
echo "Setting $LAST_ACTION_KEY_1 in Redis server"
$KUBECTL exec -n $NAMESPACE deployment/cwm-keda-external-scaler -c redis -- redis-cli SET "$LAST_ACTION_KEY_1" "$(date +"$FMT_DATETIME")"
sleep 30s
echo "Listing all in namespace [$NAMESPACE]"
$KUBECTL get all -n $NAMESPACE
echo "Checking HPA in namespace [$NAMESPACE]"
$KUBECTL describe hpa -n $NAMESPACE
POD_NAME_TEST_APP=$($KUBECTL get pods --no-headers -o custom-columns=":metadata.name" -n $NAMESPACE | grep "$PREFIX_TEST_APP_1")
echo "Waiting for pod/$POD_NAME_TEST_APP to be ready"
$KUBECTL wait --for=condition=ready --timeout=600s "pod/$POD_NAME_TEST_APP" -n $NAMESPACE
echo
echo "Redeploying test deployment [$TEST_DEPLOYMENT] with scaleMetricName [$METRIC_NAME_1]"
sed -i "s#scaleMetricName: \"$METRIC_NAME_1_NEW\"#scaleMetricName: \"$METRIC_NAME_1\"#" $TEST_DEPLOYMENT
$KUBECTL apply -f $TEST_DEPLOYMENT
sleep 30s
echo "Listing all in namespace [$NAMESPACE]"
$KUBECTL get all -n $NAMESPACE
echo "Checking HPA in namespace [$NAMESPACE]"
$KUBECTL describe hpa -n $NAMESPACE
POD_NAME_TEST_APP=$($KUBECTL get pods --no-headers -o custom-columns=":metadata.name" -n $NAMESPACE | grep "$PREFIX_TEST_APP_1")
echo "Waiting for pod/$POD_NAME_TEST_APP to be ready"
$KUBECTL wait --for=condition=ready --timeout=600s "pod/$POD_NAME_TEST_APP" -n $NAMESPACE
echo "SUCCESS: Test (reploy with different scaleMetricName) completed successfully!"

# Test # 3
echo
echo "TEST # 3: Multiple pods scaling [1-to-4]"
echo "Setting $METRIC_KEY_1 in Redis server"
$KUBECTL exec -n $NAMESPACE deployment/cwm-keda-external-scaler -c redis -- redis-cli SET "$METRIC_KEY_1" "50"
echo "Setting $LAST_ACTION_KEY_1 in Redis server"
$KUBECTL exec -n $NAMESPACE deployment/cwm-keda-external-scaler -c redis -- redis-cli SET "$LAST_ACTION_KEY_1" "$(date +"$FMT_DATETIME")"
sleep 1m
echo "Listing all in namespace [$NAMESPACE]"
$KUBECTL get all -n $NAMESPACE
echo "Checking HPA in namespace [$NAMESPACE]"
$KUBECTL describe hpa -n $NAMESPACE
POD_NAMES_TEST_APP=$($KUBECTL get pods --no-headers -o custom-columns=":metadata.name" -n $NAMESPACE | grep "$PREFIX_TEST_APP_1")
POD_NAMES_ARRAY=($POD_NAMES_TEST_APP)
POD_NAMES_ARRAY_LENGTH="${#POD_NAMES_ARRAY[@]}"
EXPECTED_POD_COUNT=4
if (( POD_NAMES_ARRAY_LENGTH != EXPECTED_POD_COUNT )); then
    echo
    echo "ERROR: Pod count mismatch! got: $POD_NAMES_ARRAY_LENGTH, expected: $EXPECTED_POD_COUNT"
    echo "       Maybe, the deplay after deployment needs to be increased. Adjust accordingly."
    echo
    $KUBECTL cluster-info dump
    exit 1
fi

POD_COUNT=0
for pod in "${POD_NAMES_ARRAY[@]}"; do
    POD_COUNT=$(( POD_COUNT + 1 ))
    echo "Waiting for pod/$pod to be ready [$POD_COUNT]"
    $KUBECTL wait --for=condition=ready --timeout=600s "pod/$pod" -n $NAMESPACE
done

if (( POD_COUNT != EXPECTED_POD_COUNT )); then
    echo
    echo "ERROR: 1-to-4 scaling failed! got: $POD_COUNT, expected: $EXPECTED_POD_COUNT"
    echo
    $KUBECTL cluster-info dump
    exit 1
fi

echo "SUCCESS: Test (1-to-4 scaling) completed successfully!"

echo "--- [LOGS] ---"
$KUBECTL logs -n $NAMESPACE deployment/cwm-keda-external-scaler cwm-keda-external-scaler
echo "--------------"

# --- TESTS - END ---

# Teardown
echo
echo "Deleting test deployment [$TEST_DEPLOYMENT]"
$KUBECTL delete -f $TEST_DEPLOYMENT

echo "Deleting keda deployment"
$KUBECTL delete -f $KEDA_DEPLOYMENT

echo
echo "SUCCESS: Scaling tests completed successfully!"
echo "--- [DONE] ---"
