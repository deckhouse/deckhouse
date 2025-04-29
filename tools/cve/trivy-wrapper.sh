#!/bin/bash

#
# Copyright 2023 Flant JSC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
#

set -Eeo pipefail
shopt -s failglob

# This helper script provide functions to generate list of CVEs found in image or render part of
# html report for specified image.
##
# Usage:
#   source trivy-wrapper.sh
#   <function> [<optional arguments>...] (-i|--image) <image to scan>
#
# Optional arguments are:
#   [(-l|--label) <HTML report label>]
#   [(-r|--registry) <repository to pull image from>]
#   [(-t|--tag) <image tag name>]
#   [--severity <comma-separated severity list>]
#   [--ignore <Trivy ignore file>]

function prepareImageArgs() {
  unset LABEL REGISTRY IMAGE TAG SEVERITY IGNORE

  while [[ $# -gt 0 ]]; do
    case $1 in
    -l | --label)
      LABEL="$2"
      shift
      shift
      ;;
    -r | --regisry)
      REGISTRY="$2"
      shift
      shift
      ;;
    -i | --image)
      IMAGE="$2"
      shift
      shift
      ;;
    -t | --tag)
      TAG="$2"
      shift
      shift
      ;;
    -s | --severity)
      SEVERITY="$2"
      shift
      shift
      ;;
    --ignore)
      IGNORE="$2"
      shift
      shift
      ;;
    -o | --output)
      OUTPUT="$2"
      shift
      shift
      ;;
    *)
      echo "Trivy-wrapper: Unknown option $1"
      exit 1
      ;;
    esac
  done

  if [ -z "$IMAGE" ]; then
    exit 1
  fi
  IMAGE_ARGS="$IMAGE"

  if [ -n "$REGISTRY" ]; then
    IMAGE_ARGS="$REGISTRY$IMAGE_ARGS"
  fi
  if [ -n "$TAG" ]; then
    IMAGE_ARGS="$IMAGE_ARGS:$TAG"
  fi

  if [ -z "$LABEL" ]; then
    LABEL="$IMAGE_ARGS"
  fi

  if [ -z "$SEVERITY" ]; then
    SEVERITY="UNKNOWN,LOW,MEDIUM,HIGH,CRITICAL"
  fi

  if [ -z "$OUTPUT" ]; then
    OUTPUT="out/report.json"
  fi
}

function trivyGetCVEListForImage() (
  prepareImageArgs "$@"
  attempt=1
  max_attempts=5
  delay=5
  debug_flag=""
  quiet_flag="--quiet"

  while true; do
    echo "[*] Trivy scan attempt $attempt (debug: ${debug_flag:+on}, quiet: ${quiet_flag:+on})"
    set +e
    bin/trivy i $debug_flag \
    --username "$REGISTRY_USER" \
    --password "$REGISTRY_PASSWORD"\
    --policy "$TRIVY_POLICY_URL" \
    --java-db-repository "$TRIVY_JAVA_DB_URL" \
    --db-repository "$TRIVY_DB_URL" \
    --exit-code 0 \
    --severity $SEVERITY \
    --ignorefile "$IGNORE" \
    --image-src remote \
    --format json \
    --scanners vuln \
    $quiet_flag "$IMAGE_ARGS" | jq -r ".Results[]?.Vulnerabilities[]?.VulnerabilityID" | uniq | sort
    exit_code=$?
    set -e
    if [ $exit_code -eq 0 ]; then
      break
    fi
    echo "[!] Trivy failed with code $exit_code on attempt $attempt"

    if [ $attempt -ge $max_attempts ]; then
      echo "[!] Failed after $max_attempts attempts, exiting"
      exit $exit_code
    fi

    attempt=$((attempt + 1))
    debug_flag="--debug"
    quiet_flag=""
    echo "[*] Retrying in $delay seconds..."
    sleep $delay
  done
  # bin/trivy i --severity=$SEVERITY --ignorefile "$IGNORE" --format json --quiet "$IMAGE_ARGS" | jq -r ".Results[]?.Vulnerabilities[]?.VulnerabilityID" | uniq | sort
)

function htmlReportHeader() (
  cat tools/cve/html/header.tpl
)

function trivyGetJSONReportPartForImage() (
  prepareImageArgs "$@"
  echo -n "    <h1>$LABEL</h1>"
  attempt=1
  max_attempts=5
  delay=5
  debug_flag=""
  quiet_flag="--quiet"

  while true; do
    echo "[*] Trivy scan attempt $attempt (debug: ${debug_flag:+on}, quiet: ${quiet_flag:+on})"
    set +e
    bin/trivy i $debug_flag \
    --username "$REGISTRY_USER" \
    --password "$REGISTRY_PASSWORD"\
    --policy "$TRIVY_POLICY_URL" \
    --java-db-repository "$TRIVY_JAVA_DB_URL" \
    --db-repository "$TRIVY_DB_URL" \
    --exit-code 0 \
    --severity "$SEVERITY" \
    --ignorefile "$IGNORE" \
    --image-src remote \
    --format json \
    --scanners vuln \
    --output "$OUTPUT" \
    $quiet_flag \
    "$IMAGE_ARGS"
    exit_code=$?
    set -e
    if [ $exit_code -eq 0 ]; then
      echo -n "    <br/>"
      break
    fi
    echo "[!] Trivy failed with code $exit_code on attempt $attempt"

    if [ $attempt -ge $max_attempts ]; then
      echo "[!] Failed after $max_attempts attempts, exiting"
      exit $exit_code
    fi

    attempt=$((attempt + 1))
    debug_flag="--debug"
    quiet_flag=""
    echo "[*] Retrying in $delay seconds..."
    sleep $delay
  done
)
