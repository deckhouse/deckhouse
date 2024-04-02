---
title: Обновление
permalink: ru/guides/production.html
lang: ru
---

<!-- Три нижеперечисленных раздела решено вынести наверх, как некие быстрогайды-->
<!-- 
## Гарантия совместимости при обновлении

Платформа гарантирует совместимость при обновлениях — все старые версии API платформы продолжают оставаться рабочими продолжительное время (обычно не мене 1 года с момента deprecation). Кроме этого, платформа поддерживает не менее 4-х версий Kubernetes и предоставляет администраторам возможности для проверки совместимости приложений запущенных в Kubernetes до обновления самого kubernetes (по данным helm релизов). -->

<!-- Нужен сценарий-->

<!-- ## Полностью автоматический процесс обновления 

(и четкий контракт с разработчиками)

Сам процесс обновления выполняется полностью автоматически (пользователю достаточно разрешить обновление) и полностью онлайн: большинство обновлений вообще не трогают приложения (даже обновление ПО на узлах происходит без прерывания работы приложений), но те которые требуют — делают это корректно, акуратно “сливая” (drain) узлы и дожидаясь запуска приложений на других узлах. Разработчикам легко обеспечить следование простому контракту (N+1 реплик и настроенный соответствующим образом PodDisruptionsBudget), чтобы любые обновления проходили без простоя. -->

<!--Приведем несколько рекомендаций касательно канала и режима обновлений:
●	Выбирайте канал и режим обновлений, который соответствует вашим ожиданиям. Чем стабильнее канал обновлений, тем позже до него доходит новая функциональность.
●	По возможности используйте разные каналы обновлений для кластеров. Для кластера разработки используйте менее стабильный канал обновлений (Alpha - EA), нежели для тестового или stage-кластера (pre-production-кластер). В случае появления нового функционала, или окончания поддержки каких-либо deprecated возможностей, будет иметься достаточный временной промежуток для адаптирования приложений в кластере под новые версии.
●	Мы рекомендуем использовать каналы обновлений `Early Access`, `Stable` или `RockSolid` для production-кластеров.
●	Если в production-окружении более одного кластера, предпочтительно использовать для них разные каналы обновлений. Например, Early Access для одного, а Stable — для другого. Если использовать разные каналы обновлений по каким-либо причинам невозможно, рекомендуется устанавливать разные окна обновлений.
●	Даже в очень нагруженных и критичных кластерах не стоит отключать использование канала обновлений. Лучшая стратегия — плановое обновление. В инсталляциях Deckhouse, которые не обновлялись полгода или более, могут присутствовать ошибки. Как правило, эти ошибки давно устранены в новых версиях. В этом случае оперативно решить возникшую проблему будет непросто.
●	Устанавливайте окна обновлений релизов Deckhouse в автоматическом режиме в периоды “затишья”, когда нагрузка на кластер далека от пиковой.

 Нужен сценарий-->

<!-- ## Единые обновления фундамента, узлов и решений

Платформа обновляется целиком — включая как весь инфраструктурно-агностический фундамент (в том числе программное обеспечение на узлах), так и все высокоуровневые решения. -->

<!-- Нужен сценарий-->

## Каналы обновлений

Существует несколько каналов обновления для кластера. Каждый из них имеет свои особенности и предназначен для определенной цели. Важно помнить, что использование неподходящего канала обновлений может привести к проблемам в работе кластера и нарушению его стабильности.

К кластерам, как элементам инфраструктуры, обычно предъявляются различные требования. Например, production-кластер, в отличие от кластера разработки, более требователен к надежности: в нем нежелательно часто обновлять или изменять какие-либо компоненты без особой необходимости, при этом сами компоненты должны быть тщательно протестированы.
По возможности используйте разные каналы обновлений для кластеров. Для кластера разработки используйте менее стабильный канал обновлений, нежели для тестового или stage-кластера (pre-production-кластер).

Для production-кластеров рекомендуется использовать канал обновлений `Early Access` или `Stable`. Если в production-окружении более одного кластера, предпочтительно использовать для них разные каналы обновлений. Например, `Early Access` для одного, а `Stable` — для другого. Если использовать разные каналы обновлений по каким-либо причинам невозможно, рекомендуется устанавливать разные окна обновлений.

Deckhouse Kubernetes Platform использует пять каналов обновлений. Между ними можно переключаться с помощью модуля [deckhouse](ссылка), достаточно указать желаемый канал обновлений в конфигурации модуля из следующего списка:

1. **Rock Solid**. Наиболее стабильный канал обновлений. Подойдет для кластеров, которым необходимо обеспечить повышенный уровень стабильности. Обновления функционала до этого канала доходят не ранее чем через месяц после их появления в релизе.
2. **Stable**. Стабильный канал обновлений для кластеров, в которых закончена активная работа и преимущественно осуществляется эксплуатация. Обновления функционала до этого канала обновлений доходят не ранее чем через две недели после их появления в релизе.
3. **Early Access**. Рекомендуемый канал обновлений, если вы не уверены в выборе. Подойдет для кластеров, в которых идет активная работа (запускаются, дорабатываются новые приложения и т. п.). Обновления функционала до этого канала обновлений доходят не ранее чем через одну неделю после их появления в релизе.
4. **Beta**. Ориентирован на кластеры разработки, как и канал обновлений Alpha. Получает версии, предварительно опробованные на канале обновлений Alpha.
5. **Alpha**. Наименее стабильный канал обновлений с наиболее частым появлением новых версий. Ориентирован на кластеры разработки с небольшим количеством разработчиков.

