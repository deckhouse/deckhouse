#!/usr/bin/env bash
#
# Verify dhctl/cmd/dhctl/commands/{edit,config}.go bodies match
# deckhouse-controller/internal/dhctlcli copies (whitespace-insensitive).
# On drift, update both files in the same PR.
#
# Exit: 0 in sync, 1 drift, 2 missing source files.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

DHCTL_EDIT="$REPO_ROOT/dhctl/cmd/dhctl/commands/edit.go"
DHCTL_CONFIG="$REPO_ROOT/dhctl/cmd/dhctl/commands/config.go"
DC_EDIT="$REPO_ROOT/deckhouse-controller/internal/dhctlcli/edit_commands.go"
DC_PARSE="$REPO_ROOT/deckhouse-controller/internal/dhctlcli/parse_config_commands.go"

for f in "$DHCTL_EDIT" "$DHCTL_CONFIG" "$DC_EDIT" "$DC_PARSE"; do
    if [ ! -f "$f" ]; then
        echo "ERROR: required file not found: $f" >&2
        exit 2
    fi
done

# Body of fn: lines between "func fn(" and the closing "}" at column 0.
# Declaration is skipped so signature reformatting doesn't trigger drift.
extract_body() {
    local file="$1"
    local fn="$2"
    awk -v fn="$fn" '
        $0 ~ "^func " fn "\\(" { in_fn = 1; next }
        in_fn {
            if ($0 == "}") exit
            print
        }
    ' "$file"
}

# Strip whitespace so reformatting alone does not trigger drift.
normalize() {
    tr -d '[:space:]'
}

fail=0
check() {
    local fn="$1"
    local src_file="$2"
    local dst_file="$3"

    local src_body dst_body
    src_body=$(extract_body "$src_file" "$fn" | normalize)
    dst_body=$(extract_body "$dst_file" "$fn" | normalize)

    if [ -z "$src_body" ]; then
        echo "FAIL: source function $fn not found in $src_file" >&2
        fail=1
        return
    fi
    if [ -z "$dst_body" ]; then
        echo "FAIL: duplicate function $fn not found in $dst_file" >&2
        fail=1
        return
    fi

    if [ "$src_body" = "$dst_body" ]; then
        echo "OK:   $fn"
    else
        echo "FAIL: $fn drifted between $src_file and $dst_file" >&2
        echo "----- diff (source ↔ duplicate) -----" >&2
        diff -u \
            <(extract_body "$src_file" "$fn") \
            <(extract_body "$dst_file" "$fn") || true
        echo "------------------------------------" >&2
        fail=1
    fi
}

# edit.go ↔ internal/dhctlcli/edit_commands.go
check "DefineEditCommands"                            "$DHCTL_EDIT"   "$DC_EDIT"
check "connectionFlags"                               "$DHCTL_EDIT"   "$DC_EDIT"
check "baseEditConfigCMD"                             "$DHCTL_EDIT"   "$DC_EDIT"
check "DefineEditClusterConfigurationCommand"         "$DHCTL_EDIT"   "$DC_EDIT"
check "DefineEditProviderClusterConfigurationCommand" "$DHCTL_EDIT"   "$DC_EDIT"
check "DefineEditStaticClusterConfigurationCommand"   "$DHCTL_EDIT"   "$DC_EDIT"

# config.go ↔ internal/dhctlcli/parse_config_commands.go
check "DefineCommandParseClusterConfiguration"        "$DHCTL_CONFIG" "$DC_PARSE"
check "DefineCommandParseCloudDiscoveryData"          "$DHCTL_CONFIG" "$DC_PARSE"

if [ $fail -eq 0 ]; then
    echo ""
    echo "All dhctl ↔ deckhouse-controller command-builder copies are in sync."
else
    echo "" >&2
    echo "Drift detected. Update both files in the same PR (the dhctl source and" >&2
    echo "the deckhouse-controller duplicate)." >&2
fi
exit $fail
