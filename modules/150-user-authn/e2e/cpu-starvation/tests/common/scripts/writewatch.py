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

"""Logs every watch event of the given resources to count the writes to the API server.

WATCH is a comma-separated list of <group>/<version>/<plural>[@<namespace>], the core group is "core":
    coordination.k8s.io/v1/leases@d8-user-authn,deckhouse.io/v1alpha1/useraccounts

Every event is one line on stdout and in OUT:
    WRITE <ts_ms> <plural> <ADDED|MODIFIED|DELETED> <namespace>/<name>

The script uses the token of the pod's ServiceAccount. It relists after an expired watch and does not
log the objects of a list, so only real changes are counted. Runs until SIGTERM.

The list and the watch request only the object metadata (PartialObjectMetadataList and
PartialObjectMetadata), so the objects with secrets, such as the refresh tokens of Dex, never reach the
pod. The header has no fallback to full objects: an API without metadata-only responses answers
HTTP 406, and the script logs it and retries.
"""

import json
import os
import signal
import ssl
import sys
import threading
import time
import urllib.error
import urllib.parse as up
import urllib.request

SA = "/var/run/secrets/kubernetes.io/serviceaccount"
API = f"https://{os.environ['KUBERNETES_SERVICE_HOST']}:{os.environ.get('KUBERNETES_SERVICE_PORT', '443')}"
CTX = ssl.create_default_context(cafile=f"{SA}/ca.crt")

stop = threading.Event()
signal.signal(signal.SIGTERM, lambda *_: stop.set())
signal.signal(signal.SIGINT, lambda *_: stop.set())
lock = threading.Lock()
# Metadata-only responses: https://kubernetes.io/docs/reference/using-api/api-concepts/#metadata-only-fetches
# A list takes PartialObjectMetadataList. A watch takes PartialObjectMetadata, as in the metadata client of
# client-go: the API server converts the object of every event, and ERROR events keep their Status object.
ACCEPT_LIST = "application/json;as=PartialObjectMetadataList;g=meta.k8s.io;v=v1"
ACCEPT_WATCH = "application/json;as=PartialObjectMetadata;g=meta.k8s.io;v=v1"
out = open(os.environ.get("OUT", "/data/writes.log"), "a", buffering=1)


def log(line):
    with lock:
        out.write(line + "\n")
        print(line, flush=True)


def call(path, accept, timeout):
    token = open(f"{SA}/token").read().strip()
    req = urllib.request.Request(API + path, headers={"Authorization": f"Bearer {token}", "Accept": accept})
    return urllib.request.urlopen(req, context=CTX, timeout=timeout)


def base_path(spec):
    gvr, _, namespace = spec.partition("@")
    group, version, plural = gvr.split("/")
    prefix = f"/api/{version}" if group == "core" else f"/apis/{group}/{version}"
    if namespace:
        prefix += f"/namespaces/{namespace}"
    return f"{prefix}/{plural}", plural


def watch(spec):
    path, plural = base_path(spec)
    rv = None
    while not stop.is_set():
        try:
            if rv is None:
                with call(f"{path}?limit=1", ACCEPT_LIST, 30) as resp:
                    rv = json.load(resp)["metadata"]["resourceVersion"]
            query = up.urlencode(
                {"watch": "1", "resourceVersion": rv, "allowWatchBookmarks": "true", "timeoutSeconds": "300"}
            )
            with call(f"{path}?{query}", ACCEPT_WATCH, 360) as resp:
                for raw in resp:
                    if stop.is_set():
                        return
                    event = json.loads(raw)
                    obj = event.get("object") or {}
                    meta = obj.get("metadata") or {}
                    if event.get("type") == "ERROR":
                        rv = None  # 410 Gone: the resourceVersion is too old, relist
                        break
                    rv = meta.get("resourceVersion", rv)
                    if event.get("type") == "BOOKMARK":
                        continue
                    log(f"WRITE {int(time.time() * 1000)} {plural} {event.get('type')} "
                        f"{meta.get('namespace', '')}/{meta.get('name', '')}")
        except urllib.error.HTTPError as e:
            print(f"{plural}: HTTP {e.code}", file=sys.stderr, flush=True)
            if e.code == 410:
                rv = None
            stop.wait(5)
        except Exception as e:  # noqa: BLE001 - reconnect on any failure
            print(f"{plural}: {type(e).__name__}: {e}", file=sys.stderr, flush=True)
            stop.wait(2)


for item in filter(None, os.environ["WATCH"].split(",")):
    threading.Thread(target=watch, args=(item.strip(),), daemon=True).start()
while not stop.is_set():
    stop.wait(1)
