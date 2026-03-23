#!/bin/bash

#Copyright 2026 Flant JSC
#
#Licensed under the Apache License, Version 2.0 (the "License");
#you may not use this file except in compliance with the License.
#You may obtain a copy of the License at
#
#    http://www.apache.org/licenses/LICENSE-2.0
#
#Unless required by applicable law or agreed to in writing, software
#distributed under the License is distributed on an "AS IS" BASIS,
#WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
#See the License for the specific language governing permissions and
#limitations under the License.

set -euo pipefail

BASE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
LIBS_DIR="${BASE_DIR}/libs"
CONSTRAINTS_ROOT="${BASE_DIR}/constraints"
CONSTRAINT_TESTGEN="${BASE_DIR}/../tools/constraint_testgen"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

OPA_TOTAL=0
OPA_PASSED=0
OPA_FAILED=0

TOTAL_CONSTRAINTS=0
PASSED_CONSTRAINTS=0

FAILED_TESTS=()
FAILED_CONSTRAINTS=()
FAILED_GATOR_TESTS=()
LOW_COVERAGE=()

require_command() {
  local cmd="$1"
  if ! command -v "${cmd}" >/dev/null 2>&1; then
    echo -e "${RED}[FAIL]${NC} required command not found: ${cmd}"
    exit 1
  fi
}

collect_constraint_dirs() {
  local group
  for group in operation security; do
    local group_dir="${CONSTRAINTS_ROOT}/${group}"
    if [ ! -d "${group_dir}" ]; then
      continue
    fi

    while IFS= read -r dir; do
      [ -n "${dir}" ] || continue
      echo "${dir}"
    done < <(find "${group_dir}" -mindepth 1 -maxdepth 1 -type d ! -name 'test_samples' ! -name '.*' | sort)
  done
}

