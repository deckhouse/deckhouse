#!/usr/bin/env bash

RED="\e[31m"
GREEN="\e[32m"
ENDCOLOR="\e[0m"

#set -o errexit
#set -o nounset
#set -o pipefail

SCRIPT_DIR=$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )
FOCUS=$1

chmod +x $SCRIPT_DIR/steps/*.sh

$SCRIPT_DIR/before_all.sh

# for each file in steps directory
files=("$SCRIPT_DIR"/steps/*.sh)
sorted_files=$(for file in "${files[@]}"; do
    filename=$(basename "$file")            # Extract the filename
    number="${filename%%_*}"               # Extract the first digit before the first underscore
    echo "$number $file"
done | sort -n | awk '{print $2}');


for step in ${sorted_files[@]}; do
  if [ -x "$step" ]; then
    if [ -n "$FOCUS" ]; then
      # if step not starts with FOCUS
      filename=$(basename -- "$step")
      if [[ $filename != "$FOCUS"* ]]; then
        continue
      fi
      echo "RUN ONLY $FOCUS step"
    fi
    $SCRIPT_DIR/before_each.sh
    $step
    if [ $? -ne 0 ]; then
      echo -e "${RED}FAIL:${ENDCOLOR} $step"
      break
    else
      echo -e "${GREEN}PASS:${ENDCOLOR} $step"
    fi
    $SCRIPT_DIR/after_each.sh
  fi
done


$SCRIPT_DIR/after_all.sh
