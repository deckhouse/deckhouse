#!/bin/bash

# Copyright 2024 Flant JSC
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

#
# Copy files with information about the licenses used in modules to _data/ossinfo folder (jekyll will construct an array with this data)

mkdir -p _data/ossinfo/

for path in $(find $MODULES_DIR -iname oss.yaml -print); do
  module_short_name=$(echo $path | sed -E 's#.+/(.+/[^/]+)$#\1#' | cut -d\/ -f-1 | cut -d- -f2-)
  module_full_name=$(echo $path | sed -E 's#.+/(.+/[^/]+)$#\1#' | cut -d\/ -f-1)
  cp -f $path _data/ossinfo/${module_short_name}.yaml
  cat $path >> _data/ossinfo-cumulative.yaml
done
