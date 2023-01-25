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

# Generates and merges CRD files after controller-gen
# (requires yq and jq)

if [ $# -ne 1 ]; then
  echo "Usage: ./gen-crd.sh <original_crd_file.yaml>"
  exit 1
fi

merge(){
  # modified example from
  # https://stackoverflow.com/questions/53661930/jq-recursively-merge-objects-and-concatenate-arrays
  jq -s '
    def meld(a; b):
      a as $a | b as $b
      | 
        if ($a|type) == "object" and ($b|type) == "object"
        then reduce ([$a,$b]|add|keys_unsorted[]) as $k ({}; .[$k] = meld( $a[$k]; $b[$k]) )
        elif ($a|type) == "array" and ($b|type) == "array"
        then [(($a|to_entries) + ($b|to_entries) | group_by(.key)) | map(meld(.[0];.[1]))[] | .value]
        elif b == null then $a
        else $b
        end;
    meld(.[0]; .[1])
  ' \
  <(yq e -o json "$1") \
  <(yq e -o json "$2") \
  | yq e -P 'sortKeys(..)' -
}

filter_descriptions() {
jq 'reduce paths(objects | has("description") or has("name")) as $p (.; setpath(["aa"] + $p; {"name": getpath($p) | .name, "description": getpath($p) | .description} )) | .aa | del(..|nulls) | del(..|.additionalPrinterColumns?) | del(.metadata)'
}

filter_apiversion_and_kind() {
jq 'del(.spec.versions[] |
  .schema.openAPIV3Schema.properties.apiVersion.description,
  .schema.openAPIV3Schema.properties.kind.description
)'
}

file="$1"
enfile="$(dirname $1)/${1##*_}"
rufile="$(dirname "$enfile")/doc-ru-$(basename "$enfile")"

descriptions=$(yq e -o json "$file" | filter_descriptions | filter_apiversion_and_kind)

# update original en crd file
touch "$enfile"
merge <(yq e -o json "$file" | filter_apiversion_and_kind) "$enfile" > "$enfile.new"
mv "$enfile.new" "$enfile"

# update russian translation crd file
touch "$rufile"
merge <(echo "$descriptions") "$rufile" > "$rufile.new"
mv "$rufile.new" "$rufile"
