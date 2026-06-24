#!/usr/bin/env bash
set -euo pipefail

: "${DIFF_PATH:?DIFF_PATH is required}"
: "${FUZZ_S3_BUCKET:?FUZZ_S3_BUCKET is required}"

S3_BRANCH="${FUZZ_S3_BRANCH:-main}"

echo "Detect changed files"

grep '^+++ b/' "${DIFF_PATH}" \
  | sed 's#^+++ b/##' \
  | grep -v '^/dev/null$' \
  | sort -u \
  > changed-files.txt

if [ ! -s changed-files.txt ]; then
  echo "No changed files"
  exit 0
fi

echo "Detect affected fuzz tests"

: > fuzz-tests.txt

while read -r changed_file; do
  dir="$(dirname "${changed_file}")"

  while [ "${dir}" != "." ] && [ "${dir}" != "/" ]; do
    find "${dir}" -maxdepth 1 -type f \
      \( -name '*_fuzz_test.go' \
      -o -name '*_fuzz_test.py' \
      -o -name '*_fuzz_test.c' \
      -o -name '*_fuzz_test.cc' \
      -o -name '*_fuzz_test.cpp' \
      -o -name '*_fuzz_test.rb' \) \
      >> fuzz-tests.txt

    dir="$(dirname "${dir}")"
  done
done < changed-files.txt

sort -u fuzz-tests.txt -o fuzz-tests.txt

if [ ! -s fuzz-tests.txt ]; then
  echo "No affected fuzz tests"
  exit 0
fi

cat fuzz-tests.txt

sync_corpus_from_s3() {
  local fuzz_file="$1"
  local dir="$2"
  local corpus_dir="$3"

  mkdir -p "${corpus_dir}"

  echo "Sync corpus from s3://${FUZZ_S3_BUCKET}/${S3_BRANCH}/${dir}/corpus/"

  aws s3 sync \
    "s3://${FUZZ_S3_BUCKET}/${S3_BRANCH}/${dir}/corpus/" \
    "${corpus_dir}/" \
    --only-show-errors || true

  if [ ! -n "$(find "${corpus_dir}" -type f -print -quit)" ]; then
    echo "No corpus found for ${fuzz_file}"
    exit 1
  fi
}

run_python_corpus() {
  local fuzz_file="$1"
  local corpus_dir="$2"

  while IFS= read -r input; do
    echo "Run ${fuzz_file} on ${input}"
    python3 "${fuzz_file}" "${input}"
  done < <(find "${corpus_dir}" -type f | sort)
}

run_cpp_corpus() {
  local fuzz_file="$1"
  local corpus_dir="$2"

  local bin="${fuzz_file%.*}"

  if [ ! -x "${bin}" ]; then
    echo "Compiled fuzz binary not found or not executable: ${bin}"
    exit 1
  fi

  "${bin}" "${corpus_dir}" -runs=0
}

while read -r fuzz_file; do
  dir="$(dirname "${fuzz_file}")"

  echo "Run fuzz smoke for ${fuzz_file}"

  corpus_dir="${dir}/corpus"
  sync_corpus_from_s3 "${fuzz_file}" "${dir}" "${corpus_dir}"

  case "${fuzz_file}" in
    *_fuzz_test.go)
      if [ -f "${dir}/go.mod" ]; then
        workdir="${dir}"
        package="./..."
      else
        workdir="$(dirname "$(find "${dir}" -name go.mod -print -quit)")"
        if [ -z "${workdir}" ] || [ "${workdir}" = "." ]; then
          echo "go.mod not found for ${fuzz_file}"
          exit 1
        fi
        package="./${dir#"${workdir}/"}"
      fi

      mkdir -p "${dir}/testdata/fuzz"
      cp -a "${corpus_dir}/." "${dir}/testdata/fuzz/"

      (
        cd "${workdir}"
        go test "${package}" -count=1 -run='^Fuzz'
      )
      ;;

    *_fuzz_test.py)
      run_python_corpus "${fuzz_file}" "${corpus_dir}"
      ;;

    *_fuzz_test.c|*_fuzz_test.cc|*_fuzz_test.cpp)
      run_cpp_corpus "${fuzz_file}" "${corpus_dir}"
      ;;

    *_fuzz_test.rb)
      while IFS= read -r input; do
        echo "Run ${fuzz_file} on ${input}"
        ruby "${fuzz_file}" "${input}"
      done < <(find "${corpus_dir}" -type f | sort)
      ;;

    *)
      echo "Unsupported fuzz test type: ${fuzz_file}"
      exit 1
      ;;
  esac
done < fuzz-tests.txt
