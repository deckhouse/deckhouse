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

PATH_TO_PDF_PAGE="docs/documentation/pages/pdf/ADMIN_GUIDE_RU.md"
PATH_TO_PAGES='docs/documentation/pages/'
PATH_TO_MODULES="modules"
MODULES=$(find $PATH_TO_MODULES -name "README_RU.md")
PAGES_ORDER=(
"README_RU.md"
"CONFIGURATION_RU.md"
"CR_RU.md"
"EXAMPLES_RU.md"
"FAQ_RU.md"
)

function clean () {
mkdir "docs/documentation/pages/pdf"
cat > $1 <<EOF
---
title: "Deckhouse Platform Certified Security Edition: Руководство администратора"
permalink: ru/deckhouse-admin-guide.html
lang: ru
sidebar: none
toc: true
layout: pdf
---
EOF
}

function getname () {
  cat $1 | grep 'title: ' | sed -r 's!^[^ ]+!!' | sed -e 's/^[[:space:]0-9-]*//' | sed s/'\"'//g
}

function gettext() {
    cat $1 | sed '1,/---/ d' | sed -E "s/^#/###/g; s#(\.\./)+#./#g"
}

function gettextaudit() {
    cat $1 | sed 's/# /## /g' | sed '1,/---/ d' | sed -E "s/^#/###/g; s#(\.\./)+#./#g"
}

clean $PATH_TO_PDF_PAGE

LIST_OF_PAGES=(
"OVERVIEW_RU.md"
)

for ix in ${!LIST_OF_PAGES[*]}
do
  echo "\n## "$(getname $PATH_TO_PAGES${LIST_OF_PAGES[$ix]}) >> $PATH_TO_PDF_PAGE
  echo "$(gettext $PATH_TO_PAGES${LIST_OF_PAGES[$ix]})" >> $PATH_TO_PDF_PAGE
done

echo "## Безопасность" >> $PATH_TO_PDF_PAGE

echo "### События безопасности" >> $PATH_TO_PDF_PAGE

unset LIST_OF_PAGES
LIST_OF_PAGES=(
"security/KUBERNETES-API-AUDIT_RU.md"
"security/RUNTIME-AUDIT_RU.md"
)

for ix in ${!LIST_OF_PAGES[*]}
do
  echo "\n#### "$(getname $PATH_TO_PAGES${LIST_OF_PAGES[$ix]}) >> $PATH_TO_PDF_PAGE
  echo "$(gettextaudit $PATH_TO_PAGES${LIST_OF_PAGES[$ix]})" >> $PATH_TO_PDF_PAGE
done

unset LIST_OF_PAGES
LIST_OF_PAGES=(
"security/SECURITY-POLICIES_RU.md"
"security/SCANNING_RU.md"
"security/CERTIFICATES_RU.md"
"security/KUMA-AND-AV-SOFTWARE_RU.md"
)

for ix in ${!LIST_OF_PAGES[*]}
do
  echo "\n### "$(getname $PATH_TO_PAGES${LIST_OF_PAGES[$ix]}) >> $PATH_TO_PDF_PAGE
  echo "$(gettext $PATH_TO_PAGES${LIST_OF_PAGES[$ix]})" >> $PATH_TO_PDF_PAGE
done

echo "## Подсистема Кластер Kubernetes" >> $PATH_TO_PDF_PAGE

unset LIST_OF_MODULES
LIST_OF_MODULES=(
"chrony"
"control-plane-manager"
"node-manager"
"local-path-provisioner"
"namespace-configurator"
"priority-class"
"038-registry"
)

