#!/usr/bin/env bash

set -e

SOURCE_REPO="cwm-keda-external-scaler"
TARGET_REPO="cwm-worker-deployment-minio"
TARGET_FILE="values.yaml"

echo "Updating $SOURCE_IMAGE's image tag in $TARGET_REPO's Helm Chart (values.yaml)"

echo "Setting up GitHub credentials"
DEPLOY_KEY_FILE="cwm_worker_deploy_key_file"
echo "$DEPLOY_KEY" > $DEPLOY_KEY_FILE
chmod 400 $DEPLOY_KEY_FILE
export GIT_SSH_COMMAND="ssh -i $(pwd)/$DEPLOY_KEY_FILE -o UserKnownHostsFile=/dev/null -o StrictHostKeyChecking=no"
git config --global user.name "$SOURCE_REPO CI"
git config --global user.email "$SOURCE_REPO-ci@localhost"

echo "Cloning $TARGET_REPO to update image tag"
git clone git@github.com:CloudWebManage/cwm-worker-deployment-minio.git

cd $TARGET_REPO/helm
IMAGE="docker.pkg.github.com/cwm-keda-external-scaler/iamazeem/cwm-keda-external-scaler"
IMAGE_WITH_SHA=$IMAGE:$GITHUB_SHA
sed -i "s#$IMAGE.*#$IMAGE_WITH_SHA#" ./$TARGET_FILE

echo "Pushing updated image tag [$IMAGE_WITH_SHA] to $TARGET_REPO"
git diff
git add ./$TARGET_FILE
git commit -m "Automatic update of image with SHA for $TARGET_REPO."
git push origin main

echo "Image with SHA [$IMAGE_WITH_SHA] updated successfully!"

echo "--- [DONE] ---"
