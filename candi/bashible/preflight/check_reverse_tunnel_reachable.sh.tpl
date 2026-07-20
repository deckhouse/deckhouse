#!/usr/bin/env bash
{{- /*
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
*/}}

{{- $python_discovery := .Files.Get "deckhouse/candi/bashible/check_python.sh.tpl" }}
{{- tpl ( $python_discovery ) . | nindent 0 }}

check_python

cat - <<EOF | $python_binary
import ssl
try:
    from urllib.request import urlopen, Request
    from urllib.error import HTTPError
except ImportError as e:
    from urllib2 import urlopen, Request, HTTPError

ssl._create_default_https_context = ssl._create_unverified_context
request = Request('{{.url}}')
alive = False
try:
    urlopen(request, timeout=5)
    alive = True
except HTTPError:
    # Any HTTP status (e.g. 404 from the rpp-get server) proves the SSH
    # channel is alive end-to-end, so the reverse tunnel is healthy.
    alive = True
except Exception as err:
    alive = False

exit(0) if alive else exit(1)

EOF
