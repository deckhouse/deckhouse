#!/usr/bin/env bash

set -euo pipefail

HOST=$1
scp -r ./vacuum "$HOST:\$HOME"
ssh -L8091:localhost:8091 "$HOST"