for ix in ${!LIST_OF_MODULES[*]}
do
  MODULE_PATH=$(find ${PATH_TO_MODULES} -maxdepth 1 -type d -name "*${LIST_OF_MODULES[$ix]}" -print | sed 's|.*/||' )
  files=$(find "${PATH_TO_MODULES}/${MODULE_PATH}" -name "*.md" | sort -t '-' -k2)
      for ixp in ${!PAGES_ORDER[*]}
      do
        for file in $files
        do
          if [[ $file == *"${PAGES_ORDER[ixp]}"* ]]; then
            echo "\n### "$(getname $file) >> $PATH_TO_PDF_PAGE
            if [[ $file == *""CONFIGURATION_RU.md""* ]]; then
              schema_path="${PATH_TO_MODULES}/${MODULE_PATH}/crds"
              module_path=$(echo $schema_path | cut -d\/ -f-2 )
              module_file_name=$(echo $schema_path | awk -F\/ '{print $NF}')
              module_name=$(echo $schema_path | cut -d\/ -f2 | sed 's/^[0-9]*-*//')
              schema_path_relative=$(echo $schema_path | cut -d\/ -f3- | sed "s#\.yaml##; s#\.##g; s#\/#\.#g")
              echo "$(gettext $file)" | sed "/<!-- SCHEMA -->/i\ " >> $PATH_TO_PDF_PAGE
              echo "#### {{ site.data.i18n.common['parameters'][page.lang] }}" >> $PATH_TO_PDF_PAGE
              echo "{{ site.data.schemas['${module_name}'].config-values | format_module_configuration: moduleKebabName }}" >> $PATH_TO_PDF_PAGE
            elif [[ $file == *""CR_RU.md""* ]]; then
              for schema_path in $(find "modules/$MODULE_PATH" -regex '^.*/crds/.*.yaml$' -print | grep -v '/crds/doc-ru-'| sort); do
                module_path=$(echo $schema_path | cut -d\/ -f-2 )
                module_file_name=$(echo $schema_path | awk -F\/ '{print $NF}')
                module_name=$(echo $schema_path | cut -d\/ -f2 | sed 's/^[0-9]*-*//')
                schema_path_relative=$(echo $schema_path | cut -d\/ -f3- | sed "s#\.yaml##; s#\.##g; s#\/#\.#g")
                echo "{{ site.data.schemas.${module_name}.${schema_path_relative} | format_crd: \"${module_name}\" }}" >> $PATH_TO_PDF_PAGE
              done
            else
              echo "$(gettext $file)" >> $PATH_TO_PDF_PAGE
            fi
          fi
        done
      done
done

echo "## Подсистема Deckhouse" >> $PATH_TO_PDF_PAGE

unset LIST_OF_MODULES
LIST_OF_MODULES=(
"console"
"990-commander"
"002-deckhouse"
"deckhouse-tools"
"documentation"
)

for ix in ${!LIST_OF_MODULES[*]}
do
  MODULE_PATH=$(find ${PATH_TO_MODULES} -maxdepth 1 -type d -name "*${LIST_OF_MODULES[$ix]}" -print | sed 's|.*/||' )
  files=$(find "${PATH_TO_MODULES}/${MODULE_PATH}" -name "*.md" | sort -t '-' -k2)
      for ixp in ${!PAGES_ORDER[*]}
      do
        for file in $files
        do
          if [[ $file == *"${PAGES_ORDER[ixp]}"* ]]; then
            echo "\n### "$(getname $file) >> $PATH_TO_PDF_PAGE
            if [[ $file == *""CONFIGURATION_RU.md""* ]]; then
              schema_path="${PATH_TO_MODULES}/${MODULE_PATH}/crds"
              module_path=$(echo $schema_path | cut -d\/ -f-2 )
              module_file_name=$(echo $schema_path | awk -F\/ '{print $NF}')
              module_name=$(echo $schema_path | cut -d\/ -f2 | sed 's/^[0-9]*-*//')
              schema_path_relative=$(echo $schema_path | cut -d\/ -f3- | sed "s#\.yaml##; s#\.##g; s#\/#\.#g")
              echo "$(gettext $file)" | sed "/<!-- SCHEMA -->/i\ " >> $PATH_TO_PDF_PAGE
              echo "#### {{ site.data.i18n.common['parameters'][page.lang] }}" >> $PATH_TO_PDF_PAGE
              echo "{{ site.data.schemas['${module_name}'].config-values | format_module_configuration: moduleKebabName }}" >> $PATH_TO_PDF_PAGE
            elif [[ $file == *""CR_RU.md""* ]]; then
              for schema_path in $(find "modules/$MODULE_PATH" -regex '^.*/crds/.*.yaml$' -print | grep -v '/crds/doc-ru-'| sort); do
                module_path=$(echo $schema_path | cut -d\/ -f-2 )
                module_file_name=$(echo $schema_path | awk -F\/ '{print $NF}')
                module_name=$(echo $schema_path | cut -d\/ -f2 | sed 's/^[0-9]*-*//')
                schema_path_relative=$(echo $schema_path | cut -d\/ -f3- | sed "s#\.yaml##; s#\.##g; s#\/#\.#g")
                echo "{{ site.data.schemas.${module_name}.${schema_path_relative} | format_crd: \"${module_name}\" }}" >> $PATH_TO_PDF_PAGE
              done
            else
              echo "$(gettext $file)" >> $PATH_TO_PDF_PAGE
            fi
          fi
        done
      done
done

echo "## Подсистема Безопасность" >> $PATH_TO_PDF_PAGE

unset LIST_OF_MODULES
LIST_OF_MODULES=(
"admission-policy-engine"
"cert-manager"
"900-gost-integrity-controller"
"multitenancy-manager"
"operator-trivy"
"user-authn"
"user-authz"
"runtime-audit-engine"
"secret-copier"
"secrets-store-integration"
)

