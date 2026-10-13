#!/usr/bin/env bash
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

set -euo pipefail
cd "$(dirname "$0")/.."

if [[ "${FUZZ_TARGET:-}" != nginx_conf_parse ]]; then
    printf 'AFL replay setup error: unsupported target %q\n' "${FUZZ_TARGET:-}" >&2
    exit 2
fi
subject="/out/fuzz/bin/$FUZZ_TARGET"
if [[ ! -x "$subject" ]]; then
    printf 'AFL replay setup error: target is not executable: %q\n' "$subject" >&2
    exit 2
fi
input_dir="${FUZZ_INPUT_DIR:-${FUZZ_CORPUS_DIR:-fuzz/corpus}}"
if [[ ! -d "$input_dir" ]]; then
    printf 'AFL replay setup error: corpus directory is missing: %q\n' "$input_dir" >&2
    exit 2
fi
artifact_dir="${FUZZ_ARTIFACT_DIR:-}"
if [[ -z "$artifact_dir" ]]; then
    artifact_dir=$(mktemp -d /tmp/nginx-fuzz-replay-XXXXXX)
fi
map_dir="$artifact_dir/maps"
mkdir -p "$map_dir"
seed_list=$(mktemp /tmp/nginx-replay-seeds-XXXXXX)
trap 'rm -f "$seed_list"' EXIT
find "$input_dir" -type f -print0 > "$seed_list"

# -Z enables quiet mode itself. Preserve child stderr and associate output
# with input filenames without reading input contents into our diagnostic log.
# Nginx module globals must not survive into the next input.
export AFL_FUZZER_LOOPCOUNT=1
export AFL_DEBUG_CHILD=1 AFL_PRINT_FILENAMES=1 AFL_DEBUG=1
unset AFL_CMIN_ALLOW_ANY AFL_CMIN_CRASHES_ONLY
timeout_ms="${FUZZ_REPLAY_TIMEOUT_MS:-5000}"
showmap=(afl-showmap -q -Z -i "$input_dir" -o "$map_dir"
    -m none -t "$timeout_ms" -- "$subject" @@)
checked=0
empty_inputs=0
while IFS= read -r -d '' seed; do
    checked=$((checked + 1))
    if [[ ! -s "$seed" ]]; then empty_inputs=$((empty_inputs + 1)); fi
done < "$seed_list"
empty_subject="/out/fuzz/replay/bin/$FUZZ_TARGET"
if [[ "$empty_inputs" -gt 0 && ! -x "$empty_subject" ]]; then
    printf 'AFL replay setup error: empty-input target is not executable: %q\n' "$empty_subject" >&2
    exit 2
fi
printf 'AFL replay target=%s inputs=%s empty_inputs=%s timeout_ms=%s memory_limit=none\n' \
    "$FUZZ_TARGET" "$checked" "$empty_inputs" "$timeout_ms"
printf 'Command: AFL_CRASH_EXITCODE=2'
printf ' %q' "${showmap[@]}"
printf '\n'

started=$SECONDS
showmap_status=0
log_status=0
if [[ "$checked" -gt "$empty_inputs" ]]; then
    if AFL_CRASH_EXITCODE=2 "${showmap[@]}" 2>&1 | tee "$artifact_dir/showmap.log"; then
        :
    else
        pipeline_status=("${PIPESTATUS[@]}")
        showmap_status=${pipeline_status[0]}
        log_status=${pipeline_status[1]}
    fi
else
    printf 'Batch showmap skipped: no non-empty inputs.\n' | tee "$artifact_dir/showmap.log"
fi
case "$showmap_status" in
    0) state='last-input-completed' ;;
    1) state='timeout-or-tool/startup-error; see showmap diagnostics' ;;
    2) state='last-input-crash (signal or AFL_CRASH_EXITCODE)' ;;
    3) state='crash/timeout flags; see showmap diagnostics' ;;
    *) state='tool-or-process-error; see showmap diagnostics' ;;
esac
if [[ "$checked" -eq "$empty_inputs" ]]; then state='batch-skipped-no-nonempty-inputs'; fi
printf 'afl-showmap exit=%s state=%s log_exit=%s elapsed_seconds=%s\n' \
    "$showmap_status" "$state" "$log_status" "$((SECONDS - started))"
printf 'Batch exit status is not an aggregate result; validating every expected map.\n'
printf 'showmap_exit\t%s\nlog_exit\t%s\nstate\t%s\n' \
    "$showmap_status" "$log_status" "$state" > "$artifact_dir/showmap-status.tsv"

# Both directory mode and the stock AFL++ driver skip zero-byte files.
# Replay each one with the stock LLVM driver linked to the same AFL objects.
# Disable libFuzzer's signal handlers so afl-showmap observes crashes directly.
# Avoid its extra callback for leak checks; LSan remains enabled at process exit.
# Keep its map in the same namespace for the shared replay's validation.
empty_failed=0
while IFS= read -r -d '' seed; do
    if [[ -s "$seed" ]]; then continue; fi
    empty_showmap=(afl-showmap -q -o "$map_dir/${seed##*/}"
        -m none -t "$timeout_ms" -- "$empty_subject" -runs=1 -detect_leaks=0
        -handle_abrt=0 -handle_segv=0 -handle_bus=0 -handle_ill=0 -handle_fpe=0 "$seed")
    printf 'Replay empty input=%q; command:' "${seed##*/}"
    printf ' %q' "${empty_showmap[@]}"
    printf '\n'
    empty_status=0
    empty_log_status=0
    if AFL_CRASH_EXITCODE=2 "${empty_showmap[@]}" 2>&1 | tee "$artifact_dir/empty-${seed##*/}.log"; then
        :
    else
        pipeline_status=("${PIPESTATUS[@]}")
        empty_status=${pipeline_status[0]}
        empty_log_status=${pipeline_status[1]}
        empty_failed=$((empty_failed + 1))
    fi
    printf 'Empty input=%q showmap_exit=%s log_exit=%s\n' \
        "${seed##*/}" "$empty_status" "$empty_log_status"
    printf 'empty_input\t%s\tshowmap_exit\t%s\tlog_exit\t%s\n' \
        "${seed##*/}" "$empty_status" "$empty_log_status" >> "$artifact_dir/showmap-status.tsv"
done < "$seed_list"

# Batch exit status can describe only the final input. Check every map instead.
# Keep all maps available for the shared replay's independent validation.
failed=0
maps_present=0
maps_empty=0
maps_missing=0
while IFS= read -r -d '' seed; do
    map="$map_dir/${seed##*/}"
    if [[ ! -f "$map" ]]; then
        map_state=missing
        maps_missing=$((maps_missing + 1))
    elif [[ ! -s "$map" ]]; then
        map_state=empty
        maps_empty=$((maps_empty + 1))
    else
        map_state=present
        maps_present=$((maps_present + 1))
    fi
    if [[ "$map_state" != present ]]; then
        reason='cause unknown from map alone; see child/showmap diagnostics'
        printf 'Replay failed: input=%s map=%s reason=%s\n' "${seed##*/}" "$map_state" "$reason" >&2
        failed=$((failed + 1))
    fi
done < "$seed_list"
printf 'Seeds checked: %s; failed map checks: %s; failed empty runs: %s; maps present=%s empty=%s missing=%s; directory=%s\n' \
    "$checked" "$failed" "$empty_failed" "$maps_present" "$maps_empty" "$maps_missing" "$map_dir"
[[ "$showmap_status" -eq 0 && "$log_status" -eq 0 && "$empty_failed" -eq 0 && "$checked" -gt 0 && "$failed" -eq 0 ]]
