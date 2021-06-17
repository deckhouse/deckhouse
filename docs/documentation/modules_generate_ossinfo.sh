#!/bin/bash

#
# Copy files with information about the licenses used in modules to _data/ossinfo folder (jekyll will construct an array with this data)

mkdir -p _data/ossinfo/

for path in $(find $MODULES_DIR -regex '^.*/[0-9]*-[^/]*/oss.yaml$' -print); do
  module_short_name=$(echo $path | sed -E 's#.+/(.+/[^/]+)$#\1#' | cut -d\/ -f-1 | cut -d- -f2-)
  module_full_name=$(echo $path | sed -E 's#.+/(.+/[^/]+)$#\1#' | cut -d\/ -f-1)
  cp -f $path _data/ossinfo/${module_short_name}.yaml
  cat $path >> _data/ossinfo-cumulative.yaml
  echo "copied $path to _data/ossinfo/${module_short_name}.yaml"
done