{% alert %}
Используйте канал обновлений `Early Access` или `Stable`. Установите [окно автоматических обновлений](/documentation/v1/modules/002-deckhouse/usage.html#конфигурация-окон-обновлений) или [ручной режим](/documentation/v1/modules/002-deckhouse/usage.html#ручное-подтверждение-обновлений).
{% endalert %}

Выберите необходимый канал обновлений и [режим обновлений](/documentation/v1/modules/002-deckhouse/configuration.html#parameters-releasechannel) <!---дать ссылку на раздел этой доки), который соответствует вашим ожиданиям. Чем стабильнее канал обновлений, тем позже до него доходит новая функциональность.-->

{% alert level="warning" %}
Даже в очень нагруженных и критичных кластерах не стоит отключать использование канала обновлений. Лучшая стратегия — плановое обновление. В инсталляциях Deckhouse Kubernetes Platform, которые не обновлялись полгода или более, могут присутствовать ошибки. Как правило, эти ошибки давно устранены в новых версиях. В этом случае оперативно решить возникшую проблему будет непросто.
{% endalert %}

### Установка желаемого канала обновлений

Чтобы перейти на другой канал обновлений автоматически, нужно в [конфигурации](modules/002-deckhouse/configuration.html) модуля `deckhouse` изменить (установить) параметр [releaseChannel](modules/002-deckhouse/configuration.html#parameters-releasechannel).

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

Информацию о том, какая версия Deckhouse находится на копределенном канале обновлений, можно получить на [сайте](https://flow.deckhouse.io).

### Cмена канала обновлений

* При смене канала обновлений на **более стабильный**, например, с `Alpha` на `EarlyAccess`, Deckhouse скачивает данные о релизе (в примере — из канала `EarlyAccess`) и сравнивает их с данными из существующих в кластере custom resouce'ов `DeckhouseRelease`:
  * Более *поздние* релизы, которые еще не были применены, они находятся в статусе `Pending`, удаляются.
  * Если более *поздние* релизы уже применены, они находятся в статусе `Deployed`, смены релиза не происходит. В этом случае Deckhouse Kubernetes Platform останется на таком релизе до тех пор, пока на канале обновлений `EarlyAccess` не появится более поздний релиз.
* При смене канала обновлений на **менее стабильный**? например, с `EarlyAcess` на `Alpha`, происходит следующее:
  * Deckhouse Kubernetes Platform скачивает данные о релизе (в примере — из канала `Alpha`) и сравнивает их с данными из существующих в кластере custom resource'ов `DeckhouseRelease`.
  * Затем Deckhouse Kubernetes Platform выполняет обновление согласно установленным [параметрам обновления](modules/002-deckhouse/configuration.html#parameters-update).

{% offtopic title="Схема использования параметра releaseChannel при установке и в процессе работы Deckhouse" %}
![Схема использования параметра releaseChannel при установке и в процессе работы Deckhouse](images/common/deckhouse-update-process.png)
{% endofftopic %}

### Проверка обновлений из канала обновлений

* Проверьте, что [настроен](#как-установить-желаемый-канал-обновлений) необходимый канал обновлений.
* Проверьте корректность разрешения DNS-имени хранилища образов Deckhouse Kubernetes Platform.

  Получите и сравните IP-адреса хранилища образов Deckhouse Kubernetes Platform (`registry.deckhouse.ru`) на одном из узлов и в поде Deckhouse Kubernetes Platform. Они должны совпадать.

  Пример получения IP-адреса хранилища образов Deckhouse Kubernetes Platform на узле:

  ```shell
  $ getent ahosts registry.deckhouse.ru
  185.193.90.38    STREAM registry.deckhouse.ru
  185.193.90.38    DGRAM
  185.193.90.38    RAW
  ```

  Пример получения IP-адреса хранилища образов Deckhouse Kubernetes Platform в поде Deckhouse Kubernetes Platform:
  
  ```shell
  $ kubectl -n d8-system exec -ti deploy/deckhouse -c deckhouse -- getent ahosts registry.deckhouse.ru
  185.193.90.38    STREAM registry.deckhouse.ru
  185.193.90.38    DGRAM  registry.deckhouse.ru
  ```
  
  Если полученные IP-адреса не совпадают, проверьте настройки DNS на узле. Обратите внимание на список доменов в параметре `search` файла `/etc/resolv.conf`, который влияет на разрешение имен в поде Deckhouse. Если в параметре `search` файла `/etc/resolv.conf` указан домен, в котором настроено разрешение wildcard-записей, это может привести к неверному разрешению IP-адреса хранилища образов Deckhouse (см. пример).
  
{% offtopic title="Пример настроек DNS, которые могут привести к ошибкам в разрешении IP-адреса хранилища образов Deckhouse..." %}

Ниже описан пример настроек DNS, которые приводят к различному результату при разрешении имен на узле и в поде Kubernetes:
- Пример файла `/etc/resolv.conf` на узле:

  ```text
  nameserver 10.0.0.10
  search company.my
  ```

  > Обратите внимание, что по умолчанию на узле параметр `ndot` равен 1 (`options ndots:1`). Но в подах Kubernetes параметр `ndot` равен **5**. Таким образом, логика разрешения DNS-имен, имеющих в имени 5 точек и менее, различается на узле и в поде.

- В DNS-зоне `company.my` настроено разрешение wildcard-записей `*.company.my` в адрес `10.0.0.100`. То есть любое DNS-имя в зоне `company.my`, для которого нет конкретной записи в DNS, разрешается в адрес `10.0.0.100`.

Тогда с учетом параметра `search`, указанного в файле `/etc/resolv.conf`, при обращении на адрес `registry.deckhouse.ru` на узле система попробует получить IP-адрес для имени `registry.deckhouse.ru` (так как считает его полностью определенным, учитывая настройку по умолчанию параметра `options ndots:1`).

При обращении на адрес `registry.deckhouse.ru` из пода Kubernetes, учитывая параметры `options ndots:5` (используется в Kubernetes по умолчанию) и `search`, система первоначально постарается получить IP-адрес для имени `registry.deckhouse.ru.company.my`. Имя `registry.deckhouse.ru.company.my` разрешится в IP-адрес `10.0.0.100`, так как в DNS-зоне `company.my` настроено разрешение wildcard-записей `*.company.my` в адрес `10.0.0.100`. В результате к хосту `registry.deckhouse.ru` будет невозможно подключиться и скачать информацию о доступных обновлениях Deckhouse.  
{% endofftopic %}

## Окна обновлений

Управление [окнами обновлений](/documentation/v1/modules/002-deckhouse/configuration.html#parameters-update-windows) позволяет планово обновлять релизы Deckhouse в автоматическом режиме в периоды «затишья», когда нагрузка на кластер далека от пиковой.

  В Deckhouse реализован механизм автоматического обновления. Этот механизм использует [5 каналов обновлений](../../deckhouse-release-channels.html), различающиеся стабильностью и частотой выхода версий. Ознакомьтесь подробнее с тем, [как работает механизм автоматического обновления](../../deckhouse-faq.html#как-работает-автоматическое-обновление-deckhouse) и [как установить желаемый канал обновлений](../../deckhouse-faq.html#как-установить-желаемый-канал-обновлений).
- **[Режим обновлений](configuration.html#parameters-update-mode)** и **[окна обновлений](configuration.html#parameters-update-windows)**

### Конфигурация окон обновлений по времени

Временные настройки позволяют определить удобное время для обновления модулей в Deckhouse Kubernetes Platform. Это позволbт обеспечить стабильность системы во время обновлений и минимизировать возможные негативные влияния на работающие приложения.
Установка обновлений в определенное время позволяет минимизировать возможные проблемы, связанные с нагрузкой на систему во время установки обновлений, а также предотвращает возможные конфликты между обновляемыми модулями и работающими приложениями.

Настроить время, когда Deckhouse будет устанавливать обновления, можно в параметре [update.windows](configuration.html#parameters-update-windows) конфигурации модуля.

Пример настройки двух ежедневных окон обновлений: с 8:00 до 10:00 и c 20:00 до 22:00 (UTC):

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: deckhouse
spec:
  version: 1
  settings:
    releaseChannel: EarlyAccess
    update:
      windows: 
        - from: "8:00"
          to: "10:00"
        - from: "20:00"
          to: "22:00"
```

Также можно настроить обновления в определенные дни, например по вторникам и субботам с 18:00 до 19:30 (UTC):

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: deckhouse
spec:
  version: 1
  settings:
    releaseChannel: Stable
    update:
      windows: 
        - from: "18:00"
          to: "19:30"
          days:
            - Tue
            - Sat
```

### Обновление без окна обновлений

Обновление без окна обновлений позволяет выполнить обновление модуля вне определенного для этого времени. Это необходимо в случае срочного обновления. Но стоит учитывать, что применение обновлений без соблюдения определенного для этого времени может вызвать проблемы стабильности системы или конфликты с работающими приложениями. Поэтому это стоит использовать только в случае действительной необходимости.

Чтобы применить обновление немедленно, не дожидаясь ближайшего окна обновлений, установите в соответствующем ресурсе [DeckhouseRelease](modules/002-deckhouse/cr.html#deckhouserelease) аннотацию `release.deckhouse.io/apply-now: "true"`.

Пример команды установки аннотации пропуска окон обновлений для версии `v1.56.2`:

```shell
kubectl annotate deckhousereleases v1.56.2 release.deckhouse.io/apply-now="true"
```

Пример ресурса с установленной аннотацией пропуска окон обновлений:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: DeckhouseRelease
metadata:
  annotations:
    release.deckhouse.io/apply-now: "true"
...
```

## Обновление в закрытом контуре

Deckhouse Kubernetes Platform использует актуальные версии компонентов для обеспечения стабильности и безопасности системы. Обновления могут включать исправления уязвимостей, улучшение производительности и добавление новых функций.
Закрытый контур может требовать использования специфических версий компонентов или патчей, которые не доступны в стандартных репозиториях. В этом случае, можно настроить Deckhouse на работу со сторонним реестром, который содержит необходимые образы. Кроме того, обновления могут быть необходимы для обеспечения совместимости с другими компонентами в системе или для поддержки новых функций, так как Deckhouse Kubernetes Platform отвечает за то, чтобы кластер одинаково работал на любой поддерживаемой инфраструктуре из следующих:

* в облаках (смотри информацию по соответствующему cloud провайдеру - добавить ссылку);
* на виртуальных машинах или железе (включая on-premises);
* в гибридной инфраструктуре.

Образы всех компонентов Deckhouse Kubernetes Platform, включая control plane, хранятся в высокодоступном и геораспределенном container registry.

### Предварительная настройка

1. В текущем каталоге создайте новый каталог `d8-modules`.

2. Выполните аутентификацию в репозитории вендора, используя в качестве имени пользователя `license-token`, а в качестве пароля ваш лицензионный ключ:

```bash
docker login registry.deckhouse.ru
```

3. Запустите установочный контейнер командой:

```bash
docker run -ti --pull=always -v $(pwd)/d8-modules:/tmp/d8-modules registry.deckhouse.ru/deckhouse/ee/install:stable bash
``` 

4. Скопируйте утилиту `dhctl` из контейнера в каталог `d8-modules`:

```bash
cp /usr/bin/dhctl /tmp/d8-modules/dhctl
```

5. Завершите работу контейнера.

### Выгрузка образов модулей DKP из репозитория вендора

1. Создайте зашифрованную base64 строку для доступа клиента Docker в репозиторий вендора. Сделать это можно, например, командой ниже, заменив `YOUR_USERNAME` на `license-token`, а `YOUR_PASSWORD` — на ваш лицензионный ключ:

```bash
base64 -w0 <<EOF
  {
    "auths": {
      "registry.deckhouse.ru": {
        "auth": "$(echo -n 'YOUR_USERNAME:YOUR_PASSWORD' | base64 -w0)"
      }
    }
  }
EOF
```

2. Создайте в текущем каталоге файл `ModuleSource`, например, `ms.yml` следующего содержания:

`ms.yml`

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleSource
metadata:
  name: deckhouse
spec:
  registry:
	# Укажите строку, полученную в п.1 вместо CHANGE
    dockerCfg: CHANGE
    repo: registry.deckhouse.ru/deckhouse/ee/modules
    scheme: HTTPS
  # Выберите подходящий канал обновлений: Alpha, Beta, EarlyAccess, Stable, RockSolid
  releaseChannel: "Stable"
```

3. Запустите загрузку модулей DKP из репозитория вендора в локальный каталог рабочей станции:

```bash
dhctl mirror-modules --modules-dir=$(pwd)/d8-modules --module-source=$(pwd)/ms.yml
```
В результате работы утилиты в каталог `d8-modules` будут сохранены все необходимые артефакты, необходимые для переноса модулей DKP в закрытое окружение. Примерный объём данных составляет 7 Гб.

4. Выполните перенос на рабочую станцию в закрытом окружении следующих элементов:

- каталога `d8-modules`
- исполняемого файла `dhctl`

### Загрузка образов модулей DKP в закрытый репозиторий

1. Из каталога рабочей станции в закрытом окружении, содержащего утилиту dhctl и каталог с образами модулей DKP d8-modules, выполните загрузку образов в закрытый репозиторий следующей командой:

```bash
dhctl mirror-modules \
	--modules-dir=$(pwd)/d8-modules \
	--registry="registry.example.com:5000/deckhouse/ee/modules" \
	--registry-login="YOUR_USERNAME" \
	--registry-password="YOUR_PASSWORD"
```

Если ваш репозиторий не требует авторизации, флаги `--registry-login` / `--registry-password` указывать не нужно.

Важно указать верный путь в репозитории: там должна находиться поставка DKP. Таким образом, в примере выше может потребоваться поменять `/deckhouse/ee` на правильный путь размещения образов DKP.

2. Проверьте, что `ModuleSource` с названием `deckhouse` в вашем кластере указывает на верный путь до модулей (`spec.registry.repo`), а также в нем нет ошибок (`status.moduleErrors`).

```bash
kubectl get ms deckhouse -o yaml
```

Пример вывода:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleSource
metadata:
  creationTimestamp: "2024-03-11T20:33:51Z"
  finalizers:
  - modules.deckhouse.io/release-exists
  generation: 1
  labels:
    heritage: deckhouse
  name: deckhouse
  resourceVersion: "20241841"
  uid: f35d10be-3ff9-4cd9-b64c-4f58abd8f595
spec:
  registry:
    ca: |
      -----BEGIN CERTIFICATE-----
      ...
      -----END CERTIFICATE-----
    dockerCfg: ...
    repo: registry.example.com:5000/deckhouse/ee/modules
    scheme: HTTPS
  releaseChannel: ""
status:
  message: ""
  moduleErrors: []
  modules:
  - name: deckhouse-admin
    policy: deckhouse
  - name: deckhouse-commander
    policy: deckhouse
  - name: deckhouse-commander-agent
    policy: deckhouse
  - name: operator-ceph
    policy: deckhouse
  - name: operator-postgres
    policy: deckhouse
  - name: sds-drbd
    policy: deckhouse
  - name: sds-node-configurator
    policy: deckhouse
  - name: secrets-store-integration
    policy: deckhouse
  - name: stronghold
    policy: deckhouse
  - name: virtualization
    policy: deckhouse
  modulesCount: 10
  syncTime: "2024-03-28T14:25:35Z"
```

Обратите внимание, что пустое значение для `spec.releaseChannel` говорит о том, что каналы обновлений для модулей будут совпадать с каналом обновлений для DKP.

3. Проверьте доступность новых выпусков для модулей, выполнив команду:

```bash
kubectl get mr
```

Пример вывода:

```yaml
NAME                               PHASE        UPDATE POLICY   TRANSITIONTIME   MESSAGE
deckhouse-admin-v1.19.3            Superseded                   91s              
deckhouse-admin-v1.21.2            Deployed     deckhouse       91s              
deckhouse-commander-agent-v1.0.1   Deployed                     16d              
deckhouse-commander-v1.2.5         Deployed                     16d              
operator-ceph-v1.0.10              Deployed                     16d              
operator-postgres-v1.0.15          Deployed                     16d              
sds-drbd-v0.1.7                    Deployed                     16d              
sds-drbd-v0.1.8                    Pending      deckhouse       17m              Waiting for manual approval
sds-node-configurator-v0.1.3       Deployed                     16d              
sds-node-configurator-v0.1.7       Pending      deckhouse       17m              Waiting for manual approval
sds-replicated-volume-v0.2.6       Pending      deckhouse       17m              Waiting for manual approval
secrets-store-integration-v1.0.9   Deployed                     16d              
stronghold-v1.0.9                  Deployed                     16d              
virtualization-v0.9.10             Deployed                     16d 
```

Если модуль требует ручного подтверждения обновления, то это можно сделать командой вида:

```bash
kubectl annotate mr sds-drbd-v0.1.8 modules.deckhouse.io/approved="true"
```

### Доступ из изолированных контуров container registry с фиксированным набором IP-адресов.

При установке Deckhouse можно настроить на работу со сторонним registry (например, проксирующий registry внутри закрытого контура). Для этого:

Установите следующие параметры в ресурсе `InitConfiguration`:

* `imagesRepo: <PROXY_REGISTRY>/<DECKHOUSE_REPO_PATH>/ee` — адрес образа Deckhouse EE в стороннем registry. Пример: `imagesRepo: registry.deckhouse.ru/deckhouse/ee`;
* `registryDockerCfg: <BASE64>` — права доступа к стороннему registry, зашифрованные в Base64.

Если разрешен анонимный доступ к образам Deckhouse в стороннем registry, `registryDockerCfg` должен выглядеть следующим образом:

```json
{"auths": { "<PROXY_REGISTRY>": {}}}
```

> Приведенное значение должно быть закодировано в Base64.

Если для доступа к образам Deckhouse Kubernetes Platform в стороннем registry необходима аутентификация, `registryDockerCfg` должен выглядеть следующим образом:

```json
{"auths": { "<PROXY_REGISTRY>": {"username":"<PROXY_USERNAME>","password":"<PROXY_PASSWORD>","auth":"<AUTH_BASE64>"}}}
```

где:

* `<PROXY_USERNAME>` — имя пользователя для аутентификации на `<PROXY_REGISTRY>`;
* `<PROXY_PASSWORD>` — пароль пользователя для аутентификации на `<PROXY_REGISTRY>`;
* `<PROXY_REGISTRY>` — адрес стороннего registry в виде `<HOSTNAME>[:PORT]`;
* `<AUTH_BASE64>` — строка вида `<PROXY_USERNAME>:<PROXY_PASSWORD>`, закодированная в Base64.

> Итоговое значение для `registryDockerCfg` должно быть также закодировано в Base64.

Для настройки нестандартных конфигураций сторонних registry в ресурсе `InitConfiguration` предусмотрены еще два параметра:

* `registryCA` — корневой сертификат, которым можно проверить сертификат registry (если registry использует самоподписанные сертификаты);
* `registryScheme` — протокол доступа к registry (`HTTP` или `HTTPS`). По умолчанию — `HTTPS`.

<div markdown="0" style="height: 0;" id="особенности-настройки-сторонних-registry"></div>

### Настройка Nexus

При использовании менеджера репозиториев [Nexus](https://github.com/sonatype/nexus-public) должны быть выполнены следующие требования:

* Включен `Docker Bearer Token Realm` (*Administration* -> *Security* -> *Realms*).
* Создан **проксирующий** репозиторий Docker (*Administration* -> *Repository* -> *Repositories*):
  * Параметр `Allow anonymous docker pull` для репозитория должен быть включен. Данный параметр включает поддержку авторизации с помощью Bearer-токенов, при этом анонимный доступ [не будет работать](https://help.sonatype.com/en/docker-authentication.html#unauthenticated-access-to-docker-repositories), если он не был явно включен в *Administration* -> *Security* -> *Anonymous Access* и пользователю `anonymous` не были даны права на доступ к репозиторию.
  * Параметр `Maximum metadata age` для репозитория должен быть установлен в `0`.
* Должен быть настроен контроль доступа:
  * Создана роль **Nexus** (*Administration* -> *Security* -> *Roles*) со следующими полномочиями:
    * `nx-repository-view-docker-<репозиторий>-browse`
    * `nx-repository-view-docker-<репозиторий>-read`
  * Создан пользователь (*Administration* -> *Security* -> *Users*) с ролью, созданной выше.

Для настройки Nexus:

1. Включите `Docker Bearer Token Realm` (*Administration* -> *Security* -> *Realms*):
  ![Включение `Docker Bearer Token Realm`](images/registry/nexus/nexus-realm.png)

2. Создайте **проксирующий** репозиторий Docker (*Administration* -> *Repository* -> *Repositories*), указывающий на [Deckhouse registry](https://registry.deckhouse.ru/):
  ![Создание проксирующего репозитория Docker](images/registry/nexus/nexus-repository.png)

3. Заполните поля страницы создания репозитория следующим образом:
  * `Name` должно содержать имя создаваемого репозитория, например `d8-proxy`.
  * `Repository Connectors / HTTP` или `Repository Connectors / HTTPS` должно содержать выделенный порт для создаваемого репозитория, например `8123` или иной.
  * `Allow anonymous docker pull` должно быть включено, чтобы работала авторизация с помощью Bearer-токенов. При этом анонимный доступ [не будет работать](https://help.sonatype.com/en/docker-authentication.html#unauthenticated-access-to-docker-repositories), если он не был явно включен в *Administration* -> *Security* -> *Anonymous Access* и пользователю `anonymous` не были даны права на доступ к репозиторию.
  * `Remote storage` должно иметь значение `https://registry.deckhouse.ru/`.
  * `Auto blocking enabled` и `Not found cache enabled` могут быть выключены для отладки; в противном случае их следует включить.
  * `Maximum Metadata Age` должно быть равно `0`.
  * Если планируется использовать Deckhouse Enterprise Edition, флажок `Authentication` должен быть включен, а связанные поля должны быть заполнены следующим образом:
    * `Authentication Type` должно иметь значение `Username`.
    * `Username` должно иметь значение `license-token`.
    * `Password` должно содержать ключ лицензии Deckhouse Enterprise Edition.

  ![Пример настроек репозитория 1](images/registry/nexus/nexus-repo-example-1.png)
  ![Пример настроек репозитория 2](images/registry/nexus/nexus-repo-example-2.png)
  ![Пример настроек репозитория 3](images/registry/nexus/nexus-repo-example-3.png)

* Настройте контроль доступа Nexus для доступа Deckhouse к созданному репозиторию:
  * Создайте роль **Nexus** (*Administration* -> *Security* -> *Roles*) с полномочиями `nx-repository-view-docker-<репозиторий>-browse` и `nx-repository-view-docker-<репозиторий>-read`.

    ![Создание роли Nexus](images/registry/nexus/nexus-role.png)

  * Создайте пользователя (*Administration* -> *Security* -> *Users*) с ролью, созданной выше.

    ![Создание пользователя Nexus](images/registry/nexus/nexus-user.png)

### Настройки Harbor

Для настройки Harbor необходимо использовать функции [Harbor](https://github.com/goharbor/harbor), как Proxy кэш.

* Настройте Registry:
  * `Administration -> Registries -> New Endpoint`.
  * `Provider`: `Docker Registry`.
  * `Name` — укажите любое, на ваше усмотрение.
  * `Endpoint URL`: `https://registry.deckhouse.ru`.
  * Укажите `Access ID` и `Access Secret` для Deckhouse Enterprise Edition.

  ![Настройка Registry](images/registry/harbor/harbor1.png)

* Создайте новый проект:
  * `Projects -> New Project`.
  * `Project Name` будет частью URL. Используйте любой, например, `d8s`.
  * `Access Level`: `Public`.
  * `Proxy Cache` — включите и выберите в списке Registry, созданный на предыдущем шаге.

  ![Создание нового проекта](images/registry/harbor/harbor2.png)

В результате образы Deckhouse будут доступны, например, по следующему адресу: `https://your-harbor.com/d8s/deckhouse/ee:{d8s-version}`.

### Закрытое окружение, работа через proxy и сторонние registry

#### Установка Deckhouse Kubernetes Platform из стороннего registry

{% alert level="warning" %}
Доступно только в Enterprise Edition.
{% endalert %}

{% alert level="warning" %}
Deckhouse поддерживает работу только с Bearer token-схемой авторизации в container registry.

Протестирована и гарантируется работа со следующими container registry:
{%- for registry in site.data.supported_versions.registries %}
[{{- registry[1].shortname }}]({{- registry[1].url }})
{%- unless forloop.last %}, {% endunless %}
{%- endfor %}.
{% endalert %}

#### Переключение работающиго кластера Deckhouse на использование стороннего registry

{% alert level="warning" %}
Использование registry отличных от `registry.deckhouse.io` и `registry.deckhouse.ru` доступно только в Enterprise Edition.
{% endalert %}

Для переключения кластера Deckhouse на использование стороннего registry выполните следующие действия:

* Выполните команду `deckhouse-controller helper change-registry` из пода Deckhouse с параметрами нового registry.
  * Пример запуска:

    ```shell
    kubectl exec -ti -n d8-system deploy/deckhouse -- deckhouse-controller helper change-registry \
      --user MY-USER --password MY-PASSWORD registry.example.com/deckhouse/ee
    ```

  * Если registry использует самоподписные сертификаты, положите корневой сертификат соответствующего сертификата registry в файл `/tmp/ca.crt` в поде Deckhouse и добавьте к вызову опцию `--ca-file /tmp/ca.crt`, или вставьте содержимое CA в переменную, как в примере ниже:

    ```shell
    $ CA_CONTENT=$(cat <<EOF
    -----BEGIN CERTIFICATE-----
    CERTIFICATE
    -----END CERTIFICATE-----
    -----BEGIN CERTIFICATE-----
    CERTIFICATE
    -----END CERTIFICATE-----
    EOF
    )
    $ kubectl exec  -n d8-system deploy/deckhouse -- bash -c "echo '$CA_CONTENT' > /tmp/ca.crt && deckhouse-controller helper change-registry --ca-file /tmp/ca.crt --user MY-USER --password MY-PASSWORD registry.example.com/deckhouse/ee"
    ```

* Дождитесь перехода пода Deckhouse в статус `Ready`. Если под будет находиться в статусе `ImagePullBackoff`, перезапустите его.
* Дождитесь применения bashible новых настроек на master-узле. В журнале bashible на master-узле (`journalctl -u bashible`) должно появится сообщение `Configuration is in sync, nothing to do`.
* Если необходимо отключить автоматическое обновление Deckhouse через сторонний registry, удалите параметр `releaseChannel` из конфигурации модуля `deckhouse`.
* Проверьте, не осталось ли в кластере подов с оригинальным адресом registry:

  ```shell
  kubectl get pods -A -o json | jq '.items[] | select(.spec.containers[] | select((.image | contains("deckhouse.io"))))
    | .metadata.namespace + "\t" + .metadata.name' -r
  ```


#### Создание кластера и запуск Deckhouse Kubernetes Platform без использования каналов обновлений

Данный способ следует использовать только в случае, если в изолированном приватном registry нет образов, содержащих информацию о каналах обновлений.

* Если вы хотите установить Deckhouse с отключенным автоматическим обновлением:
  * Используйте тег образа установщика соответствующей версии. Например, если вы хотите установить релиз `v1.44.3`, используйте образ `your.private.registry.com/deckhouse/install:v1.44.3`.
  * Укажите соответствующий номер версии в параметре [deckhouse.devBranch](installing/configuration.html#initconfiguration-deckhouse-devbranch) в ресурсе [InitConfiguration](installing/configuration.html#initconfiguration).
  * **Не указывайте** параметр [deckhouse.releaseChannel](installing/configuration.html#initconfiguration-deckhouse-releasechannel) в ресурсе [InitConfiguration](installing/configuration.html#initconfiguration).
* Если вы хотите отключить автоматические обновления у уже установленного Deckhouse (включая обновления patch-релизов), удалите параметр [releaseChannel](modules/002-deckhouse/configuration.html#parameters-releasechannel) из конфигурации модуля `deckhouse`.

#### Использование proxy-сервера

{% alert level="warning" %}
Доступно только в Enterprise Edition.
{% endalert %}

{% offtopic title="Пример шагов по настройке proxy-сервера на базе Squid..." %}
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

{% endofftopic %}

Для настройки Deckhouse на использование proxy используйте параметр [proxy](installing/configuration.html#clusterconfiguration-proxy) ресурса `ClusterConfiguration`.

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
kubernetesVersion: "Automatic"
cri: "Containerd"
clusterDomain: "cluster.local"
proxy:
  httpProxy: "http://user:password@proxy.company.my:3128"
  httpsProxy: "https://user:password@proxy.company.my:8443"
```

## Предоставление LTS обновлений на твердых носителях

Этот сценарий используется при невозможности выдачи ограниченного доступа в закрытый контур для настройки проксирующего registry.

Преимущества получения LTS обновлений на твёрдых носителях включают следующее:

* Безопасность: Получение обновлений на физических носителях позволяет убедиться, что они не были изменены или повреждены при передаче через интернет.
* Доступность: В некоторых регионах или организациях может отсутствовать стабильное интернет-соединение, поэтому получение обновлений на твёрдых носителях является единственным способом получить последние версии ПО.
* Контроль: Физическое владение обновлением даёт возможность контролировать процесс его установки и тестирования перед внедрением в производственную среду.
* Поддержка: Некоторые поставщики программного обеспечения предлагают дополнительную поддержку клиентам, которые получают LTS обновления на твёрдых носителях.

### Ручная загрузка образов в изолированный приватный registry

Эта функция позволяет загружать образы в изолированный приватный registry, что может быть полезно для организаций с повышенными требованиями к безопасности и конфиденциальности данных, например, если организация хочет контролировать доступ к образам и предотвратить несанкционированный доступ к ним. Также это может быть полезно, если организация использует свои собственные образы, которые не могут быть найдены в публичных registry.

{% alert level="warning" %}
Доступно только в Enterprise Edition.
{% endalert %}

1. При необходимости авторизуйтесь в container registry `registry.deckhouse.ru` (или `registry.deckhouse.io`) с помощью вашего лицензионного ключа.

   ```shell
   docker login -u license-token registry.deckhouse.ru
   ```

1. Запустите установщик Deckhouse версии 1.56.3 или выше.

   ```shell
   docker run -ti --pull=always -v $(pwd)/d8-images:/tmp/d8-images registry.deckhouse.ru/deckhouse/ee/install:v1.58.4 bash
   ```

   > Обратите внимание, что в контейнер установщика монтируется директория с файловой системы хоста, в которую будут загружены образы Deckhouse Kubernetes Platform.

1. Скачайте образы Deckhouse в выделенную директорию, используя команду `dhctl mirror`.

   `dhctl mirror` скачивает только последнюю доступную патч-версию минорного релиза Deckhouse. Например, для Deckhouse 1.52 будет скачана только одна версия `1.52.10`, т. к. этого достаточно для обновления Deckhouse с версии 1.51.

   Следующая команда скачает образы Deckhouse тех версий, которые находятся на каналах обновлений (о текущем статусе версий на каналах обновлений можно узнать на [flow.deckhouse.io](https://flow.deckhouse.io)):

   ```shell
   DHCTL_CLI_MIRROR_LICENSE="<DECKHOUSE_LICENSE_KEY>" dhctl mirror --source="registry.deckhouse.ru/deckhouse/ee" --images-bundle-path /tmp/d8-images/d8.tar
   ```

   > Если загрузка образов будет прервана, повторный вызов команды проверит загруженные образы и продолжит загрузку с момента ее остановки. Продолжение загрузки возможно только если с момента остановки прошло не более суток.
   > Используйте параметр `--no-pull-resume`, чтобы принудительно начать загрузку сначала.

   Чтобы скачать все версии Deckhouse Kubernetes Platform начиная с конкретной версии, укажите ее в параметре `--min-version` в формате `X.Y`.

   Например, для загрузки всех версий Deckhouse Kubernetes Platform, начиная с версии 1.45, используйте команду:

   ```shell
   DHCTL_CLI_MIRROR_LICENSE="<DECKHOUSE_LICENSE_KEY>" dhctl mirror --source="registry.deckhouse.ru/deckhouse/ee" --images-bundle-path /tmp/d8-images/d8.tar --min-version=1.45
   ```

   > Обратите внимание, параметр `--min-version` будет проигнорирован если вы укажете версию выше находящейся в канале обновлений rock-solid.

   Чтобы загрузить образы Deckhouse Kubernetes Platform из определенного репозитория registry, вы можете указать этот репозиторий с помощью флага `--source`.
   Существуют также дополнительные флаги `--source-login` и `--source-password`, используемые для аутентификации в предоставленном registry.
   Если они не указаны, `dhctl mirror` будет обращаться к registry анонимно.

   Например, можно загрузить образы из стороннего registry:

   ```shell
   DHCTL_CLI_MIRROR_SOURCE_LOGIN="user" DHCTL_CLI_MIRROR_SOURCE_PASSWORD="password" dhctl mirror --source="corp.company.ru/sys/deckhouse" --images-bundle-path /tmp/d8-images/d8.tar
   ```

   > Параметр `--license` действует как сокращение для параметров `--source-login` и `--source-password` и предназначен для использования с официальным registry Deckhouse.
   > Если вы укажете и параметр `--license`, и пару логин + пароль одновременно, будет использована последняя.

   `dhctl mirror` поддерживает расчет контрольных сумм итогового набора образов Deckhouse в формате ГОСТ Р 34.11-2012 (Стрибог) (параметр `--gost-digest`).
   Контрольная сумма будет выведена в лог и записана в файл с расширением `.tar.gostsum` рядом с tar-архивом, содержащим образы Deckhouse.

1. Опционально: Скопируйте утилиту `dhctl` из контейнера в директорию со скачанными образами Deckhouse.

   ```shell
   cp /usr/bin/dhctl /tmp/d8-images/dhctl
   ```

1. Передайте директорию с загруженными образами Deckhouse на хост с доступом к изолированному registry.
   Для продолжения установки используйте скопированную ранее утилиту `dhctl` или запустите установщик Deckhouse аналогично пунктам 1 и 2 на хосте с доступом к изолированному registry. Не забудьте смонтировать директорию с загруженными образами Deckhouse в контейнер установщика.

1. Загрузите образы Deckhouse с помощью команды `dhctl mirror` в изолированный registry.

   Пример команды для загрузки образов из файла `/tmp/d8-images/d8.tar`:

   ```shell
   DHCTL_CLI_MIRROR_USER="<USERNAME>" DHCTL_CLI_MIRROR_PASS="<PASSWORD>" dhctl mirror --images-bundle-path /tmp/d8-images/d8.tar --registry="your.private.registry.com:5000/deckhouse/ee"
   ```

   > Обратите внимание, образы будут выгружены в registry по пути, указанному в параметре `--registry` (в примере - /deckhouse/ee).
   > Перед запуском команды убедитесь, что этот путь существует и у используемой учетной записи есть права на запись.

   Если ваш registry не требует авторизации, флаги `--registry-login`/`--registry-password` указывать не нужно.

1. После загрузки образов в изолированный registry можно переходить к установке Deckhouse (доступно только в Enterprise Edition). Воспользуйтесь [руководством по быстрому старту](/gs/bm-private/step2.html).

   При запуске установщика используйте его образ из registry, в который ранее были загружены образы Deckhouse, а не из публичного registry. Например, используйте адрес вида `your.private.registry.com:5000/deckhouse/ee/install:stable` вместо `registry.deckhouse.ru/deckhouse/ee/install:stable`.

   В ресурсе `InitConfiguration` при установке также используйте адрес вашего registry и данные авторизации (параметры [imagesRepo](/documentation/v1/installing/configuration.html#initconfiguration-deckhouse-imagesrepo), [registryDockerCfg](/documentation/v1/installing/configuration.html#initconfiguration-deckhouse-registrydockercfg) или [шаг 3](/gs/bm-private/step3.html) руководства по быстрому старту).

### Ручная загрузка образов подключаемых модулей Deckhouse Kubernetes Platform в изолированный приватный registry

Ниже описаны шаги, необходимые для ручной загрузки образов модулей, подключаемых из источника модулей ресурса [*ModuleSource*](cr.html#modulesource)):

1. Запустите установщик Deckhouse версии 1.56.0 или выше.

   ```shell
   docker run -ti --pull=always -v $(HOME)/d8-modules:/tmp/d8-modules -v $(HOME)/module_source.yml:/tmp/module_source.yml registry.deckhouse.ru/deckhouse/ce/install:v1.58.4 bash
   ```

   > Обратите внимание, что в контейнер установщика монтируется директория с файловой системы хоста, в которую будут загружены образы модулей и YAML-манифест [ModuleSource](cr.html#modulesource), описывающий источник модулей.

1. Скачайте образы модулей из их источника, описанного в виде ресурса `ModuleSource` в выделенную директорию, используя команду `dhctl mirror-modules`.

   `dhctl mirror-modules` скачивает только версии модулей, доступные в каналах обновлений модуля на момент копирования.

   Следующая команда скачает образы модулей из источника, описанного в ресурсе `ModuleSource`, находящемся в файле `$HOME/module_source.yml`:

   ```shell
   dhctl mirror-modules -d /tmp/d8-modules -m /tmp/module_source.yml
   ```

1. Опционально: Скопируйте утилиту `dhctl` из контейнера в директорию со скачанными образами Deckhouse.

   ```shell
   cp /usr/bin/dhctl /tmp/d8-modules/dhctl
   ```

1. Передайте директорию с загруженными образами модулей на хост с доступом к изолированному registry.
   Для продолжения установки используйте скопированную ранее утилиту `dhctl` или запустите установщик Deckhouse аналогично пунктам 1 и 2 на хосте с доступом к изолированному registry. Не забудьте смонтировать директорию с загруженными образами модулей в контейнер установщика.

1. Загрузите образы модулей в изолированный registry с помощью команды `dhctl mirror-modules`.

   Пример команды для загрузки образов из директории `/tmp/d8-modules`:

   ```shell
   DHCTL_CLI_MIRROR_USER="<USERNAME>" DHCTL_CLI_MIRROR_PASS="<PASSWORD>" dhctl mirror-modules -d /tmp/d8-modules --registry="your.private.registry.com:5000/deckhouse-modules"
   ```

   > Обратите внимание, образы будут выгружены в registry по пути, указанному в параметре `--registry` (в примере - /deckhouse-modules).
   > Перед запуском команды убедитесь, что этот путь существует и у используемой учетной записи есть права на запись.

   Если ваш registry не требует авторизации, флаги `--registry-login`/`--registry-password` указывать не нужно.

1. После загрузки образов в изолированный registry, отредактируйте YAML-манифест `ModuleSource`:

   * Измените поле `.spec.registry.repo` на адрес, который вы указали в параметре `--registry` при выгрузке образов;
   * Измените поле `.spec.registry.dockerCfg` на Base64-строку с данными для авторизации в вашем registry в формате `dockercfg`. Обратитесь к документации вашего registry для получения информации о том, как сгенерировать этот токен.

1. Примените в кластере полученный на прошлом шаге манифест `ModuleSource`:

   ```shell
   kubectl apply -f $HOME/module_source.yml
   ```

   > После применения манифеста модули готовы к использованию. Обратитесь к документации разработчика модулей для получения дальнейших инструкций по их настройке и использованию.

## Отправка уведломления об обновлении

<!--(уточнить, что для этого нужно, это примерный сценарий).
Состав уведомлений может варьироваться в зависимости от конкретной системы и приложения. Обычно они состоят из следующих элементов:
* Заголовок уведомления - содержит информацию о типе уведомления и его источнике.
* Текст уведомления - содержит описание обновления или изменения, которое было сделано.
* Ссылка на обновление - если доступно, ссылка на страницу или приложение, где можно получить более подробную информацию об обновлении.
* Действия пользователя - какие действия пользователь может предпринять в ответ на уведомление, например, обновить страницу или установить обновление.-->

### Получение Changelog

Changelog - подробный список изменений, который можно найти для каждого обновления Deckhouse в общем списке релизов. Также, если настроены автоматические оповещения, о которых говорили выше, то ссылка на Changelog передается в строке changelogLink.
Важные изменения в кластере (обновление версии компонентов и их перезапуск, устаревшие компоненты/параметры и т.п.) внедряются в минорных версиях релиза и информацию об этих изменениях можно найти в описании нулевой patch-версии релиза. Например, в v1.49.0 для релиза v1.49 - здесь сообщается, что Docker CRI больше не поддерживается и для обновления необходимо перейти на containerd. Таким образом, перед обновлением необходимо ознакомиться с Changelog и внести соответствующие изменения в кластер, если это требуется.
Для критических изменений, из-за которых обновление невозможно, настроены алерты. Например:
* `D8NodeHasDeprecatedOSVersion` - на нодах установлена устаревшая ОС;
* `HelmReleasesHasResourcesWithDeprecatedVersions` - в helm-релизах используются устаревшие ресурсы;
* `KubernetesVersionEndOfLife` - текущая версия Kubernetes больше не поддерживается.


### Оповещение об обновлении Deckhouse Kubernetes Platform

В режиме обновлений `Auto` можно [настроить](configuration.html#parameters-update-notification) вызов webhook'а для получения оповещения о предстоящем обновлении минорной версии Deckhouse.

Пример настройки оповещения:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: deckhouse
spec:
  version: 1
  settings:
    update:
      releaseChannel: Stable
      mode: Auto
      notification:
        webhook: https://release-webhook.mydomain.com
```

После появления новой минорной версии Deckhouse на используемом канале обновлений, но до момента применения ее в кластере на адрес webhook'а будет выполнен [POST-запрос](configuration.html#parameters-update-notification-webhook).

Чтобы всегда иметь достаточно времени для реакции на оповещение об обновлении Deckhouse, достаточно настроить параметр [minimalNotificationTime](configuration.html#parameters-update-notification-minimalnotificationtime). В этом случае обновление случится по прошествии указанного времени с учетом окон обновлений.

Пример:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: deckhouse
spec:
  version: 1
  settings:
    update:
      releaseChannel: Stable
      mode: Auto
      notification:
        webhook: https://release-webhook.mydomain.com
        minimalNotificationTime: 8h
```

{% alert %}
Если не указать адрес в параметре [update.notification.webhook](configuration.html#parameters-update-notification-webhook), но указать время в параметре [update.notification.minimalNotificationTime](configuration.html#parameters-update-notification-minimalnotificationtime), применение новой версии все равно будет отложено как минимум на указанное в параметре `minimalNotificationTime` время. В этом случае оповещением о появлении новой версии можно считать появление в кластере ресурса [DeckhouseRelease](cr.html#deckhouserelease), имя которого соответствует новой версии.
{% endalert %}

## Режимы обновлений

Существуют два возможных режима обновления:
1. **Автоматический + окна обновлений не заданы.** Кластер обновится сразу после появления новой версии на соответствующем [канале обновлений](https://deckhouse.ru/documentation/deckhouse-release-channels.html) и **Автоматический + заданные окна обновлений.** Кластер обновится в ближайшее доступное окно после появления новой версии на канале обновлений.
2. **Ручной режим.** Для применения обновления требуются [ручные действия](modules/002-deckhouse/usage.html#ручное-подтверждение-обновлений).

### Автоматический режим обновления

При указании в конфигурации модуля `deckhouse` параметра `releaseChannel` Deckhouse будет каждую минуту проверять данные о релизе на канале обновлений.

При появлении нового релиза Deckhouse скачивает его в кластер и создает custom resource [DeckhouseRelease](modules/002-deckhouse/cr.html#deckhouserelease).

После появления custom resource'а `DeckhouseRelease` в кластере Deckhouse выполняет обновление на соответствующую версию согласно установленным [параметрам обновления](modules/002-deckhouse/configuration.html#parameters-update) (по умолчанию — автоматически, в любое время).

Чтобы посмотреть список и состояние всех релизов, воспользуйтесь командной:

```shell
kubectl get deckhousereleases
```

{% alert %}
Patch-релизы (например, обновление на версию `1.30.2` при установленной версии `1.30.1`) устанавливаются без учета режима и окон обновления, то есть при появлении на канале обновления patch-релиза он всегда будет установлен.
{% endalert %}

**Настройка автоматического режима обновления**

Если в автоматическом режиме окна обновлений не заданы, Deckhouse обновится сразу, как только новый релиз станет доступен.

Patch-версии (например, обновления с `1.26.1` до `1.26.2`) устанавливаются без подтверждения и без учета окон обновлений.

{% alert %}
Вы также можете настраивать окна disruption-обновлений узлов в custom resource'ах [NodeGroup](../040-node-manager/cr.html#nodegroup) (параметр `disruptions.automatic.windows`).
{% endalert %}

**Отключение автоматического обновления**

Чтобы полностью отключить механизм обновления Deckhouse, удалите в [конфигурации](modules/002-deckhouse/configuration.html) модуля `deckhouse` параметр [releaseChannel](modules/002-deckhouse/configuration.html#parameters-releasechannel).

В этом случае Deckhouse не проверяет обновления и даже обновление на patch-релизы не выполняется.

{% alert level="danger" %}
Крайне не рекомендуется отключать автоматическое обновление! Это заблокирует обновления на patch-релизы, которые могут содержать исправления критических уязвимостей и ошибок.
{% endalert %}

### Ручной режим обновления

#### Ручное подтверждение обновлений

При необходимости возможно включить ручное подтверждение обновлений. Сделать это можно следующим образом:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: deckhouse
spec:
  version: 1
  settings:
    releaseChannel: Stable
    update:
      mode: Manual
```

В этом режиме необходимо подтверждать каждое минорное обновление Deckhouse (без учета patch-версий).

Пример подтверждения обновления на версию `v1.43.2`:

```shell
kubectl patch DeckhouseRelease v1.43.2 --type=merge -p='{"approved": true}'
```

#### Ручное подтверждение потенциально опасных (disruptive) обновлений

При необходимости возможно включить ручное подтверждение потенциально опасных (disruptive) обновлений (которые меняют значения по умолчанию или поведение некоторых модулей). Сделать это можно следующим образом:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: deckhouse
spec:
  version: 1
  settings:
    releaseChannel: Stable
    update:
      disruptionApprovalMode: Manual
```

В этом режиме необходимо подтверждать каждое минорное потенциально опасное (disruptive) обновление Deckhouse (без учета patch-версий) с помощью аннотации `release.deckhouse.io/disruption-approved=true` на соответствующем ресурсе [DeckhouseRelease](cr.html#deckhouserelease).

Пример подтверждения минорного потенциально опасного обновления Deckhouse `v1.36.4`:

```shell
kubectl annotate DeckhouseRelease v1.36.4 release.deckhouse.io/disruption-approved=true
```
### Уточнение режима обновления кластера

Посмотреть режим обновления кластера можно в [конфигурации](modules/002-deckhouse/configuration.html) модуля `deckhouse`. Для этого выполните следующую команду:

```shell
kubectl get mc deckhouse -oyaml
```

Пример вывода:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  creationTimestamp: "2022-12-14T11:13:03Z"
  generation: 1
  name: deckhouse
  resourceVersion: "3258626079"
  uid: c64a2532-af0d-496b-b4b7-eafb5d9a56ee
spec:
  settings:
    releaseChannel: Stable
    update:
      windows:
      - days:
        - Mon
        from: "19:00"
        to: "20:00"
  version: 1
status:
  state: Enabled
  status: ""
  type: Embedded
  version: "1"
```

## Поддержка последних версий Kubernetes

Как только на установленном в кластере канале обновления появляется новая версия Deckhouse Kubernetes Platform:
- Загорается алерт `DeckhouseReleaseIsWaitingManualApproval`, если кластер использует ручной режим обновлений (параметр [update.mode](modules/002-deckhouse/configuration.html#parameters-update-mode) установлен в `Manual`).
- Появляется новый custom resource [DeckhouseRelease](modules/002-deckhouse/cr.html#deckhouserelease). Используйте команду `kubectl get deckhousereleases`, чтобы посмотреть список релизов. Если `DeckhouseRelease` новой версии находится в состоянии `Pending`, указанная версия еще не установлена. Возможные причины, при которых `DeckhouseRelease` может находиться в `Pending`:
  - Установлен ручной режим обновлений (параметр [update.mode](modules/002-deckhouse/configuration.html#parameters-update-mode) установлен в `Manual`).
  - Установлен автоматический режим обновлений и настроены [окна обновлений](modules/002-deckhouse/usage.html#конфигурация-окон-обновлений), интервал которых еще не наступил.
  - Установлен автоматический режим обновлений, окна обновлений не настроены, но применение версии отложено на случайный период времени из-за механизма снижения нагрузки на репозиторий образов контейнеров. В поле `status.message` ресурса `DeckhouseRelease` будет соответствующее сообщение.
  - Установлен параметр [update.notification.minimalNotificationTime](modules/002-deckhouse/configuration.html#parameters-update-notification-minimalnotificationtime) и указанное в нем время еще не прошло.

### Обновление версий Kubernetes

Чтобы обновить версию Kubernetes в кластере, измените параметр [kubernetesVersion](installing/configuration.html#clusterconfiguration-kubernetesversion) в структуре [ClusterConfiguration](installing/configuration.html#clusterconfiguration) выполнив следующие шаги:
1. Выполните команду:

   ```shell
   kubectl -n d8-system exec -ti deploy/deckhouse -- deckhouse-controller edit cluster-configuration
   ```

1. Измените параметр `kubernetesVersion`.
1. Сохраните изменения. Узлы кластера начнут последовательно обновляться.
1. Дождитесь окончания обновления.  Отслеживать ход обновления можно с помощью команды `kubectl get no`. Обновление можно считать завершенным, когда в выводе команды у каждого узла кластера в колонке `VERSION` появится обновленная версия.

## Поддержка более 2-ух последних версий Kubernetes (CE)

В базовых редакциях поддерживается только две последних версии Kubernetes, в более продвинутых поддерживается 4 последние вышедшие версии Kubernetes.						

## Поддержка LTS (EE)

Информацию о всех версиях Deckhouse Kubernetes Platform можно найти в [списке релизов](https://github.com/deckhouse/deckhouse/releases) Deckhouse.

Сводную информацию о важных изменениях, об обновлении версий компонентов, а также о том, какие компоненты в кластере буду перезапущены в процессе обновления, можно найти в описании нулевой patch-версии релиза. Например, [v1.46.0](https://github.com/deckhouse/deckhouse/releases/tag/v1.46.0) для релиза v1.46 Deckhouse.

Подробный список изменений можно найти в Changelog, ссылка на который есть в каждом [релизе](https://github.com/deckhouse/deckhouse/releases).

Долгосрочная поддержка (LTS) относится к поддержке определенного выпуска (обычно это самая последняя версия, которая поддерживается в течение длительного периода времени).

<!--нужен этот сценарий-->

### Уведомление о процедуре обновления в кластере

Получать заранее информацию об обновлении минорных версий Deckhouse на канале обновлений можно следующими способами:
- Настроить ручной [режим обновлений](modules/002-deckhouse/configuration.html#parameters-update-mode). В этом случае при появлении новой версии на канале обновлений загорится алерт `DeckhouseReleaseIsWaitingManualApproval` и в кластере появится новый custom resource [DeckhouseRelease](modules/002-deckhouse/cr.html#deckhouserelease).
- Настроить автоматический [режим обновлений](modules/002-deckhouse/configuration.html#parameters-update-mode) и указать минимальное время в параметре [minimalNotificationTime](modules/002-deckhouse/configuration.html#parameters-update-notification-minimalnotificationtime), на которое будет отложено обновление. В этом случае при появлении новой версии на канале обновлений в кластере появится новый custom resource [DeckhouseRelease](modules/002-deckhouse/cr.html#deckhouserelease). А если указать URL в параметре [update.notification.webhook](modules/002-deckhouse/configuration.html#parameters-update-notification-webhook), дополнительно произойдет вызов webhook'а.

Во время обновления:
- горит алерт `DeckhouseUpdating`;
- под `deckhouse` не в статусе `Ready`. Если под долго не переходит в статус `Ready`, это может говорить о наличии проблем в работе Deckhouse. Необходима диагностика.

### Уведомление об успешном обновлении в кластере

Если алерт `DeckhouseUpdating` погас, значит, обновление завершено.

Вы также можете проверить состояние [релизов](modules/002-deckhouse/cr.html#deckhouserelease) Deckhouse.

Пример:

```console
$ kubectl get deckhouserelease
NAME       PHASE        TRANSITIONTIME   MESSAGE
v1.46.8    Superseded   13d              
v1.46.9    Superseded   11d              
v1.47.0    Superseded   4h12m            
v1.47.1    Deployed     4h12m            
```

Статус `Deployed` у соответствующей версии говорит о том, что переключение на соответствующую версию было выполнено (но это не значит, что оно закончилось успешно).

Проверьте состояние пода Deckhouse Kubernetes Platform:

```shell
$ kubectl -n d8-system get pods -l app=deckhouse
NAME                   READY  STATUS   RESTARTS  AGE
deckhouse-7844b47bcd-qtbx9  1/1   Running  0       1d
```

* Если статус пода `Running` и в колонке READY указано `1/1` — обновление закончилось успешно.
* Если статус пода `Running` и в колонке READY указано `0/1` — обновление еще не закончилось. Если это продолжается более 20–30 минут, это может говорить о наличии проблем в работе Deckhouse. Необходима диагностика.
* Если статус пода не `Running`, это может говорить о наличии проблем в работе Deckhouse. Необходима диагностика.

{% alert level="info" %}
Возможные варианты действий, если что-то пошло не так:
- Проверьте логи, используя следующую команду:

  ```shell
  kubectl -n d8-system logs -f -l app=deckhouse | jq -Rr 'fromjson? | .msg'
  ```

- Соберите [отладочную информацию](modules/002-deckhouse/faq.html#как-собрать-информацию-для-отладки) и свяжитесь с технической поддержкой.
- Попросите помощи у [сообщества](https://deckhouse.ru/community/about.html).
{% endalert %}
