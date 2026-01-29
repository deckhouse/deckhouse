---
title: "Восстановление после потери кворума"
permalink: ru/stronghold/documentation/admin/standalone/raft-lost-quorum-recovery.html
lang: ru
---

Кворум — минимальное количество узлов в кластере, необходимое для голосования и избрания лидера.
Лидер (Raft leader) — активный узел кластера, принимающий операции чтения и записи и координирующий работу остальных узлов.

При использовании интегрированного хранилища поддержание кворума Raft является важным фактором для настройки и эксплуатации среды Stronghold с включенным HA. Кластер Stronghold окончательно теряет кворум, когда нет возможности восстановить достаточное количество серверов Stronghold для достижения консенсуса и избрания лидера.
Без кворума серверов кластера Stronghold больше не может выполнять операции чтения и записи.

Кворум кластера динамически обновляется при подключении к нему новых узлов. Stronghold рассчитывает кворум по формуле `(n+1)/2`, где `n` — количество серверов в кластере. Например, для кластера из 3 серверов потребуется как минимум 2 рабочих сервера, чтобы кластер функционировал должным образом, `(3+1)/2 = 2`.
В частности, для выполнения операций чтения и записи потребуется 2 постоянно активных сервера.

{% alert level="info" %}
Существует исключение из этого правила, если при присоединении к кластеру используется опция `-non-voter`. Эта функция доступна только в версии Stronghold формата отдельной инсталляции.
{% endalert %}

## Сценарий потери кворума

Когда два сервера из трёх вышли из строя, кластер теряет кворум и перестаёт функционировать.

Несмотря на один полностью работоспособный сервер, кластер не может обрабатывать запросы на чтение или запись.

**Примеры:**

1. Вывод в консоли после выполнения команд:

    ```text
    $ stronghold operator raft list-peers
    * local node not active but active cluster node not found
    
    $ stronghold kv get kv/apikey
    * local node not active but active cluster node not found
    ```

1. Вывод в логах одного из неработоспособных узлов:

    ```text
    окт 20 10:54:32 standalone-astra stronghold[647]: {"@level":"info","@message":"attempting to join possible raft leader node","@module":"core","@timestamp":"2025-10-20T10:54:02.578963Z","leader_addr":"https://stronghold-0.stronghold.tld:8201"}
    окт 20 10:54:32 standalone-astra stronghold[647]: {"@level":"error","@message":"failed to get raft challenge","@module":"core","@timestamp":"2025-10-20T10:54:32.597558Z","error":"error during raft bootstrap init call: Put \"https://10.0.101.22:8201/v1/sys/storage/raft/bootstrap/challenge\": dial tcp 10.0.101.22:8201: i/o timeout","leader_addr":"https://stronghold-0.stronghold.tld:8201"}
    ```

Процесс восстановления работы Stronghold при потере 2 из 3 серверов выполняется путём преобразования кластера в вариант из одного узла.

Для выполнения этой процедуры один сервер должен быть полностью работоспособным.

В кластере из 5 серверов или при наличии неголосующих узлов необходимо остановить остальные исправные серверы перед выполнением восстановления через peers.json.

### Особенности восстановления при работе Autopilot

Автопилот (Autopilot) — это механизм Stronghold, который автоматически отслеживает состояние узлов Raft-кластера и управляет их участием в кворуме.

В некоторых случаях Stronghold теряет кворум из-за автопилота и серверов, помеченных как неработоспособные, хотя Stronghold формально продолжает работать.
В таких ситуациях перед выполнением процедуры восстановления с использованием peers.json необходимо остановить службы Stronghold на неработоспособных серверах.

## Восстановление после потери кворума

### Поиск каталога хранилища

На сервере с исправным узлом Stronghold найдите каталог хранилища Raft. Чтобы узнать расположение каталога, просмотрите файл конфигурации Stronghold. Строка `storage` будет содержать `path` к каталогу.

**Пример:**

`/opt/stronghold/config.hcl`

```hcl
storage "raft" {
  path    = "/opt/stronghold/data"
  server_id = "stronghold_0"
}

listener "tcp" {
  address     = "0.0.0.0:8200"
  cluster_address     = "0.0.0.0:8201"
  tls_disable = true
}

api_addr = "http://stronghold-0.stronghold.tld:8200"
cluster_addr = "http://stronghold-0.stronghold.tld:8201"
disable_mlock = true
ui=true
```

В этом примере `path` — это путь к файловой системе, в которой Stronghold хранит данные, а `server_id` — идентификатор сервера в кластере Raft. В примере `server_id` — это `stronghold_0`.

### Создание файла peers.json

Внутри каталога хранилища (`/opt/stronghold/data`) находится папка с именем `raft`.

