#!/bin/bash
#
# Copyright 2024 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

# Function to find and switch to the directory where the script is located
cd_script_dir() {
  # Determine the path to the script directory
  local script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

  # Switch to this directory
  cd "$script_dir" || exit

  # Inform the user which directory we have switched to
  echo "Switched to script directory: $script_dir"
}

run_ansible_playbook() {
  local inventory_file="$1"
  local playbook_file="$2"
  local ansible_options="$3"

  # Check if the inventory file exists
  if [ ! -f "$inventory_file" ]; then
    echo "Error: Inventory file '$inventory_file' not found!"
    exit 1
  fi

  # Check if the playbook exists
  if [ ! -f "$playbook_file" ]; then
    echo "Error: Playbook '$playbook_file' not found!"
    exit 1
  fi

  # Run the Ansible playbook
  ansible-playbook -i "$inventory_file" $ansible_options "$playbook_file"

  # Check the result of the execution
  if [ $? -eq 0 ]; then
    echo "Playbook executed successfully!"
  else
    echo "Error executing playbook!"
    exit 1
  fi
}

wait_for_pods_inactive() {
  local namespace="$1"
  local label_selector="$2"
  local sleep_interval="$3"

  while true; do
    if kubectl get pods -n "$namespace" -l="$label_selector" 2>&1 | grep -q "No resources found"; then
      echo "All pods are inactive."
      break
    else
      echo "Pods are still active. Waiting..."
      sleep "$sleep_interval"
    fi
  done
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
################################################
#              Stopping manager                #
################################################
echo "Removing system registry manager"
PATCH=$(cat <<EOF
[
  {
    "op": "replace",
    "path": "/spec/enabled",
    "value": false
  },
  {
    "op": "replace",
    "path": "/spec/settings/cluster/size",
    "value": 1
  }
]
EOF
)
kubectl_patch_module_config "ModuleConfig system-registry" "$PATCH"
wait_for_pods_inactive "d8-system" "app=system-registry-manager" 10

################################################
#               Stopping registry              #
################################################

echo "Removing system registry"
run_ansible_playbook "inventory.yaml" "ansible-delete-registry.yaml" "--tags static-pods"
wait_for_pods_inactive "d8-system" "component=system-registry,tier=control-plane" 10
run_ansible_playbook "inventory.yaml" "ansible-delete-registry.yaml" "--tags data"

