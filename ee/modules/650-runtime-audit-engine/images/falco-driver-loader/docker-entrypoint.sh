#!/bin/bash
set -Eeuo pipefail
/usr/bin/falcoctl driver config --type modern_bpf
/usr/bin/falcoctl driver install --compile=false --download=false modern_bpf
