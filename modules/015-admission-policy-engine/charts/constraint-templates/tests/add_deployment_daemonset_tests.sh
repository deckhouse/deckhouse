#!/bin/bash

# Script to add Deployment and DaemonSet tests to all test suites
# Adds tests for both allowed and disallowed cases

set -e

BASE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# List of test directories that we created
TEST_DIRS=(
  "allow-host-network"
  "allow-host-processes"
  "allow-privilege-escalation"
  "allow-privileged"
  "allowed-apparmor-profiles"
  "allowed-capabilities"
  "allowed-host-paths"
  "allowed-proc-mount"
  "allowed-seccomp"
  "allowed-selinux"
  "allowed-sysctls"
  "allowed-users"
  "allowed-volume-types"
  "read-only-root-filesystem"
)

echo "Adding Deployment and DaemonSet tests to all test suites..."

for test_dir in "${TEST_DIRS[@]}"; do
  test_path="${BASE_DIR}/${test_dir}"
  
  if [ ! -d "${test_path}" ]; then
    echo "Skipping ${test_dir} - directory not found"
    continue
  fi
  
  echo "Processing ${test_dir}..."
  
  # Find constraint test directories
  constraint_dirs=$(find "${test_path}/test_samples/constraint_templates" -mindepth 1 -maxdepth 1 -type d 2>/dev/null | sort)
  
  for constraint_dir in ${constraint_dirs}; do
    constraint_name=$(basename "${constraint_dir}")
    
    # Find one allowed and one disallowed Pod test
    allowed_pod=$(find "${constraint_dir}" -name "allowed-*.yaml" -type f | head -1)
    disallowed_pod=$(find "${constraint_dir}" -name "disallowed-*.yaml" -type f | head -1)
    
    if [ -z "${allowed_pod}" ] || [ -z "${disallowed_pod}" ]; then
      echo "  Skipping ${constraint_name} - no suitable Pod tests found"
      continue
    fi
    
    allowed_name=$(basename "${allowed_pod}" .yaml)
    disallowed_name=$(basename "${disallowed_pod}" .yaml)
    
    # Create Deployment versions
    allowed_deployment="${constraint_dir}/allowed-deployment-${allowed_name#allowed-}.yaml"
    disallowed_deployment="${constraint_dir}/disallowed-deployment-${disallowed_name#disallowed-}.yaml"
    
    # Create DaemonSet versions
    allowed_daemonset="${constraint_dir}/allowed-daemonset-${allowed_name#allowed-}.yaml"
    disallowed_daemonset="${constraint_dir}/disallowed-daemonset-${disallowed_name#disallowed-}.yaml"
    
    # Convert Pod to Deployment
    if [ ! -f "${allowed_deployment}" ]; then
      pod_spec=$(grep -A 1000 "^spec:" "${allowed_pod}")
      cat > "${allowed_deployment}" <<EOF
apiVersion: apps/v1
kind: Deployment
metadata:
  name: allowed-deployment-${allowed_name#allowed-}
  namespace: testns
spec:
  replicas: 1
  selector:
    matchLabels:
      app: test
  template:
    metadata:
      labels:
        app: test
$(echo "${pod_spec}" | sed 's/^/    /')
EOF
      echo "  Created ${allowed_deployment}"
    fi
    
    if [ ! -f "${disallowed_deployment}" ]; then
      pod_spec=$(grep -A 1000 "^spec:" "${disallowed_pod}")
      cat > "${disallowed_deployment}" <<EOF
apiVersion: apps/v1
kind: Deployment
metadata:
  name: disallowed-deployment-${disallowed_name#disallowed-}
  namespace: testns
spec:
  replicas: 1
  selector:
    matchLabels:
      app: test
  template:
    metadata:
      labels:
        app: test
$(echo "${pod_spec}" | sed 's/^/    /')
EOF
      echo "  Created ${disallowed_deployment}"
    fi
    
    # Convert Pod to DaemonSet
    if [ ! -f "${allowed_daemonset}" ]; then
      pod_spec=$(grep -A 1000 "^spec:" "${allowed_pod}")
      cat > "${allowed_daemonset}" <<EOF
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: allowed-daemonset-${allowed_name#allowed-}
  namespace: testns
spec:
  selector:
    matchLabels:
      app: test
  template:
    metadata:
      labels:
        app: test
$(echo "${pod_spec}" | sed 's/^/    /')
EOF
      echo "  Created ${allowed_daemonset}"
    fi
    
    if [ ! -f "${disallowed_daemonset}" ]; then
      pod_spec=$(grep -A 1000 "^spec:" "${disallowed_pod}")
      cat > "${disallowed_daemonset}" <<EOF
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: disallowed-daemonset-${disallowed_name#disallowed-}
  namespace: testns
spec:
  selector:
    matchLabels:
      app: test
  template:
    metadata:
      labels:
        app: test
$(echo "${pod_spec}" | sed 's/^/    /')
EOF
      echo "  Created ${disallowed_daemonset}"
    fi
  done
done

echo ""
echo "Done creating Deployment and DaemonSet test files!"
echo "Now updating test_suite.yaml files..."

