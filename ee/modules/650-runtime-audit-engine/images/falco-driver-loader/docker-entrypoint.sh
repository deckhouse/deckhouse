#!/bin/bash
set -Eeuo pipefail
/usr/bin/falcoctl driver config --type modern_ebpf
/usr/bin/falcoctl driver install --compile=false --download=false modern_ebpf
