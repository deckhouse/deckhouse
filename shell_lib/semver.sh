#!/bin/bash

function semver::eq() {
  ret="$(semver compare "$1" "$2")"
  if [[ "$ret" == "0" ]]; then
    return 0
  fi
  return 1
}

function semver::gt() {
  ret="$(semver compare "$1" "$2")"
  if [[ "$ret" == "1" ]]; then
    return 0
  fi
  return 1
}

function semver::lt() {
  ret="$(semver compare "$1" "$2")"
  if [[ "$ret" == "-1" ]]; then
    return 0
  fi
  return 1
}

function semver::ge() {
  ret="$(semver compare "$1" "$2")"
  if [[ "$ret" == "1" || "$ret" == "0" ]]; then
    return 0
  fi
  return 1
}

function semver::le() {
  ret="$(semver compare "$1" "$2")"
  if [[ "$ret" == "-1" || "$ret" == "0" ]]; then
    return 0
  fi
  return 1
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
  if ! semver get major "$1" >/dev/null; then
    >&2 echo "ERROR: not SemVer in \"$2\": $1"
    return 1
  fi
}
