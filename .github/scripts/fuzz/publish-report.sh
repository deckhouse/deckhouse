#!/usr/bin/env bash
# Adapted from modules-gitlab-ci/templates/Replay_Fuzz.gitlab-ci.yml at 6c44e20.
set -euo pipefail

container="fuzz-report-${GITHUB_RUN_ID}-${GITHUB_RUN_ATTEMPT}"
trap 'docker rm -f "$container" >/dev/null 2>&1 || true' EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

image=$(jq -er '.Images | to_entries[0].value.DockerImageName' images_fuzz_tags_werf.json)
export FUZZ_S3_BRANCH_SLUG
FUZZ_S3_BRANCH_SLUG=$(jq -er '.Source.BranchSlug' images_fuzz_tags_werf.json)
docker run --rm --init --name "$container" --platform linux/amd64 \
  -e AWS_CA_BUNDLE=/etc/ssl/certs/ca-certificates.crt \
  -e FUZZ_S3_ENDPOINT -e FUZZ_S3_ACCESS_KEY -e FUZZ_S3_SECRET_KEY \
  -e FUZZ_S3_REPOSITORY -e FUZZ_S3_BRANCH_SLUG \
  --mount "type=bind,src=$PWD/images_fuzz_tags_werf.json,dst=/report.json,readonly" \
  --entrypoint bash "$image" -euc '
    export AWS_ACCESS_KEY_ID="$FUZZ_S3_ACCESS_KEY" AWS_SECRET_ACCESS_KEY="$FUZZ_S3_SECRET_KEY"
    export AWS_DEFAULT_REGION=us-east-1 AWS_EC2_METADATA_DISABLED=true
    aws --endpoint-url "$FUZZ_S3_ENDPOINT" s3 cp /report.json \
      "s3://anomaloys-materials/build-reports/${FUZZ_S3_REPOSITORY}/${FUZZ_S3_BRANCH_SLUG}/images_fuzz_tags_werf.json" \
      --only-show-errors
  '
