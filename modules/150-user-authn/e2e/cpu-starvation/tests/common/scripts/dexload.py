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

"""In-cluster Dex load: authorization-code logins and refresh-token grants.

The script talks to one Dex pod directly (DEX_URL, https://<POD_IP>:<PORT>) and sends the Host header
of the issuer, so the test does not depend on the ingress or on the network of the test runner, and
in a cluster with several Dex replicas the whole load reaches the replica under test.

Login workers sign in as LOGIN_EMAIL in a loop. Refresh workers sign in once as REFRESH_EMAIL and
then refresh their token in a loop; a failed refresh makes the worker sign in again. Two users keep
the login workers from evicting the refresh tokens through the per-client device limit of Dex.

Every finished flow is one line on stdout and in OUT:
    FLOW <start_ms> <duration_ms> <login|refresh> <ok 0|1> <detail>

Environment: DEX_URL, CLIENT_ID, CLIENT_SECRET, LOGIN_EMAIL, REFRESH_EMAIL, PASSWORD_FILE, OUT,
LOGIN_WORKERS, REFRESH_WORKERS, PAUSE, TIMEOUT. Runs until SIGTERM.
"""

import base64
import http.client
import json
import os
import signal
import ssl
import sys
import threading
import time
import urllib.parse as up

REDIRECT_URI = "http://localhost:8000/callback"

stop = threading.Event()
signal.signal(signal.SIGTERM, lambda *_: stop.set())
signal.signal(signal.SIGINT, lambda *_: stop.set())

env = os.environ
DEX = up.urlparse(env.get("DEX_URL", "https://dex.d8-user-authn.svc"))
CLIENT_ID = env["CLIENT_ID"]
CLIENT_SECRET = env["CLIENT_SECRET"]
LOGIN_EMAIL = env["LOGIN_EMAIL"]
REFRESH_EMAIL = env["REFRESH_EMAIL"]
LOGIN_WORKERS = int(env.get("LOGIN_WORKERS", "4"))
REFRESH_WORKERS = int(env.get("REFRESH_WORKERS", "4"))
PAUSE = float(env.get("PAUSE", "0.5"))
TIMEOUT = float(env.get("TIMEOUT", "30"))

# The Dex certificate is issued for the Service names, not for the pod IP, and the test does not pin its CA.
CTX = ssl.create_default_context()
CTX.check_hostname = False
CTX.verify_mode = ssl.CERT_NONE

lock = threading.Lock()
out = open(env.get("OUT", "/data/flows.log"), "a", buffering=1)


def record(t0, kind, ok, detail):
    detail = " ".join(str(detail).split())[:200] or "-"
    line = f"FLOW {int(t0 * 1000)} {int((time.time() - t0) * 1000)} {kind} {1 if ok else 0} {detail}"
    with lock:
        out.write(line + "\n")
        try:
            print(line, flush=True)
        except OSError:
            pass  # the file is the record; the pod log is a copy for people


def request(method, path, host_header, body=None, headers=None):
    conn = http.client.HTTPSConnection(DEX.hostname, DEX.port or 443, context=CTX, timeout=TIMEOUT)
    hdrs = {"Host": host_header}
    hdrs.update(headers or {})
    if body is not None:
        hdrs["Content-Type"] = "application/x-www-form-urlencoded"
        body = up.urlencode(body)
    try:
        conn.request(method, path, body=body, headers=hdrs)
        resp = conn.getresponse()
        data = resp.read()
        return resp.status, resp.getheader("Location") or "", data
    finally:
        conn.close()


def relative(location):
    u = up.urlparse(location)
    return (u.path or "/") + ("?" + u.query if u.query else "")


def discover():
    """Reads the issuer from the discovery document: its host goes into the Host header."""
    while not stop.is_set():
        try:
            status, _, data = request("GET", "/.well-known/openid-configuration", DEX.netloc)
            if status == 200:
                issuer = up.urlparse(json.loads(data)["issuer"].rstrip("/"))
                return issuer.netloc, issuer.path
            print(f"discovery: HTTP {status}", file=sys.stderr, flush=True)
        except Exception as e:  # noqa: BLE001 - any failure is retried
            print(f"discovery: {type(e).__name__}: {e}", file=sys.stderr, flush=True)
        stop.wait(5)
    sys.exit(0)


HOST, BASE = discover()
print(f"issuer host={HOST} path={BASE or '/'} via {DEX.geturl()}", file=sys.stderr, flush=True)
PASSWORD = open(env["PASSWORD_FILE"]).read().strip()
AUTH = {
    "Authorization": "Basic "
    + base64.b64encode(
        f"{up.quote(CLIENT_ID, safe='')}:{up.quote(CLIENT_SECRET, safe='')}".encode()
    ).decode()
}


def login(email):
    query = up.urlencode(
        {
            "client_id": CLIENT_ID,
            "redirect_uri": REDIRECT_URI,
            "response_type": "code",
            "scope": "openid email groups offline_access",
            "state": "e2e",
        }
    )
    status, loc, _ = request("GET", f"{BASE}/auth/local?{query}", HOST)
    if status not in (302, 303) or "login" not in loc:
        raise RuntimeError(f"auth HTTP {status}")
    status, loc, _ = request("POST", relative(loc), HOST, {"login": email, "password": PASSWORD})
    if status not in (302, 303):
        raise RuntimeError(f"login-post HTTP {status}")
    for _ in range(3):
        if loc.startswith(REDIRECT_URI):
            break
        status, loc, _ = request("GET", relative(loc), HOST)
        if status not in (302, 303):
            raise RuntimeError(f"approval HTTP {status}")
    code = up.parse_qs(up.urlparse(loc).query).get("code", [None])[0]
    if not code:
        raise RuntimeError("no code in the callback")
    status, _, data = request(
        "POST",
        f"{BASE}/token",
        HOST,
        {"grant_type": "authorization_code", "code": code, "redirect_uri": REDIRECT_URI},
        AUTH,
    )
    if status != 200:
        raise RuntimeError(f"token HTTP {status} {data[:80]!r}")
    return json.loads(data)["refresh_token"]


def refresh(token):
    status, _, data = request(
        "POST", f"{BASE}/token", HOST, {"grant_type": "refresh_token", "refresh_token": token}, AUTH
    )
    if status != 200:
        raise RuntimeError(f"refresh HTTP {status} {data[:80]!r}")
    return json.loads(data)["refresh_token"]


def login_worker():
    while not stop.is_set():
        t0 = time.time()
        try:
            login(LOGIN_EMAIL)
            record(t0, "login", True, "")
        except Exception as e:  # noqa: BLE001 - every failure is a data point
            record(t0, "login", False, f"{type(e).__name__}: {e}")
        stop.wait(PAUSE)


def refresh_worker():
    token = None
    while not stop.is_set():
        t0 = time.time()
        try:
            if token is None:
                token = login(REFRESH_EMAIL)
                record(t0, "login", True, "for-refresh")
                t0 = time.time()
            token = refresh(token)
            record(t0, "refresh", True, "")
        except Exception as e:  # noqa: BLE001 - every failure is a data point
            record(t0, "login" if token is None else "refresh", False, f"{type(e).__name__}: {e}")
            token = None
        stop.wait(PAUSE)


threads = [threading.Thread(target=login_worker, daemon=True) for _ in range(LOGIN_WORKERS)]
threads += [threading.Thread(target=refresh_worker, daemon=True) for _ in range(REFRESH_WORKERS)]
for t in threads:
    t.start()
while not stop.is_set():
    stop.wait(1)
