# Bashible apiserver

## Что делает

Генерирует скрипты для bashible, отдает их через апи куба.

## Зачем этот аписервер

Количество комбинаций шагов в bashible множится такими факторами:

* количество шагов в бандле для ОС
* количество версий куба
* количество поддерживаемых дистрибутивов на узлах
* количество групп узлов (`NodeGroup`)

Раньше скрипты хранились в сгенерированном виде в секретах. Содержание этих секретов дополнительно копилось в едином
секрете хелма. Секрет хелма подобрался к лимиту объекта в etcd (2 Mi). То есть этот способ больше не масштабируется.

Теперь скрипты обслуживаются не секретами чарта, а отдельным сервером. Так можно растить объем и количество скриптов и
экономить время на генерации шаблонов в декхаусе при конверже.

## Как аписервер работает

Bashible раньше обращался к апи куба за секретами по имени.

```
GET /api/v1/namespaces/d8-cloud-instance-manager/secrets/bashible-bundle-ubuntu-lts-1.19
```

Теперь башибл по имени обращается за бандлами (=наборами шагов). Данные бандлов не секретные, поэтому они не кодируются
в base64. Объекты аписервера существуют вне неймспейсов.

Аписервер реализует только операцию `Get`, то есть можно взять бандл по имени, список бандлов не реализован. Новый
башибл берет объекты с помощью kubectl:

```shell
kubectl get -o json  bashibles         ubuntu-lts.master    # <os>.<nodegroup>
kubectl get -o json  kubernetesbundles ubuntu-lts.1-19      # <os>.<version>
kubectl get -o json  nodegroupbundles  ubuntu-lts.master    # <os>.<nodegroup>
```

Объекты доступны напрямую в апи так:

```
GET /api/bashible.deckhouse.io/v1alpha1/bashibles/ubuntu-lts.master
GET /api/bashible.deckhouse.io/v1alpha1/kubernetesbundles/ubuntu-lts.1-19
GET /api/bashible.deckhouse.io/v1alpha1/nodegroupbundles/ubuntu-lts.master
```

Аписервер генерирует содержание на лету для запрошенного бандла. Шаблоны находятся в контейнере сервера. Все объекты
содержат поле `data map[string]string`, внутри которого карта скриптов, где ключ — имя скрипта, а значение — содержание.
Поле `metadata.creationTimestamp` генерируется на лету из текущего времени. Пример для `bashible`:

```shell
kubectl get -o json bashibles ubuntu-lts.master
{
  "kind": "Bashible",
  "metadata": {
    "creationTimestamp": "2021-02-08T07:59:25Z",
    "name": "ubuntu-lts.master"
  },
  "data": {
    "bashible.sh": "#!/usr/bin/env bash\n\nset -Eeo pipefail\n\nfunction kubectl_exec() {\n ..."
  }
}
```

## Как устроен код

Код основан на репозитории [sample-apiserver](https://https://github.com/kubernetes/sample-apiserver).

Чтобы генерировать бойлерплейт для объектов в апи, используется пакет `code-generator`. Сгенерированный код коммитят в
репозиторий, потому что он используется в коде, который пишут люди сами (как в кубе). Чтобы перегенерировать код, нужно
вызывать скрипт

```shell
./hack/update-codegen.sh
```

### Версии библиотек могут обгонять целевой кластер

Среди прочего скрипт кодогенерации отвечает за спецификацию Openapi для сущностей аписервера. В версии пакетов 0.19.*
эта генерация сломана, в 0.20.* она работает. Поэтому сервер использует версии библиотек 0.20+, которые могут обгонять
совместимость с текущим кластером. (Эту ситуацию, возможно, можно решить и без использования несовместимых библиотек).

Например, фича [API Priority and Fairness](https://kubernetes.io/docs/concepts/cluster-administration/flow-control/),
появилась в 1.20. Поэтому в кубернетесе 1.19 и более ранних версий этот аписервер будет писать в выводе ошибок сообщения
о недоступности группы апи `flowcontrol.apiserver.k8s.io` ( `flowschemas` и `PriorityLevelConfiguration`).

### Шаблоны

Шаблоны bashible и его шагов добавляются в контейнер на стадии сборки из каталога `candi/bashible`. Контекст для этих
шаблонов собирается в конфигмап и монтируется файлом в поде:

```shell
tree /bashible
/bashible
├── context.yaml   # монтируется из конфигмапа
└── templates      # добавляется во время сборки
    ├── bashible
    │   ├── bashible.sh.tpl
    │   ├── bundles
    │   │   ├── <os>
    │   │   │   ├── all
    │   │   │   │   └── <>.sh.tpl
    │   │   │   └── node-group
    │   │   │       └── <>.sh.tpl
    │   └── common-steps
    │       ├── all
    │       │   └── <>.sh.tpl
    │       └── node-group
    │           └── <>.sh.tpl
    └── cloud-providers
        └── <provider>
            └── bashible
                ├── bundles
                │   └── <os>
                │       └── ...
                └── common-steps
                    └── ...
```
