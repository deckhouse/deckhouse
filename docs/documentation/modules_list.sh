#!/bin/bash

# This script outputs alphabetically sorted modules list including path and revision in the YAML format.
# Example:
# ...
# modules:
#   admission-policy-engine:
#     path: modules/015-admission-policy-engine/
#     revision: ce
#   ceph-csi:
#     path: modules/031-ceph-csi/
#     revision: ce
#     parameters-ee:
#

if [[ -z ${MODULES_DIR} ]]; then
  MODULES_DIR=/src
fi

echo "modules:"

if [ -f modules_menu_skip ]; then
  modules_skip_list=$(cat modules_menu_skip)
fi

for module_edition_path in $(find ${MODULES_DIR} -regex '.*/docs/README.md' -print | sed -E "s#^${MODULES_DIR}/modules/#${MODULES_DIR}/ce/modules/#" | sed -E "s#^${MODULES_DIR}/(ce/|be/|se/|ee/|fe/)?modules/([^/]+)/.*\$#\1\2#" | sort -t/ -k 2.4 ); do
  skip=false
  module_path=$(echo $module_edition_path | sed -E 's#ce/|be/|se/|ee/|fe/##')
  # Skip unnecessary modules
  for skip_item in $modules_skip_list ; do
    if [[ $skip_item == $module_path ]] ; then skip=true; break; fi
  done
  if [[ "$skip" == 'true' ]]; then continue; fi

  cat << YAML
  $(echo $module_path | cut -d- -f2-):
    path: modules/${module_path}/
    edition: $(echo $module_edition_path | cut -d/ -f1)
YAML
done
