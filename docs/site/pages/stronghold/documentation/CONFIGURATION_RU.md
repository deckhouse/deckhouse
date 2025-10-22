---
title: "Настройка Deckhouse Stronghold"
permalink: ru/stronghold/documentation/configuration.html
lang: ru
---


## Включение модуля

Включите модуль можно выполнив команду:

```shell
d8 p module enable stronghold
```

По умолчанию модуль запустится в режиме `Automatic` с инлетом `Ingress`.
В текущей версии другие режимы и инлеты отсутствуют.

## Как выключить модуль

Выключить модуль можно выполнив команду:

```shell
d8 p module disable stronghold
```

{% alert level="danger" %}
При отключении модуля удалятся все контейнеры Stronghold из неймспейса `d8-stronghold`, а так же секрет `stronghold-keys` с root и unseal ключами. При этом данные сервиса не удалятся с узла. Вы можете включить модуль снова, создать и поместить в неймспейс `d8-stronghold` сохраненную копию секрета `stronghold-keys`, тогда доступ к данным будет восстановлен.
{% endalert %}

Если старые данные больше не нужны, нужно предварительно удалить каталог `/var/lib/deckhouse/stronghold`
со всех master-узлов кластера.

## Получение доступа к сервису

Доступ к сервису осуществляется через инлеты. Инлет - это источник входных данных для пода. В примере доступен один инлет - `Ingress`.

