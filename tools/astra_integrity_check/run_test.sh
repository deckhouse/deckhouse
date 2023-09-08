#!/bin/bash

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

# Usage example
# SSH should be up and accessible on the host
#
# ASTRA_KEY=~/.ssh/astra ASTRA_USER=astra ASTRA_HOST=1.2.3.4 ASTRA_SUMS=~/Downloads/gostsums.txt ./run_test.sh
#
# gostsums.txt can be downloaded from Astra consumer portal and should match Astra Linux version you are about to test.


CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o test_payload || exit 1

scp -i ${ASTRA_KEY} ${ASTRA_SUMS} ${ASTRA_USER}@${ASTRA_HOST}:/tmp/gostsums.txt || exit 1
scp -i ${ASTRA_KEY} ./test_payload ${ASTRA_USER}@${ASTRA_HOST}:/tmp/int-test-payload || exit 1
rm -f ./test_payload

ssh -i ${ASTRA_KEY} ${ASTRA_USER}@${ASTRA_HOST} "sudo /tmp/int-test-payload -g /tmp/gostsums.txt"
