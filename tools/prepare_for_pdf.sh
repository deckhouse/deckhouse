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
cat > $1 <<EOF
---
title: "Deckhouse Kubernetes Platform: Руководство администратора"
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

clean $PATH_TO_PDF_PAGE

LIST_OF_PAGES=(
"DECKHOUSE_CONFIGURE_RU.md"
"DECKHOUSE_CONFIGURE_GLOBAL_RU.md"
)

for ix in ${!LIST_OF_PAGES[*]}
do
  echo "\n## "$(getname $PATH_TO_PAGES${LIST_OF_PAGES[$ix]}) >> $PATH_TO_PDF_PAGE
  echo "$(gettext $PATH_TO_PAGES${LIST_OF_PAGES[$ix]})" >> $PATH_TO_PDF_PAGE
done

unset LIST_OF_PAGES
LIST_OF_PAGES=(
"security/SECURITY_SOFTWARE_SETUP_RU.md"
"security/KESL_RU.md"
"security/KUMA_RU.md"
)

echo "## Настройка ПО безопасности" >> $PATH_TO_PDF_PAGE

for ix in ${!LIST_OF_PAGES[*]}
do
  echo "\n### "$(getname $PATH_TO_PAGES${LIST_OF_PAGES[$ix]}) >> $PATH_TO_PDF_PAGE
  echo "$(gettext $PATH_TO_PAGES${LIST_OF_PAGES[$ix]})" >> $PATH_TO_PDF_PAGE
done

unset LIST_OF_PAGES
LIST_OF_PAGES=(
"DECKHOUSE-FAQ_RU.md"
)

for ix in ${!LIST_OF_PAGES[*]}
do
  echo "\n## "$(getname $PATH_TO_PAGES${LIST_OF_PAGES[$ix]}) >> $PATH_TO_PDF_PAGE
  echo "$(gettext $PATH_TO_PAGES${LIST_OF_PAGES[$ix]})" >> $PATH_TO_PDF_PAGE
done

echo "## Подсистема Кластер Kubernetes" >> $PATH_TO_PDF_PAGE

unset LIST_OF_MODULES
LIST_OF_MODULES=(
"chrony"
"cni-cilium"
"cilium-hubble"
"control-plane-manager"
"flow-schema"
"ingress-nginx"
"istio"
"node-manager"
"kube-dns"
"local-path-provisioner"
"namespace-configurator"
"priority-class"
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
"operator-prometheus"
"300-prometheus"
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
"extended-monitoring"
"loki"
"log-shipper"
"monitoring-custom"
"340-monitoring-kubernetes"
"340-monitoring-kubernetes-control-plane"
"monitoring-ping"
"operator-prometheus"
"300-prometheus"
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
"multitenancy-manager"
"operator-trivy"
"user-authn"
"user-authz"
"runtime-audit-engine"
"secret-copier"
"secrets-store-integration"
"stronghold"
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

echo "## Подсистема Хранение данных" >> $PATH_TO_PDF_PAGE

unset LIST_OF_MODULES
LIST_OF_MODULES=(
"snapshot-controller"
"csi-ceph"
"csi-nfs"
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
