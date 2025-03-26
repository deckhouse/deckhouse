#! /bin/bash
#
# Copyright 2024 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

set -e

# Input parameters:
DECKHOUSE_PATH="/deckhouse/"
IMG_PREFIX="$USER"
YC_REGISTRY_ID="<.........>"
DOCKER_REGISTRY="cr.yandex/$YC_REGISTRY_ID/sys/deckhouse-oss"
PATCH_MODULE_CONFIG_BY_IMAGE_AMD64_NAME_DIGEST=true

IMG_NAME="$DOCKER_REGISTRY/$IMG_PREFIX-embedded-registry-manager"
IMG_NAME_LATEST="$IMG_NAME:latest"

log_stage() {
    local stage_name="$1"
    local line_length=90  # Line length for the frame
    local dash_line="-"   # Character for the line
    local green="\033[1;32m"  # Green color
    local reset="\033[0m"     # Reset color

    # Create a string of dashes of the required length
    local dashes=$(printf "%-${line_length}s" "" | tr " " "-")

    # Print the dash line at the top
    echo "$dashes"

    # Calculate padding to center the stage name
    local padding=$(( (line_length - ${#stage_name} - 4) / 2 ))

    # Print the stage name in green color
    printf "|%*s ${green}%s${reset} %*s|\n" $padding "" "$stage_name" $padding ""

    # Print the bottom border of the frame
    echo "$dashes"
}

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

  # Apply the patch using kubectl with Strategic Merge Patch
  kubectl patch $args --type='merge' -p "$patch" --output=name

  # Check the result of the execution
  if [ $? -eq 0 ]; then
    echo "Patch applied successfully!"
  else
    echo "Error applying patch!"
    exit 1
  fi
}

cd_script_dir

log_stage "Create buildx"
if ! docker buildx inspect mybuilder > /dev/null 2>&1; then
    # If it does not exist, create it
    docker buildx create --name mybuilder --driver docker-container --use
else
    # If it exists, just switch to it
    docker buildx use mybuilder
fi

log_stage "Build and push docker img"
docker buildx build $DECKHOUSE_PATH \
    -f Manager.Dockerfile \
    --platform linux/amd64 \
    -t $IMG_NAME_LATEST \
    --push

log_stage "Show img with tag"
echo "$IMG_NAME_LATEST"

IMAGE_AMD64_DIGEST=$(docker buildx imagetools inspect $IMG_NAME_LATEST --raw | jq -r '.manifests[] | select(.platform.architecture == "amd64").digest')
IMAGE_AMD64_NAME_DIGEST="$IMG_NAME@$IMAGE_AMD64_DIGEST"


log_stage "Show img with digest"
echo $IMAGE_AMD64_NAME_DIGEST

if [ "$PATCH_MODULE_CONFIG_BY_IMAGE_AMD64_NAME_DIGEST" = "true" ]; then

    # Define the patch that will add or replace registryManager
    PATCH=$(cat <<EOF
{
  "spec": {
    "settings": {
      "imagesOverride": {
        "registryManager": "$IMAGE_AMD64_NAME_DIGEST"
      }
    }
  }
}
EOF
)


    before_patch=$(kubectl get ModuleConfig system-registry -o yaml)

    # Apply the patch without conditions

    log_stage "Applying patch to ModuleConfig system-registry"
    kubectl_patch_module_config "ModuleConfig/system-registry" "$PATCH"


    after_patch=$(kubectl get ModuleConfig system-registry -o yaml)
    log_stage "Show Diff"
    if diff --color=auto <(echo "$before_patch") <(echo "$after_patch"); then
        echo "No changes were made."
    fi
fi