for ix in ${!LIST_OF_MODULES[*]}
do
  case ${LIST_OF_MODULES[$ix]} in
    operator-trivy)
      MODULE_PATH="ee/modules/500-operator-trivy"
      files=$(find "${MODULE_PATH}" -name "*.md" | sort -t '-' -k2)
      for ixp in ${!PAGES_ORDER[*]}
      do
        for file in $files
        do
          if [[ $file == *"${PAGES_ORDER[ixp]}"* ]]; then
            echo "\n### "$(getname $file) >> $PATH_TO_PDF_PAGE
              if [[ $file == *""CONFIGURATION_RU.md""* ]]; then
                schema_path="${MODULE_PATH}/crds"
                module_path=$(echo $schema_path | cut -d\/ -f-2 )
                module_file_name=$(echo $schema_path | awk -F\/ '{print $NF}')
                module_name=$(echo $schema_path | cut -d\/ -f2 | sed 's/^[0-9]*-*//')
                schema_path_relative=$(echo $schema_path | cut -d\/ -f3- | sed "s#\.yaml##; s#\.##g; s#\/#\.#g")
                echo "$(gettext $file)" | sed "/<!-- SCHEMA -->/i\ " >> $PATH_TO_PDF_PAGE
                echo "#### {{ site.data.i18n.common['parameters'][page.lang] }}" >> $PATH_TO_PDF_PAGE
                echo "{{ site.data.schemas['${module_name}'].config-values | format_module_configuration: moduleKebabName }}" >> $PATH_TO_PDF_PAGE
              elif [[ $file == *""CR_RU.md""* ]]; then
                for schema_path in $(find "$MODULE_PATH" -regex '^.*/crds/.*.yaml$' -print | grep -v '/crds/doc-ru-'| sort); do
                  module_path=$(echo $schema_path | cut -d\/ -f-2 )
                  module_file_name=$(echo $schema_path | awk -F\/ '{print $NF}')
                  module_name=$(echo $schema_path | cut -d\/ -f2 | sed 's/^[0-9]*-*//')
                  schema_path_relative=$(echo $schema_path | cut -d\/ -f3- | sed "s#\.yaml##; s#\.##g; s#\/#\.#g")
                  echo "{{ site.data.schemas.${module_name}.${schema_path_relative} | format_crd: \"${module_name}\" }}" >> $PATH_TO_PDF_PAGE
                done
              else
                echo "$(gettext $file)" >> $PATH_TO_PDF_PAGE
              fi
          fi
        done
      done
      ;;
    runtime-audit-engine)
      MODULE_PATH="ee/modules/650-runtime-audit-engine"
      files=$(find "${MODULE_PATH}" -name "*.md" | sort -t '-' -k2)
      for ixp in ${!PAGES_ORDER[*]}
      do
        for file in $files
        do
          if [[ $file == *"${PAGES_ORDER[ixp]}"* ]]; then
            echo "\n### "$(getname $file) >> $PATH_TO_PDF_PAGE
              if [[ $file == *""CONFIGURATION_RU.md""* ]]; then
                schema_path="${MODULE_PATH}/crds"
                module_path=$(echo $schema_path | cut -d\/ -f-2 )
                module_file_name=$(echo $schema_path | awk -F\/ '{print $NF}')
                module_name=$(echo $schema_path | cut -d\/ -f2 | sed 's/^[0-9]*-*//')
                schema_path_relative=$(echo $schema_path | cut -d\/ -f3- | sed "s#\.yaml##; s#\.##g; s#\/#\.#g")
                echo "$(gettext $file)" | sed "/<!-- SCHEMA -->/i\ " >> $PATH_TO_PDF_PAGE
                echo "#### {{ site.data.i18n.common['parameters'][page.lang] }}" >> $PATH_TO_PDF_PAGE
                echo "{{ site.data.schemas['${module_name}'].config-values | format_module_configuration: moduleKebabName }}" >> $PATH_TO_PDF_PAGE
              elif [[ $file == *""CR_RU.md""* ]]; then
                for schema_path in $(find "$MODULE_PATH" -regex '^.*/crds/.*.yaml$' -print | grep -v '/crds/doc-ru-'| sort); do
                  module_path=$(echo $schema_path | cut -d\/ -f-2 )
                  module_file_name=$(echo $schema_path | awk -F\/ '{print $NF}')
                  module_name=$(echo $schema_path | cut -d\/ -f2 | sed 's/^[0-9]*-*//' )
                  schema_path_relative=$(echo $schema_path | cut -d\/ -f3- | sed "s#\.yaml##; s#\.##g; s#\/#\.#g")
                  echo "{{ site.data.schemas.${module_name}.${schema_path_relative} | format_crd: \"${module_name}\" }}" >> $PATH_TO_PDF_PAGE
                done
              else
                echo "$(gettext $file)" >> $PATH_TO_PDF_PAGE
              fi
          fi
        done
      done
      ;;
    *)
      MODULE_PATH=$(find ${PATH_TO_MODULES} -maxdepth 1 -type d -name "*${LIST_OF_MODULES[$ix]}" -print | sed 's|.*/||' )
        files=$(find "${PATH_TO_MODULES}/${MODULE_PATH}" -name "*.md" | sort -t '-' -k2)
            for ixp in ${!PAGES_ORDER[*]}
            do
              for file in $files
              do
                if [[ $file == *"${PAGES_ORDER[ixp]}"* ]]; then
                  echo "\n### "$(getname $file) >> $PATH_TO_PDF_PAGE
                  if [[ $file == *""CONFIGURATION_RU.md""* ]]; then
                    schema_path="${PATH_TO_MODULES}/${MODULE_PATH}/crds"
                    module_path=$(echo $schema_path | cut -d\/ -f-2 )
                    module_file_name=$(echo $schema_path | awk -F\/ '{print $NF}')
                    module_name=$(echo $schema_path | cut -d\/ -f2 | sed 's/^[0-9]*-*//')
                    schema_path_relative=$(echo $schema_path | cut -d\/ -f3- | sed "s#\.yaml##; s#\.##g; s#\/#\.#g")
                    echo "$(gettext $file)" | sed "/<!-- SCHEMA -->/i\ " >> $PATH_TO_PDF_PAGE
                    echo "#### {{ site.data.i18n.common['parameters'][page.lang] }}" >> $PATH_TO_PDF_PAGE
                    echo "{{ site.data.schemas['${module_name}'].config-values | format_module_configuration: moduleKebabName }}" >> $PATH_TO_PDF_PAGE
                  elif [[ $file == *""CR_RU.md""* ]]; then
                    for schema_path in $(find "modules/$MODULE_PATH" -regex '^.*/crds/.*.yaml$' -print | grep -v '/crds/doc-ru-'| sort); do
                      module_path=$(echo $schema_path | cut -d\/ -f-2 )
                      module_file_name=$(echo $schema_path | awk -F\/ '{print $NF}')
                      module_name=$(echo $schema_path | cut -d\/ -f2 | sed 's/^[0-9]*-*//')
                      schema_path_relative=$(echo $schema_path | cut -d\/ -f3- | sed "s#\.yaml##; s#\.##g; s#\/#\.#g")
                      echo "{{ site.data.schemas.${module_name}.${schema_path_relative} | format_crd: \"${module_name}\" }}" >> $PATH_TO_PDF_PAGE
                    done
                  else
                    echo "$(gettext $file)" >> $PATH_TO_PDF_PAGE
                  fi
                fi
              done
            done
      ;;
    esac
