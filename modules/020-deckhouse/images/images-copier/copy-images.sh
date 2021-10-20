#!/usr/bin/env bash

# Copyright 2021 Flant JSC
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

d8_conf=""
dest_conf=""
d8_images_file=""

function print_help() {
cat << EndOfMessage
Copy deckhouse images in another repository.
Usage:
  -s|--d8-repo-conf-file - deckhouse repo configuration file. See format below
  -d|--dest-repo-conf-file - destination repo configuration file. See format below
  -i|--images-conf-file - deckhouse images config file. See format below
  -h|--help - print this message

Repo configuration file (must be valid JSON):
{
  "username": "abc",
  "password": "xxxxxxxxx",
  "insecure": false,
  "image": "registry.address:5000/repo/image:rock-solid-1.24.17"
}

where:
  "username" - username for login into repo
  "password" - password for login into repo
  "insecure" - use insecure connection
  "image"    - source or destination image WITH TAG to pull/push main image.
               All submodules images will pull/push with tags to this image
                 ex: registry.address:5000/repo/image:9a271f2a916b0b6ee6cecb2426f0b3206ef074578be55d9bc94f6f3fe3ab86aa
               Port in URI is optional

Deckhouse images config file format (must be valid JSON):
{
  "moduleOne": {
    "appOne": "image-tag-1"
  },
  "moduleTwo": {
    "appTwo": "image-tag-2",
    "appThree": "image-tag-3"
  }
}
EndOfMessage
}

# $1 - msg $2 exit code
function die() {
  echo "$1" 1>&2
  echo ""
  print_help
  exit "$2"
}

# $1 - path to file
# $2 - file type
function json_file_exist_or_die() {
  path="$1"
  type="$2"

  if ! [ -f "$path" ] ; then
    die "$type file '$path' not found" 1
  fi

  if ! jq -e '.' >/dev/null 2>&1 "$path"; then
    die "$type '$path' is not valid json file" 1
  fi
}

function parse_arguments() {
  while [[ $# -gt 0 ]]; do
      key="$1"
      case $key in
        -h|--help)
          print_help
          exit 0
          ;;
        -s|--d8-repo-conf-file)
          d8_conf="$2"
          shift # past argument
          shift # past value
          ;;
        -d|--dest-repo-conf-file)
          dest_conf="$2"
          shift # past argument
          shift # past value
          ;;
        -i|--images-conf-file)
          d8_images_file="$2"
          shift # past argument
          shift # past value
          ;;
      esac
    done

    json_file_exist_or_die "$d8_conf" "deckhouse configuration file"

    json_file_exist_or_die "$dest_conf" "destination repo file"

    json_file_exist_or_die "$d8_images_file" "images file"
}

# $1 - conf file
# $2 - destination file
#
# convert from repo config format to
#
# {
#   "quay.io/coreos": {
#     "username": "abc",
#     "password": "xxxxxxxxx",
#     "insecure": true
#   }
# }
# output - image with tag
function convert_auth_config() {
  source="$1"
  dest="$2"

  full_image=$(jq -r '.image' "$source")
  registry=$(echo "$full_image" | cut -d'/' -f 1)
  jq -rc --arg r "${registry}" 'to_entries | map(select(.key != "image")) | from_entries | {($r): .}'  "$source" > "$dest"

  echo -n "$full_image"
}

# 1 - path to log file
function parse_sync_status() {
  expr='Finished, ([0-9]+) sync tasks failed, ([0-9]+) tasks generate failed'
  # extract final message from logs
  if ! msg=$(grep -E -m 1 "$expr" "$1"); then
    die "cannot found status line in log" 1
  fi
  # extract failed counters
  if [[ "$msg" =~ $expr ]]; then
    tasks_failed="${BASH_REMATCH[1]}"
    gen_failed="${BASH_REMATCH[2]}"

    if [ "$tasks_failed" == "0" ] && [ "$gen_failed" == "0" ] ; then
      return 0
    fi
  fi

  return 1
}

function main() {
    parse_arguments "$@"

    # convert d8 repo auth configuration to correct format
    d8_out_file="$(mktemp)"
    d8_main_image=$(convert_auth_config "$d8_conf" "$d8_out_file")


    # convert destination repo auth configuration to correct format
    dest_out_file="$(mktemp)"
    dest_main_image=$(convert_auth_config "$dest_conf" "$dest_out_file")

    # merge auth configurations in one file
    auth_conf="$(mktemp --suffix '.json')"
    jq -s '.[0] * .[1]' "$d8_out_file" "$dest_out_file" > "$auth_conf"

    rm "$d8_out_file" "$dest_out_file"

    # extract repo for d8
    d8_repo="${d8_main_image%:*}"
    # remove trailing suffix for dev clusters
    d8_repo="${d8_repo%"/dev"}"

    # extract destination repo
    dest_repo="${dest_main_image%:*}"

    # convert images file from modules_images.json
    # format to objects with key pair
    # key is deckhouse fullimage; value copy destination
    # {
    #     "d8-registry.co/deckhouse:fullimage-tag-1": "dest-registry.co/deckhouse:fullimage-tag-1"
    #     "d8-registry.co/deckhouse:fullimage-tag-2": "dest-registry.co/deckhouse:fullimage-tag-2"
    #     "d8-registry.co/deckhouse:fullimage-tag-3": "dest-registry.co/deckhouse:fullimage-tag-3"
    # }
    # and add deckhouse main image with release channel
    # total will be
    # {
    #     "d8-registry.co/deckhouse:fullimage-tag-1": "dest-registry.co/deckhouse:fullimage-tag-1"
    #     "d8-registry.co/deckhouse:fullimage-tag-2": "dest-registry.co/deckhouse:fullimage-tag-2"
    #     "d8-registry.co/deckhouse:fullimage-tag-3": "dest-registry.co/deckhouse:fullimage-tag-3"
    #     "d8-registry.co/deckhouse:rock-solid": "dest-registry.co/deckhouse:some-dest-tag"
    # }
    #
    # jq filter expansion
    # extract images tags. tags here are leaves of object and add their into tags array
    extract_tags='[.. | select(type == "string")]'
    # accumulator is empty object
    # on each iteration we create object with one key/pair for one tag
    # and merge it with accumulator
    # shellcheck disable=SC2016
    reduce_tags_onto_object='reduce .[] as $tag ({}; . * {($d8_repo + ":" + $tag):($dest_repo + ":" + $tag)})'
    # add main image source and destination
    # shellcheck disable=SC2016
    add_main_image='. * {($d8_main_img):($dest_main_img)}'

    images_conf=$(mktemp --suffix '.json')
    jq \
      --arg d8_repo "$d8_repo" \
      --arg dest_repo "$dest_repo" \
      --arg d8_main_img "$d8_main_image" \
      --arg dest_main_img "$dest_main_image" \
      "$extract_tags | $reduce_tags_onto_object | $add_main_image" \
      "$d8_images_file" > "$images_conf"

    log_file="$(mktemp)"
    # sync images
    image-syncer --auth="$auth_conf" --images="$images_conf" --retries=3 --proc=6 | tee "$log_file"
    rm "$auth_conf"

    # yes image-syncer always return 0 code and we need to parse message from log
    # non-zero return code indicate error
    parse_sync_status "$log_file"
    exit $?
}

main "$@"
