#!/bin/sh

# Copyright 2026 Flant JSC
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

set -eu

BASE_URL="${1:-http://ru.localhost}"
PAGE_PATH="${2:-/products/kubernetes-platform/documentation/v1/}"
EXPECTED_FRAGMENT="${3:-reusable-content source=/modules/operator-trivy/alpha/partials/feature-test1.html}"
EXPECTED_STATIC_PATH="${4:-/modules/operator-trivy/alpha/partials/static/feature-test1.png}"

page_html="$(mktemp)"
metrics_out="$(mktemp)"

cleanup() {
  rm -f "$page_html" "$metrics_out"
}

trap cleanup EXIT

curl -fsSL "${BASE_URL}${PAGE_PATH}" -o "$page_html"

if grep -q 'application/x-module-include' "$page_html"; then
  echo "placeholder tag is still present in rendered HTML"
  exit 1
fi

if ! grep -q "$EXPECTED_FRAGMENT" "$page_html"; then
  echo "expected reusable content fragment was not injected"
  exit 1
fi

if ! grep -q "$EXPECTED_STATIC_PATH" "$page_html"; then
  echo "expected rewritten static asset URL was not found"
  exit 1
fi

curl -fsSL "${BASE_URL}/reusable-content-metrics" -o "$metrics_out"

for metric in \
  reusable_content_pages_total \
  reusable_content_placeholders_total \
  reusable_content_include_requests_total
do
  if ! grep -q "^${metric} " "$metrics_out"; then
    echo "metric ${metric} is missing"
    exit 1
  fi
done

echo "reusable content smoke check passed"
