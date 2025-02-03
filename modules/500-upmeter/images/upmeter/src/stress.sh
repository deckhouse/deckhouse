#!/bin/bash

# Incopmplete
# go build -ldflags="-s -w" -o upmeter ./cmd/...
# UPMETER_DB_PATH=./stress.db UPMETER_DB_MIGRATIONS_PATH=./pkg/db/migrations/server UPMETER_LISTEN_HOST=127.0.0.1 UPMETER_LISTEN_PORT=8091 ./upmeter start --origins=1

if ! ok=$(which hey); then
  echo "Install hey from https://github.com/rakyll/hey#installation"
  exit 1
fi

DURATION=5s
COUNT=20

ts=$(echo $(date +%s)' / 30 * 30' | bc)
from=$(echo "$ts - 600" | bc)
to=$(echo "$ts + 600" | bc)

episodes=$(
  cat <<EOF
{
  "origin": "stress",
  "episodes": [
    {
      "probeRef": { "group": "control-plane", "probe": "access" },
      "ts": "${ts}",
      "fail": 25000000000,
      "success": 5000000000,
      "unknown": 0,
      "nodata": 0
    },
    {
      "probeRef": { "group": "control-plane", "probe": "namespace" },
      "ts": "${ts}",
      "fail": 15000000000,
      "success": 15000000000,
      "unknown": 0,
      "nodata": 0
    },
    {
      "probeRef": { "group": "synthetic", "probe": "dns" },
      "ts": "${ts}",
      "fail": 20000000000,
      "success": 10000000000,
      "unknown": 0,
      "nodata": 0
    }
  ]
}

EOF
)

# Emulate agents
cat <<EOF >stress-downtime.log
+============================================
| Emulate agents: Send downtime episodes
+============================================
EOF
hey -z $DURATION -c $COUNT -D <(echo "${episodes}") -m POST -T application/json http://127.0.0.1:8091/downtime >>stress-downtime.log &

# Emulate webui
cat <<EOF >stress-probe-list.log
+============================================
| Emulate webui: Probe List
+============================================
EOF
hey -z $DURATION -c $COUNT -T application/json http://127.0.0.1:8091/api/probe >>stress-probe-list.log &

cat <<EOF >stress-range.log
+============================================
| Emulate webui: Status Range
+============================================
EOF
hey -z $DURATION -c $COUNT -T application/json http://127.0.0.1:8091/api/status/range\?from\=${from}\&to\=${to}\&step\=300\&group\=control-plane\&probe\=__total__ >>stress-range.log &

# Emulate public status webui
cat <<EOF >stress-public-status.log
+============================================
| Emulate public webui: Instant Status
+============================================
EOF
hey -z $DURATION -c $COUNT -T application/json http://127.0.0.1:8091/public/api/status >>stress-public-status.log &

# Emulate stats
cat <<EOF >stress-stats.log
+============================================
| Emulate stats: Debug info from /stats
+============================================
EOF
hey -z $DURATION -c $COUNT -T application/json http://127.0.0.1:8091/stats >>stress-stats.log &

watch -n 0.5 bash -c 'ps aux | grep hey'

cat stress-downtime.log stress-probe-list.log stress-range.log stress-public-status.log stress-stats.log
