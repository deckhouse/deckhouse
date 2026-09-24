#!/usr/bin/env bash
# Adapted from modules-gitlab-ci/templates/Replay_Fuzz.gitlab-ci.yml at 6c44e20.
# Runs inside a fuzz image; invoked by replay.sh, never during werf build.
set -euo pipefail

report=${FUZZ_REPLAY_REPORT_DIR:-/tmp/fuzz-replay}
mkdir -p "$report"
summary="$report/summary.txt"
printf '=== FUZZ REPLAY SUMMARY ===\nImage: %s\nCommit: %s\nWorkdir: %s\n' \
  "$FUZZ_REPLAY_IMAGE" "$FUZZ_COMMIT" "$PWD" > "$summary"
go version >> "$summary"
printf 'Workflow run and artifacts: %s\n' "$FUZZ_RUN_URL" >> "$summary"
export AWS_ACCESS_KEY_ID="$FUZZ_S3_ACCESS_KEY" AWS_SECRET_ACCESS_KEY="$FUZZ_S3_SECRET_KEY"
export AWS_DEFAULT_REGION=us-east-1 AWS_EC2_METADATA_DISABLED=true
aws configure set default.s3.preferred_transfer_client classic
aws configure set default.s3.max_concurrent_requests 2
module=$(GOWORK=off go list -m)
# Match the fuzzing dispatcher: source path, or the image name without -fuzz.
component=${FUZZ_S3_COMPONENT#./}
component=${component#/}
component=${component%/}
component=${component%-fuzz}
component=${component%/fuzz}
source="s3://anomaloys-materials/${FUZZ_S3_REPOSITORY}/${FUZZ_S3_BRANCH_SLUG}/${component}/"
corpus=$(mktemp -d)
trap 'rm -rf "$corpus"' EXIT
echo "Downloading S3 corpus: $source (Go module: $module)"
while true; do
  if aws --endpoint-url "$FUZZ_S3_ENDPOINT" s3 sync \
    "$source" "$corpus/" --exclude '*' \
    --include '*/corpus/minimized/*' --include '*/corpus/recovery/*' \
    --include "*/corpus/gocache-fuzz/${module}/*" \
    --exclude '*/attempts/*' --exclude '*/report/*' \
    --only-show-errors 2> "$report/s3.log"; then
    break
  else
    status=$?
  fi
  cat "$report/s3.log" >&2
  grep -qE '\(429\)|Too Many Requests' "$report/s3.log" || exit "$status"
  echo 'S3 rate limit (429), retrying corpus download in 30 seconds' >&2
  sleep 30
done
unset AWS_ACCESS_KEY_ID AWS_SECRET_ACCESS_KEY FUZZ_S3_ACCESS_KEY FUZZ_S3_SECRET_KEY
echo "Downloaded corpus files: $(find "$corpus" -type f | wc -l)"
echo 'Downloaded paths (first 30):'
find "$corpus" -type f | sort | sed -n '1,30p'
export GIT_AUTHOR_NAME=fuzz GIT_AUTHOR_EMAIL=fuzz@local
export GIT_COMMITTER_NAME=fuzz GIT_COMMITTER_EMAIL=fuzz@local
# As before, discover targets even if unrelated packages fail to compile.
task fuzz:list > "$report/targets.log" 2>&1 || true
cat "$report/targets.log"
awk '
  /^Fuzz[[:alnum:]_]*$/ { targets[++count] = $0; next }
  /^(ok|FAIL|\?)[[:space:]]+/ {
    for (i = 1; i <= count; i++) print $2 "|" targets[i]
    delete targets; count = 0
  }
  END { if (count != 0) exit 1 }
' "$report/targets.log" > "$report/targets.txt"
[[ -s "$report/targets.txt" ]] || { echo 'No Go fuzz targets found' >&2; exit 1; }
# The dispatcher sorts packages before assigning S3 keys to duplicate names.
LC_ALL=C sort -t '|' -k1,1 -k2,2 -u -o "$report/targets.txt" "$report/targets.txt"
declare -A seen_targets=()
failed=0
number=0
while IFS='|' read -r package target; do
  s3_key=$target
  if [[ -n "${seen_targets[$target]:-}" ]]; then s3_key="$package/$target"; fi
  seen_targets[$target]=1
  destination="$(go list -f '{{.Dir}}' "$package")/testdata/fuzz/$target"
  copied=0
  for seeds in "$corpus/$s3_key/corpus/minimized" "$corpus/$s3_key/corpus/recovery" \
    "$corpus/$s3_key/corpus/gocache-fuzz/$package/$target"; do
    [[ -d "$seeds" ]] || continue
    while IFS= read -r -d '' seed; do
      [[ -e "$destination/${seed##*/}" ]] && continue
      mkdir -p "$destination"
      cp -- "$seed" "$destination/"
      copied=$((copied + 1))
    done < <(find "$seeds" -type f -print0)
  done
  echo "Copied $copied S3 corpus files into $destination"
  number=$((number + 1))
  report_name="$number-$target"
  output="$report/$report_name"
  mkdir -p "$output"
  # JSON keeps diagnostics associated with tests; render normal Go output live.
  if go test -vet=off "$package" -run "^${target}$" -count=1 -json 2> "$output/stderr.log" |
    tee "$output/events.jsonl" | jq --unbuffered -rj '.Output // empty'; then
    rm -rf "$output"
    continue
  else
    replay_status=$?
  fi
  failed=$((failed + 1))
  cat "$output/stderr.log"
  mkdir -p "$output/corpus"
  if [[ -d "$destination" ]]; then cp -R "$destination/." "$output/corpus/"; fi
  jq -rj '.Output // empty' "$output/events.jsonl" > "$output/replay.log"
  cat "$output/stderr.log" >> "$output/replay.log"
  jq -rs --arg target "$target" '
    [.[] | select(.Action == "fail" and .Test != null) | .Test | select(. != $target)] |
    unique | if length == 0 then [$target] else . end | .[]
  ' "$output/events.jsonl" > "$output/failed-tests.txt"
  {
    printf '\nFAILED: %s / %s (exit %s)\nS3: %s%s/corpus/\nLog: fuzz-replay/%s/replay.log\n' \
      "$package" "$target" "$replay_status" "$source" "$s3_key" "$report_name"
    # Omit successful subtests and progress lines, retaining errors and stacks.
    sed -E '/^=== (RUN|PAUSE|CONT|NAME)/d; /^[[:space:]]*--- (PASS|SKIP):/d' "$output/replay.log"
    echo 'Reproduce after downloading and unpacking this job artifacts; run from their root directory:'
    while IFS= read -r test; do
      run=$(printf '%s' "$test" | sed 's/[][(){}.^$*+?|\\]/\\&/g; s|/|$/^|g; s/^/^/; s/$/$/')
      printf '\nTest: %s\n' "$test"
      printf 'docker run --rm --platform linux/amd64 \\\n  -v "$PWD"/%q:%q:ro --workdir %q \\\n' \
        "fuzz-replay/$report_name/corpus" "$destination" "$PWD"
      printf '  -e GIT_AUTHOR_NAME=fuzz -e GIT_AUTHOR_EMAIL=fuzz@local -e GIT_COMMITTER_NAME=fuzz -e GIT_COMMITTER_EMAIL=fuzz@local \\\n'
      printf '  --entrypoint go %q test -vet=off %q -run %q -count=1 -v\n' "$FUZZ_REPLAY_IMAGE" "$package" "$run"
    done < "$output/failed-tests.txt"
  } >> "$summary"
done < "$report/targets.txt"
printf '\nTargets checked: %s; failed: %s\n' "$number" "$failed" >> "$summary"
[[ "$failed" -eq 0 ]]