done

unset LIST_OF_MODULES
LIST_OF_MODULES=(
"stronghold"
)
LIST_OF_STRONGHOLD_PAGES=(
"README_RU.md"
"CONFIGURATION_RU.md"
"USAGE_RU.md"
"ADMIN_GUIDE_AUTH_METHODS_RU.md"
"ADMIN_GUIDE_KV_RU.md"
"ADMIN_GUIDE_PKI_RU.md"
"ADMIN_GUIDE_RU.md"
"ADMIN_GUIDE_SECRET_ENGINES_RU.md"
"CHARACTERISTICS_DESCRIPTION_RU.md"
)

for ix in ${!LIST_OF_MODULES[*]}
do
  MODULE_PATH=$(find ${PATH_TO_MODULES} -maxdepth 1 -type d -name "*${LIST_OF_MODULES[$ix]}" -print | sed 's|.*/||' )
  files=$(find "${PATH_TO_MODULES}/${MODULE_PATH}" -name "*.md" | sort -t '-' -k2)
      for ixp in ${!LIST_OF_STRONGHOLD_PAGES[*]}
      do
        for file in $files
        do
          if [[ $file == *"${LIST_OF_STRONGHOLD_PAGES[ixp]}"* ]]; then
            echo "\n### "$(getname $file) >> $PATH_TO_PDF_PAGE
            if [[ $file == *""CONFIGURATION_RU.md""* ]]; then
              schema_path="${PATH_TO_MODULES}/${MODULE_PATH}/crds"
              module_path=$(echo $schema_path | cut -d\/ -f-2 )
              module_file_name=$(echo $schema_path | awk -F\/ '{print $NF}')
              module_name=$(echo $schema_path | cut -d\/ -f2 | sed 's/^[0-9]*-*//')
              schema_path_relative=$(echo $schema_path | cut -d\/ -f3- | sed "s#\.yaml##; s#\.##g; s#\/#\.#g")
              echo "$(gettext $file)" | sed "/<!-- SCHEMA -->/i\ " >> $PATH_TO_PDF_PAGE
              echo "#### {{ site.data.i18n.common['parameters'][page.lang] }}" >> $PATH_TO_PDF_PAGE
              echo "{{ site.data.schemas['${module_name}'].config-values | format_module_configuration: moduleKebabName }}" >> $PATH_TO_PDF_PAGE
            elif [[ $file == *""CR_RU.md""* ]]; then
              for schema_path in $(find "modules/$MODULE_PATH" -regex '^.*/crds/.*.yaml$' -print | grep -v '/crds/doc-ru-'| sort); do
                module_path=$(echo $schema_path | cut -d\/ -f-2 )
                module_file_name=$(echo $schema_path | awk -F\/ '{print $NF}')
                module_name=$(echo $schema_path | cut -d\/ -f2 | sed 's/^[0-9]*-*//' )
                schema_path_relative=$(echo $schema_path | cut -d\/ -f3- | sed "s#\.yaml##; s#\.##g; s#\/#\.#g")
                echo "{{ site.data.schemas.${module_name}.${schema_path_relative} | format_crd: \"${module_name}\" }}" >> $PATH_TO_PDF_PAGE
              done
            else
              echo "$(gettext $file)" >> $PATH_TO_PDF_PAGE
            fi
          fi
        done
      done
