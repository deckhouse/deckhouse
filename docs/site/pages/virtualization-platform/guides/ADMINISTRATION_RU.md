---
title: "Руководство администратора"
permalink: ru/virtualization-platform/guides/administration.html
lang: ru
---

{% alert level="info" %}
С детальным описанием параметров настройки ресурсов приведенных в данном документе вы можете ознакомится в разделе [Custom Resources](cr.html)
{% endalert %}

## Кластерные образы

`ClusterVirtualImage` используются для хранения образов виртуальных машин.

Образы могут быть следующих видов:

- Образ диска виртуальной машины, который предназначен для тиражирования идентичных дисков виртуальных машин.
- ISO-образ, содержащий файлы для установки ОС. Этот тип образа подключается к виртуальной машине как cdrom.

`ClusterVirtualImage` доступен для всех пространств имен внутри кластера.

Образы могут быть получены из различных источников, таких как HTTP-серверы, на которых расположены файлы образов, или контейнерные реестры (container registries), где образы сохраняются и становятся доступны для скачивания. Также существует возможность загрузить образы напрямую из командной строки, используя утилиту `curl`.

### Создание и использование образа c HTTP-ресурса

1. Создайте `ClusterVirtualImage`:

   ```yaml
   apiVersion: virtualization.deckhouse.io/v1alpha2
   kind: ClusterVirtualImage
   metadata:
     name: ubuntu-img
   spec:
     dataSource:
       type: HTTP
       http:
         url: "https://cloud-images.ubuntu.com/minimal/releases/jammy/release-20230615/ubuntu-22.04-minimal-cloudimg-amd64.img"
   ```

2. Проверьте статус `ClusterVirtualImage` с помощью команды:

   ```bash
   kubectl get clustervirtualimage -o wide

   # NAME          PHASE   CDROM   PROGRESS   STOREDSIZE   UNPACKEDSIZE   REGISTRY URL                                 AGE
   # ubuntu-img    Ready   false   100%       285.9Mi      2.2Gi          dvcr.d8-virtualization.svc/cvi/ubuntu-img    52s
   ```

### Создание и использование образа из container registry

Cформируйте образ для хранения в `container registry`.

Ниже представлен пример создания образа c диском Ubuntu 22.04.

- Загрузите образ локально:

  ```bash
  curl -L https://cloud-images.ubuntu.com/minimal/releases/jammy/release-20230615/ubuntu-22.04-minimal-cloudimg-amd64.img -o ubuntu2204.img
  ```

- Создайте Dockerfile со следующим содержимым:

  ```Dockerfile
  FROM scratch
  COPY ubuntu2204.img /disk/ubuntu2204.img
  ```

- Соберите образ и загрузите его в `container registry`. В качестве `container registry` в примере ниже использован docker.io. для выполнения вам необходимо иметь учетную запись сервиса и настроенное окружение.

  ```bash
  docker build -t docker.io/username/ubuntu2204:latest
  ```

  где `username` — имя пользователя, указанное при регистрации в docker.io.

- Загрузите созданный образ в `container registry` с помощью команды:

  ```bash
  docker push docker.io/username/ubuntu2204:latest
  ```

- Чтобы использовать этот образ, создайте в качестве примера ресурс `ClusterVirtualImage`:

  ```yaml
  apiVersion: virtualization.deckhouse.io/v1alpha2
  kind: ClusterVirtualImage
  metadata:
    name: ubuntu-2204
  spec:
    dataSource:
      type: ContainerImage
      containerImage:
        image: docker.io/username/ubuntu2204:latest
  ```

- Чтобы посмотреть ресурс и его статус, выполните команду:

  ```bash
  kubectl get clustervirtualimage
  ```

### Загрузка образа из командной строки

1. Чтобы загрузить образ из командной строки, предварительно создайте следующий ресурс, как представлено ниже на примере `ClusterVirtualImage`:

   ```yaml
   apiVersion: virtualization.deckhouse.io/v1alpha2
   kind: ClusterVirtualImage
   metadata:
     name: some-image
   spec:
     dataSource:
       type: Upload
   ```

2. После того как ресурс будет создан, проверьте его статус с помощью команды:

   ```bash
   kubectl get clustervirtualimages some-image -o json | jq .status.uploadCommand -r

   > uploadCommand: curl https://virtualization.example.com/upload/dSJSQW0fSOerjH5ziJo4PEWbnZ4q6ffc -T example.iso
   ```

   > ClusterVirtualImage с типом **Upload** ожидает начала загрузки образа 15 минут после создания. По истечении этого срока ресурс перейдет в состояние **Failed**.

