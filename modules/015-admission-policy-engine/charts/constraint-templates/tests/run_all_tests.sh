#!/bin/bash

# Script to run all tests and show summary statistics
# Usage: ./run_all_tests.sh [test_directory]

set -o pipefail

BASE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Function to discover all test directories
# Finds all directories containing test_suite.yaml file
discover_test_dirs() {
  local dirs=()
  for dir in "${BASE_DIR}"/*; do
    if [ -d "${dir}" ] && [ -f "${dir}/test_suite.yaml" ]; then
      local dirname=$(basename "${dir}")
      # Skip common directory and other non-test directories
      if [ "${dirname}" != "common" ]; then
        dirs+=("${dirname}")
      fi
    fi
  done
  # Sort directories alphabetically
  printf '%s\n' "${dirs[@]}" | sort
}

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Function to run tests for a single directory
run_test() {
  local test_dir="$1"
  local test_path="${BASE_DIR}/${test_dir}"
  
  if [ ! -d "${test_path}" ]; then
    echo -e "${YELLOW}[SKIP]${NC} ${test_dir} - directory not found"
    return 2
  fi
  
  if [ ! -f "${test_path}/test_suite.yaml" ]; then
    echo -e "${YELLOW}[SKIP]${NC} ${test_dir} - no test_suite.yaml found"
    return 2
  fi
  
  echo -e "\n${YELLOW}Running tests for: ${test_dir}${NC}"
  echo "----------------------------------------"
  
  cd "${test_path}"
  
  # Run gator verify and capture output
  output=$(gator verify -v . 2>&1) || true
  exit_code=$?
  
  if [ ${exit_code} -eq 0 ] || echo "${output}" | grep -q "PASS\|FAIL"; then
    # Count tests - use awk to avoid grep issues with -- on macOS
    local passed=$(echo "${output}" | awk '/--- PASS:/ {count++} END {print count+0}')
    local failed=$(echo "${output}" | awk '/--- FAIL:/ {count++} END {print count+0}')
    local total=$((passed + failed))
    
    if [ ${failed} -eq 0 ] && [ ${total} -gt 0 ]; then
      echo -e "${GREEN}[PASS]${NC} ${test_dir} - ${passed}/${total} tests passed"
      return 0
    elif [ ${failed} -gt 0 ]; then
      echo -e "${RED}[FAIL]${NC} ${test_dir} - ${passed}/${total} passed, ${failed} failed"
      echo "${output}" | awk '/--- FAIL:/ {flag=1} flag && NR<=c+5 {print} /--- FAIL:/ {c=NR}' || echo "${output}" | tail -20
      return 1
    else
      echo -e "${YELLOW}[SKIP]${NC} ${test_dir} - no tests found"
      return 2
    fi
  else
    echo -e "${RED}[FAIL]${NC} ${test_dir} - test execution failed (exit code: ${exit_code})"
    echo "${output}" | tail -30
    return 1
  fi
}

# Main execution
main() {
  # Discover all test directories automatically
  local test_dirs=($(discover_test_dirs))
  
  # If specific directory provided, test only that one
  if [ $# -eq 1 ]; then
    test_dirs=("$1")
  fi
  
  if [ ${#test_dirs[@]} -eq 0 ]; then
    echo "No test directories found in ${BASE_DIR}"
    exit 1
  fi
  
  echo "=========================================="
  echo "Gatekeeper Tests Runner"
  echo "Found ${#test_dirs[@]} test directory(ies)"
  echo "=========================================="
  echo ""
  
  local total_passed=0
  local total_failed=0
  local total_skipped=0
  local passed_dirs=()
  local failed_dirs=()
  local skipped_dirs=()
  
  # Run tests for each directory
  for test_dir in "${test_dirs[@]}"; do
    if run_test "${test_dir}"; then
      ((total_passed++))
      passed_dirs+=("${test_dir}")
    elif [ $? -eq 1 ]; then
      ((total_failed++))
      failed_dirs+=("${test_dir}")
    else
      ((total_skipped++))
      skipped_dirs+=("${test_dir}")
    fi
  done
  
  # Print summary
  echo ""
  echo "=========================================="
  echo "Summary"
  echo "=========================================="
  echo -e "${GREEN}Passed:${NC} ${total_passed}"
  echo -e "${RED}Failed:${NC} ${total_failed}"
  echo -e "${YELLOW}Skipped:${NC} ${total_skipped}"
  echo ""
  
  if [ ${total_passed} -gt 0 ]; then
    echo -e "${GREEN}Passed directories:${NC}"
    for dir in "${passed_dirs[@]}"; do
      echo "  ✓ ${dir}"
    done
    echo ""
  fi
  
  if [ ${total_failed} -gt 0 ]; then
    echo -e "${RED}Failed directories:${NC}"
    for dir in "${failed_dirs[@]}"; do
      echo "  ✗ ${dir}"
    done
    echo ""
  fi
  
  if [ ${total_skipped} -gt 0 ]; then
    echo -e "${YELLOW}Skipped directories:${NC}"
    for dir in "${skipped_dirs[@]}"; do
      echo "  ⊘ ${dir}"
    done
    echo ""
  fi
  
  # Exit with appropriate code
  if [ ${total_failed} -gt 0 ]; then
    exit 1
  else
    exit 0
  fi
}

main "$@"

