#! /bin/bash
#
# Copyright 2024 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

set -e

# Input parameters:
DECKHOUSE_PATH="/deckhouse/"
IMG_PREFIX="$USER"
DOCKER_REGISTRY="cr.yandex/crp8n201pre28pm81udl/sys/deckhouse-oss"
PATCH_MODULE_CONFIG_BY_IMAGE_AMD64_NAME_DIGEST=true

IMG_NAME="$DOCKER_REGISTRY/$IMG_PREFIX-manager"
IMG_NAME_LATEST="$IMG_NAME:latest"

# Function to find and switch to the directory where the script is located
cd_script_dir() {
  # Determine the path to the script directory
  local script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

  # Switch to this directory
  cd "$script_dir" || exit

  # Inform the user which directory we have switched to
  echo "Switched to script directory: $script_dir"
}

# Function to apply a patch
kubectl_patch_module_config() {
  local args="$1"
  local patch="$2"

  # Apply the patch
  kubectl patch $args --type='json' -p "$patch"

  # Check the result of the execution
  if [ $? -eq 0 ]; then
    echo "Patch applied successfully!"
  else
    echo "Error applying patch!"
    exit 1
  fi
}

cd_script_dir

if ! docker buildx inspect mybuilder > /dev/null 2>&1; then
    # If it does not exist, create it
    docker buildx create --name mybuilder --driver docker-container --use
else
    # If it exists, just switch to it
    docker buildx use mybuilder
fi

docker buildx build $DECKHOUSE_PATH \
    -f Manager.Dockerfile \
    --platform linux/amd64 \
    -t $IMG_NAME_LATEST \
    --push

echo "------------------------------------------------------------------------------------------"
echo "Image successfully built and pushed, full image name: '$IMG_NAME_LATEST'"
echo "------------------------------------------------------------------------------------------"

IMAGE_AMD64_DIGEST=$(docker buildx imagetools inspect $IMG_NAME_LATEST --raw | jq -r '.manifests[] | select(.platform.architecture == "amd64").digest')
IMAGE_AMD64_NAME_DIGEST="$IMG_NAME@$IMAGE_AMD64_DIGEST"
echo "------------------------------------------------------------------------------------------"
echo "Image digest: '$IMAGE_AMD64_NAME_DIGEST'"
echo "------------------------------------------------------------------------------------------"

if [ "$PATCH_MODULE_CONFIG_BY_IMAGE_AMD64_NAME_DIGEST" = "true" ]; then
    ################################################
    #        Updating the image in ConfigModule    #
    ################################################
    echo "Updating registry manager image"
    PATCH=$(cat <<EOF
[
  {
    "op": "replace",
    "path": "/spec/settings/imagesOverride/registryManager",
    "value": "$IMAGE_AMD64_NAME_DIGEST"
  }
]
EOF
)
    kubectl_patch_module_config "ModuleConfig system-registry" "$PATCH"
fi