3. Загрузите образ Cirros (представлено в качестве примера):

   ```bash
   curl -L http://download.cirros-cloud.net/0.5.1/cirros-0.5.1-x86_64-disk.img -o cirros.img
   ```

4. Выполните загрузку образа:

   ```bash
   curl https://virtualization.example.com/upload/dSJSQW0fSOerjH5ziJo4PEWbnZ4q6ffc -T cirros.img
   ```

   После завершения работы команды `curl` образ должен быть создан.

4. Проверьте, что статус созданного образа `Ready`:

   ```bash
   kubectl get clustervirtualimages -o wide

   # NAME          PHASE   CDROM   PROGRESS   STOREDSIZE   UNPACKEDSIZE   REGISTRY URL                                 AGE
   # some-image    Ready   false   100%       285.9Mi      2.2Gi          dvcr.d8-virtualization.svc/cvi/some-image    2m21s
   ```

## Классы виртуальных машин

Ресурс `VirtualMachineClass` предназначен для централизованной конфигурации предпочтительных параметров виртуальных машин. Он позволяет определять инструкции CPU и политики конфигурации ресурсов CPU и памяти для виртуальных машин, а также определять соотношения этих ресурсов. Помимо этого, `VirtualMachineClass` обеспечивает управление размещением виртуальных машин по узлам платформы. Это позволяет администраторам эффективно управлять ресурсами платформы виртуализации и оптимально размещать виртуальные машины на узлах платформы.

Платформа виртуализации предоставляет 3 предустановленных ресурса `VirtualMachineClass`:

```bash
kubectl get virtualmachineclass
NAME               PHASE   AGE
host               Ready   6d1h
host-passthrough   Ready   6d1h
generic            Ready   6d1h
```

- `host` - данный класс использует виртуальный CPU, максимально близкий к CPU узла платформы по набору инструкций. Это обеспечивает высокую производительность и функциональность, а также совместимость с живой миграцией для узлов с похожими типами процессоров. Например, миграция ВМ между узлами с процессорами Intel и AMD не будет работать. Это также справедливо для процессоров разных поколений, так как набор инструкций у них отличается.
- `host-passthrough` - используется физический CPU узла платформы напрямую без каких-либо изменений. При использовании данного класса, гостевая ВМ может быть мигрирована только на целевой узел, у которого CPU точно соответствует CPU исходного узла.
- `generic` - универсальная модель CPU, использующая достаточно старую, но поддерживаемую большинством современных процессоров модель Nehalem. Это позволяет запускать ВМ на любых узлах кластера с возможностью живой миграции.

`VirtualMachineClass` является обязательным для узказания в конфигурации виртуальной машины, пример того как указывать класс в спецификаии ВМ:

```yaml
apiVersion: virtualization.deckhouse.io/v1alpha2
kind: VirtualMachine
metadata:
  name: linux-vm
spec:
  virtualMachineClassName: generic # название ресурса VirtualMachineClass
  ...
```

Администраторы платформы могут создавать требуемые классы виртуальных машин по своим потребностям, но рекомендуется создавать необходимый миниум. Рассмотрим на следующем примере:

### Пример конфигурации VirtualMachineClass

![](/images/virtualization-platform/vmclass-examples.ru.png)

Представим, что у нас есть кластер из четырех узлов. Два из этих узлов с лейблом `group=blue` оснащены процессором "CPU X" с тремя наборами инструкций, а остальные два узла с лейблом `group=green` имеют более новый процессор "CPU Y" с четырьмя наборами инструкций.

Для оптимального использования ресурсов данного кластера, рекомендуется создать три дополнительных класса виртуальных машин (VirtualMachineClass):

- **universal**: Этот класс позволит виртуальным машинам запускаться на всех узлах платформы и мигрировать между ними. При этом будет использоваться набор инструкций для самой младшей модели CPU, что обеспечит наибольшую совместимость.
- **cpuX**: Этот класс будет предназначен для виртуальных машин, которые должны запускаться только на узлах с процессором "CPU X". ВМ смогут мигрировать между этими узлами, используя доступные наборы инструкций "CPU X".
- **cpuY**: Этот класс предназначен для виртуальных машин, которые должны запускаться только на узлах с процессором "CPU Y". ВМ смогут мигрировать между этими узлами, используя доступные наборы инструкций "CPU Y".

> Наборы инструкций для процессора — это список всех команд, которые процессор может выполнять, таких как сложение, вычитание или работа с памятью. Они определяют, какие операции возможны, влияют на совместимость программ и производительность, а также могут меняться от одного поколения процессоров к другому.

Примерные конфигурации ресурсов для данного кластера:

