---
title: "Конфигурирование узлов"
permalink: ru/stronghold/documentation/admin/platform-management/node-management/configuration.html
lang: ru
---

{% raw %}

## Пользовательские настройки на узлах

Для автоматизации действий на узлах группы предусмотрен ресурс [NodeGroupConfiguration](/modules/node-manager/cr.html#nodegroupconfiguration). Ресурс позволяет выполнять на узлах bash-скрипты, в которых можно пользоваться набором команд [bashbooster](https://github.com/deckhouse/deckhouse/tree/main/candi/bashible/bashbooster), а также позволяет использовать шаблонизатор [Go Template](https://pkg.go.dev/text/template). Это удобно для автоматизации таких операций, как:

- Установка и настройки дополнительных пакетов ОС.

  Пример:
  - [установка kubectl-плагина](./os.html#установка-плагина-cert-manager-для-kubectl-на-master-узлах).

- Обновление ядра ОС на конкретную версию.

  Примеры:
  - [обновление ядра Debian](./os.html#для-дистрибутивов-основанных-на-debian).
  - [обновление ядра CentOS](./os.html#для-дистрибутивов-основанных-на-centos).

- Изменение параметров ОС.

  Примеры:
  - [настройка параметра sysctl](./os.html#задание-параметра-sysctl).
  - [добавление корневого сертификата](./os.html#добавление-корневого-сертификата).

- Сбор информации на узле и выполнение других подобных действий.

- Настройка containerd.

  Примеры:
  - [настройка метрик](./containerd.html#дополнительные-настройки-containerd).
  - [добавление приватного registry](./containerd.html#добавление-приватного-registry-с-авторизацией).

## Настройки NodeGroupConfiguration

Ресурс NodeGroupConfiguration позволяет указывать [приоритет](/modules/node-manager/cr.html#nodegroupconfiguration-v1alpha1-spec-weight) выполняемым скриптам, ограничивать их выполнение определенными [группами узлов](/modules/node-manager/cr.html#nodegroupconfiguration-v1alpha1-spec-nodegroups) и [типами ОС](/modules/node-manager/cr.html#nodegroupconfiguration-v1alpha1-spec-bundles).

Код скрипта указывается в параметре [`content`](/modules/node-manager/cr.html#nodegroupconfiguration-v1alpha1-spec-content) ресурса. При создании скрипта на узле содержимое параметра `content` проходит через шаблонизатор [Go Template](https://pkg.go.dev/text/template), который позволят встроить дополнительный уровень логики при генерации скрипта. При прохождении через шаблонизатор становится доступным контекст с набором динамических переменных.

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
      imageName: ubuntu-22-04-cloud-amd64
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
  kubernetesVersion: "1.27"
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

Пример использования команд bashbooster:

```shell
bb-event-on 'bb-package-installed' 'post-install'
post-install() {
  bb-log-info "Setting reboot flag due to kernel was updated"
  bb-flag-set reboot
}
```

{% endraw %}

Ход выполнения скриптов можно увидеть на узле в журнале сервиса bashible c помощью команды:

```bash
journalctl -u bashible.service
```  

Сами скрипты находятся на узле в директории `/var/lib/bashible/bundle_steps/`.

Сервис принимает решение о повторном запуске скриптов путем сравнения единой контрольной суммы всех файлов, расположенной по пути `/var/lib/bashible/configuration_checksum` с контрольной суммой размещенной в кластере Kubernetes в секрете `configuration-checksums` пространства имен `d8-cloud-instance-manager`.

Проверить контрольную сумму можно следующей командой:

```bash
d8 k -n d8-cloud-instance-manager get secret configuration-checksums -o yaml
```  

Сравнение контрольных сумм сервис совершает каждую минуту.
Сравнение контрольных сумм сервис совершает каждую минуту.

Контрольная сумма в кластере изменяется раз в 4 часа, тем самым повторно запуская скрипты на всех узлах.  
Принудительный вызов исполнения bashible на узле можно произвести путем удаления файла с контрольной суммой скриптов с помощью следующей команды:

```bash
rm /var/lib/bashible/configuration_checksum
```  

### Особенности написания скриптов

При написании скриптов важно учитывать следующие особенности их использования в Deckhouse:

1. Скрипты в deckhouse выполняются раз в 4 часа или на основании внешних триггеров. Поэтому важно писать скрипты таким образом, чтобы они производили проверку необходимости своих изменений в системе перед выполнением действий, а не производили изменения каждый раз при запуске.
1. При выборе [приоритета](/modules/node-manager/cr.html#nodegroupconfiguration-v1alpha1-spec-weight) пользовательских скриптов важно учитывать [встроенные скрипты](https://github.com/deckhouse/deckhouse/tree/main/candi/bashible/common-steps/all) которые производят различные действия в т.ч. установку и настройку сервисов. Например, если в скрипте планируется произвести перезапуск сервиса, а сервис устанавливается встроенным скриптом с приоритетом N, то приоритет пользовательского скрипта должен быть как минимум N+1, иначе, при развертывании нового узла пользовательский скрипт выйдет с ошибкой.

{% endraw %}
