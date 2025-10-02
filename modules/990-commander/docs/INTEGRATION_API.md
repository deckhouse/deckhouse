---
title: "Integration API"
---

## Creating a Cluster using a Record from the Catalog

Let's look at some fields of a cluster template:

```shell
curl -s -X 'GET' \
        'https://$COMMANDER_HOST/api/v1/cluster_templates?without_archived=true' \
        -H 'accept: application/json' \
        -H 'X-Auth-Token: $COMMANDER_TOKEN' |
        jq -r '.[] | select(.name == "YC Dev") | keys'
[
  "id",                                   # ID of the template.
  "name",                                 # Name of the template.

  "current_cluster_template_version_id",  # ID of the current available version of the template.

  "cluster_template_versions",            # List of versions of the template, with content of go-template and description of input parameters.

  "comment",                              # Comment on the template.
]
```

To create a cluster, we need the ID of the version of the template. We'll take the latest version.
In this example, we'll select the template by name and take its field
`current_cluster_template_version_id`:

```shell
curl -s -X 'GET' \
        'https://$COMMANDER_HOST/api/v1/cluster_templates?without_archived=true' \
        -H 'accept: application/json' \
        -H 'X-Auth-Token: $COMMANDER_TOKEN' |
        jq -r '.[] | select(.name == "YC Dev") | del(.cluster_template_versions)'
```

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

Let's get the scheme of input parameters and make sure that there is a record from the
`yandex-cloud-slot` catalog among them.

```shell
curl -s -X 'GET' \
    'https://$COMMANDER_HOST/api/v1/cluster_templates?without_archived=true' \
    -H 'accept: application/json' \
    -H 'X-Auth-Token: $COMMANDER_TOKEN' |
    jq -r '
        .[] | select(.name == "YC Dev")
        | .cluster_template_versions[0] | select(.id == "8e75210a-f05c-421d-84b3-fc0697814d6d")
        | .params'
```

The scheme includes three mandatory parameters, including a record from the `yandex-cloud-slot`
catalog (the `catalog` field):

```json
[
  {
    "header": "Cluster Parameters"
  },
  {
    "key": "slot",
    "span": 4,
    "title": "Slot for cluster in Yandex Cloud",
    "catalog": "yandex-cloud-slot",
    "immutable": true
  },
  {
    "key": "releaseChannel",
    "enum": [ "Alpha", "Beta", "EarlyAccess", "Stable", "RockSolid" ],
    "span": 1,
    "title": "Update channel",
    "default": "EarlyAccess"
  },
  {
    "key": "kubeVersion",
    "enum": [ "Automatic", "1.25", "1.26", "1.27", "1.28", "1.29" ],
    "span": 1,
    "title": "Kubernetes version",
    "default": "Automatic"
  }
]
```

Let's find a record from this catalog. First, we'll determine the catalog ID by its identifier
(slug).

```shell
curl -s -X 'GET' \
    'https://$COMMANDER_HOST/api/v1/catalogs?without_archived=true' \
    -H 'accept: application/json' \
    -H 'X-Auth-Token: $COMMANDER_TOKEN' |
    jq -r  '.[] | select(.slug == "yandex-cloud-slot") | .id'

7a620f64-852c-4595-b41e-364dad7c3e61
```

Now, we'll choose the record from this catalog that is not already taken by another cluster:

```shell
curl -s -X 'GET' \
    'https://$COMMANDER_HOST/api/v1/records?without_archived=true' \
    -H 'accept: application/json' \
    -H 'X-Auth-Token: $COMMANDER_TOKEN' |
    jq -r  '[
    .[] |
    select(
            .catalog_id == "7a620f64-852c-4595-b41e-364dad7c3e61"
            and
            .cluster_id == null
    )
    ][0]'
```

From the received record, we will need the `values` content and ID:

```json
{
  "id": "5f6727e7-630c-4b18-bcf0-868ea96a27ee",
  "current_revision": 10,
  "catalog_id": "7a620f64-852c-4595-b41e-364dad7c3e61",
  "cluster_id": null,
  "values": {
    "ip": "118.166.177.188",
    "name": "x"
  },
  "schema_matches": true,
  "created_at": "2024-04-03T12:50:07.266+03:00",
  "updated_at": "2024-04-17T16:51:33.200+03:00",
  "archived_at": null,
  "archive_number": null
}

```

Now we can create a cluster. For the record, we use the ID in the special field
`x-commander-record-id` so as not to set a limit on the `id` field, which may be required by users
in records:

```shell
curl -X 'POST' \
  'https://$COMMANDER_HOST/api/v1/clusters' \
  -H 'accept: application/json' \
  -H 'X-Auth-Token: $COMMANDER_TOKEN' \
  -H 'Content-Type: application/json' \
  -d '{
  "name": "Кластер из API",
  "cluster_template_version_id": "8e75210a-f05c-421d-84b3-fc0697814d6d",
  "values": {
      "kubeVersion": "1.29",
      "releaseChannel": "EarlyAccess",
      "slot": {
         "x-commander-record-id": "5f6727e7-630c-4b18-bcf0-868ea96a27ee",
         "ip": "118.166.177.188",
         "name": "x"
       }
   }
}'
```

In response to the creation request, cluster data will arrive. We omitted some fields in the example
below for brevity, including the rendered configuration:

```json
{
    "id": "5436e6ef-d811-472f-9c9c-46cb9c6321d9",
    "name": "Cluster from API",
    "values": {
        "slot": {
            "ip": "118.166.177.188",
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

Now let's trace the process of cluster creation. We have to wait for the 'in_sync' status:

```shell
cluster_status="$(curl -s -X 'GET' \
    'https://$COMMANDER_HOST/api/v1/clusters/5436e6ef-d811-472f-9c9c-46cb9c6321d9' \
    -H 'accept: application/json' \
    -H 'X-Auth-Token: $COMMANDER_TOKEN' |
    jq -r '.status')"

while [ "in_sync" != "$cluster_status" ]
do
    cluster_status="$(curl -s -X 'GET' \
        'https://$COMMANDER_HOST/api/v1/clusters/5436e6ef-d811-472f-9c9c-46cb9c6321d9' \
        -H 'accept: application/json' \
        -H 'X-Auth-Token: $COMMANDER_TOKEN' |
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
