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

PATH_TO_PDF_PAGE="ADMIN_GUIDE.md"
PATH_TO_PDF_PAGE_RU="ADMIN_GUIDE_RU.md"
PATH_TO_PAGES='documentation/pages/'
PATH_TO_MODULES="modules"
MODULES=$(find $PATH_TO_MODULES -name "README_RU.md")
PAGES_ORDER=(
"README.md"
"CR.md"
"EXAMPLES.md"
"FAQ.md"
)

function clean () {
cat > $1 <<EOF
---
title: "Deckhouse Kubernetes Platform: $3"
permalink: $2/deckhouse-admin-guide.html
lang: $2
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

clean $PATH_TO_PDF_PAGE "en" "The Administrator's Guide"
clean $PATH_TO_PDF_PAGE_RU "ru" "Руководство администратора"

echo "# Deckhouse Kubernetes Platform" >> $PATH_TO_PDF_PAGE
echo "# Deckhouse Kubernetes Platform" >> $PATH_TO_PDF_PAGE_RU

echo "# Platform installation" >> $PATH_TO_PDF_PAGE
echo "# Установка платформы" >> $PATH_TO_PDF_PAGE_RU

unset LIST_OF_PAGES
LIST_OF_PAGES=(
"installing/README.md"
"installing/CONFIGURATION.md"
)

for ix in ${!LIST_OF_PAGES[*]}
do
  echo "Preparing page $PATH_TO_PAGES${LIST_OF_PAGES[$ix]}"
  echo "## "$(getname $PATH_TO_PAGES${LIST_OF_PAGES[$ix]}) >> $PATH_TO_PDF_PAGE
  echo "$(gettext $PATH_TO_PAGES${LIST_OF_PAGES[$ix]})" >> $PATH_TO_PDF_PAGE
  RU_PAGE="$(echo $PATH_TO_PAGES${LIST_OF_PAGES[$ix]} | sed 's/\.md$//')_RU.md"
  echo "Preparing page $RU_PAGE"
  echo "## "$(getname $RU_PAGE) >> $PATH_TO_PDF_PAGE_RU
  echo "$(gettext $RU_PAGE) | " >> $PATH_TO_PDF_PAGE_RU
done

echo "# Platform configuration" >> $PATH_TO_PDF_PAGE
echo "# Настройка платформы" >> $PATH_TO_PDF_PAGE_RU

unset LIST_OF_PAGES
LIST_OF_PAGES=(
"DECKHOUSE_CONFIGURE.md"
"DECKHOUSE_CONFIGURE_GLOBAL.md"
"DECKHOUSE_CR.md"
)

for ix in ${!LIST_OF_PAGES[*]}
do
  echo "Preparing page $PATH_TO_PAGES${LIST_OF_PAGES[$ix]}"
  echo "## "$(getname $PATH_TO_PAGES${LIST_OF_PAGES[$ix]}) >> $PATH_TO_PDF_PAGE
  echo "$(gettext $PATH_TO_PAGES${LIST_OF_PAGES[$ix]})" >> $PATH_TO_PDF_PAGE
  RU_PAGE="$(echo $PATH_TO_PAGES${LIST_OF_PAGES[$ix]} | sed 's/\.md$//')_RU.md"
  echo "Preparing page $RU_PAGE"
  echo "## "$(getname $RU_PAGE) >> $PATH_TO_PDF_PAGE_RU
  echo "$(gettext $RU_PAGE) | " >> $PATH_TO_PDF_PAGE_RU
done

echo "# Platform uninstalling" >> $PATH_TO_PDF_PAGE
echo "# Удаление платформы" >> $PATH_TO_PDF_PAGE_RU

unset LIST_OF_PAGES
LIST_OF_PAGES=(
"installing/UNINSTALL.md"
)

for ix in ${!LIST_OF_PAGES[*]}
do
  echo "Preparing page $PATH_TO_PAGES${LIST_OF_PAGES[$ix]}"
  echo "## "$(getname $PATH_TO_PAGES${LIST_OF_PAGES[$ix]}) >> $PATH_TO_PDF_PAGE
  echo "$(gettext $PATH_TO_PAGES${LIST_OF_PAGES[$ix]})" >> $PATH_TO_PDF_PAGE
  RU_PAGE="$(echo $PATH_TO_PAGES${LIST_OF_PAGES[$ix]} | sed 's/\.md$//')_RU.md"
  echo "Preparing page $RU_PAGE"
  echo "## "$(getname $RU_PAGE) >> $PATH_TO_PDF_PAGE_RU
  echo "$(gettext $RU_PAGE) | " >> $PATH_TO_PDF_PAGE_RU
done

unset LIST_OF_PAGES
LIST_OF_PAGES=(
"DECKHOUSE-RELEASE-CHANNELS.md"
)

for ix in ${!LIST_OF_PAGES[*]}
do
  echo "Preparing page $PATH_TO_PAGES${LIST_OF_PAGES[$ix]}"
  echo "# "$(getname $PATH_TO_PAGES${LIST_OF_PAGES[$ix]}) >> $PATH_TO_PDF_PAGE
  echo "$(gettext $PATH_TO_PAGES${LIST_OF_PAGES[$ix]})" >> $PATH_TO_PDF_PAGE
  RU_PAGE="$(echo $PATH_TO_PAGES${LIST_OF_PAGES[$ix]} | sed 's/\.md$//')_RU.md"
  echo "Preparing page $RU_PAGE"
  echo "# "$(getname $RU_PAGE) >> $PATH_TO_PDF_PAGE_RU
  echo "$(gettext $RU_PAGE) | " >> $PATH_TO_PDF_PAGE_RU
done

unset LIST_OF_PAGES
LIST_OF_PAGES=(
"DECKHOUSE_ALERTS.md"
"NETWORK_SECURITY_SETUP.md"
"DECKHOUSE-FAQ.md"
)

for ix in ${!LIST_OF_PAGES[*]}
do
  echo "Preparing page $PATH_TO_PAGES${LIST_OF_PAGES[$ix]}"
  echo "# "$(getname $PATH_TO_PAGES${LIST_OF_PAGES[$ix]}) >> $PATH_TO_PDF_PAGE
  echo "$(gettext $PATH_TO_PAGES${LIST_OF_PAGES[$ix]})" >> $PATH_TO_PDF_PAGE
  RU_PAGE="$(echo $PATH_TO_PAGES${LIST_OF_PAGES[$ix]} | sed 's/\.md$//')_RU.md"
  echo "Preparing page $RU_PAGE"
  echo "# "$(getname $RU_PAGE) >> $PATH_TO_PDF_PAGE_RU
  echo "$(gettext $RU_PAGE) | " >> $PATH_TO_PDF_PAGE_RU
done

echo "# Модули" >> $PATH_TO_PDF_PAGE_RU
echo "# Modules" >> $PATH_TO_PDF_PAGE

unset LIST_OF_MODULES
LIST_OF_MODULES=(
"deckhouse"
"documentation"
"flow-schema"
"admission-policy-engine"
"cni-cilium"
"cloud-provider-aws"
"cloud-provider-azure"
"cloud-provider-gcp"
"cloud-provider-yandex"
"ceph-csi"
"local-path-provisioner"
"cni-flannel"
"cni-simple-bridge"
"kube-proxy"
"registry-packages-proxy"
"control-plane-manager"
"node-manager"
"terraform-manager"
"kube-dns"
"snapshot-controller"
"network-policy-engine"
"cert-manager"
"user-authz"
"multitenancy-manager"
"operator-prometheus"
"prometheus-metrics-adapter"
"vertical-pod-autoscaler"
"prometheus-pushgateway"
"extended-monitoring"
"monitoring-custom"
"monitoring-deckhouse"
"monitoring-kubernetes"
"monitoring-kubernetes-control-plane"
"monitoring-ping"
"descheduler"
"ingress-nginx"
"loki"
"pod-reloader"
"chrony"
"cilium-hubble"
"dashboard"
"okmeter"
"openvpn"
"upmeter"
"namespace-configurator"
"secret-copier"
"deckhouse-tools"
)

for ix in ${!LIST_OF_MODULES[*]}
do
  MODULE_PATH=$(find ${PATH_TO_MODULES} -maxdepth 1 -type d -name "*${LIST_OF_MODULES[$ix]}" -print | sed 's|.*/||' )
  files=$(find "${PATH_TO_MODULES}/${MODULE_PATH}" -maxdepth 2 -name "*.md" | sort -t '-' -k2)
      for ixp in ${!PAGES_ORDER[*]}
      do
        for file in $files
        do
          if [[ $file == *"${PAGES_ORDER[ixp]}"* ]]; then
            echo "Preparing page $file"
            echo "### "$(getname $file) >> $PATH_TO_PDF_PAGE
            if [[ $file == *""CONFIGURATION.md""* ]]; then
              schema_path="${PATH_TO_MODULES}/${MODULE_PATH}/crds"
              module_path=$(echo $schema_path | cut -d\/ -f-2 )
              module_file_name=$(echo $schema_path | awk -F\/ '{print $NF}')
              module_name=$(echo $schema_path | cut -d\/ -f2 | sed 's/^[0-9]*-*//')
              schema_path_relative=$(echo $schema_path | cut -d\/ -f3- | sed "s#\.yaml##; s#\.##g; s#\/#\.#g")
              echo "$(gettext $file)" | sed "/<!-- SCHEMA -->/i\ " >> $PATH_TO_PDF_PAGE
              echo "#### {{ site.data.i18n.common['parameters'][page.lang] }}" >> $PATH_TO_PDF_PAGE
              echo "{{ site.data.schemas['${module_name}'].config-values | format_module_configuration: moduleKebabName }}" >> $PATH_TO_PDF_PAGE
            elif [[ $file == *""CR.md""* ]]; then
              for schema_path in $(find "modules/$MODULE_PATH" -regex '^.*/crds/.*.yaml$' -print | sort); do
                if [[ $schema_path == *"doc-ru"* ]]; then
                  continue
                fi
                module_path=$(echo $schema_path | cut -d\/ -f-2 )
                module_file_name=$(echo $schema_path | awk -F\/ '{print $NF}')
                module_name=$(echo $schema_path | cut -d\/ -f2 | sed 's/^[0-9]*-*//')
                schema_path_relative=$(echo $schema_path | cut -d\/ -f3- | sed "s#\.yaml##; s#\.##g; s#\/#\.#g")
                echo "{{ site.data.schemas.${module_name}.${schema_path_relative} | format_crd: \"${module_name}\" }}" >> $PATH_TO_PDF_PAGE
              done
            else
              echo "$(gettext $file)" >> $PATH_TO_PDF_PAGE
            fi

            unset RU_PAGE
            RU_PAGE="$(echo $file | sed 's/\.md$//')_RU.md"
            echo "Preparing page $RU_PAGE"
            echo "### "$(getname $RU_PAGE) >> $PATH_TO_PDF_PAGE_RU
            if [[ $RU_PAGE == *""CONFIGURATION_RU.md""* ]]; then
              schema_path="${PATH_TO_MODULES}/${MODULE_PATH}/crds"
              module_path=$(echo $schema_path | cut -d\/ -f-2 )
              module_file_name=$(echo $schema_path | awk -F\/ '{print $NF}')
              module_name=$(echo $schema_path | cut -d\/ -f2 | sed 's/^[0-9]*-*//')
              schema_path_relative=$(echo $schema_path | cut -d\/ -f3- | sed "s#\.yaml##; s#\.##g; s#\/#\.#g")
              echo "$(gettext $RU_PAGE)" | sed "/<!-- SCHEMA -->/i\ " >> $PATH_TO_PDF_PAGE
              echo "#### {{ site.data.i18n.common['parameters'][page.lang] }}" >> $PATH_TO_PDF_PAGE
              echo "{{ site.data.schemas['${module_name}'].config-values | format_module_configuration: moduleKebabName }}" >> $PATH_TO_PDF_PAGE_RU
            elif [[ $RU_PAGE == *""CR_RU.md""* ]]; then
              for schema_path in $(find "modules/$MODULE_PATH" -regex '^.*/crds/.*.yaml$' -print | grep -v '/crds/doc-ru-'| sort); do
                module_path=$(echo $schema_path | cut -d\/ -f-2 )
                module_file_name=$(echo $schema_path | awk -F\/ '{print $NF}')
                module_name=$(echo $schema_path | cut -d\/ -f2 | sed 's/^[0-9]*-*//')
                schema_path_relative=$(echo $schema_path | cut -d\/ -f3- | sed "s#\.yaml##; s#\.##g; s#\/#\.#g")
                echo "{{ site.data.schemas.${module_name}.${schema_path_relative} | format_crd: \"${module_name}\" }}" >> $PATH_TO_PDF_PAGE_RU
              done
            else
              echo "$(gettext $RU_PAGE)" >> $PATH_TO_PDF_PAGE_RU
            fi
          fi
        done
      done
done

unset PATH_TO_MODULES
PATH_TO_MODULES="ee/modules"

unset LIST_OF_MODULES
LIST_OF_MODULES=(
"admission-policy-engine"
"static-routing-manager"
"cloud-provider-dynamix"
"cloud-provider-huaweicloud"
"cloud-provider-openstack"
"cloud-provider-vcd"
"node-manager"
"terraform-manager"
"metallb"
"keepalived"
"network-gateway"
"operator-trivy"
"service-with-healthchecks"
)

for ix in ${!LIST_OF_MODULES[*]}
do
  MODULE_PATH=$(find ${PATH_TO_MODULES} -maxdepth 1 -type d -name "*${LIST_OF_MODULES[$ix]}" -print | sed 's|.*/||' )
  files=$(find "${PATH_TO_MODULES}/${MODULE_PATH}" -maxdepth 2 -name "*.md" | sort -t '-' -k2)
      for ixp in ${!PAGES_ORDER[*]}
      do
        for file in $files
        do
          if [[ $file == *"${PAGES_ORDER[ixp]}"* ]]; then
            echo "Preparing page $file"
            echo "### "$(getname $file) >> $PATH_TO_PDF_PAGE
            if [[ $file == *""CONFIGURATION.md""* ]]; then
              schema_path="${PATH_TO_MODULES}/${MODULE_PATH}/crds"
              module_path=$(echo $schema_path | cut -d\/ -f-2 )
              module_file_name=$(echo $schema_path | awk -F\/ '{print $NF}')
              module_name=$(echo $schema_path | cut -d\/ -f2 | sed 's/^[0-9]*-*//')
              schema_path_relative=$(echo $schema_path | cut -d\/ -f3- | sed "s#\.yaml##; s#\.##g; s#\/#\.#g")
              echo "$(gettext $file)" | sed "/<!-- SCHEMA -->/i\ " >> $PATH_TO_PDF_PAGE
              echo "#### {{ site.data.i18n.common['parameters'][page.lang] }}" >> $PATH_TO_PDF_PAGE
              echo "{{ site.data.schemas['${module_name}'].config-values | format_module_configuration: moduleKebabName }}" >> $PATH_TO_PDF_PAGE
            elif [[ $file == *""CR.md""* ]]; then
              for schema_path in $(find "modules/$MODULE_PATH" -regex '^.*/crds/.*.yaml$' -print | sort); do
                if [[ $schema_path == *"doc-ru"* ]]; then
                  continue
                fi
                module_path=$(echo $schema_path | cut -d\/ -f-2 )
                module_file_name=$(echo $schema_path | awk -F\/ '{print $NF}')
                module_name=$(echo $schema_path | cut -d\/ -f2 | sed 's/^[0-9]*-*//')
                schema_path_relative=$(echo $schema_path | cut -d\/ -f3- | sed "s#\.yaml##; s#\.##g; s#\/#\.#g")
                echo "{{ site.data.schemas.${module_name}.${schema_path_relative} | format_crd: \"${module_name}\" }}" >> $PATH_TO_PDF_PAGE
              done
            else
              echo "$(gettext $file)" >> $PATH_TO_PDF_PAGE
            fi

            unset RU_PAGE
            RU_PAGE="$(echo $file | sed 's/\.md$//')_RU.md"
            echo "Preparing page $RU_PAGE"
            echo "### "$(getname $RU_PAGE) >> $PATH_TO_PDF_PAGE_RU
            if [[ $RU_PAGE == *""CONFIGURATION_RU.md""* ]]; then
              schema_path="${PATH_TO_MODULES}/${MODULE_PATH}/crds"
              module_path=$(echo $schema_path | cut -d\/ -f-2 )
              module_file_name=$(echo $schema_path | awk -F\/ '{print $NF}')
              module_name=$(echo $schema_path | cut -d\/ -f2 | sed 's/^[0-9]*-*//')
              schema_path_relative=$(echo $schema_path | cut -d\/ -f3- | sed "s#\.yaml##; s#\.##g; s#\/#\.#g")
              echo "$(gettext $RU_PAGE)" | sed "/<!-- SCHEMA -->/i\ " >> $PATH_TO_PDF_PAGE
              echo "#### {{ site.data.i18n.common['parameters'][page.lang] }}" >> $PATH_TO_PDF_PAGE
              echo "{{ site.data.schemas['${module_name}'].config-values | format_module_configuration: moduleKebabName }}" >> $PATH_TO_PDF_PAGE_RU
            elif [[ $RU_PAGE == *""CR_RU.md""* ]]; then
              for schema_path in $(find "modules/$MODULE_PATH" -regex '^.*/crds/.*.yaml$' -print | grep -v '/crds/doc-ru-'| sort); do
                module_path=$(echo $schema_path | cut -d\/ -f-2 )
                module_file_name=$(echo $schema_path | awk -F\/ '{print $NF}')
                module_name=$(echo $schema_path | cut -d\/ -f2 | sed 's/^[0-9]*-*//')
                schema_path_relative=$(echo $schema_path | cut -d\/ -f3- | sed "s#\.yaml##; s#\.##g; s#\/#\.#g")
                echo "{{ site.data.schemas.${module_name}.${schema_path_relative} | format_crd: \"${module_name}\" }}" >> $PATH_TO_PDF_PAGE_RU
              done
            else
              echo "$(gettext $RU_PAGE)" >> $PATH_TO_PDF_PAGE_RU
            fi
          fi
        done
      done
done
