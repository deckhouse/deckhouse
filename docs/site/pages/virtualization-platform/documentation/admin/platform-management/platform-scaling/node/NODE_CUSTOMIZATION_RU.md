---
title: "Пользовательские настройки на узлах"
permalink: ru/virtualization-platform/documentation/admin/platform-management/platform-scaling/node/node-customization.html
lang: ru
---

Для автоматизации действий на узлах группы предусмотрен ресурс [NodeGroupConfiguration](/modules/node-manager/cr.html#nodegroupconfiguration). С его помощью можно выполнять на узлах bash-скрипты, используя набор команд [Bash Booster](https://github.com/deckhouse/deckhouse/tree/main/candi/bashible/bashbooster), а также применять шаблонизатор [Go Template](https://pkg.go.dev/text/template). Это удобно для автоматизации таких операций, как:

- Установка и настройки дополнительных пакетов ОС.  

  Примеры:  
  - [установка kubectl-плагина](node-management.html#установка-плагина-cert-manager-для-kubectl-на-master-узлах);

- Обновление ядра ОС на конкретную версию.

- Изменение параметров ОС.

- Сбор информации на узле и выполнение других подобных действий.

Ресурс NodeGroupConfiguration позволяет указывать приоритет выполняемым скриптам, ограничивать их выполнение определенными группами узлов и типами ОС.

Код скрипта указывается в параметре `content` ресурса. При создании скрипта на узле содержимое параметра `content` проходит через шаблонизатор [Go Template](https://pkg.go.dev/text/template), который позволят встроить дополнительный уровень логики при генерации скрипта. При прохождении через шаблонизатор становится доступным контекст с набором динамических переменных.

Переменные, которые доступны для использования в шаблонизаторе:
<ul>
<li><code>.cloudProvider</code> (для групп узлов с nodeType <code>CloudEphemeral</code> или <code>CloudPermanent</code>) — массив данных облачного провайдера.
{% offtopic title="Пример данных..." %}
```yaml
cloudProvider:
  instanceClassKind: OpenStackInstanceClass
  machineClassKind: OpenStackMachineClass
  openstack:
    connection:
      authURL: https://cloud.provider.com/v3/
      domainName: Default
      password: p@ssw0rd
      region: region2
      tenantName: mytenantname
      username: mytenantusername
    externalNetworkNames:
    - public
    instances:
      imageName: ubuntu-24-04-cloud-amd64
      mainNetwork: kube
      securityGroups:
      - kube
      sshKeyPairName: kube
    internalNetworkNames:
    - kube
    podNetworkMode: DirectRoutingWithPortSecurityEnabled
  region: region2
  type: openstack
  zones:
  - nova
```
{% endofftopic %}</li>
<li><code>.cri</code> — используемый CRI (с версии Deckhouse 1.49 используется только <code>Containerd</code>).</li>
<li><code>.kubernetesVersion</code> — используемая версия Kubernetes.</li>
<li><code>.nodeUsers</code> — массив данных о пользователях узла, добавленных через ресурс <a href="/modules/node-manager/cr.html#nodeuser">NodeUser</a>.
{% offtopic title="Пример данных..." %}
```yaml
nodeUsers:
- name: user1
  spec:
    isSudoer: true
    nodeGroups:
    - '*'
    passwordHash: PASSWORD_HASH
    sshPublicKey: SSH_PUBLIC_KEY
    uid: 1050
```
{% endofftopic %}
</li>
<li><code>.nodeGroup</code> — массив данных группы узлов.
{% offtopic title="Пример данных..." %}
```yaml
nodeGroup:
  cri:
    type: Containerd
  disruptions:
    approvalMode: Automatic
  kubelet:
    containerLogMaxFiles: 4
    containerLogMaxSize: 50Mi
    resourceReservation:
      mode: "Off"
  kubernetesVersion: "1.31"
  manualRolloutID: ""
  name: master
  nodeTemplate:
    labels:
      node-role.kubernetes.io/control-plane: ""
      node-role.kubernetes.io/master: ""
    taints:
    - effect: NoSchedule
      key: node-role.kubernetes.io/master
  nodeType: CloudPermanent
  updateEpoch: "1699879470"
```
{% endofftopic %}</li>
</ul>

{% raw %}
Пример использования переменных в шаблонизаторе:

```shell
{{- range .nodeUsers }}
echo 'Tuning environment for user {{ .name }}'
# Some code for tuning user environment
{{- end }}
```

Пример использования команд Bash Booster:

```shell
bb-event-on 'bb-package-installed' 'post-install'
post-install() {
  bb-log-info "Setting reboot flag due to kernel was updated"
  bb-flag-set reboot
}
```

{% endraw %}

## Мониторинг выполнения скриптов

Ход выполнения скриптов можно увидеть на узле в журнале сервиса `bashible` c помощью команды:

```bash
journalctl -u bashible.service
```  

Сами скрипты находятся на узле в директории `/var/lib/bashible/bundle_steps/`.  

## Механизм повторного запуска скриптов

Сервис принимает решение о повторном запуске скриптов путем сравнения единой контрольной суммы всех файлов, расположенной по пути `/var/lib/bashible/configuration_checksum` с контрольной суммой размещенной в кластере Kubernetes в секрете `configuration-checksums` пространства имён `d8-cloud-instance-manager`.

Проверить контрольную сумму можно следующей командой:  

```bash
d8 k -n d8-cloud-instance-manager get secret configuration-checksums -o yaml
```  

Сравнение контрольных сумм сервис совершает каждую минуту.  

Контрольная сумма в кластере изменяется раз в 4 часа, тем самым повторно запуская скрипты на всех узлах.  

Принудительный запуск скриптов `bashible` на узле можно выполнить, удалив файл контрольной суммы с помощью следующей команды:

```bash
rm /var/lib/bashible/configuration_checksum
```  

## Особенности написания скриптов

При написании скриптов важно учитывать следующие особенности их использования в виртуализации Deckhouse Platform:

1. Скрипты в виртуализации Deckhouse Platform выполняются раз в 4 часа или на основании внешних триггеров. Поэтому важно писать скрипты таким образом, чтобы они производили предварительную проверку необходимости внесения изменений, чтобы избежать повторяющихся/ненужных действий при каждом запуске.
1. Существуют [предзаготовленные скрипты](https://github.com/deckhouse/deckhouse/tree/main/candi/bashible/common-steps/all), выполняющие различные действия, в том числе установку и настройку сервисов. Важно учитывать это при выборе приоритета пользовательских скриптов. Например, если пользовательский скрипт выполняет перезапуск сервиса, он должен запускаться после скрипта, отвечающего за установку этого сервиса. В противном случае пользовательский скрипт не сможет выполниться при первичном развёртывании узла (так как сервис ещё не будет установлен).

Полезные особенности некоторых скриптов:

- [`032_configure_containerd.sh`](https://github.com/deckhouse/deckhouse/blob/main/candi/bashible/common-steps/all/032_configure_containerd.sh.tpl) — объединяет конфигурационные файлы containerd и **перезапускает** сервис. Размещение конфигурационных файлов зависит от CRI на узлах кластера:

  - `/etc/containerd/conf.d/*.toml` — для containerd v1;
  - `/etc/containerd/conf2.d/*.toml` — для containerd v2.
  
  Следует учитывать, что директории с конфигурационными файлами не создаются автоматически. Файлы в них нужно добавлять в скриптах с приоритетом менее `32`.

## Как использовать containerd с поддержкой Nvidia GPU

На GPU-узлах containerd настраивает модуль [`gpu`](/modules/gpu/): он добавляет runtime `nvidia` и нужные системные настройки на узлы групп с секцией [`spec.gpu`](/modules/node-manager/cr.html#nodegroup-v1-spec-gpu). Не настраивайте runtime `nvidia` своими ресурсами NodeGroupConfiguration: такая настройка конфликтует с настройкой модуля `gpu`.

Как установить драйвер NVIDIA и NVIDIA Container Toolkit на узлы и как проверить GPU-узел, описано в [FAQ модуля `gpu`](/modules/gpu/faq.html).

## Как развернуть кастомный конфигурационный файл containerd

{% alert level="danger" %}
Добавление кастомных настроек вызывает перезапуск сервиса `containerd`.
{% endalert %}

Bashible на узлах объединяет конфигурацию containerd для виртуализация Deckhouse Platform с конфигурацией из файла:

- `/etc/containerd/conf.d/*.toml` — если в качестве CRI на узлах кластера используется containerd v1;
- `/etc/containerd/conf2.d/*.toml` — если в качестве CRI на узлах кластера используется containerd v2.

{% alert level="warning" %}
Вы можете переопределять значения параметров, которые заданы в файле `/etc/containerd/deckhouse.toml`. При этом корректную работу таких изменений необходимо обеспечить самостоятельно. Рекомендуется **не изменять** конфигурацию на управляющих (master) узлах (NodeGroup `master`).
{% endalert %}

Далее приведены примеры конфигурации ресурсов NodeGroupConfiguration, добавляющих кастомный конфигурационный файл для соответствующей версии containerd.

{% tabs containerd_version %}
{% tab "Для containerd v1" %}
{% raw %}

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroupConfiguration
metadata:
  name: containerd-option-config.sh
spec:
  bundles:
    - '*'
  content: |
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

    mkdir -p /etc/containerd/conf.d
    bb-sync-file /etc/containerd/conf.d/additional_option.toml - << EOF
    oom_score = 500
    [metrics]
    address = "127.0.0.1"
    grpc_histogram = true
    EOF
  nodeGroups:
    - "worker"
  weight: 31
```

{% endraw %}
{% endtab %}
{% tab "Для containerd v2" %}
{% raw %}

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroupConfiguration
metadata:
  name: containerd-option-config.sh
spec:
  bundles:
    - '*'
  content: |
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

    mkdir -p /etc/containerd/conf2.d
    bb-sync-file /etc/containerd/conf2.d/runtimeclass.toml - << "EOF"
    [plugins."io.containerd.cri.v1.runtime".containerd.runtimes.gpu-large-shm]
      runtime_type = "io.containerd.runc.v2"
      [plugins."io.containerd.cri.v1.runtime".containerd.runtimes.gpu-large-shm.options]
        BinaryName = ""
        SystemdCgroup = true
        ShmSize = 17179869184
    EOF
  nodeGroups:
    - "worker"
  weight: 31
```

{% endraw %}
{% endtab %}
{% endtabs %}

## Добавление конфигурации для дополнительного registry

В containerd существует два способа описания конфигурации registry: **устаревший** и **актуальный**.

Для проверки наличия **устаревшего** способа конфигурации выполните на узлах кластера следующие команды:

```bash
cat /etc/containerd/config.toml | grep 'plugins."io.containerd.grpc.v1.cri".registry.mirrors'
cat /etc/containerd/config.toml | grep 'plugins."io.containerd.grpc.v1.cri".registry.configs'
```

Пример вывода:

```console
[plugins."io.containerd.grpc.v1.cri".registry.mirrors]
  [plugins."io.containerd.grpc.v1.cri".registry.mirrors."<REGISTRY_URL>"]
[plugins."io.containerd.grpc.v1.cri".registry.configs]
  [plugins."io.containerd.grpc.v1.cri".registry.configs."<REGISTRY_URL>".auth]
```

Для проверки наличия **актуального** способа конфигурации выполните на узлах кластера следующую команду:

```bash
cat /etc/containerd/config.toml | grep '/etc/containerd/registry.d'
```

Пример вывода:

```console
config_path = "/etc/containerd/registry.d"
```

{% alert level="warning" %}
Больше примеров конфигураций для дополнительного хранилища образов (например, развёртывание кастомного конфигурационного файла containerd) доступно в разделе [«FAQ»](/modules/node-manager/faq.html#как-развернуть-кастомный-конфигурационный-файл-containerd) документации модуля `node-manager`.
{% endalert %}

### Устаревший способ добавления конфигурации для дополнительного registry

{% alert level="warning" %}
Этот формат конфигурации containerd устарел (deprecated).
{% endalert %}

{% alert level="info" %}
Используется в containerd v1, если Deckhouse Platform не управляется с помощью модуля [registry](/modules/registry/).
{% endalert %}

Конфигурация описывается в основном конфигурационном файле containerd `/etc/containerd/config.toml`.

Пользовательская конфигурация добавляется через механизм `toml merge`. Конфигурационные файлы из директории `/etc/containerd/conf.d` объединяются с основным файлом `/etc/containerd/config.toml`. Применение merge происходит на этапе выполнения скрипта `032_configure_containerd.sh`, поэтому соответствующие файлы должны быть добавлены заранее.

Пример конфигурационного файла для директории `/etc/containerd/conf.d/`:

```toml
[plugins]
  [plugins."io.containerd.grpc.v1.cri"]
    [plugins."io.containerd.grpc.v1.cri".registry]
      [plugins."io.containerd.grpc.v1.cri".registry.mirrors]
        [plugins."io.containerd.grpc.v1.cri".registry.mirrors."${REGISTRY_URL}"]
          endpoint = ["https://${REGISTRY_URL}"]
      [plugins."io.containerd.grpc.v1.cri".registry.configs]
        [plugins."io.containerd.grpc.v1.cri".registry.configs."${REGISTRY_URL}".auth]
          auth = "${BASE_64_AUTH}"
          username = "${USERNAME}"
          password = "${PASSWORD}"
        [plugins."io.containerd.grpc.v1.cri".registry.configs."${REGISTRY_URL}".tls]
          ca_file = "${CERT_DIR}/${CERT_NAME}.crt"
          insecure_skip_verify = true
```

{% alert level="danger" %}
Добавление кастомных настроек через механизм `toml merge` вызывает перезапуск сервиса containerd.
{% endalert %}

#### Добавление авторизации в дополнительный registry (устаревший способ)

Пример добавления авторизации в дополнительный registry при использовании **устаревшего** способа конфигурации:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroupConfiguration
metadata:
  name: containerd-additional-config-auth.sh
spec:
  # Для добавления файла перед шагом '032_configure_containerd.sh'
  weight: 31
  bundles:
    - '*'
  nodeGroups:
    - "*"
  content: |
    # Copyright 2023 Flant JSC
    #
      [plugins."io.containerd.grpc.v1.cri"]
        [plugins."io.containerd.grpc.v1.cri".registry]
          [plugins."io.containerd.grpc.v1.cri".registry.mirrors]
            [plugins."io.containerd.grpc.v1.cri".registry.mirrors."${REGISTRY_URL}"]
              endpoint = ["https://${REGISTRY_URL}"]
          [plugins."io.containerd.grpc.v1.cri".registry.configs]
            [plugins."io.containerd.grpc.v1.cri".registry.configs."${REGISTRY_URL}".auth]
              username = "username"
              password = "password"
              # OR
              auth = "dXNlcm5hbWU6cGFzc3dvcmQ="
    EOF
```

#### Настройка сертификата для дополнительного registry (устаревший способ)

Пример настройки сертификата для дополнительного registry при использовании **устаревшего** способа конфигурации:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroupConfiguration
metadata:
  name: containerd-additional-config-tls.sh
spec:
  # Для добавления файла перед шагом '032_configure_containerd.sh'
  weight: 31
  bundles:
    - '*'
  nodeGroups:
    - "*"
  content: |
    # Copyright 2023 Flant JSC
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
    
    REGISTRY_URL=private.registry.example
    CERT_FILE_NAME=${REGISTRY_URL}
    CERTS_FOLDER="/var/lib/containerd/certs/"


    mkdir -p ${CERTS_FOLDER}
    bb-sync-file "${CERTS_FOLDER}/${CERT_FILE_NAME}.crt" - << EOF
    -----BEGIN CERTIFICATE-----
    ...
    -----END CERTIFICATE-----
    EOF

    mkdir -p /etc/containerd/conf.d
    bb-sync-file /etc/containerd/conf.d/additional_registry.toml - << EOF
    [plugins]
      [plugins."io.containerd.grpc.v1.cri"]
        [plugins."io.containerd.grpc.v1.cri".registry]
          [plugins."io.containerd.grpc.v1.cri".registry.configs]
            [plugins."io.containerd.grpc.v1.cri".registry.configs."${REGISTRY_URL}".tls]
              ca_file = "${CERTS_FOLDER}/${CERT_FILE_NAME}.crt"
    EOF
```

{% alert level="info" %}
Помимо containerd, сертификат можно добавить в операционную систему.
{% endalert %}

#### Добавление TLS skip verify (устаревший способ)

Пример добавления TLS skip verify при использовании **устаревшего** способа конфигурации:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroupConfiguration
metadata:
  name: containerd-additional-config-skip-tls.sh
spec:
  # Для добавления файла перед шагом '032_configure_containerd.sh'
  weight: 31
  bundles:
    - '*'
  nodeGroups:
    - "*"
  content: |
    # Copyright 2023 Flant JSC
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
    
    REGISTRY_URL=private.registry.example

    mkdir -p /etc/containerd/conf.d
    bb-sync-file /etc/containerd/conf.d/additional_registry.toml - << EOF
    [plugins]
      [plugins."io.containerd.grpc.v1.cri"]
        [plugins."io.containerd.grpc.v1.cri".registry]
          [plugins."io.containerd.grpc.v1.cri".registry.configs]
            [plugins."io.containerd.grpc.v1.cri".registry.configs."${REGISTRY_URL}".tls]
              insecure_skip_verify = true
    EOF
```

После применения конфигурационного файла проверьте доступ к registry с узлов, используя команду:

```bash
crictl pull private.registry.example/image/repo:tag
```

#### Настройка зеркала для доступа к публичным registries (устаревший способ)

Пример настройки зеркала к публичным registries при использовании **устаревшего** способа конфигурации:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroupConfiguration
metadata:
  name: mirror-to-harbor.sh
spec:
  weight: 31
  bundles:
    - '*'
  nodeGroups:
    - "*"
  content: |
    # Copyright 2023 Flant JSC
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

    sed -i '/endpoint = \["https:\/\/registry-1.docker.io"\]/d' /var/lib/bashible/bundle_steps/032_configure_containerd.sh
    mkdir -p /etc/containerd/conf.d
    bb-sync-file /etc/containerd/conf.d/mirror-to-harbor.toml - << "EOF"
    [plugins]
      [plugins."io.containerd.grpc.v1.cri"]
        [plugins."io.containerd.grpc.v1.cri".registry]
          [plugins."io.containerd.grpc.v1.cri".registry.mirrors]
            [plugins."io.containerd.grpc.v1.cri".registry.mirrors."docker.io"]
              endpoint = ["https://registry.private.network/v2/dockerhub-proxy/"]
            [plugins."io.containerd.grpc.v1.cri".registry.mirrors."gcr.io"]
              endpoint = ["https://registry.private.network/v2/YOUR_GCR_PROXY_REPO/"]
    EOF
```

### Новый способ добавления конфигурации для дополнительного registry

{% alert level="info" %}
Используется в containerd v2.  

Используется в containerd v1, если управление осуществляется через модуль [`registry`](/modules/registry/) (например, в режиме [`Direct`](/modules/deckhouse/configuration.html#parameters-registry)).
{% endalert %}

Конфигурация описывается в каталоге `/etc/containerd/registry.d` и задаётся через создание подкаталогов с именами, соответствующими адресу registry:

```bash
/etc/containerd/registry.d
├── private.registry.example:5001
│   ├── ca.crt
│   └── hosts.toml
└── registry.deckhouse.ru
    ├── ca.crt
    └── hosts.toml
```

Пример содержимого файла `hosts.toml`:

```toml
[host]
  # Mirror 1.
  [host."https://${REGISTRY_URL_1}"]
    capabilities = ["pull", "resolve"]
    ca = ["${CERT_DIR}/${CERT_NAME}.crt"]

    [host."https://${REGISTRY_URL_1}".auth]
      username = "${USERNAME}"
      password = "${PASSWORD}"

  # Mirror 2.
  [host."http://${REGISTRY_URL_2}"]
    capabilities = ["pull", "resolve"]
    skip_verify = true
```

{% alert level="info" %}
Изменения конфигураций не приводят к перезапуску сервиса containerd.
{% endalert %}

#### Добавление авторизации в дополнительный registry (актуальный способ)

Пример добавления авторизации в дополнительный registry при использовании **актуального** способа конфигурации:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroupConfiguration
metadata:
  name: containerd-additional-config-auth.sh
spec:
  # Шаг может быть любой, так как не требуется перезапуск сервиса containerd.
  weight: 0
  bundles:
    - '*'
  nodeGroups:
    - "*"
  content: |
    # Copyright 2023 Flant JSC
    #
    # Licensed under the Apache License, Version 2.0 (the "License");
    # you may not use this file except in compliance with the License.
    # WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
    # See the License for the specific language governing permissions and
    # limitations under the License.
    
    REGISTRY_URL=private.registry.example

    mkdir -p "/etc/containerd/registry.d/${REGISTRY_URL}"
    bb-sync-file "/etc/containerd/registry.d/${REGISTRY_URL}/hosts.toml" - << EOF
    [host]
      [host."https://${REGISTRY_URL}"]
        capabilities = ["pull", "resolve"]
        [host."https://${REGISTRY_URL}".auth]
          username = "username"
          password = "password"
    EOF
```

#### Настройка сертификата для дополнительного registry (актуальный способ)

Пример настройки сертификата для дополнительного registry при использовании **актуального** способа конфигурации:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroupConfiguration
metadata:
  name: containerd-additional-config-tls.sh
spec:
  # Шаг может быть любой, так как не требуется перезапуск сервиса containerd.
  weight: 0
  bundles:
    - '*'
  nodeGroups:
    - "*"
  content: |
    # Copyright 2023 Flant JSC
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
    
    REGISTRY_URL=private.registry.example

    mkdir -p "/etc/containerd/registry.d/${REGISTRY_URL}"

    bb-sync-file "/etc/containerd/registry.d/${REGISTRY_URL}/ca.crt" - << EOF
    -----BEGIN CERTIFICATE-----
    ...
    -----END CERTIFICATE-----
    EOF

    bb-sync-file "/etc/containerd/registry.d/${REGISTRY_URL}/hosts.toml" - << EOF
    [host]
      [host."https://${REGISTRY_URL}"]
        capabilities = ["pull", "resolve"]
        ca = ["/etc/containerd/registry.d/${REGISTRY_URL}/ca.crt"]
    EOF
```

{% alert level="info" %}
Помимо containerd, сертификат можно добавить в операционную систему.
{% endalert %}

#### Как добавить TLS skip verify (актуальный способ)?

Пример добавления TLS skip verify при использовании **актуального** способа конфигурации:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroupConfiguration
metadata:
  name: containerd-additional-config-skip-tls.sh
spec:
  # Шаг может быть любой, так как не требуется перезапуск сервиса containerd.
  weight: 0
  bundles:
    - '*'
  nodeGroups:
    - "*"
  content: |
    # Copyright 2023 Flant JSC
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
    
    REGISTRY_URL=private.registry.example

    mkdir -p "/etc/containerd/registry.d/${REGISTRY_URL}"
    bb-sync-file "/etc/containerd/registry.d/${REGISTRY_URL}/hosts.toml" - << EOF
    [host]
      [host."https://${REGISTRY_URL}"]
        capabilities = ["pull", "resolve"]
        skip_verify = true
    EOF
```

После применения конфигурационного файла проверьте доступ к registry с узлов, используя команды:

- Через cri интерфейс:

  ```bash
  crictl pull private.registry.example/image/repo:tag
  ```

- Через ctr с указанием директории с конфигурациями:

  ```bash
  ctr -n k8s.io images pull --hosts-dir=/etc/containerd/registry.d/ private.registry.example/image/repo:tag
  ```

- Через ctr для http репозитория:

  ```bash
  ctr -n k8s.io images pull --hosts-dir=/etc/containerd/registry.d/ --plain-http private.registry.example/image/repo:tag
  ```

#### Настройка зеркала для доступа к публичным registries (актуальный способ)

Пример настройки зеркала к публичным registries при использовании **актуального** способа конфигурации:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroupConfiguration
metadata:
  name: mirror-to-harbor.sh
spec:
  weight: 31
  bundles:
    - '*'
  nodeGroups:
    - "*"
  content: |
    # Copyright 2023 Flant JSC
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

    REGISTRY1_URL=docker.io
    mkdir -p "/etc/containerd/registry.d/${REGISTRY1_URL}"
    bb-sync-file "/etc/containerd/registry.d/${REGISTRY1_URL}/hosts.toml" - << EOF
    [host."https://registry.private.network/v2/dockerhub-proxy/"]
      capabilities = ["pull", "resolve"]
      override_path = true
    EOF
    REGISTRY2_URL=gcr.io
    mkdir -p "/etc/containerd/registry.d/${REGISTRY2_URL}"
    bb-sync-file "/etc/containerd/registry.d/${REGISTRY2_URL}/hosts.toml" - << EOF
    [host."https://registry.private.network/v2/dockerhub-proxy/"]
      capabilities = ["pull", "resolve"]
      override_path = true
    EOF
```

## Как автоматически проставить на узел кастомные лейблы

1. На узле создайте каталог `/var/lib/node_labels`.

1. Создайте в нём файл или файлы, содержащие необходимые лейблы. Количество файлов может быть любым, как и вложенность подкаталогов, их содержащих.

1. Добавьте в файлы нужные лейблы в формате `key=value`. Например:

   ```console
   example-label=test
   ```

1. Сохраните файлы.

При добавлении узла в кластер указанные в файлах лейблы будут автоматически проставлены на узел.

{% alert level="warning" %}
Обратите внимание, что добавить таким образом лейблы, использующиеся в виртуализации Deckhouse Platform, невозможно. Работать такой метод будет только с кастомными лейблами, не пересекающимися с зарезервированными для Deckhouse.
{% endalert %}
