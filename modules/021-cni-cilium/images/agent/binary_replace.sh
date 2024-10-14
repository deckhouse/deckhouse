#!/bin/bash

# Copyright 2023 Flant JSC
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

set -Eeuo pipefail
shopt -s failglob

FILE_TEMPLATE_BINS=""
TEMPLATE_BINS=""
RDIR=""

function Help() {
   # Display Help
   echo "Copy binaries and their libraries to a folder"
   echo "Only one input parameter allowed (-f or -i) !!!"
   echo
   echo "Syntax: scriptTemplate [-h|f|i|o]"
   echo "options:"
   echo "f     Files with paths to binaries; Support mask like /sbin/m*"
   echo "i     Paths to binaries separated by space; Support mask like /sbin/m*; Example: /bin/chmod /bin/mount /sbin/m*"
   echo '      List of binaries should be in double quotes, -i "/bin/chmod /bin/mount" '
   echo "o     Output directory (Default value: '/relocate')"
   echo "h     Print this help"
   echo
   echo
}

while getopts ":h:i:f:o:" option; do
    case $option in
      h) # display Help
         Help
         exit;;
      f)
        FILE_TEMPLATE_BINS=$OPTARG
        ;;
      i)
        TEMPLATE_BINS=$OPTARG
        ;;
      o)
        RDIR=$OPTARG
        ;;
      \?)
        echo "Error: Invalid option"
        exit;;
    esac
done

if [[ -z $RDIR ]];then
  RDIR="/relocate"
fi
mkdir -p "${RDIR}"

function relocate() {
  local binary=$1
  relocate_item ${binary}

  for lib in $(ldd ${binary} 2>/dev/null | awk '{if ($2=="=>") print $3; else print $1}'); do
    # don't try to relocate linux-vdso.so lib due to this lib is virtual
    if [[ "${lib}" =~ "linux-vdso" ]]; then
      continue
    fi
    relocate_item ${lib}
  done
}

function relocate_item() {
  local file=$1
  local new_place="${RDIR}$(dirname ${file})"

  mkdir -p ${new_place}
  cp -a --remove-destination ${file} ${new_place}

  # if symlink, copy original file too
  local orig_file="$(readlink -f ${file})"
  if [[ "${file}" != "${orig_file}" ]]; then
    cp -a --remove-destination ${orig_file} ${new_place}
  fi
}

function get_binary_path () {
  local bin
  BINARY_LIST=()

  for bin in "$@"; do
    if [[ ! -f $bin ]] || [ "${bin}" == "${RDIR}" ]; then
      continue
    fi
    BINARY_LIST+=$(ls -la $bin 2>/dev/null | awk '{print $9}')" "
  done

  if [[ -z $BINARY_LIST ]]; then echo "No binaryes for replace"; exit 1; fi;
}

# if get file with binaryes (-f)
if [[ -n $FILE_TEMPLATE_BINS ]] && [[ -f $FILE_TEMPLATE_BINS ]] && [[ -z $TEMPLATE_BINS ]]; then
  BIN_TEMPLATE=$(cat $FILE_TEMPLATE_BINS)
  get_binary_path ${BIN_TEMPLATE}
# Or get paths to bin via raw input (-i)
elif [[ -n $TEMPLATE_BINS ]] && [[ -z $FILE_TEMPLATE_BINS ]]; then
  get_binary_path ${TEMPLATE_BINS}
else
  Help
  exit
fi

for binary in ${BINARY_LIST[@]}; do
  relocate ${binary}
done
