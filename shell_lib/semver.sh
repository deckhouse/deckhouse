#!/bin/bash

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

# IMPORTANT!!! Because we widely use two-component versioning, instead of
# full semver, we support two-component versions as the first class citizens.

function semver::normalize() {
  if [ -z "$(echo $1 | cut -d. -f3-)" ] ; then
    echo "${1}.0"
  else
    echo "$1"
  fi
}

function semver::majmin() {
  echo "$(echo $1 | cut -d. -f1,2)"
}

function semver::eq() {
  ret="$(semver compare "$(semver::normalize "$1")" "$(semver::normalize "$2")")"
  if [[ "$ret" == "0" ]]; then
    return 0
  fi
  return 1
}

function semver::gt() {
  ret="$(semver compare "$(semver::normalize "$1")" "$(semver::normalize "$2")")"
  if [[ "$ret" == "1" ]]; then
    return 0
  fi
  return 1
}

function semver::lt() {
  ret="$(semver compare "$(semver::normalize "$1")" "$(semver::normalize "$2")")"
  if [[ "$ret" == "-1" ]]; then
    return 0
  fi
  return 1
}

function semver::ge() {
  ret="$(semver compare "$(semver::normalize "$1")" "$(semver::normalize "$2")")"
  if [[ "$ret" == "1" || "$ret" == "0" ]]; then
    return 0
  fi
  return 1
}

function semver::le() {
  ret="$(semver compare "$(semver::normalize "$1")" "$(semver::normalize "$2")")"
  if [[ "$ret" == "-1" || "$ret" == "0" ]]; then
    return 0
  fi
  return 1
}

function semver::bump_minor() {
  minor_part=$(echo $1 | cut -d. -f2)
  new_minor_part=$(( minor_part + 1 ))

  r="$(echo $1 | cut -d. -f1).$new_minor_part"
  if [ -n "$(echo $1 | cut -d. -f3-)" ] ; then
    r="$r$(echo $1 | cut -d. -f3-)"
  fi
  
  echo "$r"
}

function semver::unbump_minor() {
  minor_part=$(echo $1 | cut -d. -f2)
  new_minor_part=$(( minor_part - 1 ))

  r="$(echo $1 | cut -d. -f1).$new_minor_part"
  if [ -n "$(echo $1 | cut -d. -f3-)" ] ; then
    r="$r$(echo $1 | cut -d. -f3-)"
  fi
  
  echo "$r"
}

function semver::get_max() {
  local a=("$@")
  local j
  local t
  local i=${#a[@]}
  while (( 0 < i )); do
    j=0
    while (( j+1 < i )); do
    arg="${*: -1}"
        if semver::gt "${a[j+1]}" "${a[j]}"; then
          t=${a[j+1]}
          a[j+1]=${a[j]}
          a[j]=$t
        fi
      t=$(( ++j ))
    done
    t=$(( --i ))
  done
  echo "${a[0]}"
}

function semver::get_min() {
  local a=("$@")
  local j
  local t
  local i=${#a[@]}
  while (( 0 < i )); do
    j=0
    while (( j+1 < i )); do
    arg="${*: -1}"
        if semver::lt "${a[j+1]}" "${a[j]}"; then
          t=${a[j+1]}
          a[j+1]=${a[j]}
          a[j]=$t
        fi
      t=$(( ++j ))
    done
    t=$(( --i ))
  done
  echo "${a[0]}"
}

function semver::assert() {
  if ! semver get major "$(semver::normalize "$1")" >/dev/null; then
    >&2 echo "ERROR: not SemVer in \"$2\": $1"
    return 1
  fi
}
