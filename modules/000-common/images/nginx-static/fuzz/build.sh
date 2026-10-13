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

# Apply the Linux harness change in this image, keeping it reviewable in this MR.
# A repeated local build accepts an already applied patch; source drift fails.
if git apply --check fuzz-platform/conf-parse-memfd.patch; then
    git apply fuzz-platform/conf-parse-memfd.patch
else
    git apply --reverse --check fuzz-platform/conf-parse-memfd.patch
fi

# The fork's libfuzzer mode passes -fsanitize=fuzzer at the final link;
# afl-clang-fast automatically substitutes the stock libAFLDriver.a.
mkdir -p /out/fuzz/bin /out/fuzz/replay/bin /out/fuzz/coverage/bin

# The fork contains prebuilt Darwin objects and binaries. Remove them, including
# objs/nginx, which stream-run.sh would otherwise select before our Linux build.
rm -rf objs objs-fuzz objs-libfuzzer
CC=afl-clang-fast sh fuzz/build.sh libfuzzer
install -m 0755 fuzz/conf_parse_fuzz /out/fuzz/bin/nginx_conf_parse

# AFL++ 5.03c's driver skips zero-length inputs even in single-file mode.
# Relink the same AFL-instrumented objects with the stock LLVM libFuzzer driver
# for empty-file replay. Keep AFL's coverage hooks by placing its runtime first;
# libFuzzer also defines these hooks, hence --allow-multiple-definition.
coverage_clang="${FUZZ_COV_CC:-clang-22}"
coverage_clangxx="${FUZZ_COV_CXX:-clang++-22}"
afl_runtime="${AFL_PATH:-/usr/lib/afl}/afl-compiler-rt.o"
llvm_driver=$("$coverage_clang" -print-file-name=libclang_rt.fuzzer-x86_64.a)
test -s "$afl_runtime"
test -s "$llvm_driver"
mapfile -d '' afl_objects < <(find objs-libfuzzer/src -name '*.o' ! -path '*/core/nginx.o' -print0)
mapfile -t core_libs < <(grep -oE '(^|[[:space:]])(-l[a-zA-Z0-9_.+-]+|-Wl,[^[:space:]\\]+)' \
    objs-libfuzzer/Makefile | sed 's/^[[:space:]]*//' | sort -u)
"$coverage_clangxx" -fsanitize=address,undefined -Wl,--allow-multiple-definition \
    "$afl_runtime" fuzz/conf_parse_fuzzer.o fuzz/nginx_nomain.o \
    "${afl_objects[@]}" objs-libfuzzer/ngx_modules.o "${core_libs[@]}" \
    -Wl,--whole-archive "$llvm_driver" -Wl,--no-whole-archive \
    -o /out/fuzz/replay/bin/nginx_conf_parse

coverage_cc=$(mktemp /tmp/nginx-coverage-cc-XXXXXX)
trap 'rm -f "$coverage_cc"' EXIT
cat > "$coverage_cc" <<'CC'
#!/bin/sh
exec "${FUZZ_COV_CC:-clang-22}" -fprofile-instr-generate -fcoverage-mapping "$@"
CC
chmod +x "$coverage_cc"

# Make does not track changes to CC, so rebuild every Nginx object for coverage.
rm -rf objs-libfuzzer
CC="$coverage_cc" sh fuzz/build.sh libfuzzer
install -m 0755 fuzz/conf_parse_fuzz /out/fuzz/coverage/bin/nginx_conf_parse
# objs-libfuzzer/nginx remains available to the fork's stream-run.sh.
