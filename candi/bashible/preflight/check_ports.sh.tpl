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

function check_python() {
    for pybin in python3 python2 python; do
      if command -v "$pybin" >/dev/null 2>&1; then
        python_binary="$pybin"
        return 0
      fi
    done
    echo "Python not found"
    return 1
}

function try_connect() {
    cat - <<EOF | $python_binary
try:
    from urllib.request import urlopen, Request
    from urllib.error import URLError
except ImportError as e:
    from urllib2 import urlopen, Request, URLError

req = Request('http://127.0.0.1:$1')
try: urlopen(req, timeout=1)
except URLError as e:
    exit(1)
except TimeoutError as e:
    exit(0)
exit(0)
EOF
}

function start_http_server() {
  cat - <<EOF | $python_binary
import sys

try:
    from SimpleHTTPServer import SimpleHTTPRequestHandler
except ImportError:
    from http.server import SimpleHTTPRequestHandler

try:
    from SocketServer import TCPServer as HTTPServer
except ImportError:
    from http.server import HTTPServer

http_server = HTTPServer(("", $1), SimpleHTTPRequestHandler)
http_server.serve_forever()
EOF
}

function check_port() {
    try_connect $1

    if [ $? -eq 0 ]; then
        echo -n "port is already open "; return 1
    fi

    start_http_server $1 > /dev/null 2>&1 &
    local PID=$!
    sleep 0.1

    try_connect $1
    local exit_code=$?

    if ps -p $PID > /dev/null
    then
        pkill -P $PID
        wait $PID 2>/dev/null
    fi

    return $exit_code
}

has_error=false

# Without a python there is nothing to open a socket with, so every port below would be reported
# as unavailable. Saying "port 6443 is closed" when the truth is "this node has no python" sends
# the reader to the firewall for a problem that is not there.
if ! check_python; then
    exit 1
fi

firewall_note="The control-plane node is likely behind firewall rules or another tool (such as an antivirus) that blocks incoming connections."

# The ports the control plane binds on the node, and who binds each one. A port already held is
# not a firewall problem at all — it is another process, usually a previous install that was
# never cleaned — so the two cases get different advice.
#
# One group per line: a label, then "port=what needs it" pairs.
#
# 4282 is deliberately absent: registry-packages-proxy listens on it inside a pod, not on the
# node, so nothing on the host competes for it.
port_groups=(
  "kubernetes API (6443)|6443=the Kubernetes API server"
  "Etcd (2379, 2380)|2379=etcd client connections|2380=etcd peer communication"
  "kubelet (10250)|10250=kubelet"
  "kubernetes-api-proxy (6445, 6480)|6445=kubernetes-api-proxy|6480=the kubernetes-api-proxy health endpoint"
  "registry (5001, 5444)|5001=the in-cluster registry|5444=the registry packages proxy dhctl brings up during the bootstrap"
)

for group in "${port_groups[@]}"; do
    IFS='|' read -r -a fields <<< "$group"
    echo -n "Checking if ${fields[0]} ports are available "

    group_ok=true
    for entry in "${fields[@]:1}"; do
        port="${entry%%=*}"
        purpose="${entry#*=}"

        if ! check_port "$port"; then
            echo "Port ${port} is not available but is required by ${purpose}. ${firewall_note} If a previous installation is still running on this node, clean it up first; \`ss -lntp | grep :${port}\` names the process holding it."
            group_ok=false
        fi
    done

    if [ "$group_ok" == true ]; then
        echo "SUCCESS"
    else
        has_error=true
    fi
done

if [ "$has_error" == true ]; then
  exit 1
fi

exit 0
