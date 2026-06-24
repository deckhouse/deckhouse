#!/usr/bin/env bash
set -euo pipefail

: "${DIFF_PATH:?DIFF_PATH is required}"

echo "Detect changed fuzz packages"

grep '^+++ b/.*_fuzz_test\.' "${DIFF_PATH}" \
  | sed 's#^+++ b/##' \
  | xargs -r dirname \
  | sort -u \
  > fuzz-packages.txt

if [ ! -s fuzz-packages.txt ]; then
  echo "No changed fuzz tests"
  exit 0
fi

cat fuzz-packages.txt

while read -r dir; do
  echo "Run fuzz smoke in ${dir}"

  case "${dir}" in
    *.go|*)
      if [ -f "${dir}/go.mod" ]; then
        workdir="${dir}"
        package="./..."
      else
        workdir="$(dirname "$(find "${dir}" -name go.mod -print -quit)")"
        package="./${dir#"${workdir}/"}"
      fi

      mkdir -p "${dir}/testdata/fuzz"

      # aws s3 sync "s3://${FUZZ_S3_BUCKET}/main/${dir}/testdata/fuzz" "${dir}/testdata/fuzz"

      (
        cd "${workdir}"
        go test "${package}" -count=1 -run='^Fuzz'
      )
      ;;
  esac
done < fuzz-packages.txt