run_opa_library_tests() {
  echo "=========================================="
  echo "OPA library tests"
  echo "=========================================="

  local output=""
  if output="$(cd "${LIBS_DIR}" && opa test . -v 2>&1)"; then
    :
  else
    FAILED_TESTS+=("opa library tests")
  fi

  local summary_line
  summary_line="$(printf '%s\n' "${output}" | awk '/^(PASS|FAIL): [0-9]+\/[0-9]+$/ {line=$0} END {print line}')"

  if [[ "${summary_line}" =~ ^PASS:[[:space:]]([0-9]+)/([0-9]+)$ ]]; then
    OPA_PASSED="${BASH_REMATCH[1]}"
    OPA_TOTAL="${BASH_REMATCH[2]}"
    OPA_FAILED=$((OPA_TOTAL - OPA_PASSED))
  elif [[ "${summary_line}" =~ ^FAIL:[[:space:]]([0-9]+)/([0-9]+)$ ]]; then
    OPA_FAILED="${BASH_REMATCH[1]}"
    OPA_TOTAL="${BASH_REMATCH[2]}"
    OPA_PASSED=$((OPA_TOTAL - OPA_FAILED))
  fi

  if [ -z "${summary_line}" ] && [ ${#FAILED_TESTS[@]} -gt 0 ]; then
    OPA_TOTAL=1
    OPA_PASSED=0
    OPA_FAILED=1
  fi

  if [ ${#FAILED_TESTS[@]} -eq 0 ]; then
    echo -e "${GREEN}[PASS]${NC} OPA library tests: passed=${OPA_PASSED}, failed=${OPA_FAILED}, total=${OPA_TOTAL}"
  else
    echo -e "${RED}[FAIL]${NC} OPA library tests: passed=${OPA_PASSED}, failed=${OPA_FAILED}, total=${OPA_TOTAL}"
    printf '%s\n' "${output}" | tail -n 40
  fi
}

run_constraint_tests() {
  echo ""
  echo "=========================================="
  echo "Constraint tests (generate + gator verify)"
  echo "=========================================="

  while IFS= read -r constraint_dir; do
    [ -n "${constraint_dir}" ] || continue

    TOTAL_CONSTRAINTS=$((TOTAL_CONSTRAINTS + 1))
    local rel
    rel="${constraint_dir#${CONSTRAINTS_ROOT}/}"

    echo ""
    echo -e "${YELLOW}[RUN]${NC} ${rel}"

    rm -rf "${constraint_dir}/rendered"

    if ! (cd "${constraint_dir}" && go run "${CONSTRAINT_TESTGEN}" generate -bundle ./test-matrix.yaml); then
      echo -e "${RED}[FAIL]${NC} ${rel}: generate failed"
      FAILED_CONSTRAINTS+=("${rel} (generate)")
      continue
    fi

    local verify_output=""
    local verify_exit=0
    set +e
    verify_output="$(cd "${constraint_dir}" && gator verify -v ./rendered 2>&1)"
    verify_exit=$?
    set -e
    if [ ${verify_exit} -ne 0 ]; then
      echo -e "${RED}[FAIL]${NC} ${rel}: gator verify failed"
      printf '%s\n' "${verify_output}" | tail -n 40
      FAILED_CONSTRAINTS+=("${rel} (verify)")

      while IFS= read -r failed_test; do
        [ -n "${failed_test}" ] || continue
        FAILED_GATOR_TESTS+=("${rel}: ${failed_test}")
      done < <(printf '%s\n' "${verify_output}" | awk '/--- FAIL:/ {sub(/^--- FAIL:[[:space:]]*/, "", $0); print $0}')

      continue
    fi

    PASSED_CONSTRAINTS=$((PASSED_CONSTRAINTS + 1))
    echo -e "${GREEN}[PASS]${NC} ${rel}"
  done < <(collect_constraint_dirs)
}

run_coverage_checks() {
  echo ""
  echo "=========================================="
  echo "Coverage"
  echo "=========================================="

  local coverage_output
  if ! coverage_output="$(go run "${CONSTRAINT_TESTGEN}" coverage -tests-root "${CONSTRAINTS_ROOT}" -format json 2>&1)"; then
    echo -e "${RED}[FAIL]${NC} coverage command failed"
    printf '%s\n' "${coverage_output}" | tail -n 40
    FAILED_TESTS+=("coverage")
    return
  fi

  local coverage_parse_output
  if ! coverage_parse_output="$(printf '%s\n' "${coverage_output}" | python3 -c '
import json
import re
import sys

raw = sys.stdin.read()
match = re.search(r"\{.*\}", raw, re.S)
if not match:
    print("PARSE_ERROR")
    sys.exit(1)

report = json.loads(match.group(0))
constraints = report.get("constraints", [])
print(f"TOTAL={len(constraints)}")

for c in constraints:
    fields = c.get("fields")
    if isinstance(fields, dict):
        pct = fields.get("coverage_pct")
        if isinstance(pct, int) and pct < 100:
            name = c.get("directory") or c.get("name") or "unknown"
            print(f"LOW={name}:{pct}%")
')"; then
    echo -e "${RED}[FAIL]${NC} coverage output parse failed"
    printf '%s\n' "${coverage_output}" | tail -n 40
    FAILED_TESTS+=("coverage(parse)")
    return
  fi

  local total
  total="$(printf '%s\n' "${coverage_parse_output}" | awk -F= '/^TOTAL=/{print $2}')"
  [ -n "${total}" ] || total="0"

  while IFS= read -r low_line; do
    [ -n "${low_line}" ] || continue
    LOW_COVERAGE+=("${low_line#LOW=}")
  done < <(printf '%s\n' "${coverage_parse_output}" | awk '/^LOW=/ {print}')

  if [ ${#LOW_COVERAGE[@]} -eq 0 ]; then
    echo -e "${GREEN}[PASS]${NC} coverage is 100% for all ${total} constraints with reported field coverage"
  else
    echo -e "${RED}[FAIL]${NC} constraints with coverage < 100%: ${#LOW_COVERAGE[@]}"
    local item
    for item in "${LOW_COVERAGE[@]}"; do
      echo "  - ${item}"
    done
  fi
}

print_final_summary() {
  echo ""
  echo "============================================================"
  echo "                    TEST RUNNER SUMMARY"
  echo "============================================================"

  local all_tests_passed=true
  local coverage_everywhere=true
  local constraint_failed=$((TOTAL_CONSTRAINTS - PASSED_CONSTRAINTS))

  if [ ${#FAILED_TESTS[@]} -gt 0 ] || [ ${#FAILED_CONSTRAINTS[@]} -gt 0 ]; then
    all_tests_passed=false
  fi

  if [ ${#LOW_COVERAGE[@]} -gt 0 ] || [[ " ${FAILED_TESTS[*]-} " == *" coverage"* ]] || [[ " ${FAILED_TESTS[*]-} " == *" coverage(parse)"* ]]; then
    coverage_everywhere=false
  fi

  echo ""
  printf "%-34s %s\n" "OPA tests" "${OPA_PASSED}/${OPA_TOTAL} passed, ${OPA_FAILED} failed"
  printf "%-34s %s\n" "Gatekeeper constraints" "${PASSED_CONSTRAINTS}/${TOTAL_CONSTRAINTS} passed, ${constraint_failed} failed"

  if [ "${all_tests_passed}" = "true" ]; then
    printf "%-34s %b\n" "All tests passed" "${GREEN}YES${NC}"
  else
    printf "%-34s %b\n" "All tests passed" "${RED}NO${NC}"
  fi

  if [ "${coverage_everywhere}" = "true" ]; then
    printf "%-34s %b\n" "Coverage is 100% everywhere" "${GREEN}YES${NC}"
  else
    printf "%-34s %b\n" "Coverage is 100% everywhere" "${RED}NO${NC}"
  fi

  if [ ${#FAILED_TESTS[@]} -gt 0 ]; then
    echo ""
    echo "[Failed stages]"
    local item
    for item in "${FAILED_TESTS[@]}"; do
      echo "  - ${item}"
    done
  fi

  if [ ${#FAILED_CONSTRAINTS[@]} -gt 0 ]; then
    echo ""
    echo "[Failed constraints]"
    local item
    for item in "${FAILED_CONSTRAINTS[@]}"; do
      echo "  - ${item}"
    done
  fi

  if [ ${#FAILED_GATOR_TESTS[@]} -gt 0 ]; then
    echo ""
    echo "[Failed Gatekeeper tests]"
    local item
    for item in "${FAILED_GATOR_TESTS[@]}"; do
      echo "  - ${item}"
    done
  fi

  if [ ${#LOW_COVERAGE[@]} -gt 0 ]; then
    echo ""
    echo "[Coverage < 100%]"
    local item
    for item in "${LOW_COVERAGE[@]}"; do
      echo "  - ${item}"
    done
  fi

  echo "============================================================"

  if [ "${all_tests_passed}" != "true" ] || [ "${coverage_everywhere}" != "true" ]; then
    exit 1
  fi

  exit 0
}

main() {
  require_command opa
  require_command gator
  require_command go
  require_command python3

  run_opa_library_tests
  run_constraint_tests
  run_coverage_checks
  print_final_summary
}

main "$@"
