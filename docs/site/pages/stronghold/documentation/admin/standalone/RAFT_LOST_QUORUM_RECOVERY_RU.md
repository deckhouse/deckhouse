---
title: "Восстановление после потери кворума"
permalink: ru/stronghold/documentation/admin/standalone/raft-lost-quorum-recovery.html
lang: ru
---

> Кворум – минимальное количество узлов в кластере для возможности выполнения голосования с целью достижения консенсуса.

При использовании интегрированного хранилища, поддержание кворума Raft является важным фактором для настройки и эксплуатации среды Stronghold с включенным HA. Кластер Stronghold окончательно теряет кворум, когда нет возможности восстановить достаточное количество серверов Stronghold для достижения консенсуса и избрания лидера. Без кворума серверов кластера, Stronghold больше не может выполнять операции чтения и записи.

Кворум кластера динамически обновляется при подключении новых узлов к кластеру. Stronghold рассчитывает кворум по формуле `(n+1)/2`, где `n` — количество серверов в кластере. Например, для кластера из 3 серверов потребуется как минимум 2 рабочих сервера, чтобы кластер функционировал должным образом, `(3+1)/2 = 2`. В частности, для выполнения операций чтения и записи потребуется 2 постоянно активных сервера.

> **Примечание:** Существует исключение из этого правила, если при присоединении к кластеру используется опция `-non-voter`. Эта функция доступна только в версии Stronghold формата отдельной инсталляции.

## Обзор сценария

Когда два сервера из трёх вышли из строя, кластер теряет кворум и перестает функционировать.

Несмотря на один полностью работоспособный сервер, кластер не может обрабатывать запросы на чтение или запись.

**Примеры:**

1) Вывод в консоли после выполнения комманд:
```
$ stronghold operator raft list-peers
* local node not active but active cluster node not found

$ stronghold kv get kv/apikey
* local node not active but active cluster node not found
```

2) В логах нерабочего узла:
```
окт 20 10:54:32 standalone-astra stronghold[647]: {"@level":"info","@message":"attempting to join possible raft leader node","@module":"core","@timestamp":"2025-10-20T10:54:02.578963Z","leader_addr":"https://stronghold-0.stronghold.tld:8201"}
окт 20 10:54:32 standalone-astra stronghold[647]: {"@level":"error","@message":"failed to get raft challenge","@module":"core","@timestamp":"2025-10-20T10:54:32.597558Z","error":"error during raft bootstrap init call: Put \"https://10.0.101.22:8201/v1/sys/storage/raft/bootstrap/challenge\": dial tcp 10.0.101.22:8201: i/o timeout","leader_addr":"https://stronghold-0.stronghold.tld:8201"}
```

Процесс восстановления работы Stronghold при потере 2 из 3 серверов будет выполнена путем преобразования кластера в вариант из одного узла.

Для выполнения этой процедуры обязательно, чтобы один сервер был полностью работоспособным.

> **Примечание:** Иногда Stronghold теряет кворум из-за автопилота и серверов, помеченных как неработоспособные, но служба по-прежнему работает. На неработоспособных серверах необходимо остановить службы перед запуском процедуры peers.json.
>
> В кластере из 5 серверов или в случае отсутствия голосующих нужно остановить другие исправные серверы перед выполнением восстановления peers.json.

## Найдите каталог хранилища

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

## Создайте файл peers.json

Внутри каталога хранилища (`/opt/stronghold/data`) находится папка с именем `raft`.

```
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

Убедитесь, что пользователь `stronghold` имеет право на *чтение* и *изменение* файла `peers.json`.
```bash
chown stronghold:stronghold /opt/stronghold/data/raft/peers.json
chmod 600 /opt/stronghold/data/raft/peers.json
```

## Перезапустите Stronghold

Перезапустите процесс Stronghold, чтобы Stronghold мог загрузить новый файл `peers.json`.

```bash
$ sudo systemctl restart stronghold
```

> **Примечание:** если вы используете Systemd, сигнал `SIGHUP` не будет работать.

## Распечатайте Stronghold

Если не настроено использование автоматической распечатки, распечатайте Stronghold, а затем проверьте статус.

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

## Проверка успешности

Процедура восстановления прошла успешно, если Stronghold запустился и отобразил следующие сообщения в системных журналах.

```
...
[INFO]  core.cluster-listener: serving cluster requests: cluster_listen_address=[::]:8201
[INFO]  storage.raft: raft recovery initiated: recovery_file=peers.json
[INFO]  storage.raft: raft recovery found new config: config="{[{Voter stronghold_0 https://stronghold-0.stronghold.tld:8201}]}"
[INFO]  storage.raft: raft recovery deleted peers.json
...
```

## Просмотр списка узлов

Теперь в кластере числится только один сервер. Это позволило Stronghold достичь кворума и восстановить работоспособность. Чтобы убедиться в их количестве, выполните команду `stronghold operator raft list-peers`.

```bash
$ stronghold operator raft list-peers
server          Address                                     State     Voter
----            -------                                     -----     -----
stronghold_0    https://stronghold-0.stronghold.tld:8201    leader    true
```

Как видно, в списке узлов кластера указан только один сервер. 

## Следующие шаги

В этом руководстве мы восстановили кворум, преобразовав кластер из 3 серверов в кластер из одного сервера с помощью файла `peers.json`. Файл `peers.json` позволил нам вручную обновить список узлов Raft оставив единственный работоспособный сервер, что позволило этому серверу достичь кворума и успешно выборать лидера.

Если вышедшие из строя серверы **поддаются восстановлению**, лучшим вариантом будет вернуть их в сеть и подключить к кластеру с использованием тех же адресов хостов. Это вернет кластер в полностью рабочее состояние. Для этого файле `raft/peers.json` должны быть указаны данные: идентификатор сервера, *адрес:порт* и информация о возможности голосовать, для каждого сервера, который вы хотите включить в кластер.

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