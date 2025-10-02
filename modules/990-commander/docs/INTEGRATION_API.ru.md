---
title: "API интеграции"
---

## Возможности API

Документация API для внешней интеграции доступна в интерфейсе Deckhouse Commander в меню настроек в правом
верхнем углу.

Для того чтобы использовать это API, нужно выпустить токен. Токен также выпускается в интерфейсе
Deckhouse Commander. Токен нужно передавать в заголовке `X-Auth-Token`.

Что доступно в API

1. Чтение шаблонов кластеров
   1. `GET  /api/v1/cluster_templates`
   2. `GET  /api/v1/cluster_templates/:id`
2. Чтение каталогов инвентаря
   1. `GET  /api/v1/catalogs`
   2. `GET  /api/v1/catalogs/:id`
3. Создание, изменение, удаление кластеров
   1. `POST   /api/v1/clusters`
   2. `GET    /api/v1/clusters`
   3. `GET    /api/v1/clusters/:id`
   4. `PUT    /api/v1/clusters/:id`
   5. `DELETE /api/v1/clusters/:id`
4. Создание, изменение, удаление записей в каталогах
   1. `POST   /api/v1/records`
   2. `GET    /api/v1/records`
   3. `GET    /api/v1/records/:id`
   4. `PUT    /api/v1/records/:id`
   5. `DELETE /api/v1/records/:id`

## Создание кластера с использованием записи из каталога

### Получение актуальной версии шаблона

Чтобы создать кластер, нам нужен ID **версии шаблона**. Мы возьмем последнюю версию, которая
записана в шаблоне в поле `current_cluster_template_version_id`. Тело версий шаблона громоздкое,
поэтому опустим вывод версий:

```shell
curl -s -X 'GET' \
        "https://$COMMANDER_HOST/api/v1/cluster_templates/$TEMPLATE_ID?without_archived=true" \
        -H 'accept: application/json' \
        -H "X-Auth-Token: $COMMANDER_TOKEN" |
        jq -r 'del(.cluster_template_versions)'
```

Нас интересует поле `current_cluster_template_version_id`:

```json
{
  "id": "fb999a72-efe7-4db7-af53-11b17bc0a687",
  "name": "YC Dev",
  "current_cluster_template_version_id": "8e75210a-f05c-421d-84b3-fc0697814d6d",
  "comment": "Канал обновлений и версия k8s задаются",
  "current_revision": 12,
  "immutable": false,
  "created_at": "2024-02-05T17:35:44.318+03:00",
  "updated_at": "2024-04-10T18:00:57.835+03:00",
  "archived_at": null,
  "archive_number": null
}
```

```shell
TEMPLATE_VERSION_ID="$(curl -s -X 'GET' \
        "https://$COMMANDER_HOST/api/v1/cluster_templates/$TEMPLATE_ID?without_archived=true" \
        -H 'accept: application/json' \
        -H "X-Auth-Token: $COMMANDER_TOKEN" |
        jq -r '.current_cluster_template_version_id')"
```

### Получение записи для входных параметров

Получим схему входных параметров и убедимся, что среди них есть запись из каталога `yandex-cloud-slot`

```shell
curl -s -X 'GET' \
    "https://$COMMANDER_HOST/api/v1/cluster_templates/$TEMPLATE_ID?without_archived=true" \
    -H 'accept: application/json' \
    -H "X-Auth-Token: $COMMANDER_TOKEN" |
    jq -r --arg  templ_version "$TEMPLATE_VERSION_ID" '
        .cluster_template_versions[]
        | select(.id == $templ_version)
        | .params'
```

Рассмотрим схему входных параметров шаблона, она же — схема параметров кластера. Схема
предусматривает три обязательных параметра среди которых запись из каталога `yandex-cloud-slot`
(параметр `slot`, свойство `catalog`):

