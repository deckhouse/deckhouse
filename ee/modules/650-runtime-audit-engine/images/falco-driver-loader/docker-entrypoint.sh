#!/bin/bash
# Copyright 2024 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

set -Eeuo pipefail
/usr/bin/falcoctl driver config --type modern_ebpf
/usr/bin/falcoctl driver install --compile=false --download=false modern_ebpf
