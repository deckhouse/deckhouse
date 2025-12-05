---
title: Установка DKP в закрытом окружении
permalink: ru/guides/private-environment.html
description: Руководство по установке Deckhouse Kubernetes Platform в закрытом окружении
lang: ru
layout: sidebar-guides
---

В приведённом ниже руководстве рассказывается, как развернуть кластер под управлением Deckhouse Kubernetes Platform в закрытом окружении, из которого нет прямого доступа к хранилищу образов контейнеров DKP (`registry.deckhouse.ru`) и к внешним репозиториям deb/rpm-пакетов установленных на узлах [операционных систем](../documentation/v1/reference/supported_versions.html#linux).

## Особенности приватного окружения

Установка в закрытое окружение практически не отличается от установки [на bare metal](../gs/bm/step2.html). 

Главные особенности:

* доступ в Интернет для приложений, развёрнутых в закрытом контуре, осуществляется через прокси-сервер, который необходимо указать в конфигурации кластера;
* container registry с образами контейнеров Deckhouse также разворачиваеся отдельно с доступом изнутри контура, а в кластере настраивается его использование и необходимые права доступа.

Все взаимоотношения с внешними ресурсами осуществляются через отдельную машину (назовё её «бастион»). На ней же разворачиваются container registry и прокси-сервер, а также с неё производятся все операции с кластером.

Общая схема закрытого окружения представлена на рисунке:

<img src="/images/gs/private-env-schema-RU.svg" alt="Схема развертывания Deckhouse в закрытом окружении">

{% alert level="info" %}
На схеме также показан внутренний репозиторий пакетов ОС, который необходим для установки curl на узлах будущего кластера при отсутствии возможности доступа к официальным репозиториям через прокси-сервер.
{% endalert %}

## Подготовка инфраструктуры

В этом руководстве мы будем разворачивать в закрытом окружении кластер, состоящий из одного master-узла и одного worker-узла.

На понядобятся персональный компьютер, с которого будут выполняться работы, отдельная машина под бастион, на котором будет развёрнут container registry и сопутствующие компоненты, а также две машины под узлы кластера.

Требования к машинам следующие:

* Бастион — 4 ядра, 8 ГБ ОЗУ, 150 ГБ на быстром диске. Такой объем объясняется тем, что на этой машине будут храниться все образы Deckhouse, необходиимые для установки, причем скачиваться с registry Фланта перед заливкой в приватный registry, они будут так же на эту же машину;
* [Ресурсы под будущие узлы кластера](./hardware-requirements.html#выбор-ресурсов-для-узлов) нужно выбрать исходя из ваших требований к будущей нагрузке в кластере. Для примера возьмём минимально рекомендуемые 4 ядра, 8 ГБ ОЗУ, 60 ГБ на быстром диске (400+ IOPS) для каждого из узов.

### Подготовка машины-бастиона

Бастион-машина — это точка входа в будущий кластер. Подготовим его в к работе.

#### Подготовка container registry

##### Установка Harbor

В качестве приватного registry будем использовать [Harbor](https://goharbor.io/). Это container registry с открытым исходным кодом, который имеет возможности настройки политик и управления доступом на основе ролей, проверяет образы на наличие уязвимостей и помечает их как надежные. Harbor входит в состав проектов CNCF.

Устанавливать мы будем latest-версию [из GitHub-репозитория](https://github.com/goharbor/harbor/releases) проекта. Для этого нужно скачать архив с установщиком из нужного релиза, выбрав вариант с `harbor-offline-installer` в названии.

<div style="text-align: center;">
<img src="/images/gs/guides/private_environment/download-harbor-installer.png" alt="Скачивание установщика Harbor...">
</div>

Скопируйте адрес ссылки. Для версии `harbor-offline-installer-v2.14.1.tgz` ссылка будет выглядеть следующим образом: `https://github.com/goharbor/harbor/releases/download/v2.14.1/harbor-offline-installer-v2.14.1.tgz`.

Подключитель к вашему бастиону по SSH и скачайте файл любым удобным вам способом.

{% offtopic title="Как скачать файл с помощью wget..." %}
Выполните команду:
```console
wget https://github.com/goharbor/harbor/releases/download/v2.14.1/harbor-offline-installer-v2.14.1.tgz
```

Пример выполнени команды:
```text
$ wget https://github.com/goharbor/harbor/releases/download/v2.14.1/harbor-offline-installer-v2.14.1.tgz
--2025-12-04 12:38:42--  https://github.com/goharbor/harbor/releases/download/v2.14.1/harbor-offline-installer-v2.14.1.tgz
Resolving github.com (github.com)... 140.82.121.4
Connecting to github.com (github.com)|140.82.121.4|:443... connected.
HTTP request sent, awaiting response... 302 Found
Location: https://release-assets.githubusercontent.com/github-production-release-asset/50613991/01508bef-5c2c-40bb-bc66-f40e34ad2cae?sp=r&sv=2018-11-09&sr=b&spr=https&se=2025-12-04T13%3A28%3A53Z&rscd=attachment%3B+filename%3Dharbor-offline-installer-v2.14.1.tgz&rsct=application%2Foctet-stream&skoid=96c2d410-5711-43a1-aedd-ab1947aa7ab0&sktid=398a6654-997b-47e9-b12b-9515b896b4de&skt=2025-12-04T12%3A28%3A27Z&ske=2025-12-04T13%3A28%3A53Z&sks=b&skv=2018-11-09&sig=bUJ%2Bo6Bx7brkGAvaf2Pq9cXHah1aPJi9PDlc7G3WwS0%3D&jwt=eyJ0eXAiOiJKV1QiLCJhbGciOiJIUzI1NiJ9.eyJpc3MiOiJnaXRodWIuY29tIiwiYXVkIjoicmVsZWFzZS1hc3NldHMuZ2l0aHVidXNlcmNvbnRlbnQuY29tIiwia2V5Ijoia2V5MSIsImV4cCI6MTc2NDg1NTUyMiwibmJmIjoxNzY0ODUxOTIyLCJwYXRoIjoicmVsZWFzZWFzc2V0cHJvZHVjdGlvbi5ibG9iLmNvcmUud2luZG93cy5uZXQifQ.DHH2Fz_xRutTNEnN5p3uxxG_T03dhqppICQhc0gJQq0&response-content-disposition=attachment%3B%20filename%3Dharbor-offline-installer-v2.14.1.tgz&response-content-type=application%2Foctet-stream [following]
--2025-12-04 12:38:42--  https://release-assets.githubusercontent.com/github-production-release-asset/50613991/01508bef-5c2c-40bb-bc66-f40e34ad2cae?sp=r&sv=2018-11-09&sr=b&spr=https&se=2025-12-04T13%3A28%3A53Z&rscd=attachment%3B+filename%3Dharbor-offline-installer-v2.14.1.tgz&rsct=application%2Foctet-stream&skoid=96c2d410-5711-43a1-aedd-ab1947aa7ab0&sktid=398a6654-997b-47e9-b12b-9515b896b4de&skt=2025-12-04T12%3A28%3A27Z&ske=2025-12-04T13%3A28%3A53Z&sks=b&skv=2018-11-09&sig=bUJ%2Bo6Bx7brkGAvaf2Pq9cXHah1aPJi9PDlc7G3WwS0%3D&jwt=eyJ0eXAiOiJKV1QiLCJhbGciOiJIUzI1NiJ9.eyJpc3MiOiJnaXRodWIuY29tIiwiYXVkIjoicmVsZWFzZS1hc3NldHMuZ2l0aHVidXNlcmNvbnRlbnQuY29tIiwia2V5Ijoia2V5MSIsImV4cCI6MTc2NDg1NTUyMiwibmJmIjoxNzY0ODUxOTIyLCJwYXRoIjoicmVsZWFzZWFzc2V0cHJvZHVjdGlvbi5ibG9iLmNvcmUud2luZG93cy5uZXQifQ.DHH2Fz_xRutTNEnN5p3uxxG_T03dhqppICQhc0gJQq0&response-content-disposition=attachment%3B%20filename%3Dharbor-offline-installer-v2.14.1.tgz&response-content-type=application%2Foctet-stream
Resolving release-assets.githubusercontent.com (release-assets.githubusercontent.com)... 185.199.108.133, 185.199.109.133, 185.199.110.133, ...
Connecting to release-assets.githubusercontent.com (release-assets.githubusercontent.com)|185.199.108.133|:443... connected.
HTTP request sent, awaiting response... 200 OK
Length: 680961237 (649M) [application/octet-stream]
Saving to: ‘harbor-offline-installer-v2.14.1.tgz’

harbor-offline-installer-v2.14.1.tgz                         100%[=============================================================================================================================================>] 649.42M  77.2MB/s    in 8.2s

2025-12-04 12:38:50 (79.4 MB/s) - ‘harbor-offline-installer-v2.14.1.tgz’ saved [680961237/680961237]
```
{% endofftopic %}

{% offtopic title="Как скачать файл с помощью curl..." %}
Выполните команду:
```console
curl -O https://github.com/goharbor/harbor/releases/download/v2.14.1/harbor-offline-installer-v2.14.1.tgz
```

Пример выполнени команды:
```text
$ curl -O https://github.com/goharbor/harbor/releases/download/v2.14.1/harbor-offline-installer-v2.14.1.tgz
  % Total    % Received % Xferd  Average Speed   Time    Time     Time  Current
                                 Dload  Upload   Total   Spent    Left  Speed
  0     0    0     0    0     0      0      0 --:--:-- --:--:-- --:--:--     0
```
{% endofftopic %}

Распакуйте скачанный файл:

```console
tar -zxf ./harbor-offline-installer-v2.14.1.tgz
```

В полученном каталоге `harbor` расположены файлы, необходимые для установки.

Прежде чем разворачивать хранилище нужно сгенерировать сертификаты, т.к. в закрытом окружении невозможно получить сертификаты от Let's Encrypt (он не сможет достучаться до внутренний ресурсов при проверке доступности).

Создадим каталог `certs` в каталоге `harbor`:

```console
cd harbor/
mkdir certs
``` 

Сгенерируем сертификаты командами:

```console
openssl genrsa -out ca.key 4096 -new -nodes -sha512 -days 3650 -subj "/C=RU/ST-Moscow/L=Moscow/O=example/OU=Personal/CN-myca. local" -key ca.key -out ca.crt
```

```console
openssl genrsa -out harbor.local.key 4096
```

```console
openssl req -sha512 -new -subj "/C-RU/ST-Moscow/L=Moscow/0=example/OU=Personal/CN=harbor.local" -key harbor.local.key -out harbor.local.csr
```

```console
cat > v3.ext
authorityKeyIdentifier=keyid, issuer basicConstraints=CA:FALSE
keyUsage = digitalSignature, nonRepudiation, keyEncipherment, dataEncipherment
extendedKeyUsage = serverAuth
subjectAltName = @alt_names

[alt_names]
IP.1=10.128.0.30
DNS.1=harbor.local
EOF
```

**Важно!** Не забудьте заменить в этой команде `IP.1` на свой внутренниый серый IP-адрес. По нему будет происходить обращение к container-registry изнутри закрытого контура. С этим же адресом будет связано доменное имя `harbor.local`.


```console
openssl x509 -req -sha512 -days 3650 -extfile v3.ext -CA ca.crt -CAkey ca.key -CAcreateserial -in harbor.local.csr -out harbor.local.crt
```

```console
openssl x509 -inform PEM -in harbor.local.crt -out harbor.local.cert
```




