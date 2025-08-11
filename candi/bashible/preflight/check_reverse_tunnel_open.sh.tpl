#!/usr/bin/env bash
{{- /*
# Copyright 2024 Flant JSC
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
except ImportError as e:
    from urllib2 import urlopen, Request

ssl._create_default_https_context = ssl._create_unverified_context
request = Request('{{.url}}')
res = False
try:
    response = urlopen(request, timeout=5)
    res = True if response else False
except Exception as err:
    res = False

exit(0) if res else exit(1)

EOF


