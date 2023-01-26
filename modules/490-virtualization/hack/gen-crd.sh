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

# merges two yaml files
# arg1: keep fields (true/false)
# arg2: file1
# arg3: file2
merge(){
  # modified example from
  # https://stackoverflow.com/questions/53661930/jq-recursively-merge-objects-and-concatenate-arrays
  jq --arg keepFields "$1" -s '
    def meld(a; b):
      a as $a | b as $b
      | 
        if a == null and $keepFields == "false" then null 
        elif ($a|type) == "object" and ($b|type) == "object"
        then reduce ([$a,$b]|add|keys_unsorted[]) as $k ({}; .[$k] = meld( $a[$k]; $b[$k]) )
        elif ($a|type) == "array" and ($b|type) == "array"
        then [(($a|to_entries) + ($b|to_entries) | group_by(.key)) | map(meld(.[0];.[1]))[] | .value]
        elif b == null then $a
        else $b
        end;
    meld(.[0]; .[1]) | del(..|nulls)
  ' \
  <(yq e -o json "$2") \
  <(yq e -o json "$3") \
  | yq e -P 'sortKeys(..)' -
}

# gets nested map of description and name fields
# removes .additionalPrinterColumns and .metadata fields
filter_descriptions() {
  jq 'reduce paths(objects | has("description") or has("name")) as $p (.; setpath(["aa"] + $p; {"name": getpath($p) | .name, "description": getpath($p) | .description} )) | .aa | del(..|nulls) | del(..|.additionalPrinterColumns?) | del(.metadata)'
}

# gets nested map of .x-examples fields
filter_extra_fields() {
  jq 'reduce paths(objects | has("x-examples")) as $p (.; setpath(["aa"] + $p; {"name": getpath($p) | .name, "x-examples": getpath($p) | ."x-examples"} )) | .aa | del(..|nulls)'
}

# removes .description for .apiVersion and .kind
filter_apiversion_and_kind() {
  jq 'del(.spec.versions[] |
    .schema.openAPIV3Schema.properties.apiVersion.description,
    .schema.openAPIV3Schema.properties.kind.description
  )'
}

file="$1"
enfile="$(dirname $1)/${1##*_}"
rufile="$(dirname "$enfile")/doc-ru-$(basename "$enfile")"

# update original en crd file
touch "$enfile"
extra_fields=$(yq e -o json "$enfile" | filter_extra_fields)
content=$(yq e -o json "$file" | filter_apiversion_and_kind)
content=$(merge true <(echo "$extra_fields") <(echo "$content"))
content=$(merge false <(echo "$content") "$enfile")
echo "$content" > "$enfile"

# update russian translation crd file
touch "$rufile"
extra_fields=$(yq e -o json "$rufile" | filter_extra_fields)
descriptions=$(yq e -o json "$file" | filter_descriptions | filter_apiversion_and_kind)
content=$(merge true <(echo "$extra_fields") "$rufile")
content=$(merge false  <(echo "$descriptions") <(echo "$content"))
content=$(merge false <(echo "$content") "$rufile")
echo "$content" > "$rufile"