done

echo "## Подсистема Мониторинг" >> $PATH_TO_PDF_PAGE

unset LIST_OF_MODULES
LIST_OF_MODULES=(
"extended-monitoring"
"loki"
"log-shipper"
"monitoring-custom"
"340-monitoring-kubernetes"
"340-monitoring-kubernetes-control-plane"
"monitoring-ping"
"990-observability"
"operator-prometheus"
"300-prometheus"
"500-upmeter"
)

for ix in ${!LIST_OF_MODULES[*]}
do
  MODULE_PATH=$(find ${PATH_TO_MODULES} -maxdepth 1 -type d -name "*${LIST_OF_MODULES[$ix]}" -print | sed 's|.*/||' )
  files=$(find "${PATH_TO_MODULES}/${MODULE_PATH}" -name "*.md" | sort -t '-' -k2)
      for ixp in ${!PAGES_ORDER[*]}
      do
        for file in $files
        do
          if [[ $file == *"${PAGES_ORDER[ixp]}"* ]]; then
            echo "\n### "$(getname $file) >> $PATH_TO_PDF_PAGE
            if [[ $file == *""CONFIGURATION_RU.md""* ]]; then
              schema_path="${PATH_TO_MODULES}/${MODULE_PATH}/crds"
              module_path=$(echo $schema_path | cut -d\/ -f-2 )
              module_file_name=$(echo $schema_path | awk -F\/ '{print $NF}')
              module_name=$(echo $schema_path | cut -d\/ -f2 | sed 's/^[0-9]*-*//')
              schema_path_relative=$(echo $schema_path | cut -d\/ -f3- | sed "s#\.yaml##; s#\.##g; s#\/#\.#g")
              echo "$(gettext $file)" | sed "/<!-- SCHEMA -->/i\ " >> $PATH_TO_PDF_PAGE
              echo "#### {{ site.data.i18n.common['parameters'][page.lang] }}" >> $PATH_TO_PDF_PAGE
              echo "{{ site.data.schemas['${module_name}'].config-values | format_module_configuration: moduleKebabName }}" >> $PATH_TO_PDF_PAGE
            elif [[ $file == *""CR_RU.md""* ]]; then
              for schema_path in $(find "modules/$MODULE_PATH" -regex '^.*/crds/.*.yaml$' -print | grep -v '/crds/doc-ru-'| sort); do
                module_path=$(echo $schema_path | cut -d\/ -f-2 )
                module_file_name=$(echo $schema_path | awk -F\/ '{print $NF}')
                module_name=$(echo $schema_path | cut -d\/ -f2 | sed 's/^[0-9]*-*//')
                schema_path_relative=$(echo $schema_path | cut -d\/ -f3- | sed "s#\.yaml##; s#\.##g; s#\/#\.#g")
                echo "{{ site.data.schemas.${module_name}.${schema_path_relative} | format_crd: \"${module_name}\" }}" >> $PATH_TO_PDF_PAGE
              done
            else
              echo "$(gettext $file)" >> $PATH_TO_PDF_PAGE
            fi
          fi
        done
      done
