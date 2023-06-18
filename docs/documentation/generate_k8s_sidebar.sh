#!/bin/bash

# the first argument is URL prefix for items in the YAML structure
# the second argument is a starting indent for YAML structure

function print_item {
  local DIR=${1:+$1/}
  local INDENT=$2
  local list=$(find $DIR -mindepth 1 -maxdepth 1 -type d -print | sed "s|^./||; s|^$DIR||" | sort)

  for item in $list ; do
    echo $item | grep -Eq '^[./]?images$' && continue
    echo ${DIR}$item | grep -Eq '^[./]?reference/generated' && continue
    echo ${DIR}$item | grep -Eq '^[./]?reference/glossary' && continue

    local title=$(grep -s 'title:' $DIR$item/index.html | head -n 1 | sed 's/^title: //')
    if [[ -z "${title}" ]] ; then
      title=$(echo "$item" | sed "s|^.*/||; s|'|''|g" )
    fi
    printf "%${INDENT}s%s\n" '' "- title: '$title'"

    if [[ -n "$(find $DIR$item/ -mindepth 1 -maxdepth 1 -type d -print | sed "s|^./||; s|^$DIR||" | sort)"  ]]; then
      printf "%${INDENT}s%s\n" '' "  folders:"
      print_item "${DIR}${item}" $(( $INDENT + 2 ))
    else
      printf "%${INDENT}s%s\n" '' "  url: ${URL_PREFIX}${DIR}${item}/"
    fi
  done
}

URL_PREFIX=$1

print_item "" $2
