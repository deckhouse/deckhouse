---
title: FAQ
permalink: ru/deckhouse-faq.html
lang: ru
---

## Как узнать все параметры Deckhouse?

Настройки конфигурации Deckhouse, включая параметры модулей, хранятся в cluster-scoped ресурсах `ModuleConfig` и custom resource’ах. ModuleConfig `global` содержит глобальные настройки Deckhouse. Подробнее — [в документации](./).

Вывести список всех ModuleConfig:

```shell
kubectl get mc
```

Вывести глобальные настройки:

```shell
kubectl get mc global -o yaml
```

## Как найти документацию по установленной у меня версии?

> Документация доступна внутри кластера при включенном модуле [deckhouse-web](modules/810-deckhouse-web/) (включен по умолчанию, кроме [варианта поставки](modules/002-deckhouse/configuration.html#parameters-bundle) `Minimal`).

Документация запущенной в кластере версии Deckhouse доступна по адресу `deckhouse.<cluster_domain>`, где `<cluster_domain>` — DNS имя в соответствии с шаблоном из параметра `global.modules.publicDomainTemplate` конфигурации.

## Как установить желаемый канал обновлений?

Чтобы перейти на другой канал обновлений автоматически, нужно в [конфигурации](modules/002-deckhouse/configuration.html#parameters-releasechannel) модуля `deckhouse` изменить (установить) параметр `releaseChannel`.

В этом случае включится механизм [автоматической стабилизации релизного канала](#как-работает-автоматическое-обновление-deckhouse).

Пример конфигурации модуля `deckhouse` с установленным каналом обновлений `Stable`:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: deckhouse
spec:
  version: 1
  settings:
    releaseChannel: Stable
```

## Как отключить автоматическое обновление?

Чтобы полностью отключить механизм обновления Deckhouse, удалите в [конфигурации](modules/002-deckhouse/configuration.html#parameters-releasechannel) модуля `deckhouse` параметр `releaseChannel`.

В этом случае Deckhouse не проверяет обновления, и даже обновление на patch-релизы не выполняется.

> Крайне не рекомендуется отключать автоматическое обновление! Это заблокирует обновления на patch-релизы, которые могут содержать исправления критических уязвимостей и ошибок.

## Как работает автоматическое обновление Deckhouse?

При указании в конфигурации модуля `deckhouse` параметра `releaseChannel`, Deckhouse будет каждую минуту проверять данные о релизе на канале обновлений.

При появлении нового релиза Deckhouse скачивает его в кластер и создает custom resource `DeckhouseRelease`.

После появления custom resource'а `DeckhouseRelease` в кластере Deckhouse выполняет обновление на соответствующую версию согласно установленным [параметрам обновления](modules/002-deckhouse/configuration.html#parameters-update) (по-умолчанию — автоматически, в любое время).

Чтобы посмотреть список и состояние всех релизов, воспользуйтесь командной:

```shell
kubectl get deckhousereleases
```

> Patch-релизы (например, обновление на версию `1.30.2` при установленной версии `1.30.1`) устанавливаются без учета режима и окон обновления, т.е. при появлении на канале обновления patch-релиза, он всегда будет установлен.

### Что происходит при смене канала обновлений?

* При смене канала обновлений на **более стабильный** (например с `Alpha` на `EarlyAccess`) Deckhouse скачивает данные о релизе (в примере — из канала `EarlyAccess`) и сравнивает их с данными из существующих в кластере custom resouce'ов `DeckhouseRelease`:
  * Более *поздние* релизы, которые еще не были применены (в статусе `Pending`), — удаляются.
  * Если более *поздние* релизы уже применены (в статусе `Deployed`), то смены релиза не происходит. В этом случае Deckhouse останется на таком релизе до тех пор, пока на канале обновлений `EarlyAccess` не появится более поздний релиз.
* При смене канала обновлений на **менее стабильный** (например с `EarlyAcess` на `Alpha`):
  * Deckhouse скачивает данные о релизе (в примере — из канала `Alpha`) и сравнивает их с данными из существующих в кластере custom resource'ов `DeckhouseRelease`.
  * Затем Deckhouse выполняет обновление согласно установленным [параметрам обновления](modules/002-deckhouse/configuration.html#parameters-update).

## Как запускать Deckhouse на произвольном узле?

Для запуска Deckhouse на произвольном узле установите у модуля `deckhouse` соответствующий [параметр](modules/002-deckhouse/configuration.html) `nodeSelector` и не задавайте `tolerations`.  Необходимые значения `tolerations` в этом случае будут проставлены автоматически.

Также стоит избегать использования узлов **CloudEphemeral**. В противном случае может произойти ситуация, когда целевого узла нет в кластере, и его заказ по какой-то причине невозможен.

Пример конфигурации модуля:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: deckhouse
spec:
  version: 1
  settings:
    nodeSelector:
      node-role.deckhouse.io/deckhouse: ""
```

## Как установить Deckhouse из стороннего registry?

При установке Deckhouse можно настроить на работу со сторонним registry (например, проксирующий registry внутри закрытого контура).

### Подготовка конфигурации

Установите следующие параметры в ресурсе `InitConfiguration`:

* `imagesRepo: <PROXY_REGISTRY>/<DECKHOUSE_REPO_PATH>/<DECKHOUSE_REVISION>` — адрес образа Deckhouse в стороннем registry с учетом используемой редакции - CE или EE. Пример: `imagesRepo: registry.deckhouse.io/deckhouse/ce`;
* `registryDockerCfg: <BASE64>` — права доступа к стороннему registry, зашифрованные в Base64.

Если разрешен анонимный доступ к образам Deckhouse в стороннем registry, то `registryDockerCfg` должен выглядеть следующим образом:

```json
{"auths": { "<PROXY_REGISTRY>": {}}}
```

Приведенное значение должно быть закодировано в Base64.

Если для доступа к образам Deckhouse в стороннем registry необходима аутентификация, то `registryDockerCfg` должен выглядеть следующим образом:

```json
{"auths": { "<PROXY_REGISTRY>": {"username":"<PROXY_USERNAME>","password":"<PROXY_PASSWORD>","auth":"<AUTH_BASE64>"}}}
```

где:

* `<PROXY_USERNAME>` — имя пользователя для аутентификации на `<PROXY_REGISTRY>`;
* `<PROXY_PASSWORD>` — пароль пользователя для аутентификации на `<PROXY_REGISTRY>`;
* `<PROXY_REGISTRY>` — адрес стороннего registry в виде `<HOSTNAME>[:PORT]`;
* `<AUTH_BASE64>` — строка вида `<PROXY_USERNAME>:<PROXY_PASSWORD>`, закодированная в Base64.

Итоговое значение для `registryDockerCfg` должно быть также закодировано в Base64.

Для настройки нестандартных конфигураций сторонних registry в ресурсе `InitConfiguration` предусмотрены еще два параметра:

* `registryCA` — корневой сертификат, которым можно проверить сертификат registry (если registry использует самоподписанные сертификаты);
* `registryScheme` — протокол доступа к registry (`HTTP` или `HTTPS`). По умолчанию - `HTTPS`.

### Особенности настройки сторонних registry

> **Внимание!** Deckhouse поддерживает работу только с Bearer token-схемой авторизации в registry.

#### Nexus

##### Требования

При использовании менеджера репозиториев [Nexus](https://github.com/sonatype/nexus-public) должны быть выполнены следующие требования:

* Включен `Docker Bearer Token Realm`.
* Создан проксирующий репозиторий Docker.
* Параметр `Allow anonymous docker pull` должен быть включен.
* Должен быть настроен контроль доступа:
  * Создана роль Nexus с полномочиями `nx-repository-view-docker-<репозиторий>-browse` и `nx-repository-view-docker-<репозиторий>-read`.
  * Создан пользователь Nexus с ролью, созданной выше.
* Параметр `Maximum metadata age` созданного репозитория должен быть установлен в 0.

##### Настройка

* Включите `Docker Bearer Token Realm`:
  ![Включение `Docker Bearer Token Realm`](images/registry/nexus/nexus-realm.png)

* Создайте проксирующий репозиторий Docker, указывающий на [Deckhouse registry](https://registry.deckhouse.io/):
  ![Создание проксирующего репозитория Docker](images/registry/nexus/nexus-repository.png)

* Заполните поля страницы создания следующим образом:
  * `Name` должно содержать имя создаваемого репозитория, например `d8-proxy`.
  * `Repository Connectors / HTTP` или `Repository Connectors / HTTPS` должно содержать выделенный порт для создаваемого репозитория, например, `8123` или иной.
  * `Allow anonymous docker pull` должно быть включено, чтобы [работала](https://help.sonatype.com/repomanager3/system-configuration/user-authentication#UserAuthentication-security-realms) авторизация с помощью Bearer-токенов, при этом анонимный доступ [не будет работать](https://help.sonatype.com/repomanager3/nexus-repository-administration/formats/docker-registry/docker-authentication#DockerAuthentication-UnauthenticatedAccesstoDockerRepositories), если он не был явно включен в Settings -> Security -> Anonymous Access, и пользователю `anonymous` не были даны права на доступ к репозиторию.
  * `Remote storage` должно иметь значение `https://registry.deckhouse.io/`.
  * `Auto blocking enabled` и `Not found cache enabled` могут быть выключены для отладки; в противном случае их следует включить.
  * `Maximum Metadata Age` должно быть равно 0.
  * Если планируется использовать Deckhouse Enterprise Edition, флажок `Authentication` должен быть включен, а связанные поля должны быть заполнены следующим образом:
    * `Authentication Type` должно иметь значение `Username`.
    * `Username` должно иметь значение `license-token`.
    * `Password` должно содержать ключ лицензии Deckhouse Enterprise Edition.

  ![Пример настроек репозитория 1](images/registry/nexus/nexus-repo-example-1.png)
  ![Пример настроек репозитория 2](images/registry/nexus/nexus-repo-example-2.png)
  ![Пример настроек репозитория 3](images/registry/nexus/nexus-repo-example-3.png)

* Настройте контроль доступа Nexus для доступа Deckhouse к созданному репозиторию:
  * Создайте роль Nexus с полномочиями `nx-repository-view-docker-<репозиторий>-browse` и `nx-repository-view-docker-<репозиторий>-read`.
    ![Создание роли Nexus](images/registry/nexus/nexus-role.png)
  * Создайте пользователя Nexus с ролью, созданной выше.
    ![Создание пользователя Nexus](images/registry/nexus/nexus-user.png)

#### Harbor

Необходимо использовать такой функционал [Harbor](https://github.com/goharbor/harbor), как Proxy Cache.

* Настройте Registry:
  * `Administration -> Registries -> New Endpoint`.
  * `Provider`: `Docker Registry`.
  * `Name` — укажите любое, на ваше усмотрение.
  * `Endpoint URL`: `https://registry.deckhouse.io`.
  * Укажите `Access ID` и `Access Secret`, если используете Deckhouse Enterprise Edition, иначе оставьте пустыми.

![Настройка Registry](images/registry/harbor/harbor1.png)

* Создайте новый проект:
  * `Projects -> New Project`.
  * `Project Name` будет частью URL. Используйте любой, например, `d8s`.
  * `Access Level`: `Public`.
  * `Proxy Cache` — включите и выберите в списке Registry, созданный на предыдущем шаге.

![Создание нового проекта](images/registry/harbor/harbor2.png)

В результате образы Deckhouse будут доступны по адресу, например, `https://your-harbor.com/d8s/deckhouse/{d8s-edition}:{d8s-version}`.

### Ручная загрузка образов в изолированный приватный registry

Загрузите скрипт на хост, с которого есть доступ до `registry.deckhouse.io`. `Docker`, `crane` и `jq` должны быть установлены на хосте.

```shell
curl -fsSL -o d8-pull.sh https://raw.githubusercontent.com/deckhouse/deckhouse/main/tools/release/d8-pull.sh
chmod 700 d8-pull.sh
```

Пример вызова команды скачивания образов:

```shell
./d8-pull.sh --license YOUR_DECKHOUSE_LICENSE_KEY --output-dir /your/output-dir/
```

Загрузите директорию с образами, полученную на предыдущем шаге, на хост, с которого есть доступ до изолированного приватного registry. `Crane` должен быть установлен на хосте.
Загрузите скрипт на этот хост:

```shell
curl -fsSL -o d8-push.sh https://raw.githubusercontent.com/deckhouse/deckhouse/main/tools/release/d8-push.sh
chmod 700 d8-push.sh
```

Пример вызова команды загрузки образов в изолированный приватный registry:

```shell
./d8-push.sh --source-dir /your/source-dir/ --path your.private.registry.com/deckhouse --username YOUR_USERNAME --password YOUR_PASSWORD
```

## Как создать кластер и запустить Deckhouse без использования каналов обновлений?

Данный способ следует использовать только в случае, если в изолированном приватном registry нет образов, содержащих информацию о каналах обновлений.

* Если вы хотите установить Deckhouse с отключенным автоматическим обновлением:
  * Используйте тэг образа установщика соответствующей версии. Например, если вы хотите установить релиз `v1.44.3`, то используйте образ `your.private.registry.com/deckhouse/install:v1.44.3`.
  * Укажите соответствующий номер версии в параметре [deckhouse.devBranch](installing/configuration.html#initconfiguration-deckhouse-devbranch) в ресурсе `InitConfiguration`.
  * **Не указывайте** параметр [deckhouse.releaseChannel](installing/configuration.html#initconfiguration-deckhouse-releasechannel) в ресурсе `InitConfiguration`.
* Если вы хотите отключить автоматические обновления у уже установленного Deckhouse (включая обновления patch-релизов), то удалите параметр [releaseChannel](modules/002-deckhouse/configuration.html#parameters-releasechannel) из конфигурации модуля `deckhouse`.

## Использование proxy-сервера

### Настройка proxy-сервера

Пример шагов по настройке proxy-сервера на базе Squid:
* Подготовьте сервер (или виртуальную машину). Сервер должен быть доступен с необходимых узлов кластера, и у него должен быть выход в интернет.
* Установите Squid (здесь и далее примеры для Ubuntu):

  ```shell
  apt-get install squid
  ```

* Создайте файл конфигурации Squid:

  ```shell
  cat <<EOF > /etc/squid/squid.conf
  auth_param basic program /usr/lib/squid3/basic_ncsa_auth /etc/squid/passwords
  auth_param basic realm proxy
  acl authenticated proxy_auth REQUIRED
  http_access allow authenticated
  
  # Choose the port you want. Below we set it to default 3128.
  http_port 3128
  ```

* Создайте пользователя и пароль для аутентификации на proxy-сервере:

  Пример для пользователя `test` с паролем `test` (обязательно измените):

  ```shell
  echo "test:$(openssl passwd -crypt test)" >> /etc/squid/passwords
  ```

* Запустите Squid и включите его автоматический запуск при загрузке сервера:

  ```shell
  systemctl restart squid
  systemctl enable squid
  ```

### Настройка Deckhouse на использование proxy

Для настройки работы через proxy-сервер используйте параметр [proxy](installing/configuration.html#clusterconfiguration-proxy) ресурса `ClusterConfiguration`.

Пример:

```yaml
apiVersion: deckhouse.io/v1
kind: ClusterConfiguration
clusterType: Cloud
cloud:
  provider: OpenStack
  prefix: main
podSubnetCIDR: 10.111.0.0/16
serviceSubnetCIDR: 10.222.0.0/16
kubernetesVersion: "1.23"
cri: "Containerd"
clusterDomain: "cluster.local"
proxy:
  httpProxy: "http://user:password@proxy.company.my:3128"
  httpsProxy: "https://user:password@proxy.company.my:8443"
```

## Как переключить работающий кластер Deckhouse на использование стороннего registry?

Для переключения кластера Deckhouse на использование стороннего registry выполните следующие действия:

* Измените поле `image` в Deployment `d8-system/deckhouse` на адрес образа Deckhouse в новом registry;
* Скачайте скрипт на мастер-узел и запустите его с параметрами нового registry.
  * Пример запуска:

  ```shell
  curl -fsSL -o change-registry.sh https://raw.githubusercontent.com/deckhouse/deckhouse/main/tools/change-registry.sh
  chmod 700 change-registry.sh
  ./change-registry.sh --registry-url https://my-new-registry/deckhouse --user my-user --password my-password
  ```

  * Если registry использует самоподписные сертификаты, то положите корневой сертификат соответствующего сертификата registry в файл `ca.crt` возле скрипта и добавьте к вызову опцию `--ca-file ca.crt`.
* Дождитесь перехода Pod'а Deckhouse в статус `Ready`. Если Pod будет находиться в статусе `ImagePullBackoff`, то перезапустите его.
* Дождитесь применения bashible новых настроек на master-узле. В журнале bashible на master-узле (`journalctl -u bashible`) должно появится сообщение `Configuration is in sync, nothing to do`.
* Если необходимо отключить автоматическое обновление Deckhouse через сторонний registry, то удалите параметр `releaseChannel` из конфигурации модуля `deckhouse`.
* Проверьте, не осталось ли в кластере Pod'ов с оригинальным адресом registry:

  ```shell
  kubectl get pods -A -o json | jq '.items[] | select(.spec.containers[] | select((.image | contains("deckhouse.io")))) 
    | .metadata.namespace + "\t" + .metadata.name' -r
  ```

## Как переключить Deckhouse EE на CE?

> Инструкция подразумевает использование публичного адреса container registry: `registry.deckhouse.io`. В случае использования другого адреса container registry измените команды или воспользуйтесь [инструкцией по переключению Deckhouse на использование стороннего registry](#как-переключить-работающий-кластер-deckhouse-на-использование-стороннего-registry).

Для переключения кластера Deckhouse Enterprise Edition на Community Edition выполните следующие действия:

1. Убедитесь, что используемые в кластере модули [поддерживаются в версии CE](revision-comparison.html). Отключите модули, которые не поддерживаются в Deckhouse CE.

   > Обратите внимание, что в Deckhouse CE не поддерживается работа облачных кластеров на OpenStack и VMware vSphere.
1. Выполните следующую команду:

   ```shell
   bash -c "$(curl -Ls https://raw.githubusercontent.com/deckhouse/deckhouse/main/tools/change-registry.sh)" -- --registry-url https://registry.deckhouse.io/deckhouse/ce && \
   kubectl -n d8-system set image deployment/deckhouse deckhouse=$(kubectl -n d8-system get deployment deckhouse -o jsonpath='{.spec.template.spec.containers[?(@.name=="deckhouse")].image}' | awk -F: '{print "registry.deckhouse.io/deckhouse/ce:" $2}')    
   ```

1. Дождитесь перехода Pod'а Deckhouse в статус `Ready`:

   ```shell
   kubectl -n d8-system get po -l app=deckhouse
   ```

1. Если Pod будет находиться в статусе `ImagePullBackoff`, то перезапустите его:

   ```shell
   kubectl -n d8-system delete po -l app=deckhouse
   ```

1. Дождитесь перезапуска Deckhouse и выполнения всех задач в очереди:

   ```shell
   kubectl -n d8-system exec deploy/deckhouse -- deckhouse-controller queue main | grep status:
   ```

   Пример вывода, когда в очереди еще есть задания (`length 38`):

   ```console
   # kubectl -n d8-system exec deploy/deckhouse -- deckhouse-controller queue main | grep status:
   Queue 'main': length 38, status: 'run first task'
   ```

   Пример вывода, когда очередь пуста (`length 0`):

   ```console
   # kubectl -n d8-system exec deploy/deckhouse -- deckhouse-controller queue main | grep status:
   Queue 'main': length 0, status: 'waiting for task 0s'
   ```

1. На master-узле проверьте применение новых настроек.

   В журнале systemd-сервиса bashible на master-узле должно появиться сообщение `Configuration is in sync, nothing to do`.

   Пример:

   ```console
   # journalctl -u bashible -n 5
   Jan 12 12:38:20 demo-master-0 bashible.sh[868379]: Configuration is in sync, nothing to do.
   Jan 12 12:38:20 demo-master-0 systemd[1]: bashible.service: Deactivated successfully.
   Jan 12 12:39:18 demo-master-0 systemd[1]: Started Bashible service.
   Jan 12 12:39:19 demo-master-0 bashible.sh[869714]: Configuration is in sync, nothing to do.
   Jan 12 12:39:19 demo-master-0 systemd[1]: bashible.service: Deactivated successfully.
   ```

1. Проверьте, не осталось ли в кластере Pod'ов с адресом registry для Deckhouse EE:

   ```shell
   kubectl get pods -A -o json | jq '.items[] | select(.spec.containers[] | select((.image | contains("deckhouse.io/deckhouse/ee")))) 
     | .metadata.namespace + "\t" + .metadata.name' -r | sort | uniq
   ```

   Иногда, могут оставаться запущенными некоторые static Pod'ы (например, `kubernetes-api-proxy-*`). Это связанно с тем, что kubelet не перезапускает Pod несмотря на изменение соответствующего манифеста, т.к. используемый образ одинаков для редакций Deckhouse CE и EE. Выполните на любом master-узле следующую команду, чтобы убедиться что соответствующие манифесты также были изменены:

   ```shell
   grep -ri 'deckhouse.io/deckhouse/ee' /etc/kubernetes | grep -v backup
   ```

   Вывод команды должен быть пуст.

## Как переключить Deckhouse CE на EE?

Вам потребуется действующий лицензионный ключ (вы можете [запросить временный ключ](https://deckhouse.ru/products/enterprise_edition.html) при необходимости).  

> Инструкция подразумевает использование публичного адреса container registry: `registry.deckhouse.io`. В случае использования другого адреса container registry измените команды или воспользуйтесь [инструкцией по переключению Deckhouse на использование стороннего registry](#как-переключить-работающий-кластер-deckhouse-на-использование-стороннего-registry).

Для переключения кластера Deckhouse Community Edition на Enterprise Edition выполните следующие действия:

1. Выполните следующую команду:

   ```shell
   LICENSE_TOKEN=<PUT_YOUR_LICENSE_TOKEN_HERE>
   bash -c "$(curl -Ls https://raw.githubusercontent.com/deckhouse/deckhouse/main/tools/change-registry.sh)" -- --user license-token --password $LICENSE_TOKEN --registry-url https://registry.deckhouse.io/deckhouse/ee && \
   kubectl -n d8-system set image deployment/deckhouse deckhouse=$(kubectl -n d8-system get deployment deckhouse -o jsonpath='{.spec.template.spec.containers[?(@.name=="deckhouse")].image}' | awk -F: '{print "registry.deckhouse.io/deckhouse/ee:" $2}') 
   ```

1. Дождитесь перехода Pod'а Deckhouse в статус `Ready`:

   ```shell
   kubectl -n d8-system get po -l app=deckhouse
   ```

1. Если Pod будет находиться в статусе `ImagePullBackoff`, то перезапустите его:

   ```shell
   kubectl -n d8-system delete po -l app=deckhouse
   ```

1. Дождитесь перезапуска Deckhouse и выполнения всех задач в очереди:

   ```shell
   kubectl -n d8-system exec deploy/deckhouse -- deckhouse-controller queue main | grep status:
   ```

   Пример вывода, когда в очереди еще есть задания (`length 38`):

   ```console
   # kubectl -n d8-system exec deploy/deckhouse -- deckhouse-controller queue main | grep status:
   Queue 'main': length 38, status: 'run first task'
   ```

   Пример вывода, когда очередь пуста (`length 0`):

   ```console
   # kubectl -n d8-system exec deploy/deckhouse -- deckhouse-controller queue main | grep status:
   Queue 'main': length 0, status: 'waiting for task 0s'
   ```

1. На master-узле проверьте применение новых настроек.

   В журнале systemd-сервиса bashible на master-узле должно появиться сообщение `Configuration is in sync, nothing to do`.

   Пример:

   ```console
   # journalctl -u bashible -n 5
   Jan 12 12:38:20 demo-master-0 bashible.sh[868379]: Configuration is in sync, nothing to do.
   Jan 12 12:38:20 demo-master-0 systemd[1]: bashible.service: Deactivated successfully.
   Jan 12 12:39:18 demo-master-0 systemd[1]: Started Bashible service.
   Jan 12 12:39:19 demo-master-0 bashible.sh[869714]: Configuration is in sync, nothing to do.
   Jan 12 12:39:19 demo-master-0 systemd[1]: bashible.service: Deactivated successfully.
   ```

1. Проверьте, не осталось ли в кластере Pod'ов с адресом registry для Deckhouse CE:

   ```shell
   kubectl get pods -A -o json | jq '.items[] | select(.spec.containers[] | select((.image | contains("deckhouse.io/deckhouse/ce")))) 
     | .metadata.namespace + "\t" + .metadata.name' -r | sort | uniq
   ```

   Иногда, могут оставаться запущенными некоторые static Pod'ы (например, `kubernetes-api-proxy-*`). Это связанно с тем, что kubelet не перезапускает Pod несмотря на изменение соответствующего манифеста, т.к. используемый образ одинаков для редакций Deckhouse CE и EE. Выполните на любом master-узле следующую команду, чтобы убедиться что соответствующие манифесты также были изменены:

   ```shell
   grep -ri 'deckhouse.io/deckhouse/ce' /etc/kubernetes | grep -v backup
   ```

   Вывод команды должен быть пуст.

## Как изменить конфигурацию кластера

Общие параметры кластера хранятся в структуре [ClusterConfiguration](installing/configuration.html#clusterconfiguration).

Чтобы изменить общие параметры кластера, выполните команду:

```shell
kubectl -n d8-system exec -ti deploy/deckhouse -- deckhouse-controller edit cluster-configuration
```

После сохранения изменений Deckhouse приведет конфигурацию кластера к измененному состоянию. В зависимости от размеров кластера это может занять какое-то время.

## Как изменить конфигурацию облачного провайдера в кластере?

Настройки используемого облачного провайдера в облачном или гибридном кластере хранятся в структуре `<PROVIDER_NAME>ClusterConfiguration`, где `<PROVIDER_NAME>` — название/код провайдера. Например, для провайдера OpenStack структура будет называться [OpenStackClusterConfiguration]({% if site.mode == 'local' and site.d8Revision == 'CE' %}{{ site.urls[page.lang] }}/documentation/v1/{% endif %}modules/030-cloud-provider-openstack/cluster_configuration.html).

Независимо от используемого облачного провайдера, его настройки можно изменить с помощью команды:

```shell
kubectl -n d8-system exec -ti deploy/deckhouse -- deckhouse-controller edit provider-cluster-configuration
```

## Как изменить конфигурацию статического кластера?

Настройки статического кластера хранятся в структуре [StaticClusterConfiguration](installing/configuration.html#staticclusterconfiguration).

Чтобы изменить параметры статического кластера, выполните команду:

```shell
kubectl -n d8-system exec -ti deploy/deckhouse -- deckhouse-controller edit static-cluster-configuration
```

## Как обновить версию Kubernetes в кластере?

Чтобы обновить версию Kubernetes в кластере, измените параметр [kubernetesVersion](installing/configuration.html#clusterconfiguration-kubernetesversion) в структуре [ClusterConfiguration](installing/configuration.html#clusterconfiguration) выполнив следующие шаги:
1. Выполните команду:

   ```shell
   kubectl -n d8-system exec -ti deploy/deckhouse -- deckhouse-controller edit cluster-configuration
   ```

1. Измените параметр `kubernetesVersion`.
1. Сохраните изменения. Узлы кластера начнут последовательно обновляться.
1. Дождитесь окончания обновления.  Отслеживать ход обновления можно с помощью команды `kubectl get no`. Обновление можно считать завершенным, когда в выводе команды у каждого узла кластера в колонке `VERSION` появится обновленная версия.