```json
[
  {
    "header": "Параметры кластера"
  },
  {
    "key": "slot",
    "span": 4,
    "title": "Слот для кластера в Yandex Cloud",
    "catalog": "yandex-cloud-slot",
    "immutable": true
  },
  {
    "key": "releaseChannel",
    "enum": [ "Alpha", "Beta", "EarlyAccess", "Stable", "RockSolid" ],
    "span": 1,
    "title": "Канал обновлений",
    "default": "EarlyAccess"
  },
  {
    "key": "kubeVersion",
    "enum": [ "Automatic", "1.25", "1.26", "1.27", "1.28", "1.29" ],
    "span": 1,
    "title": "Версия Kubernetes",
    "default": "Automatic"
  }
]
```

Найдем запись из этого каталога. Для начала определим ID каталога по его идентификатору (slug).

```shell
CATALOG_ID="$(curl -s -X 'GET' \
    "https://$COMMANDER_HOST/api/v1/catalogs?without_archived=true" \
    -H 'accept: application/json' \
    -H "X-Auth-Token: $COMMANDER_TOKEN" |
    jq -r  '.[] | select(.slug == "yandex-cloud-slot") | .id')"
```

Теперь выберем первую попавшуюся запись из этого каталога, который еще не занят другим кластером.
Эту запись нужно подготовить к использованию в кластере. Для записи необходимо помимо значений
указать его ID в специальном поле `x-commander-record-id`. Это поле названо так, чтобы не вводить
ограничение на поле `id`, которое может потребоваться пользователям в самих записях:

```shell
SLOT_RECORD="$(curl -s -X 'GET' \
    "https://$COMMANDER_HOST/api/v1/records?without_archived=true" \
    -H 'accept: application/json' \
    -H "X-Auth-Token: $COMMANDER_TOKEN" |
    jq -rc --arg catalog_id "$CATALOG_ID" '[
            .[] |
                select(
                    .catalog_id == $catalog_id
                    and
                    .cluster_id == null
                )
            ][0]
            | .values + { "x-commander-record-id": .id }')"

```

Полученная структура:

```json
{
  "ip": "158.166.177.188",
  "name": "x",
  "x-commander-record-id": "5f6727e7-630c-4b18-bcf0-868ea96a27ee"
}
```

### Создание кластера

Теперь можем создать кластер:

```shell
PAYLOAD="$(jq -nc --argjson slot "$SLOT_RECORD" '{
    "name": "Кластер из API",
    "cluster_template_version_id": "8e75210a-f05c-421d-84b3-fc0697814d6d",
    "values": {
        "kubeVersion": "1.29",
        "releaseChannel": "EarlyAccess",
        "slot": $slot
    }
}')"
curl -v -X 'POST' \
    "https://$COMMANDER_HOST/api/v1/clusters" \
    -H 'accept: application/json' \
    -H "X-Auth-Token: $COMMANDER_TOKEN" \
    -H 'Content-Type: application/json' \
    -d "'$PAYLOAD'"



curl -v -X 'POST' \
    "https://$COMMANDER_HOST/api/v1/clusters" \
    -H 'accept: application/json' \
    -H "X-Auth-Token: $COMMANDER_TOKEN" \
    -H 'Content-Type: application/json' \
    -d '{"name":"Кластер из API","cluster_template_version_id":"8e75210a-f05c-421d-84b3-fc0697814d6d","values":{"kubeVersion":"1.29","releaseChannel":"EarlyAccess","slot":{"ip":"158.160.110.223","name":"b","x-commander-record-id":"5f6727e7-630c-4b18-bcf0-868ea96a27ee"}}}'



{
  "name": "Кластер из API",
  "cluster_template_version_id": "8e75210a-f05c-421d-84b3-fc0697814d6d",
  "values": {
      "kubeVersion": "1.29",
      "releaseChannel": "EarlyAccess",
      "slot": {
         "x-commander-record-id": "5f6727e7-630c-4b18-bcf0-868ea96a27ee",
         "ip": "158.166.177.188",
         "name": "x"
       }
   }
}

"'" "$(jq -n --argjson slot "$SLOT_RECORD" '{
        "name": "Кластер из API",
        "cluster_template_version_id": "8e75210a-f05c-421d-84b3-fc0697814d6d",
        "values": {
            "kubeVersion": "1.29",
            "releaseChannel": "EarlyAccess",
            "slot": $slot
        }
    }')" "'"

```

