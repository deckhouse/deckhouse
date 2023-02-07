#!/bin/bash

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
