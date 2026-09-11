---
title: "Изменение конфигурации виртуальной машины"
permalink: ru/user/virtualization/vm-configuration.html
description: "Изменение конфигурации работающей виртуальной машины: какие параметры применяются сразу, какие требуют перезапуска, изменение ядер и памяти без перезапуска."
search: изменение конфигурации ВМ, перезапуск ВМ, hotplug CPU, hotplug памяти
lang: ru
---

Конфигурацию машины можно менять в любой момент после создания. У выключенной машины изменения применяются сразу, у работающей — по-разному, в зависимости от того, что именно вы изменили.

| Блок конфигурации                                                                                               | Как применяется у работающей ВМ                                                                                                                                  |
|-----------------------------------------------------------------------------------------------------------------|------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `.metadata.labels`                                                                                              | Сразу и распространяется на под ВМ                                                                                                                               |
| `.metadata.annotations`                                                                                         | Сразу и распространяется на под ВМ                                                                                                                               |
| [`.spec.liveMigrationPolicy`](/modules/virtualization/cr.html#virtualmachine-v1alpha2-spec-livemigrationpolicy)                         | Сразу                                                                                                                                                            |
| [`.spec.runPolicy`](/modules/virtualization/cr.html#virtualmachine-v1alpha2-spec-runpolicy)                                             | Сразу                                                                                                                                                            |
| [`.spec.disruptions.restartApprovalMode`](/modules/virtualization/cr.html#virtualmachine-v1alpha2-spec-disruptions-restartapprovalmode) | Сразу                                                                                                                                                            |
| [`.spec.affinity`](/modules/virtualization/cr.html#virtualmachine-v1alpha2-spec-affinity)                                               | Сразу в коммерческих редакциях DP, в DP Open нужен перезапуск                                                                                                    |
| [`.spec.nodeSelector`](/modules/virtualization/cr.html#virtualmachine-v1alpha2-spec-nodeselector)                                       | Сразу в коммерческих редакциях DP, в DP Open нужен перезапуск                                                                                                    |
| [`.spec.cpu.cores`](/modules/virtualization/cr.html#virtualmachine-v1alpha2-spec-cpu-cores)                                             | Без перезапуска, если включено [изменение числа ядер без перезапуска](#изменение-числа-ядер-без-перезапуска) в коммерческих редакциях DP, иначе нужен перезапуск |
| [`.spec.networks`](/modules/virtualization/cr.html#virtualmachine-v1alpha2-spec-networks)                                               | Добавление и удаление сетей применяется на работающей ВМ, если гостевая ОС поддерживает подключение интерфейсов на ходу                                          |
| Остальные поля `.spec`                                                                                          | Нужен перезапуск                                                                                                                                                 |

Ниже показано, как изменить конфигурацию виртуальной машины:

{% tabs vm-config %}

{% tab "В командной строке" %}

Ниже показан пример с изменением числа ядер.

1. Посмотрите, сколько ядер видит гостевая ОС сейчас:

   ```bash
   d8 v ssh cloud@linux-vm --command "nproc"
   ```

   Пример вывода:

   ```console
   1
   ```

1. Задайте новое число ядер:

   ```bash
   d8 k patch vm linux-vm --type merge -p '{"spec":{"cpu":{"cores":2}}}'

   # Того же результата можно добиться, отредактировав ресурс.
   d8 k edit vm linux-vm
   ```

1. Убедитесь, что изменение принято, но ещё не применено. Гостевая ОС по-прежнему видит одно ядро, а список ожидающих изменений не пуст:

   ```bash
   d8 k get vm linux-vm -o jsonpath="{.status.restartAwaitingChanges}" | jq .
   ```

   Пример вывода:

   ```json
   [
     {
       "currentValue": 1,
       "desiredValue": 2,
       "operation": "replace",
       "path": "cpu.cores"
     }
   ]
   ```

   То же самое показывает колонка `NEED RESTART`:

   ```bash
   d8 k get vm linux-vm -o wide
   ```

   Пример вывода:

   <!-- markdownlint-disable MD031 -->
   ```console
   NAME       PHASE     UPTIME   CORES   COREFRACTION   MEMORY   NEED RESTART   AGENT   MIGRATABLE   NODE           IPADDRESS     AGE
   linux-vm   Running   5m16s    2       100%           1Gi      True           True    True         virtlab-pt-1   10.66.10.13   5m16s
   ```
   {: .nowrap-default }
   <!-- markdownlint-enable MD031 -->

1. Перезапустите машину:

   ```bash
   d8 v restart linux-vm
   ```

1. Проверьте результат. После перезапуска блок [`.status.restartAwaitingChanges`](/modules/virtualization/cr.html#virtualmachine-v1alpha2-status-restartawaitingchanges) пуст, а гостевая ОС видит два ядра:

   ```bash
   d8 v ssh cloud@linux-vm --command "nproc"
   ```

   Пример вывода:

   ```console
   2
   ```

По умолчанию перезапуск подтверждаете вы. Чтобы модуль применял изменения сам, задайте в параметре [`.spec.disruptions.restartApprovalMode`](/modules/virtualization/cr.html#virtualmachine-v1alpha2-spec-disruptions-restartapprovalmode) значение `Automatic`:

```yaml
spec:
  disruptions:
    restartApprovalMode: Automatic
```

{% endtab %}

{% tab "В веб-интерфейсе" %}

1. Перейдите на вкладку «Проекты» и выберите нужный проект.
1. Перейдите в раздел «Виртуализация» → «Виртуальные машины».
1. Из списка выберите нужную ВМ и нажмите на её имя.
1. Внесите изменения на вкладке «Конфигурация». Если машину требуется перезапустить, модуль покажет предупреждение и список ожидающих изменений.
1. Чтобы изменения применялись без вашего подтверждения, прокрутите страницу до раздела «Жизненный цикл», включите переключатель «Автоприменение изменений» и нажмите кнопку «Сохранить».

{% endtab %}

{% endtabs %}

## Изменение числа ядер без перезапуска

Число ядер работающей машины можно менять, не перезагружая её, если изменение применимо через живую миграцию. В пределах текущей топологии CPU ядра можно и добавлять, и убирать.

Возможность выключена по умолчанию. Чтобы её включить, администратор добавляет `HotplugCPUWithLiveMigration` в параметр [`.spec.settings.featureGates`](../../admin/configuration/virtualization/settings.html) модуля:

```yaml
kind: ModuleConfig
metadata:
  name: virtualization
spec:
  settings:
    featureGates:
      - HotplugCPUWithLiveMigration
```

В веб-интерфейсе тот же переключатель называется «Изменение CPU без перезагрузки» и находится в блоке «Экспериментальные возможности» на вкладке «Система» → «Deckhouse» → «Модули» → `virtualization` → «Конфигурация». Права на это есть только у администратора платформы.

Когда возможность включена, а новое значение [`.spec.cpu.cores`](/modules/virtualization/cr.html#virtualmachine-v1alpha2-spec-cpu-cores) остаётся в пределах текущей топологии, модуль применяет изменение живой миграцией. Если изменение требует смены топологии, машину придётся перезагрузить. Правила расчёта топологии описаны в разделе [«Топологии CPU»](vm-resources.html#топологии-cpu).

{% tabs vm-cpu-change %}

{% tab "В командной строке" %}

Задайте новое число ядер:

```bash
d8 k patch vm linux-vm --type merge -p '{"spec":{"cpu":{"cores":4}}}'
```

{% endtab %}

{% tab "В веб-интерфейсе" %}

1. Перейдите на вкладку «Проекты» и выберите нужный проект.
1. Перейдите в раздел «Виртуализация» → «Виртуальные машины».
1. Из списка выберите нужную ВМ и нажмите на её имя.
1. На вкладке «Конфигурация» в разделе «Ресурсы» задайте новое значение в поле «Ядра ЦП».
1. Нажмите появившуюся кнопку «Сохранить».

{% endtab %}

{% endtabs %}

Гостевая ОС не всегда вводит новые ядра в работу сама, особенно после живой миграции. В Linux ядро включается через sysfs:

```bash
echo 1 > /sys/devices/system/cpu/cpu1/online
```

Чтобы это происходило автоматически, добавьте правило `udev`:

<!-- markdownlint-disable MD031 -->
```bash
cat <<'EOF' > /etc/udev/rules.d/99-hotplug-cpu.rules
SUBSYSTEM=="cpu",ACTION=="add",RUN+="/bin/sh -c '[ ! -e /sys$devpath/online ] || echo 1 > /sys$devpath/online'"
EOF
```
{: .nowrap-default }
<!-- markdownlint-enable MD031 -->

Введённые в работу ядра появляются в выводе `nproc`, `cat /proc/cpuinfo` и `top`.

При уменьшении числа ядер в пределах текущей топологии распределение ядер по сокетам сохраняется.

## Изменение объёма памяти без перезапуска

Объём памяти работающей машины можно увеличивать, не перезагружая её. Уменьшение требует перезапуска.

Возможность выключена по умолчанию. Чтобы её включить, администратор добавляет `HotplugMemoryWithLiveMigration` в параметр [`.spec.settings.featureGates`](../../admin/configuration/virtualization/settings.html) модуля:

```yaml
kind: ModuleConfig
metadata:
  name: virtualization
spec:
  settings:
    featureGates:
      - HotplugMemoryWithLiveMigration
```

В веб-интерфейсе переключатель называется «Изменение памяти без перезагрузки» и лежит там же, в блоке «Экспериментальные возможности» настроек модуля.

Когда возможность включена, новое значение [`.spec.memory.size`](/modules/virtualization/cr.html#virtualmachine-v1alpha2-spec-memory-size) больше текущего и машина допускает миграцию, модуль применяет изменение живой миграцией. Перезапуск понадобится, если память уменьшают, если исходный размер меньше 1 ГиБ или если машину нельзя мигрировать. Без перезапуска память растёт до 256 ГиБ, это предел, заложенный в конфигурацию машины при первом запуске.

{% tabs vm-memory-change %}

{% tab "В командной строке" %}

Задайте новый объём памяти:

```bash
d8 k patch vm linux-vm --type merge -p '{"spec":{"memory":{"size":"4Gi"}}}'
```

{% endtab %}

{% tab "В веб-интерфейсе" %}

1. Перейдите на вкладку «Проекты» и выберите нужный проект.
1. Перейдите в раздел «Виртуализация» → «Виртуальные машины».
1. Из списка выберите нужную ВМ и нажмите на её имя.
1. На вкладке «Конфигурация» в разделе «Ресурсы» задайте новое значение в поле «Объём памяти».
1. Нажмите появившуюся кнопку «Сохранить».

{% endtab %}

{% endtabs %}

Как и с ядрами, гостевая ОС может не ввести новые блоки памяти в работу сама. В Linux блок включается через sysfs, а имя устройства видно в выводе `lsmem` или в каталоге `/sys/bus/memory/devices/`:

```bash
echo 1 > /sys/bus/memory/devices/memoryXXX/online
```

Чтобы это происходило автоматически, добавьте правило `udev`:

<!-- markdownlint-disable MD031 -->
```bash
cat <<'EOF' > /etc/udev/rules.d/99-hotplug-memory.rules
SUBSYSTEM=="memory",ACTION=="add",DEVPATH=="/devices/system/memory/memory[0-9]*", TEST=="state", ATTR{state}!="online", ATTR{state}="online"
EOF
```
{: .nowrap-default }
<!-- markdownlint-enable MD031 -->
