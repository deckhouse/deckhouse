---
Title: Deckhouse Admin 
---

Консоль администратора Deckhouse («админка»). Поставляется как внешний модуль Deckhouse.

## Структура

* [Бекенд](./images/backend/README.md)
* [Веб-приложение](./images/frontend/README.md)
* Хуки, написаны на python

## Как установить

Для работы модуля потребуется dockerConfigJSON с токеном в продовый регистри. В тестовых кластерах
нужно завести отдельный ключ лицензии для этого.

```shell
$ cat <<EOF>deckhouse-admin-manifests.yaml
---
apiVersion: deckhouse.io/v1alpha1
kind: ExternalModuleSource
metadata:
  name: deckhouse
spec:
  registry:
    # Чтобы достать докерконфиг, в котором уже используется ключ лицензии
    # kubectl -n d8-system get secret deckhouse-registry -ojson | jq '.data.".dockerconfigjson"' -r
    dockerCfg: "$(kubectl -n d8-system get secret deckhouse-registry -ojson | jq '.data.".dockerconfigjson"' -r)"
    repo: registry.deckhouse.io/deckhouse/fe/modules
  releaseChannel: alpha
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: deckhouse-admin
spec:
  enabled: true
  settings: {}
  version: 1
EOF

# Применяем
$ kubectl apply -f deckhouse-admin-manifests.yaml
```

[Общие  сведения о разработке внешнего модуля Deckhouse](./DECKHOUSE_EXTERNAL_MODULE.md)
