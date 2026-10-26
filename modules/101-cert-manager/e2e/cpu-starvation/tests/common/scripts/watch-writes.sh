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

# Appends every change of the watched objects to OUT, one line per watch event
# ("MODIFIED <name> ..."), and the errors of kubectl to OUT with the .log suffix replaced by .err.
# kubectl writes each event as soon as it arrives, so the line count of OUT at a moment
# is the number of events seen until then; e2e.sh uses it to split the log into phases.
# The watch is restarted when the API server closes it; --watch-only keeps a restart from
# replaying the existing objects as ADDED events.
#
# e2e.sh bg-start runs the script with KUBECONFIG pointing to its own copy of the kubeconfig and with
# BG_PID_FILE and BG_DEADLINE set. The script stops on SIGTERM, when BG_PID_FILE disappears or when
# the epoch time in the BG_DEADLINE file passes, so an interrupted run does not leave it running.
#
# Usage: watch-writes.sh <OUT> <RESOURCE> [kubectl get arguments...]

set -u

: "${BG_PID_FILE:?run the script through e2e.sh bg-start}"
: "${BG_DEADLINE:?run the script through e2e.sh bg-start}"

OUT=$1
shift
ERR=${OUT%.log}.err

running() {
  [ -f "$BG_PID_FILE" ] && [ "$(date +%s)" -lt "$(cat "$BG_DEADLINE" 2>/dev/null || echo 0)" ]
}

CHILD=
stop() {
  if [ -n "$CHILD" ]; then kill "$CHILD" 2>/dev/null; fi
  exit 0
}
trap stop TERM INT HUP

while running; do
  kubectl get "$@" --watch-only --output-watch-events --no-headers >>"$OUT" 2>>"$ERR" &
  CHILD=$!
  # Poll instead of a blocking wait, so the loop notices the pid file or the deadline.
  while running && kill -0 "$CHILD" 2>/dev/null; do
    sleep 2
  done
  if kill -0 "$CHILD" 2>/dev/null; then
    kill "$CHILD" 2>/dev/null
  fi
  wait "$CHILD"
  RC=$?
  CHILD=
  # kubectl ends with 0 when the API server closes the watch; anything else is a broken watch.
  if [ "$RC" -ne 0 ] && running; then
    printf '%s kubectl watch exited with status %s\n' "$(date -u +%H:%M:%S)" "$RC" >>"$ERR"
  fi
  sleep 1
done
