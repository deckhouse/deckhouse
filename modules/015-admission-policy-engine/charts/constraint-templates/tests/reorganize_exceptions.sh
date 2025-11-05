#!/bin/bash

# Script to reorganize security_policy_exceptions directories
# Splits files into 'exceptions' (SecurityPolicyException CRD) and 'pods' (Pod objects) subdirectories

set -e

BASE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# List of test directories that have security_policy_exceptions
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

echo "Reorganizing security_policy_exceptions directories..."

for test_dir in "${TEST_DIRS[@]}"; do
  exceptions_dir="${BASE_DIR}/${test_dir}/test_samples/security_policy_exceptions"
  
  if [ ! -d "${exceptions_dir}" ]; then
    echo "Skipping ${test_dir} - no security_policy_exceptions directory found"
    continue
  fi
  
  echo "Processing ${test_dir}..."
  
  # Create subdirectories
  mkdir -p "${exceptions_dir}/exceptions"
  mkdir -p "${exceptions_dir}/pods"
  
  # Move files based on their content
  for file in "${exceptions_dir}"/*.yaml; do
    if [ ! -f "${file}" ]; then
      continue
    fi
    
    filename=$(basename "${file}")
    
    # Check if it's a SecurityPolicyException (exceptions) or Pod (pods)
    if grep -q "kind: SecurityPolicyException" "${file}" && grep -q "apiVersion: deckhouse.io/v1alpha1" "${file}"; then
      mv "${file}" "${exceptions_dir}/exceptions/${filename}"
      echo "  Moved ${filename} -> exceptions/"
    elif grep -q "kind: Pod" "${file}" && grep -q "apiVersion: v1" "${file}"; then
      mv "${file}" "${exceptions_dir}/pods/${filename}"
      echo "  Moved ${filename} -> pods/"
    else
      echo "  WARNING: Could not determine type of ${filename}, skipping"
    fi
  done
  
  echo "  Done with ${test_dir}"
done

echo ""
echo "Reorganization complete!"

