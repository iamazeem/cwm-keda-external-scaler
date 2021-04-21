#!/bin/bash

set -e

MICROK8S_DOCKER_REGISTRY_ADDR="localhost:32000"
REPO_NAME="cwm-keda-external-scaler"
TEST_DEPLOYMENT="./test/deploy.yaml"
NAMESPACE="cwm-keda-external-scaler-ns"
TZ="UTC"
FMT_DATETIME="%Y-%m-%dT%H:%M:%S.%8NZ"
METRIC_KEY="deploymentid:minio-metrics:bytes_out"
LAST_ACTION_KEY="deploymentid:last_action"
PREFIX_TEST_APP="test-app"

# Build and push to microk8s docker registry
IMAGE_NAME="$REPO_NAME:latest"
MICROK8S_IMAGE_NAME="$MICROK8S_DOCKER_REGISTRY_ADDR/$IMAGE_NAME"
sed -i "s#$IMAGE_NAME#$MICROK8S_IMAGE_NAME#" "$TEST_DEPLOYMENT"
echo "Building docker image [$MICROK8S_IMAGE_NAME]"
docker build -t "$MICROK8S_IMAGE_NAME" .
docker push "$MICROK8S_IMAGE_NAME"

# Deploy
sudo snap alias microk8s.kubectl kubectl
echo "Deploying test deployment [$TEST_DEPLOYMENT] with ScaledObject"
kubectl apply -f $TEST_DEPLOYMENT
sleep 60s
echo "Listing all in namespace [$NAMESPACE]"
kubectl get all -n $NAMESPACE
POD_NAME_SCALER=$(kubectl get pods --no-headers -o custom-columns=":metadata.name" -n $NAMESPACE)
echo "Waiting for pod/$POD_NAME_SCALER to be ready"
kubectl wait --for=condition=ready --timeout=600s "pod/$POD_NAME_SCALER" -n $NAMESPACE
echo "SUCCESS: pod [$POD_NAME_SCALER] is ready"

echo "Pinging Redis server"
REDIS_STATUS="down"
for i in {1..5}; do
    if kubectl exec -n $NAMESPACE "$POD_NAME_SCALER" -c redis -- redis-cli PING; then
        REDIS_STATUS="up"
        break
    fi
done

if [[ "${REDIS_STATUS}" == "down" ]]; then
    echo "ERROR: Redis server is down! Exiting..."
    exit 1
fi

# Test
echo
echo "TEST # 1: Zero-to-one scaling"
echo "Setting $METRIC_KEY in Redis server"
kubectl exec -n $NAMESPACE "$POD_NAME_SCALER" -c redis -- redis-cli SET "$METRIC_KEY" "10"
echo "Setting $LAST_ACTION_KEY in Redis server"
kubectl exec -n $NAMESPACE "$POD_NAME_SCALER" -c redis -- redis-cli SET "$LAST_ACTION_KEY" "$(date +"$FMT_DATETIME")"
sleep 10s
POD_NAME_TEST_APP=$(kubectl get pods --no-headers -o custom-columns=":metadata.name" -n $NAMESPACE | grep "$PREFIX_TEST_APP")
echo "Waiting for pod/$POD_NAME_TEST_APP to be ready"
kubectl wait --for=condition=ready --timeout=600s "pod/$POD_NAME_TEST_APP" -n $NAMESPACE
echo "SUCCESS: pod/$POD_NAME_TEST_APP is ready"
echo "SUCCESS: Zero-to-one scaling completed"

# Test
echo
echo "TEST # 2: Multiple pods scaling [1-to-4]"
echo "Setting $METRIC_KEY in Redis server"
kubectl exec -n $NAMESPACE "$POD_NAME_SCALER" -c redis -- redis-cli SET "$METRIC_KEY" "50"
echo "Setting $LAST_ACTION_KEY in Redis server"
kubectl exec -n $NAMESPACE "$POD_NAME_SCALER" -c redis -- redis-cli SET "$LAST_ACTION_KEY" "$(date +"$FMT_DATETIME")"
sleep 10s
POD_NAMES_TEST_APP=$(kubectl get pods --no-headers -o custom-columns=":metadata.name" -n $NAMESPACE | grep "$PREFIX_TEST_APP")
POD_NAMES_ARRAY=($POD_NAMES_TEST_APP)
echo "Verifying pods' readiness"
for pod in "${POD_NAMES_ARRAY[@]}"; do
    echo "Waiting for pod/$pod to be ready"
    kubectl wait --for=condition=ready --timeout=600s "pod/$pod" -n $NAMESPACE
done
echo "SUCCESS: Multiple pods scaling completed"

# Teardown
echo "Deleting namespace [$NAMESPACE]"
kubectl delete ns $NAMESPACE

# Retag docker image
echo "Retaging image"
docker tag $MICROK8S_IMAGE_NAME $IMAGE_NAME

echo "--- [DONE] ---"
