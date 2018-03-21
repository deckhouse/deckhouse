# Установка Antiopa

Antiopa - приложение которое обновляет инфраструктурные модули и запускает их при выпуске новой версии. Модуль — это установка приложений типа prometheus, ingress и подобного. 

На данный момент Antiopa:

* устанавливает kube-lego
* устанавливает ingress controller (необходима соответствующая [настройка](rfc-ingress.md))
* создает роль cluster-admin
* устанавливает отдельный экземпляр tiller
* управляет дополнениями для работы кластера
* подтюнивает кластер

Удаление Antiopa подразумевает [заморозку](#Удаление) на определенной версии.


## Установка

1. Узнать token пользователя antiopa в канале #tech-kubernetes

2. На машине, где настроен kubectl (например, где есть deploy runner) запустить:

```
$ TOKEN=$(cat); curl -fLs -H "PRIVATE-TOKEN: $TOKEN" https://github.com/deckhouse/deckhouse/raw/stable/get-antiopa.sh | bash -s -- --token $TOKEN
```

Вставить token, нажать `<Enter>` и `<Ctrl-D>`

### Production кластер

В кластер, внедрение в котором преимущественно закончено и осуществляется поддержка -  необходимо устанавливать из ветки **stable** (по умолчанию):
```
$ TOKEN=$(cat); curl -fLs -H "PRIVATE-TOKEN: $TOKEN" https://github.com/deckhouse/deckhouse/raw/stable/get-antiopa.sh | bash -s -- --token $TOKEN
```

В кластере, в котором идет активная работа (запускаются, дорабатываются новые приложения, и т.п.) — ставить из ветки **ea** (early access):
```
$ TOKEN=$(cat); curl -fLs -H "PRIVATE-TOKEN: $TOKEN" https://github.com/deckhouse/deckhouse/raw/stable/get-antiopa.sh | bash -s -- --token $TOKEN --version ea
```

### Dev кластер

Ставить из ветки master или своих веток, со своими модификациями.

## Переключение уже установленной antiopa на другую ветку
1. `kubectl -n antiopa test edit deploy/antiopa`
2. Меняем образ `registry.flant.com/sys/antiopa:stable`, например, на `registry.flant.com/sys/antiopa:ea`, чтобы переключить на ветку ea.

## Отключение модулей
Модули, которые указаны в ключе *disable-modules* в *ConfigMap* antiopa, будут выключены. Перечисляются через запятую, можно использовать glob'ы.
```
apiVersion: v1
kind: ConfigMap
metadata:
name: antiopa
data:
  disable-modules: test*, kube-dashboard
```

### Пользователь fox.flant.com

Единый пользователь fox.flant.com: antiopa.
Токен: в канале #tech-kubernetes.

Если надо поменять token получаем админа в fox, находим пользователя antiopa, меняем. После смены можно заново вызвать скрипт get-antiopa.sh одним из указанных ниже способов уже с новым token.
