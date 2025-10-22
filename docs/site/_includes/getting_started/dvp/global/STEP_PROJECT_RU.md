<script type="text/javascript" src='{% javascript_asset_tag getting-started %}[_assets/js/getting-started.js]{% endjavascript_asset_tag %}'></script>
<script type="text/javascript" src='{% javascript_asset_tag getting-started-access %}[_assets/js/getting-started-access.js]{% endjavascript_asset_tag %}'></script>
<script type="text/javascript" src='{% javascript_asset_tag bcrypt %}[_assets/js/bcrypt.js]{% endjavascript_asset_tag %}'></script>

Создайте проект. Для создания проекта используйте команду (в примере используется проект `test-project`, измените его, если необходимо):

{% capture includePath %}_includes/getting_started/dvp/{{ page.platform_code }}/partials/project-rbac.yml.inc{% endcapture %}
{% include_file "{{ includePath }}" syntax="yaml" %}

Дождитесь создания пространства имён. Чтобы убедиться в том, что оно создалось, используйте команду:

```shell
d8 k get ns test-project
```

Создайте администратора проекта и свяжите его с ролью `d8:use:role:admin` в созданном ранее пространстве имён.
Для этого используйте команду (в примере используется пользователь `test-user@deckhouse.io`, измените его, если необходимо):

{% capture includePath %}_includes/getting_started/dvp/{{ page.platform_code }}/partials/project-rbac-user.yml.inc{% endcapture %}
{% include_file "{{ includePath }}" syntax="yaml" %}

Откройте веб-интерфейс генерации файла kubeconfig, для удаленного доступа к API-серверу. Адрес веб-интерфейса формируется в соответствии с шаблоном DNS-имен, указанным в глобальном параметре [publicDomainTemplate](/products/kubernetes-platform/documentation/v1/reference/api/global.html#parameters-modules-publicdomaintemplate). Например, если `publicDomainTemplate: %s.kube.my`, то веб-интерфейс будет доступен по адресу `kubeconfig.kube.my`.

Введите логин (в примере — `test-user@deckhouse.io`) и пароль созданного пользователя и получите конфигурационный файл для доступа к кластеру:

На компьютере, имеющем сетевой доступ к развернутому кластеру, создайте файл `~/.kube/config` (для Linux/MacOS) или `%USERPROFILE%\.kube\config` (для Windows) и вставьте в него конфигурацию kubectl, приведенную на вкладке *Raw Config*.

Вы настроили kubectl на этом компьютере для управления кластером. Дальнейшие команды выполняйте на этом компьютере.

Придумайте пароль для пользователя внутри виртуальной машины и сгенерируйте его хэш:

```bash
mkpasswd --method=SHA-512 --rounds=4096
```

Для добавления пользователя и ssh-ключа в виртуальную машину создайте файл `cloud-config`.
В примере по желанию измените поля:

- `name` — содержит имя пользователя `test-user`, замените на своё.
- `passwd` — содержит в кавычках хэш пароля `test-user`, замените на свой хэш.
- `ssh_authorized_keys` — содержит публичный ssh-ключ, сгенерируйте свой и замените.

```bash
#cloud-config
ssh_pwauth: True
users:
- name: test-user
  passwd: '$6$rounds=4096$.ed4Qtpv1WeKmhH6$3ZCZGvv1QIe2bIsEGT549mAPnmCUVLG5TJAVsBr02bhdyKTGPt3HFC9Bc7x/NiGAwAqibIuUpRQk4SltW4Kd//'
  shell: /bin/bash
  sudo: ALL=(ALL) NOPASSWD:ALL
  lock_passwd: False
  ssh_authorized_keys:
    - ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIFxcXHmwaGnJ8scJaEN5RzklBPZpVSic4GdaAsKjQoeA your_email@example.com
packages:
  - qemu-guest-agent
runcmd:
  - systemctl enable qemu-guest-agent --now
  - chown -R cloud:cloud /home/cloud
```

Создайте файл секрета, содержащий `cloud-config` в формате base64.

```yaml
d8 k create -f - <<EOF
apiVersion: v1
data:
  userData: |
    `cat cloud-config | base64 -w0`
kind: Secret
metadata:
  name: secret-cloud-init
  namespace: test-project
type: provisioning.virtualization.deckhouse.io/cloud-init
EOF
```

Создайте виртуальную машину:

```yaml
d8 k create -f - <<EOF
---
apiVersion: virtualization.deckhouse.io/v1alpha2
kind: VirtualImage
metadata:
  name: ubuntu-2204
  namespace: test-project
spec:
  storage: ContainerRegistry
  dataSource:
    type: HTTP
    http:
      url: https://cloud-images.ubuntu.com/jammy/current/jammy-server-cloudimg-amd64.img
---
apiVersion: virtualization.deckhouse.io/v1alpha2
kind: VirtualDisk
metadata:
  name: disk
  namespace: test-project
spec:
  dataSource:
    objectRef:
      kind: VirtualImage
      name: ubuntu-2204
    type: ObjectRef
  persistentVolumeClaim:
    size: 4G
---
apiVersion: virtualization.deckhouse.io/v1alpha2
kind: VirtualMachine
metadata:
  name: vm
  namespace: test-project
spec:
  provisioning:
    type: UserDataRef
    userDataRef:
      kind: Secret
      name: secret-cloud-init
  virtualMachineClassName: generic
  runPolicy: AlwaysOn
  blockDeviceRefs:
  - kind: VirtualDisk
    name: disk
  cpu:
    cores: 1
  memory:
    size: 1Gi
EOF
```

Выведите список виртуальных машин, чтобы увидеть статус виртуальной машины:

```shell
d8 k get vm -o wide
```

После успешного старта виртуальная машина должна перейти в статус `Running`.

Пример вывода:

```console
NAME   PHASE     CORES   COREFRACTION   MEMORY   NEED RESTART   AGENT   MIGRATABLE   NODE           IPADDRESS     AGE
vm     Running   1       100%           1Gi      False          False   True         virtlab-pt-1   10.66.10.19   6m18s
```

Подключитесь к виртуальной машине, введите логин (в примере — `test-user`) и пароль:

```shell
d8 v console -n test-project vm
```

Для выхода из консоли нажмите `Ctrl+]`.

Поздравляем! Вы создали виртуальную машину и подключились к ней.

<script type="text/javascript">
$(document).ready(function () {
    generate_password(true);
    update_parameter('dhctl-user-password-hash', 'password', '<GENERATED_PASSWORD_HASH>', null, null);
    update_parameter('dhctl-user-password-hash', null, '<GENERATED_PASSWORD_HASH>', null, '[project-rbac-yml]');
    update_parameter('dhctl-user-password', null, '<GENERATED_PASSWORD>', null, '[project-rbac-yml]');
    update_parameter('dhctl-user-password', null, '<GENERATED_PASSWORD>', null, 'code span.c1');
    update_domain_parameters();
    config_highlight();
});

</script>
