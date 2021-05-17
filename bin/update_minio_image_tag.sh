#!/usr/bin/env bash

set -e

SOURCE_REPO="cwm-keda-external-scaler"
TARGET_REPO="cwm-worker-deployment-minio"

echo "Updating $SOURCE_IMAGE's image tag in $TARGET_REPO's Helm Chart [$TARGET_FILE]"

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
IMAGE="docker.pkg.github.com/iamazeem/cwm-keda-external-scaler"
IMAGE_WITH_SHA="$IMAGE:$GITHUB_SHA"
IMAGE_FILE_NAME="$SOURCE_REPO.image"
echo "$IMAGE_WITH_SHA" > ./$IMAGE_FILE_NAME

echo "Pushing updated image tag [$IMAGE_WITH_SHA] to $TARGET_REPO"
git add ./$IMAGE_FILE_NAME
git commit -m "Automatic update of image with SHA for $TARGET_REPO."
git push origin main

echo "Updated SHA [$IMAGE_WITH_SHA] in $IMAGE_FILE_NAME successfully!"

echo "--- [DONE] ---"
