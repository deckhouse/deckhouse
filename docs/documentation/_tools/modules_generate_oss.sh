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
# Generate OSS.md pages for modules that have oss.yaml files
# Pages are generated only if the module has OSS data in the generated oss.yaml
#

if [[ -z "${OSS_DATA_FILE}" ]]; then
  OSS_DATA_FILE="_data/oss.yaml"
fi

if [[ ! -f "${OSS_DATA_FILE}" ]]; then
  echo "WARN: OSS data file not found: ${OSS_DATA_FILE}"
  exit 0
fi

# Get list of modules that have OSS data
modules_with_oss=$(yq 'keys | .[]' "${OSS_DATA_FILE}" 2>/dev/null)

if [[ -z "${modules_with_oss}" ]]; then
  echo "INFO: No modules with OSS data found"
  exit 0
fi

# Process each module
while IFS= read -r module_name; do
  # Skip empty lines
  [[ -z "${module_name}" ]] && continue
  
  # Find module docs directory (can be in modules/, ee/modules/, ee/se/modules/, etc.)
  # Try to find by pattern: modules/XXX-module-name/docs or ee/modules/XXX-module-name/docs
  module_docs_dir=$(find "${MODULES_DIR}" -type d -path "*/${module_name}/docs" -o -type d -path "*/[0-9]*-${module_name}/docs" 2>/dev/null | head -1)
  
  # If still not found, try to construct path from module directory
  if [[ -z "${module_docs_dir}" ]] || [[ ! -d "${module_docs_dir}" ]]; then
    # Find module directory first
    module_dir=$(find "${MODULES_DIR}" -type d -name "${module_name}" -o -type d -name "[0-9]*-${module_name}" 2>/dev/null | head -1)
    if [[ -n "${module_dir}" ]] && [[ -d "${module_dir}/docs" ]]; then
      module_docs_dir="${module_dir}/docs"
    fi
  fi
  
  # If still not found, try glob patterns
  if [[ -z "${module_docs_dir}" ]] || [[ ! -d "${module_docs_dir}" ]]; then
    for pattern in "${MODULES_DIR}"/*"${module_name}"*/docs "${MODULES_DIR}"/*/*"${module_name}"*/docs; do
      if [[ -d "${pattern}" ]]; then
        module_docs_dir="${pattern}"
        break
      fi
    done
  fi
  
  if [[ -z "${module_docs_dir}" ]] || [[ ! -d "${module_docs_dir}" ]]; then
    continue
  fi
  
  # Check if OSS.md already exists
  oss_md_file="${module_docs_dir}/OSS.md"
  
  # Check if module has OSS data
  oss_count=$(yq ".[\"${module_name}\"] | length" "${OSS_DATA_FILE}" 2>/dev/null || echo "0")
  
  if [[ "${oss_count}" == "0" ]] || [[ -z "${oss_count}" ]]; then
    # Remove OSS.md if it exists but module has no OSS data
    if [[ -f "${oss_md_file}" ]]; then
      rm -f "${oss_md_file}"
    fi
    continue
  fi
  
  # Generate OSS.md if it doesn't exist or if it's a placeholder
  if [[ ! -f "${oss_md_file}" ]] || grep -q "<!-- SCHEMA -->" "${oss_md_file}" 2>/dev/null; then
    # Determine language from MODULES_DIR
    lang="en"
    if [[ "${MODULES_DIR}" == *"_ru"* ]] || [[ "${MODULES_DIR}" == *"ru"* ]]; then
      lang="ru"
    fi
    
    # Generate frontmatter and content
    if [[ "${lang}" == "ru" ]]; then
      title="Используемые компоненты"
      content="Модуль использует следующие open source компоненты:"
    else
      title="Open Source Components"
      content="The module uses the following open source components:"
    fi
    
    cat > "${oss_md_file}" <<EOF
---
title: "${title}"
---

${content}

{% include module-oss.liquid %}
EOF
    
    echo "Generated OSS.md for module: ${module_name} (${module_docs_dir})"
  fi
done <<< "${modules_with_oss}"
