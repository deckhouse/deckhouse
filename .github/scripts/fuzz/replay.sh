#!/usr/bin/env bash
# Adapted from modules-gitlab-ci/templates/Replay_Fuzz.gitlab-ci.yml at 6c44e20.
set -euo pipefail

container="fuzz-replay-${GITHUB_RUN_ID}-${GITHUB_RUN_ATTEMPT}-${FUZZ_ARTIFACT}"
trap 'docker rm -f "$container" >/dev/null 2>&1 || true' EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

image=$(jq -er --arg image "$FUZZ_IMAGE" '.Images[$image].DockerImageName' images_fuzz_tags_werf.json)
component=$(jq -r --arg image "$FUZZ_IMAGE" '
  .Images[$image] | (.GitHubPath // .GithubPath // .GitPath // .Context // .Path // "") |
  if . == "" then $image else . end' images_fuzz_tags_werf.json)
export FUZZ_COMMIT
FUZZ_COMMIT=$(jq -er '.Source.Commit' images_fuzz_tags_werf.json)
echo "Replay $FUZZ_IMAGE"

# Retain the stopped container until its diagnostics have been collected.
status=0
docker run --init -i --name "$container" --platform linux/amd64 \
  -e AWS_CA_BUNDLE=/etc/ssl/certs/ca-certificates.crt \
  -e FUZZ_S3_ENDPOINT -e FUZZ_S3_ACCESS_KEY -e FUZZ_S3_SECRET_KEY \
  -e FUZZ_S3_REPOSITORY -e FUZZ_S3_BRANCH_SLUG \
  -e FUZZ_S3_COMPONENT="$component" -e FUZZ_REPLAY_IMAGE="$image" \
  -e FUZZ_COMMIT -e FUZZ_RUN_URL \
  --entrypoint bash "$image" -euo pipefail -s \
  < "$(dirname "$0")/replay-corpus.sh" || status=$?

mkdir -p fuzz-replay
if docker cp "$container:/tmp/fuzz-replay/." fuzz-replay/; then
  cat fuzz-replay/summary.txt
  if [[ -n "${GITHUB_STEP_SUMMARY:-}" ]]; then
    {
      printf '### Fuzz replay: %s\n\n```text\n' "$FUZZ_IMAGE"
      cat fuzz-replay/summary.txt
      printf '\n```\n'
    } >> "$GITHUB_STEP_SUMMARY"
  fi
else
  echo 'Could not collect replay artifacts' >&2
  status=1
fi
exit "$status"