```yaml
---
apiVersion: virtualization.deckhouse.io/v1alpha2
kind: VirtualMachineClass
metadata:
  name: universal
spec:
  cpu:
    discovery: {}
    type: Discovery
  sizingPolicies: { ... }
---
apiVersion: virtualization.deckhouse.io/v1alpha2
kind: VirtualMachineClass
metadata:
  name: cpuX
spec:
  cpu:
    discovery: {}
    type: Discovery
  nodeSelector:
    matchExpressions:
      - key: group
        operator: In
        values: ["blue"]
  sizingPolicies: { ... }
---
apiVersion: virtualization.deckhouse.io/v1alpha2
kind: VirtualMachineClass
metadata:
  name: cpuY
spec:
  cpu:
    discovery:
      matchExpressions:
        - key: group
          operator: In
          values: ["green"]
    type: Discovery
  sizingPolicies: { ... }
```

### Прочие варианты конфигурации

Пример конфигурации ресурса `VirtualMachineClass`:

```yaml
apiVersion: virtualization.deckhouse.io/v1alpha2
kind: VirtualMachineClass
metadata:
  name: discovery
spec:
  cpu:
    # сконфигурировать универсальный vCPU для заданного набора узлов
    discovery:
      matchExpressions:
        - key: node-role.kubernetes.io/control-plane
          operator: DoesNotExist
    type: Discovery
  # разрешать запуск ВМ с данным классом только на узлах группы worker
  nodeSelector:
    matchExpressions:
      - key: node.deckhouse.io/group
        operator: In
        values:
          - worker
  # политика конфигурации ресурсов
  sizingPolicies:
    # для диапазона от 1 до 4 ядер возможно использовать от 1 до 8 Гб оперативной памяти с шагом 512Mi
    # т.е 1Гб, 1,5Гб, 2Гб, 2,5Гб итд
    # запрещено использовать выделенные ядра
    # и доступны все варианты параметра corefraction
    - cores:
        min: 1
        max: 4
      memory:
        min: 1Gi
        max: 8Gi
        step: 512Mi
      dedicatedCores: [false]
      coreFractions: [5, 10, 20, 50, 100]
    # для диапазона от 5 до 8 ядер возможно использовать от 5 до 16 Гб оперативной памяти с шагом 1Гб
    # т.е. 5Гб, 6Гб, 7Гб, итд
    # запрещено использовать выделенные ядра
    # и доступны некоторые варианты параметра corefraction
    - cores:
        min: 5
        max: 8
      memory:
        min: 5Gi
        max: 16Gi
        step: 1Gi
      dedicatedCores: [false]
      coreFractions: [20, 50, 100]
    # для диапазона от 9 до 16 ядер возможно использовать от 9 до 32 Гб оперативной памяти с шагом 1Гб
    # можно использовать выделенные ядра (а можно и не использовать)
    # и доступны некоторые варианты параметра corefraction
    - cores:
        min: 9
        max: 16
      memory:
        min: 9Gi
        max: 32Gi
        step: 1Gi
      dedicatedCores: [true, false]
      coreFractions: [50, 100]
    # для диапазона от 17 до 1024 ядер возможно использовать от 1 до 2 Гб оперативной памяти из расчета на одно ядро
    # доступны для использования только выделенные ядра
    # и единственный параметр corefraction = 100%
    - cores:
        min: 17
        max: 1024
      memory:
        perCore:
          min: 1Gi
          max: 2Gi
      dedicatedCores: [true]
      coreFractions: [100]
```

Далее приведены фрагменты конфигураций `VirtualMachineClass` для решения различных задач:

- класс с vCPU с требуемым набором процессорных инструкций, для этого используем `type: Features`, чтобы задать необходимый набор поддерживаемых инструкций для процессора:

  ```yaml
  spec:
    cpu:
      features:
        - vmx
      type: Features
  ```

- класс c универсальным vCPU для заданного набора узлов, для этого используем `type: Discovery`:

  ```yaml
  spec:
    cpu:
      discovery:
        matchExpressions:
          - key: node-role.kubernetes.io/control-plane
            operator: DoesNotExist
      type: Discovery
  ```

- чтобы создать vCPU конкретного процессора с предварительно определенным набором интрукций, используем тип `type: Model`. Предварительно, чтобы получить перечень названий поддерживаемых CPU для узла кластера, выполните команду:

  ```bash
  kubectl get nodes <node-name> -o json | jq '.metadata.labels | to_entries[] | select(.key | test("cpu-model")) | .key | split("/")[1]' -r

  # Примерный вывод:
  #
  # IvyBridge
  # Nehalem
  # Opteron_G1
  # Penryn
  # SandyBridge
  # Westmere
  ```

далее указать в спецификации ресурса `VirtualMachineClass`:

```yaml
spec:
  cpu:
    model: IvyBridge
    type: Model
```