done

echo "## Подсистема Масштабирование и управление ресурсами" >> $PATH_TO_PDF_PAGE

unset LIST_OF_MODULES
LIST_OF_MODULES=(
"400-descheduler"
"301-prometheus-metrics-adapter"
"302-vertical-pod-autoscaler"
)

for ix in ${!LIST_OF_MODULES[*]}
do
  MODULE_PATH=$(find ${PATH_TO_MODULES} -maxdepth 1 -type d -name "*${LIST_OF_MODULES[$ix]}" -print | sed 's|.*/||' )
  files=$(find "${PATH_TO_MODULES}/${MODULE_PATH}" -name "*.md" | sort -t '-' -k2)
      for ixp in ${!PAGES_ORDER[*]}
      do
        for file in $files
        do
          if [[ $file == *"${PAGES_ORDER[ixp]}"* ]]; then
            echo "\n### "$(getname $file) >> $PATH_TO_PDF_PAGE
            if [[ $file == *""CONFIGURATION_RU.md""* ]]; then
              schema_path="${PATH_TO_MODULES}/${MODULE_PATH}/crds"
              module_path=$(echo $schema_path | cut -d\/ -f-2 )
              module_file_name=$(echo $schema_path | awk -F\/ '{print $NF}')
              module_name=$(echo $schema_path | cut -d\/ -f2 | sed 's/^[0-9]*-*//')
              schema_path_relative=$(echo $schema_path | cut -d\/ -f3- | sed "s#\.yaml##; s#\.##g; s#\/#\.#g")
              echo "$(gettext $file)" | sed "/<!-- SCHEMA -->/i\ " >> $PATH_TO_PDF_PAGE
              echo "#### {{ site.data.i18n.common['parameters'][page.lang] }}" >> $PATH_TO_PDF_PAGE
              echo "{{ site.data.schemas['${module_name}'].config-values | format_module_configuration: moduleKebabName }}" >> $PATH_TO_PDF_PAGE
            elif [[ $file == *""CR_RU.md""* ]]; then
              for schema_path in $(find "modules/$MODULE_PATH" -regex '^.*/crds/.*.yaml$' -print | grep -v '/crds/doc-ru-'| sort); do
                module_path=$(echo $schema_path | cut -d\/ -f-2 )
                module_file_name=$(echo $schema_path | awk -F\/ '{print $NF}')
                module_name=$(echo $schema_path | cut -d\/ -f2 | sed 's/^[0-9]*-*//')
                schema_path_relative=$(echo $schema_path | cut -d\/ -f3- | sed "s#\.yaml##; s#\.##g; s#\/#\.#g")
                echo "{{ site.data.schemas.${module_name}.${schema_path_relative} | format_crd: \"${module_name}\" }}" >> $PATH_TO_PDF_PAGE
              done
            else
              echo "$(gettext $file)" >> $PATH_TO_PDF_PAGE
            fi
          fi
        done
      done
done

echo "## Подсистема Сеть" >> $PATH_TO_PDF_PAGE

unset LIST_OF_MODULES
LIST_OF_MODULES=(
"021-cni-cilium"
"500-cilium-hubble"
"402-ingress-nginx"
"110-istio"
"042-kube-dns"
""
)

for ix in ${!LIST_OF_MODULES[*]}
do
  MODULE_PATH=$(find ${PATH_TO_MODULES} -maxdepth 1 -type d -name "*${LIST_OF_MODULES[$ix]}" -print | sed 's|.*/||' )
  files=$(find "${PATH_TO_MODULES}/${MODULE_PATH}" -name "*.md" | sort -t '-' -k2)
      for ixp in ${!PAGES_ORDER[*]}
      do
        for file in $files
        do
          if [[ $file == *"${PAGES_ORDER[ixp]}"* ]]; then
            echo "\n### "$(getname $file) >> $PATH_TO_PDF_PAGE
            if [[ $file == *""CONFIGURATION_RU.md""* ]]; then
              schema_path="${PATH_TO_MODULES}/${MODULE_PATH}/crds"
              module_path=$(echo $schema_path | cut -d\/ -f-2 )
              module_file_name=$(echo $schema_path | awk -F\/ '{print $NF}')
              module_name=$(echo $schema_path | cut -d\/ -f2 | sed 's/^[0-9]*-*//')
              schema_path_relative=$(echo $schema_path | cut -d\/ -f3- | sed "s#\.yaml##; s#\.##g; s#\/#\.#g")
              echo "$(gettext $file)" | sed "/<!-- SCHEMA -->/i\ " >> $PATH_TO_PDF_PAGE
              echo "#### {{ site.data.i18n.common['parameters'][page.lang] }}" >> $PATH_TO_PDF_PAGE
              echo "{{ site.data.schemas['${module_name}'].config-values | format_module_configuration: moduleKebabName }}" >> $PATH_TO_PDF_PAGE
            elif [[ $file == *""CR_RU.md""* ]]; then
              for schema_path in $(find "modules/$MODULE_PATH" -regex '^.*/crds/.*.yaml$' -print | grep -v '/crds/doc-ru-'| sort); do
                module_path=$(echo $schema_path | cut -d\/ -f-2 )
                module_file_name=$(echo $schema_path | awk -F\/ '{print $NF}')
                module_name=$(echo $schema_path | cut -d\/ -f2 | sed 's/^[0-9]*-*//')
                schema_path_relative=$(echo $schema_path | cut -d\/ -f3- | sed "s#\.yaml##; s#\.##g; s#\/#\.#g")
                echo "{{ site.data.schemas.${module_name}.${schema_path_relative} | format_crd: \"${module_name}\" }}" >> $PATH_TO_PDF_PAGE
              done
            else
              echo "$(gettext $file)" >> $PATH_TO_PDF_PAGE
            fi
          fi
        done
      done
