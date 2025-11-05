#!/bin/bash

# Script to update paths in test_suite.yaml files after reorganization
# Updates paths from security_policy_exceptions/ to security_policy_exceptions/exceptions/ or security_policy_exceptions/pods/

set -e

BASE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# List of test directories
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

echo "Updating test_suite.yaml files..."

for test_dir in "${TEST_DIRS[@]}"; do
  suite_file="${BASE_DIR}/${test_dir}/test_suite.yaml"
  exceptions_dir="${BASE_DIR}/${test_dir}/test_samples/security_policy_exceptions"
  
  if [ ! -f "${suite_file}" ]; then
    echo "Skipping ${test_dir} - no test_suite.yaml found"
    continue
  fi
  
  if [ ! -d "${exceptions_dir}" ]; then
    echo "Skipping ${test_dir} - no security_policy_exceptions directory found"
    continue
  fi
  
  echo "Processing ${test_dir}..."
  
  # Create temporary file
  temp_file=$(mktemp)
  
  # Read file and update paths
  while IFS= read -r line; do
    # Check if line contains security_policy_exceptions path
    if echo "${line}" | grep -q "test_samples/security_policy_exceptions/"; then
      # Get filename from path
      filename=$(echo "${line}" | sed -n 's/.*test_samples\/security_policy_exceptions\/\([^"]*\)\.yaml.*/\1/p')
      
      if [ -n "${filename}" ]; then
        # Check if file exists in exceptions or pods directory
        if [ -f "${exceptions_dir}/exceptions/${filename}.yaml" ]; then
          echo "${line}" | sed "s|test_samples/security_policy_exceptions/${filename}\.yaml|test_samples/security_policy_exceptions/exceptions/${filename}.yaml|g" >> "${temp_file}"
        elif [ -f "${exceptions_dir}/pods/${filename}.yaml" ]; then
          echo "${line}" | sed "s|test_samples/security_policy_exceptions/${filename}\.yaml|test_samples/security_policy_exceptions/pods/${filename}.yaml|g" >> "${temp_file}"
        else
          echo "  WARNING: File ${filename}.yaml not found in exceptions/ or pods/, keeping original path"
          echo "${line}" >> "${temp_file}"
        fi
      else
        echo "${line}" >> "${temp_file}"
      fi
    else
      echo "${line}" >> "${temp_file}"
    fi
  done < "${suite_file}"
  
  mv "${temp_file}" "${suite_file}"
  echo "  Updated ${test_dir}/test_suite.yaml"
done

echo ""
echo "Path updates complete!"

