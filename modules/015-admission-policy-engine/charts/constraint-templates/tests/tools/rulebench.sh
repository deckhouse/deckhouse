#!/usr/bin/env bash
# Convenience wrapper for tools/rulebench.
#
# rulebench has its own go.mod/go.sum (see rulebench/go.mod) so that it can
# depend on github.com/open-policy-agent/opa without touching the deckhouse
# monorepo's root go.mod. That isolation has one downside: `go run` resolves
# a relative package path in the module of the CURRENT working directory, so
# running `go run ../../tools/rulebench .` from test_cases/constraints fails
# with "main module ... does not contain package ..." - the CWD belongs to
# the root deckhouse module, which does not (and should not) contain rulebench.
#
# This script resolves the constraints-root argument to an absolute path
# first, then cd's into rulebench's own module directory before invoking
# `go run .`, so it works from anywhere.
#
# Usage (same arguments as `go run tools/rulebench`):
#   tools/rulebench.sh <constraints_root> [only-name]
#
# Example, from tests/test_cases/constraints:
#   ../../tools/rulebench.sh .
#   ../../tools/rulebench.sh . allow-privileged

set -euo pipefail

if [ $# -lt 1 ]; then
  echo "usage: $0 <constraints_root> [only-name]" >&2
  exit 2
fi

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CONSTRAINTS_ROOT="$(cd "$1" && pwd)"
shift

(cd "${SCRIPT_DIR}/rulebench" && go run . "${CONSTRAINTS_ROOT}" "$@")