done

echo "## Подсистема Хранение данных" >> $PATH_TO_PDF_PAGE

unset LIST_OF_MODULES
LIST_OF_MODULES=(
"snapshot-controller"
"csi-ceph"
"csi-nfs"
"990-csi-scsi-generic"
"990-csi-yadro-tatlin-unified"
"sds-local-volume"
"sds-node-configurator"
)

for ix in ${!LIST_OF_MODULES[*]}
do
  MODULE_PATH=$(find ${PATH_TO_MODULES} -maxdepth 1 -type d -name "*${LIST_OF_MODULES[$ix]}" -print | sed 's|.*/||' )
  files=$(find "${PATH_TO_MODULES}/${MODULE_PATH}" -name "*.md" | sort -t '-' -k2)
      for ixp in ${!PAGES_ORDER[*]}
      do
        for file in $files
        do
          if [[ $file == *"${PAGES_ORDER[ixp]}"* ]]; then
            echo "\n### "$(getname $file) >> $PATH_TO_PDF_PAGE
            if [[ $file == *""CONFIGURATION_RU.md""* ]]; then
              schema_path="${PATH_TO_MODULES}/${MODULE_PATH}/crds"
              module_path=$(echo $schema_path | cut -d\/ -f-2 )
              module_file_name=$(echo $schema_path | awk -F\/ '{print $NF}')
              module_name=$(echo $schema_path | cut -d\/ -f2 | sed 's/^[0-9]*-*//')
              schema_path_relative=$(echo $schema_path | cut -d\/ -f3- | sed "s#\.yaml##; s#\.##g; s#\/#\.#g")
              echo "$(gettext $file)" | sed "/<!-- SCHEMA -->/i\ " >> $PATH_TO_PDF_PAGE
              echo "#### {{ site.data.i18n.common['parameters'][page.lang] }}" >> $PATH_TO_PDF_PAGE
              echo "{{ site.data.schemas['${module_name}'].config-values | format_module_configuration: moduleKebabName }}" >> $PATH_TO_PDF_PAGE
            elif [[ $file == *""CR_RU.md""* ]]; then
              for schema_path in $(find "modules/$MODULE_PATH" -regex '^.*/crds/.*.yaml$' -print | grep -v '/crds/doc-ru-'| sort); do
                module_path=$(echo $schema_path | cut -d\/ -f-2 )
                module_file_name=$(echo $schema_path | awk -F\/ '{print $NF}')
                module_name=$(echo $schema_path | cut -d\/ -f2 | sed 's/^[0-9]*-*//' )
                schema_path_relative=$(echo $schema_path | cut -d\/ -f3- | sed "s#\.yaml##; s#\.##g; s#\/#\.#g")
                echo "{{ site.data.schemas.${module_name}.${schema_path_relative} | format_crd: \"${module_name}\" }}" >> $PATH_TO_PDF_PAGE
              done
            else
              echo "$(gettext $file)" >> $PATH_TO_PDF_PAGE
            fi
          fi
        done
      done
done

echo "## Подсистема Инфраструктура" >> $PATH_TO_PDF_PAGE

unset LIST_OF_MODULES
LIST_OF_MODULES=(
"030-cloud-provider-dvp"
"040-terraform-manager"
)

