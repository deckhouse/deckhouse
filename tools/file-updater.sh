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

set -e

folder="$1"
fromVersion="$2"
toVersion="$3"

commentRegex='(#[ \t]+https?|ftp|file)://[-[:alnum:]\+&@#/%?=~_|!:,.;]+'

function usage() {
  echo "This script allows you to update files with the download URL in the first file comment."
  echo "run script with following parameters:"
  echo "file-updater.sh <folder> <source version> <target version>"
  exit 1
}

if [ -z "$folder" ]; then
  echo "<folder> parameter can't be empty"
  usage
fi
if [ -z "$fromVersion" ]; then
  echo "<source version> parameter can't be empty"
  usage
fi
if [ -z "$toVersion" ]; then
  echo "<target version> parameter can't be empty"
  usage
fi


for file in "$folder"/*.yaml; do
  echo Updating "$file"
  updateInfo=$(head -n 1 "$file")
  echo "$updateInfo"
  if [[ $updateInfo =~ $commentRegex ]]; then
    downloadUrl=$(echo "${updateInfo}" | sed -e 's/#//g' -e 's/[[:blank:]]//g' -e "s/$fromVersion/$toVersion/g")
    echo Downloading from the "$downloadUrl"
    echo "# $downloadUrl" >"$file" && curl "$downloadUrl" >>"$file"
  else
    echo "No update info found in file. Skipping..."
  fi
done
