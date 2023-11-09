#!/usr/bin/env bash
{{- /*
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
*/}}
function try_connect() {
   python3 << EOF
import urllib.request
req = urllib.request.Request('http://127.0.0.1:$1')
try: urllib.request.urlopen(req, timeout=1)
except urllib.error.URLError as e:
    exit(1)
except TimeoutError as e:
    exit(0)
exit(0)
EOF
}

function check_port() {
    try_connect $1

    if [ $? -eq 0 ]; then
        echo -n "it is already open "; return 1
    fi

    python3 -m http.server $1 > /dev/null 2>&1 &
    local PID=$!
    sleep 0.1

    try_connect $1
    local exit_code=$?

    if ps -p $PID > /dev/null
    then
        kill -9 $PID
        wait $PID 2>/dev/null
    fi

    return $exit_code
}

has_error=false

echo -n "Checking if kubernetes API port is open (6443) "
check_port 6443
if [ $? -ne 0 ]; then
    echo "Port 6443 is closed, but required for Kubernetes API server to function. Probably control-plane node is protected by firewall rules or another software (like antivirus) and blocks connections."
    has_error=true
fi
echo "SUCCESS"

echo -n "Checking if Etcd ports are available (2379, 2380) "
check_port 2379
if [ $? -ne 0 ]; then
    echo "Port 2379 is closed, but required for Etcd clients to communicate with it. Probably control-plane node is protected by firewall rules or another software (like antivirus) and blocks connections."
    has_error=true
fi

check_port 2380
if [ $? -ne 0 ]; then
    echo "Port 2380 is closed, but required for Etcd database server peers communications. Probably control-plane node is protected by firewall rules or another software (like antivirus) and blocks connections."
    has_error=true
fi
echo "SUCCESS"

if [ "$has_error" == true ]; then
  exit 1
fi

exit 0
