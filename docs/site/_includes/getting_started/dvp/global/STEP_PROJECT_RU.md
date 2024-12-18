<script type="text/javascript" src='{{ assets["getting-started.js"].digest_path }}'></script>
<script type="text/javascript" src='{{ assets["getting-started-access.js"].digest_path }}'></script>
<script type="text/javascript" src='{{ assets["bcrypt.js"].digest_path }}'></script>

Создайте проект и администратора проекта (в примере используется проект `test-project` и пользователь `test-user@deckhouse.io`, измените их, если необходимо):

{% snippetcut selector="project-rbac-yml" %}
{% include_file "_includes/getting_started/dvp/{{ page.platform_code }}/partials/project-rbac.yml.inc" syntax="yaml" %}
{% endsnippetcut %}

Откройте веб-интерфейс генерации файла kubeconfig, для удаленного доступа к API-серверу. Адрес веб-интерфейса формируется в соответствии с шаблоном DNS-имен, указанным в глобальном параметре [publicDomainTemplate](/products/virtualization-platform/reference/mc.html#parameters-modules-publicdomaintemplate). Например, если `publicDomainTemplate: %s.kube.my`, то веб-интерфейс будет доступен по адресу `kubeconfig.kube.my`.
 
Введите логин (в примере — `test-user@deckhouse.io`) и пароль созданного пользователя и получите конфигурационный файл для доступа к кластеру:

На компьютере, имеющем сетевой доступ к развернутому кластеру, создайте файл `~/.kube/config` (для Linux/MacOS) или `%USERPROFILE%\.kube\config` (для Windows) и вставьте в него конфигурацию kubectl, приведенную на вкладке *Raw Config*.

Вы настроили kubectl на этом компьютере для управления кластером. Дальнейшие команды выполняйте на этом компьютере. 

Создайте виртуальную машину:

{% snippetcut %}
```yaml
kubectl create -f - <<EOF
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
{% endsnippetcut %}

Выведите список виртуальных машин, чтобы увидеть статус виртуальной машины:

{% snippetcut %}
```shell
kubectl get vm -o wide
```
{% endsnippetcut %}

После успешного старта виртуальная машина должна перейти в статус `Running`.

Пример вывода:
```console
# kubectl get vm -o wide
NAME   PHASE     CORES   COREFRACTION   MEMORY   NEED RESTART   AGENT   MIGRATABLE   NODE           IPADDRESS     AGE
vm     Running   1       100%           1Gi      False          False   True         virtlab-pt-1   10.66.10.19   6m18s
```

Подключитесь к виртуальной машине, введите логин (в примере — `test-user@deckhouse.io`) и пароль:

{% snippetcut %}
```shell
d8 v console -n test-project vm
```
{% endsnippetcut %}

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