```text
/opt
└ stronghold
  └── data
      ├── raft
      │   ├── raft.db
      │   └── snapshots
      └── vault.db
```

Чтобы единственный оставшийся сервер Stronghold мог достичь кворума и избрать себя лидером, создайте файл `raft/peers.json`, содержащий информацию о сервере. Формат файла — массив JSON, содержащий ID **работоспособного** сервера Stronghold, его *адрес:порт* и информацию о возможности голосовать.

**Пример:**

```bash
$ cat > /opt/stronghold/data/raft/peers.json << EOF
[
  {
    "id": "stronghold_0",
    "address": "stronghold-0.stronghold.tld:8201",
    "non_voter": false
  }
]
EOF
```

Параметры:
- **id** (строка: \<обязательно\>) — указывает идентификатор сервера.
- **address** (строка: \<обязательно\>) — указывает хост и порт сервера. Порт — это порт кластера сервера.
- **non_voter** (bool: \<false\>) — указывает, участвует ли сервер в голосовании.

Убедитесь, что пользователь `stronghold` имеет право на *чтение* и *изменение* файла `peers.json`:

```bash
chown stronghold:stronghold /opt/stronghold/data/raft/peers.json
chmod 600 /opt/stronghold/data/raft/peers.json
```

### Перезапуск Stronghold

Перезапустите процесс Stronghold, чтобы Stronghold мог загрузить новый файл `peers.json`.

```bash
sudo systemctl restart stronghold
```

{% alert level="info" %}
Если вы используете Systemd, сигнал `SIGHUP` не будет работать.
{% endalert %}

### Распечатывание Stronghold

Если не настроено автоматическое распечатывание (auto-unseal), выполните его вручную, а затем проверьте статус.

**Пример:**

```bash
$ stronghold operator unseal
Unseal Key (will be hidden):

$ stronghold status
Key                      Value
---                      -----
Recovery Seal Type       shamir
Initialized              true
Sealed                   false
Total Recovery Shares    1
Threshold                1
Version                  1.16.0+hsm
Storage Type             raft
Cluster Name             stronghold-cluster-4a1a40af
Cluster ID               d09df2c7-1d3e-f7d0-a9f7-93fadcc29110
HA Enabled               true
HA Cluster               https://stronghold-0.stronghold.tld:8201
HA Mode                  active
Active Since             2021-07-20T00:07:32.215236307Z
Raft Committed Index     155344
Raft Applied Index       155344
```

### Проверка успешности восстановления

Процедура восстановления прошла успешно, если Stronghold запустился и отобразил следующие сообщения в системных журналах.

```text
...
[INFO]  core.cluster-listener: serving cluster requests: cluster_listen_address=[::]:8201
[INFO]  storage.raft: raft recovery initiated: recovery_file=peers.json
[INFO]  storage.raft: raft recovery found new config: config="{[{Voter stronghold_0 https://stronghold-0.stronghold.tld:8201}]}"
[INFO]  storage.raft: raft recovery deleted peers.json
...
```

### Проверка списка узлов

Теперь в кластере числится только один сервер. Это позволит Stronghold достичь кворума и восстановить работоспособность. Чтобы убедиться в количестве серверов, выполните команду `stronghold operator raft list-peers`.

```bash
$ stronghold operator raft list-peers
server          Address                                     State     Voter
----            -------                                     -----     -----
stronghold_0    https://stronghold-0.stronghold.tld:8201    leader    true
```

Как видно, в списке узлов кластера указан только один сервер.

### Дальнейшие действия

В этом руководстве мы восстановили кворум, преобразовав кластер из трёх серверов в кластер из одного сервера с помощью файла `peers.json`.
С его помощью мы вручную обновили список узлов Raft, оставив единственный работоспособный сервер. Это позволило серверу достичь кворума и успешно выбрать лидера.

Если вышедшие из строя серверы **поддаются восстановлению**, лучшим вариантом будет вернуть их в сеть и подключить к кластеру с использованием тех же адресов хостов. Это вернет кластер в полностью рабочее состояние.
Для этого в файле `raft/peers.json` должны быть указаны данные: идентификатор сервера, *адрес:порт* и информация о возможности голосовать. Данные должны быть указаны для каждого сервера, который вы хотите включить в кластер.

```json
[
  {
    "id": "stronghold_0",
    "address": "stronghold-0.stronghold.tld:8201",
    "non_voter": false
  },
  {
    "id": "stronghold_1",
    "address": "stronghold-1.stronghold.tld:8201",
    "non_voter": false
  },
  {
    "id": "stronghold_2",
    "address": "stronghold-2.stronghold.tld:8201",
    "non_voter": false
  }
]
```
