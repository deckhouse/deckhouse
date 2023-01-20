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

# Generate and merge russian doc files
# (requires yq and jq)
# 

if [ $# -ne 1 ]; then
  echo "Usage: ./gen-ru-crd.sh <original_crd_file.yaml>"
  exit 1
fi


merge(){
  # modified example from
  # https://stackoverflow.com/questions/53661930/jq-recursively-merge-objects-and-concatenate-arrays
  jq -s '
    def meld(a; b):
      a as $a | b as $b
      | if a == null
        then null
        elif ($a|type) == "object" and ($b|type) == "object"
        then reduce ([$a,$b]|add|keys_unsorted[]) as $k ({}; 
          .[$k] = meld( $a[$k]; $b[$k]) )
        elif ($a|type) == "array" and ($b|type) == "array"
        then [(($a|to_entries) + ($b|to_entries) | group_by(.key)) | map(meld(.[0];.[1]))[] | .value]
        elif $b == null then $a
        else $b
        end;
    
    meld(
    (.[0] | reduce paths(objects | has("description") or has("name")) as $p (.; setpath(["aa"] + $p; {"name": getpath($p) | .name, "description": getpath($p) | .description} )) | .aa);
      .[1]
    )
    | del(..|nulls) | del(..|.additionalPrinterColumns?) | del(.metadata)
  ' \
 <(yq e -o json "$1") \
 <(yq e -o json "$2") \
  | yq e -P 'sortKeys(..)' -
}

file="$1"
rufile="$(dirname "$1")/doc-ru-$(basename "$1")"

touch "$rufile"
merge "$file" "$rufile" > "$rufile.new"
mv "$rufile.new" "$rufile"