for ix in ${!LIST_OF_MODULES[*]}
do
  MODULE_PATH=$(find ${PATH_TO_MODULES} -maxdepth 1 -type d -name "*${LIST_OF_MODULES[$ix]}" -print | sed 's|.*/||' )
  files=$(find "${PATH_TO_MODULES}/${MODULE_PATH}" -name "*.md" | sort -t '-' -k2)
      for ixp in ${!PAGES_ORDER[*]}
      do
        for file in $files
        do
          if [[ $file == *"${PAGES_ORDER[ixp]}"* ]]; then
            echo "\n### "$(getname $file) >> $PATH_TO_PDF_PAGE
            if [[ $file == *""CONFIGURATION_RU.md""* ]]; then
              schema_path="${PATH_TO_MODULES}/${MODULE_PATH}/crds"
              module_path=$(echo $schema_path | cut -d\/ -f-2 )
              module_file_name=$(echo $schema_path | awk -F\/ '{print $NF}')
              module_name=$(echo $schema_path | cut -d\/ -f2 | sed 's/^[0-9]*-*//')
              schema_path_relative=$(echo $schema_path | cut -d\/ -f3- | sed "s#\.yaml##; s#\.##g; s#\/#\.#g")
              echo "$(gettext $file)" | sed "/<!-- SCHEMA -->/i\ " >> $PATH_TO_PDF_PAGE
              echo "#### {{ site.data.i18n.common['parameters'][page.lang] }}" >> $PATH_TO_PDF_PAGE
              echo "{{ site.data.schemas['${module_name}'].config-values | format_module_configuration: moduleKebabName }}" >> $PATH_TO_PDF_PAGE
            elif [[ $file == *""CR_RU.md""* ]]; then
              for schema_path in $(find "modules/$MODULE_PATH" -regex '^.*/crds/.*.yaml$' -print | grep -v '/crds/doc-ru-'| sort); do
                module_path=$(echo $schema_path | cut -d\/ -f-2 )
                module_file_name=$(echo $schema_path | awk -F\/ '{print $NF}')
                module_name=$(echo $schema_path | cut -d\/ -f2 | sed 's/^[0-9]*-*//' )
                schema_path_relative=$(echo $schema_path | cut -d\/ -f3- | sed "s#\.yaml##; s#\.##g; s#\/#\.#g")
                echo "{{ site.data.schemas.${module_name}.${schema_path_relative} | format_crd: \"${module_name}\" }}" >> $PATH_TO_PDF_PAGE
              done
            else
              echo "$(gettext $file)" >> $PATH_TO_PDF_PAGE
            fi
          fi
        done
      done
done

echo "## Справка" >> $PATH_TO_PDF_PAGE

echo "### API" >> $PATH_TO_PDF_PAGE

unset LIST_OF_PAGES
LIST_OF_PAGES=(
"reference/api/GLOBAL_RU.md"
"reference/api/CR_RU.md"
)



for ix in ${!LIST_OF_PAGES[*]}
do
  echo "<div markdown="1">" >> $PATH_TO_PDF_PAGE
  echo "\n#### "$(getname $PATH_TO_PAGES${LIST_OF_PAGES[$ix]}) >> $PATH_TO_PDF_PAGE
  echo "</div>" >> $PATH_TO_PDF_PAGE
  echo "$(gettext $PATH_TO_PAGES${LIST_OF_PAGES[$ix]})" >> $PATH_TO_PDF_PAGE
done

unset LIST_OF_PAGES
LIST_OF_PAGES=(
"reference/NETWORK_INTERACTION_RU.md"
"reference/SYSCTL_RU.md"
"reference/USED_DIRECTORIES_RU.md"
)

for ix in ${!LIST_OF_PAGES[*]}
do
  echo "<div markdown="1">" >> $PATH_TO_PDF_PAGE
  echo "\n### "$(getname $PATH_TO_PAGES${LIST_OF_PAGES[$ix]}) >> $PATH_TO_PDF_PAGE
  echo "$(gettext $PATH_TO_PAGES${LIST_OF_PAGES[$ix]})" >> $PATH_TO_PDF_PAGE
  echo "</div>" >> $PATH_TO_PDF_PAGE
done

echo "## Консольные утилиты" >> $PATH_TO_PDF_PAGE

unset LIST_OF_PAGES
LIST_OF_PAGES=(
"DECKHOUSE-CLI_RU.md"
)

for ix in ${!LIST_OF_PAGES[*]}
do
  echo "<div markdown="1">" >> $PATH_TO_PDF_PAGE
  echo "\n## "$(getname $PATH_TO_PAGES${LIST_OF_PAGES[$ix]}) >> $PATH_TO_PDF_PAGE
  echo "$(gettext $PATH_TO_PAGES${LIST_OF_PAGES[$ix]})" >> $PATH_TO_PDF_PAGE
  echo "</div>" >> $PATH_TO_PDF_PAGE
done