Адрес веб-интерфейса Stronghold формируется следующим образом: в шаблоне [`publicDomainTemplate`](/products/kubernetes-platform/documentation/v1/reference/api/global.html#parameters-modules-publicdomaintemplate) глобального параметра конфигурации Deckhouse ключ `%s` заменяется на `stronghold`.
Например, если `publicDomainTemplate` установлен как `%s-kube.mycompany.tld`, веб-интерфейс Stronghold будет доступен по адресу `stronghold-kube.cmycompany.tld`.

## Использование хранилища данных. Режимы работы

Информация, содержащаяся в Stronghold, защищена шифрованием. Для того чтобы раскрыть данные хранилища, нужен ключ шифрования. Этот ключ также сохраняется вместе с данными (в хранилище ключей), однако он зашифрован иным ключом шифрования, который известен как корневой ключ.

Для раскрытия данных Stronghold расшифрует ключ шифрования, требующий для этого корневой ключ. Доступ к корневому ключу можно получить с помощью процесса, называемого разблокировкой хранилища. Корневой ключ сохраняется вместе со всеми остальными данными хранилища, однако шифруется еще одной технологией: ключом разблокировки.

В текущей версии модуля присутствует только режим `Automatic`, в котором при первом запуске модуля происходит автоматическая инициализация хранилища. В процессе инициализации ключ разблокирования и root-token помещаются в секрет `stronghold-keys` неймспейса kubernetes `d8-stronghold`. После инициализации модуль автоматически разблокирует узлы кластера Stronghold.
В автоматическом режиме, при перезапуске узлов Stronghold, хранилище также будет автоматически разблокировано без вмешательства пользователя.

## Управление доступами

В автоматическом режиме `Automatic` в Stronghold после инициализации хранилища создается роль `deckhouse_administrators`, для которой включается доступ к веб-интерфейсу через OIDC аутентификацию [Dex](/modules/user-authn/).
Также настраивается автоматическое подключение текущего кластера Deckhouse к Stronghold для работы модуля [`secrets-store-integration`](/modules/secrets-store-integration/stable/).

Для того, чтоб выдать пользователям, находящимся в группе `admins` (членство в группе передаётся из используемого IdP или LDAP с помощью [Dex](/modules/user-authn/)), нужно указать эту группу в массиве `administrators` в `ModuleConfig`:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: stronghold
spec:
  enabled: true
  version: 1
  settings:
    management:
      mode: Automatic
      administrators:
      - type: Group
        name: admins
```

Для того, чтоб выдать права `administrator` пользователям `manager` и `securityoperator`, можно использовать следующие параметры в `ModuleConfig`:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: stronghold
spec:
  enabled: true
  version: 1
  settings:
    management:
      mode: Automatic
      administrators:
      - type: User
        name: manager@mycompany.tld
      - type: User
        name: securityoperator@mycompany.tld
```

Несмотря на то, что доступ можно выдавать конкретным пользователям индивидуально, при этом сами пользователи должны находиться в какой-либо группе из-за ограничений аутентификации через OIDC.

В дальнейшем можно создать пользователей в Stronghold с различными правами доступа к секретам с помощью встроенного механизма хранилища.

## Первый запуск

Первый запуск подразумевает отсутствие папки `/var/lib/deckhouse/stronghold` в файловой системе узлов, на которых будут запускаться узлы *Stronghold* (по умолчанию это master-узлы) и [отключенный модуль *Stronghold*](#как-выключить-модуль).

> Так же нужен опыт работы с утилитой `kubectl`

Ниже приведены варианты организации доступа к модулю через [инлет Ingress](/modules/ingress-nginx/) и далее процесс включения модуля и проверки работоспособности.

### Способы организации доступа через инлет Ingress

#### ClusterIssuer LetsEncrypt

Этот метод получения сертификата настроен по умолчанию. Однако, подойдёт только для сервисов доступных **из Интернета** (не для внутренних сетей). Выполняем проверку доступности:
1. Получаем адрес платформы аутентификации командой:

    ```shell
    d8 k -n d8-user-authn get ing dex
    # Ожидаемый ответ
    # NAME   CLASS   HOSTS               ADDRESS         PORTS     AGE
    # dex    nginx   dex.mycompany.tld   34.85.243.109   80, 443   4d20h
    ```

    Под столбцом `HOSTS` наш проверяемый домен, а под `ADDRESS` – его IP адрес. Теперь нужно убедиться, что домен правильно резолвится на указанный IP адрес. Для этого выполняем команду:

    ```shell
    nslookup dex.mycompany.tld 8.8.8.8
    # Ожидаемый ответ
    # ...
    # Name: dex.mycompany.tld
    # Address: 34.85.243.109
    # ...

    # Либо
    dig @8.8.8.8 dex.mycompany.tld
    # Ожидаемый ответ
    # ...
    # ;; ANSWER SECTION:
    # dex.mycompany.tld. 3600 IN A 34.85.243.109
    # ...
    ```

    Если ответом стала ошибка с кодом `NXDOMAIN`, нужно настроить DNS пользователя.
2. В браузере открываем <https://dex.mycompany.tld/healthz>, либо выполняем команду `curl -kL https://dex.mycompany.tld/healthz`. Должен вернуться ответ `Health check passed`.
3. Проверяем, что Ingress контроллер обрабатывает запросы на ваш поддомен `stronghold.mycompany.tld`. Снова в браузере, либо командой `curl -kL` открываем <https://stronghold.mycompany.tld>. Должна вернуться 404 ошибка.

#### ClusterIssuer с самоподписанным центром сертификации

Эта опция подходит, если вы хотите использовать свой самоподписанный Центр сертификации. В качестве примера мы будем использовать уже созданный `ClusterIssuer` ресурс **selfsigned**. Для добавления Issuer или ClusterIssuer со своим самоподписанным Центром сертификации, воспользуйтесь [официальной документацией](https://cert-manager.io/docs/configuration/ca/)

> Для этого способа подойдут как наличие публичного доменного имени, так и доступ только из внутренней сети.

Редактируем настройки **global** модуля. Сделать это можно, например, командой `d8 k edit mc global`.
Добавляем параметр `settings.modules.https.certManager.clusterIssuerName: selfsigned`. В результате конфигурация модуля должна выглядеть так:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: global
spec:
  settings:
    modules:
      https:
        certManager:
          clusterIssuerName: selfsigned    # Единственный параметр, который нужно добавить
      publicDomainTemplate: '%s.mycompany.tld'
  version: 1
```

Далее редактируем настройки **user-authn** модуля. Выполняем команду `d8 k edit mc user-authn` и изменяем параметр `settings.controlPlaneConfigurator.dexCAMode` на `FromIngressSecret`:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: user-authn
spec:
  enabled: true
  settings:
    controlPlaneConfigurator:
      dexCAMode: FromIngressSecret    # Параметр, который нужно изменить
  ...
```

Перед запуском модуля убедимся, что ключевые сервисы доступны из **рабочей сети**.
1. Получаем адрес платформы аутентификации командой:

    ```shell
    d8 k -n d8-user-authn get ing dex
    # Ожидаемый ответ
    # NAME   CLASS   HOSTS               ADDRESS         PORTS     AGE
    # dex    nginx   dex.mycompany.tld   34.85.243.109   80, 443   4d20h
    ```

    Под столбцом `HOSTS` наш проверяемый домен, а под `ADDRESS` – его IP адрес. Теперь нужно убедиться, что домен правильно резолвится на указанный IP адрес. Для этого выполняем команду:

    ```shell
    nslookup dex.mycompany.tld
    # Ожидаемый ответ
    # ...
    # Name: dex.mycompany.tld
    # Address: 34.85.243.109
    # ...

    # Либо
    dig dex.mycompany.tld
    # Ожидаемый ответ
    # ...
    # ;; ANSWER SECTION:
    # dex.mycompany.tld. 3600 IN A 34.85.243.109
    # ...
    ```

    Если ответом стала ошибка с кодом `NXDOMAIN`, нужно настроить DNS пользователя. Если домен не доступен из Интернета, дополнительно необходимо выполнить [дополнительный шаг](#не-резолвится-доменное-имя-dexmycompanytld)
    > Как временное решение можно добавить следующую строку в файл `/etc/hosts` вашей Unix системы
    >
    > ```shell
    > 34.85.243.109 dex.mycompany.tld stronghold.mycompany.tld
    > ```
    >
2. В браузере открываем <https://dex.mycompany.tld/healthz>, либо выполняем команду `curl -kL https://dex.mycompany.tld/healthz`. Должен вернуться ответ `Health check passed`.
3. Проверяем, что Ingress контроллер обрабатывает запросы на ваш поддомен `stronghold.mycompany.tld`. Снова в браузере, либо командой `curl -kL` открываем <https://stronghold.mycompany.tld>. Должна вернуться 404 ошибка.

#### Используя файл сертификата

Нужно создать СА, сертификат, и подписать его созданным СА. Если уже есть СА, сертификат можно подписать существующим.
Важно сделать сертификат с цепочкой (fullchain).

Ниже представлен скрипт `createCertificate.sh`, который с помощью openssl создает нужную пару сертификат + ключ
для домена `mycompany.tld` (`*.mycompany.tld`).

```shell
#!/bin/bash

set -e
caName="MyOrg-RootCA"            # Имя CA (CN)
publicDomain="mycompany.tld"     # Имя кластерного домена (см. publicDomainTemplate)
certName="kubernetes"            # Имя сертификата для кластера (CN)

mkdir -p "${caName}"
cd "${caName}"

[ ! -f "${caName}.key" ] && openssl genrsa -out "${caName}.key" 4096

[ ! -f "${caName}.crt" ] &&  openssl req -x509 -new -nodes -key "${caName}.key" -sha256 -days 1826 -out "${caName}.crt" \
   -subj "/CN=${caName}/O=MyOrganisation"

openssl req -new -nodes -out ${certName}.csr -newkey rsa:4096 -keyout "${certName}.key" \
  -subj "/CN=${certName}/O=MyOrganisation"

# v3 ext file
cat > "${certName}.v3.ext" << EOF
authorityKeyIdentifier=keyid,issuer
basicConstraints=CA:FALSE
keyUsage = digitalSignature, nonRepudiation, keyEncipherment, dataEncipherment
subjectAltName = @alt_names
[alt_names]
DNS.1 = ${publicDomain}
DNS.2 = *.${publicDomain}
EOF

openssl x509 -req -in "${certName}.csr" -CA "${caName}.crt" -CAkey "${caName}.key" -CAcreateserial -out "${certName}.crt" -days 730 -sha256 -extfile "${certName}.v3.ext"

cat "${certName}.crt" "${caName}.crt" > "${certName}_fullchain.crt"
```

Используя полученные файлы `kubernetes.key` и `kubernetes_fullchain.crt` нужно создать секрет в неймспейсе d8-system

```shell
d8 k -n d8-system create secret tls mycompany-wildcard-tls --cert=kubernetes_fullchain.crt --key=kubernetes.key
```

Для использования полученного сертификата в кластере нужно привести конфигурацию модуля `global` к такому виду.
Сделать это можно например командой `d8 k edit mc global`

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: global
spec:
  settings:
    modules:
      https:
        customCertificate:
          secretName: mycompany-wildcard-tls    # здесь указываем название объекта secret, содержащий fullchain сертификат и ключ
        mode: CustomCertificate                 # меняем режим работы с tls для всех модулей
      publicDomainTemplate: '%s.mycompany.tld'
```

Так же требуется настроить модуль `user-authn`, включив в настройках `controlPlaneConfigurator.dexCAMode` в значение `FromIngressSecret`
В этом случае CA будет получен из цепочки, которую мы поместили в файл `kubernetes_fullchain.crt`

Пример

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: user-authn
spec:
  enabled: true
  settings:
    controlPlaneConfigurator:
      dexCAMode: FromIngressSecret
  ...
```

Перед запуском модуля убедимся, что ключевые сервисы доступны из **рабочей сети**.
1. Получаем адрес платформы аутентификации командой:

    ```shell
    d8 k -n d8-user-authn get ing dex
    # Ожидаемый ответ
    # NAME   CLASS   HOSTS               ADDRESS         PORTS     AGE
    # dex    nginx   dex.mycompany.tld   34.85.243.109   80, 443   4d20h
    ```

    Под столбцом `HOSTS` наш проверяемый домен, а под `ADDRESS` – его IP адрес. Теперь нужно убедиться, что домен правильно резолвится на указанный IP адрес. Для этого выполняем команду:

    ```shell
    nslookup dex.mycompany.tld
    # Ожидаемый ответ
    # ...
    # Name: dex.mycompany.tld
    # Address: 34.85.243.109
    # ...

    # Либо
    dig dex.mycompany.tld
    # Ожидаемый ответ
    # ...
    # ;; ANSWER SECTION:
    # dex.mycompany.tld. 3600 IN A 34.85.243.109
    # ...
    ```

    Если ответом стала ошибка с кодом `NXDOMAIN`, нужно настроить DNS пользователя. Если домен не доступен из Интернета, дополнительно необходимо выполнить [дополнительный шаг](#не-резолвится-доменное-имя-dexmycompanytld)
    > Как временное решение можно добавить следующую строку в файл `/etc/hosts` вашей Unix системы
    >
    > ```shell
    > 34.85.243.109 dex.mycompany.tld stronghold.mycompany.tld
    > ```
    >
2. В браузере открываем <https://dex.mycompany.tld/healthz>, либо выполняем команду `curl -kL https://dex.mycompany.tld/healthz`. Должен вернуться ответ `Health check passed`.
3. Проверяем, что Ingress контроллер обрабатывает запросы на ваш поддомен `stronghold.mycompany.tld`. Снова в браузере, либо командой `curl -kL` открываем <https://stronghold.mycompany.tld>. Должна вернуться 404 ошибка.

### Не резолвится доменное имя dex.mycompany.tld

Если ваш домен не резолвится через DNS, и вы планируете использовать файл `hosts`, то для работы Dex нужно добавить
адрес балансировщика или IP frontend-узла в кластерный DNS. В его роли можно использовать [модуль `kube-dns`](/modules/kube-dns/), чтобы поды могли получить доступ к домену `dex.mycompany.tld` по имени.

Пример получения IP для Ingress `nginx-load-balancer` с типом `LoadBlancer`

```shell
d8 k -n d8-ingress-nginx get svc nginx-load-balancer -o jsonpath='{ .spec.clusterIP }'
```

Допустим наш адрес `34.85.243.109`, тогда модуль-конфиг kube-dns будет выглядеть так

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: kube-dns
spec:
  version: 1
  enabled: true
  settings:
    hosts:
    - domain: dex.mycompany.tld
      ip: 34.85.243.109
```

### Включаем модуль

После этого можно включить модуль `stronghold`, инициализация и настройка интеграции с `dex` произойдет автоматически.

```shell
d8 p module enable stronghold
```

После запуска модуля проследить:
1. убедиться в наличии сертификата для домена stronghold.*
    `d8 k -n d8-stronghold get ingress-tls` (либо через Консоль)
  В разделе [Трудности](#трудности) есть описания решений возможных проблем с отсутствием сертификата.
2. Убедиться в доступности адреса <https://stronghold.mycompany.tld/v1/sys/health>
3. Проверить соответствие Издателя сертификата с CA сертификатом (Опционально)

### Трудности

#### Поды в состоянии ContainerCreating, объекта Secret с названием ingress-tls нет

Проверьте статус пода Stronghold:
`d8 k -n d8-stronghold describe pod stronghold-0`
Ищем строку:

```log
MountVolume.SetUp failed for volume "certificates" : secret "ingress-tls" not found
```

При использовании метода [ClusterIssuer с LetsEncrypt](#clusterissuer-letsencrypt) может возникнуть проблема с автоматическим созданием сертификата для домена `stronghold.mycompany.tld` центром сертификации LetsEncrypt.

Получаем список *CertificateRequest*

```bash
d8 k -n d8-stronghold get certificaterequest
```

Ищем среди них объект начинающийся с **stronghold-**, для примера это будет **stronghold-b5wc6**

Смотрим его статус

```bash
d8 k -n d8-stronghold describe certificaterequest stronghold-b5wc6
```

Одной из причин может быть ошибка `too many certificates already issued for mycompany.tld`, в особенности, если используется бесплатный dynDNS сервис наподобие `sslip.io` или `getmoss.site`. В таком случае нужно либо подождать, пока не пройдёт таймаут ограничения, либо сменить способ создания сертификата для домена `stronghold.mycompany.tld` (*ClusterIssuer* **selfsigned**, ручная подпись сертификата).

При успешном завершении генерации сертификата в статусе *CertificateRequest* должны быть строчки:

```log
Message:               Certificate fetched from issuer successfully
Reason:                Issued
Status:                True
Type:                  Ready
```

и присутствовать объект *Secret* типа `kubernetes.io/tls` с названием **ingress-tls**
