#!/usr/bin/python3

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

"""Runs Python hooks for webhook-operator without starting an interpreter per request.

Started once with the path of a unix socket. It imports what the hooks import, then for every
connection forks and runs the requested hook file in the child, as `python3 <hook>` would:
`__name__ == "__main__"`, sys.argv, sys.path[0] set to the hook directory, the environment of
the request, and stdout/stderr on the descriptors webhook-operator passed along. The parent never
runs hook code, so no state is carried from one request to the next.

Protocol, one request per connection (internal/hookrunner is the other side):
  request:  4-byte big-endian length, then JSON {"path", "args", "env", "dir"}; the stdout and
            stderr descriptors travel with it as SCM_RIGHTS.
  response: a JSON line {"exit_code", "utime", "stime", "maxrss"} when the hook is done. A
            connection closed without it means the child was killed.
"""

import builtins
import hashlib
import importlib
import json
import marshal
import os
import resource
import signal
import socket
import struct
import sys
import tempfile
import traceback
import types

# What the hooks import, loaded once here instead of once per request. A module that is missing
# is simply imported by the hook itself, as before.
PRELOAD = (
    "deckhouse.hook",
    "deckhouse.utils",
    "dotmap",
    "typing",
    "re",
    "json",
    "subprocess",
)
for _name in PRELOAD:
    try:
        importlib.import_module(_name)
    except ImportError:
        pass

# sitecustomize already ran for this server and appended the lib path of whatever module root it
# found above this file. A hook gets the one for its own path instead; see _prepare_child.
try:
    import sitecustomize
except ImportError:
    sitecustomize = None

_SERVER_SYS_PATH = list(sys.path[1:])
if sitecustomize is not None and getattr(sitecustomize, "mod_root", None):
    _own_lib = os.path.join(sitecustomize.mod_root, "lib", "python", "dist")
    _SERVER_SYS_PATH = [p for p in _SERVER_SYS_PATH if p != _own_lib]

_MAX_REQUEST = 4 << 20

# Compiled hook code, kept between requests. A hook runs as a script, and Python never caches
# bytecode for a script; compiling one costs 0.2-1 ms per request.
_CODE_CACHE_DIR = os.path.join(tempfile.gettempdir(), "python-hook-server-code")


def _read_request(conn):
    data, fds, _, _ = socket.recv_fds(conn, 64 << 10, 2)
    if not data and not fds:
        # Connected and closed without a request, as a readiness check does.
        return None, None
    if len(fds) != 2:
        raise RuntimeError(f"expected stdout and stderr descriptors, got {len(fds)}")
    while len(data) < 4:
        chunk = conn.recv(64 << 10)
        if not chunk:
            raise RuntimeError("connection closed before the request length")
        data += chunk
    (length,) = struct.unpack(">I", data[:4])
    if length > _MAX_REQUEST:
        raise RuntimeError(f"request of {length} bytes is over the limit")
    data = data[4:]
    while len(data) < length:
        chunk = conn.recv(64 << 10)
        if not chunk:
            raise RuntimeError("connection closed in the middle of the request")
        data += chunk
    return json.loads(data[:length]), fds


def _prepare_child(req, fds):
    os.dup2(fds[0], 1)
    os.dup2(fds[1], 2)
    for fd in fds:
        os.close(fd)
    devnull = os.open(os.devnull, os.O_RDONLY)
    os.dup2(devnull, 0)
    os.close(devnull)

    if req.get("dir"):
        os.chdir(req["dir"])

    os.environ.clear()
    os.environ.update(req.get("env") or {})

    path = req["path"]
    sys.argv = [path] + list(req.get("args") or [])
    # `python3 <hook>` puts the hook directory first, with symlinks resolved.
    sys.path[:] = [os.path.dirname(os.path.realpath(path))] + _SERVER_SYS_PATH
    if sitecustomize is not None:
        # Recomputes the module lib path from sys.argv[0], as at interpreter start-up.
        importlib.reload(sitecustomize)
    return path


def _load_code(path):
    """The hook's code object, compiled once per content of the file."""
    with open(path, "rb") as f:
        source = f.read()
    cached = os.path.join(_CODE_CACHE_DIR, hashlib.sha256(path.encode() + b"\0" + source).hexdigest())
    try:
        with open(cached, "rb") as f:
            return marshal.load(f)
    except (OSError, EOFError, ValueError, TypeError):
        pass

    code = compile(source, path, "exec")
    try:
        os.makedirs(_CODE_CACHE_DIR, exist_ok=True)
        partial = f"{cached}.{os.getpid()}"
        with open(partial, "wb") as f:
            marshal.dump(code, f)
        os.replace(partial, cached)
    except OSError:
        pass  # a cache that cannot be written only costs the compile next time
    return code


def _run_as_main(path):
    """What `python3 <path>` does with the script once the interpreter is up."""
    code = _load_code(path)
    main = types.ModuleType("__main__")
    main.__file__ = path
    main.__builtins__ = builtins
    sys.modules["__main__"] = main
    exec(code, main.__dict__)  # pylint: disable=exec-used


def _exit_code(exc):
    code = exc.code
    if code is None:
        return 0
    if isinstance(code, int):
        return code
    print(code, file=sys.stderr)
    return 1


def _serve_connection(conn):
    """Runs in the forked child; never returns."""
    code = 1
    try:
        req, fds = _read_request(conn)
        if req is None:
            os._exit(0)  # pylint: disable=protected-access
        path = _prepare_child(req, fds)
        try:
            _run_as_main(path)
            code = 0
        except SystemExit as exc:
            code = _exit_code(exc)
        except BaseException:  # pylint: disable=broad-except
            traceback.print_exc()
            code = 1
    except BaseException:  # pylint: disable=broad-except
        traceback.print_exc()
        code = 1
    finally:
        for stream in (sys.stdout, sys.stderr):
            try:
                stream.flush()
            except Exception:  # pylint: disable=broad-except
                pass
        usage = resource.getrusage(resource.RUSAGE_SELF)
        result = {
            "exit_code": code,
            "utime": usage.ru_utime,
            "stime": usage.ru_stime,
            "maxrss": usage.ru_maxrss,
        }
        try:
            conn.sendall(json.dumps(result).encode() + b"\n")
        except OSError:
            pass
        os._exit(code)  # pylint: disable=protected-access


def serve(socket_path):
    if os.path.exists(socket_path):
        os.unlink(socket_path)
    server = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
    server.bind(socket_path)
    server.listen(128)

    # Children are reaped by the kernel; nobody here waits for them.
    signal.signal(signal.SIGCHLD, signal.SIG_IGN)

    while True:
        conn, _ = server.accept()
        pid = os.fork()
        if pid == 0:
            server.close()
            # A hook that runs subprocesses must be able to wait for them.
            signal.signal(signal.SIGCHLD, signal.SIG_DFL)
            _serve_connection(conn)
        conn.close()


if __name__ == "__main__":
    serve(sys.argv[1])