В ответ на запрос создания придут данные кластера. Часть полей в примере ниже мы опустили для
краткости, в том числе отрендеренную конфигурацию:

```json
{
    "id": "5436e6ef-d811-472f-9c9c-46cb9c6321d9",
    "name": "Кластер из API",
    "values": {
        "slot": {
            "ip": "158.166.177.188",
            "name": "x",
            "x-commander-record-id": "5f6727e7-630c-4b18-bcf0-868ea96a27ee"
        },
        "kubeVersion": "1.29",
        "releaseChannel": "EarlyAccess"
    },
    "cluster_template_version_id": "8e75210a-f05c-421d-84b3-fc0697814d6d",
    "was_created": false,
    "status": "new"
}
```

Теперь проследим процесс создания кластера. Мы должны дождаться статуса `in_sync`:

```shell
cluster_status="$(curl -s -X 'GET' \
    "https://$COMMANDER_HOST/api/v1/clusters/5436e6ef-d811-472f-9c9c-46cb9c6321d9" \
    -H 'accept: application/json' \
    -H "X-Auth-Token: $COMMANDER_TOKEN" |
    jq -r '.status')"

while [ "in_sync" != "$cluster_status" ]
do
    cluster_status="$(curl -s -X 'GET' \
        "https://$COMMANDER_HOST/api/v1/clusters/5436e6ef-d811-472f-9c9c-46cb9c6321d9" \
        -H 'accept: application/json' \
        -H "X-Auth-Token: $COMMANDER_TOKEN" |
        jq -r '.status')"
    echo $cluster_status
    sleep 5
done

creating
creating
creating
creating
creating
creating
creating
creating
creating
creating
creating
creating
creating
creating
creating
creating
# ...
in_sync
```

### Удаление кластера

Если кластер больше не нужен, его можно удалить. В ответ придет состояние кластера, но уже со
статусом `delete`.

```shell
curl -s -X 'DELETE' \
    "https://$COMMANDER_HOST/api/v1/clusters/5436e6ef-d811-472f-9c9c-46cb9c6321d9" \
    -H 'accept: application/json' \
    -H "X-Auth-Token: $COMMANDER_TOKEN"

{
    "id": "5436e6ef-d811-472f-9c9c-46cb9c6321d9",
    "current_revision": 1834,
    "name": "Кластер из API",
    "values": {
        "slot": {
            "ip": "158.166.177.188",
            "name": "x",
            "x-commander-record-id": "5f6727e7-630c-4b18-bcf0-868ea96a27ee"
        },
        "kubeVersion": "1.29",
        "createWorker": true,
        "releaseChannel": "EarlyAccess",
        "installResources": true
    },
    "cluster_template_version_id": "8e75210a-f05c-421d-84b3-fc0697814d6d",
    "cluster_template_version_switched_at": "2024-04-24T21:40:35.222+03:00",
    "created_at": "2024-04-24T21:40:35.525+03:00",
    "updated_at": "2024-04-25T12:57:51.621+03:00",
    "archived_at": null,
    "archive_number": null,
    "was_created": true,
    "is_locked": true,
    "cluster_type": "cloud",
    "status": "delete",
    "agent_status": null,
    "agent_api_key": null,
    "cluster_configuration_applied_at": "2024-04-24T22:10:57.191+03:00",
    "cluster_configuration_checked_at": "2024-04-25T12:57:45.479+03:00",
    "resources_sync_state": "no",
    "resources_state_results": [
        // ...
    ],
    "resources_checked_at": "2024-04-25T12:57:31.084+03:00",
    "resources_applied_at": "2024-04-24T22:04:59.813+03:00",
    "init_configuration_rendered": "...",
    "init_resources_rendered": "...",
    "dhctl_configuration_rendered": "...",
    "applied_cluster_configuration_rendered": "...",
    "applied_provider_specific_cluster_configuration_rendered": "...",
    "applied_resources_rendered": "......",
    "desired_cluster_configuration_rendered": "...",
    "desired_provider_specific_cluster_configuration_rendered": "...",
    "desired_resources_rendered": "......",
    "render_errors": [],
    "cluster_agent_data": [
        // ...
    ]
}
```
