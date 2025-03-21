---
title: Каналы обновлений
permalink: ru/admin/configuration/update/release-channels.html
lang: ru
---

{% capture asset_url %}{%- css_asset_tag releases %}[_assets/css/releases.css]{% endcss_asset_tag %}{% endcapture %}
<link rel="stylesheet" type="text/css" href='{{ asset_url | strip_newlines  | true_relative_url }}' />

{%- assign releases = site.data.releases.channels | sort: "stability" -%}

{% alert %}
Информацию о том, какие версии Deckhouse находятся в настоящий момент на каких каналах обновлений, а также о планируемой дате смены версии на канале обновлений смотрите на сайте <a href="https://releases.deckhouse.ru" target="_blank">releases.deckhouse.ru</a>.
{% endalert %}

К кластерам как элементам инфраструктуры обычно предъявляются различные требования.

Например, production-кластер, в отличие от кластера разработки, более требователен к надежности: в нем нежелательно часто обновлять или изменять какие-либо компоненты без особой необходимости, при этом сами компоненты должны быть тщательно протестированы.

Deckhouse использует **пять каналов обновлений**. *Мягко* переключаться между ними можно с помощью модуля [deckhouse](modules/deckhouse/): достаточно указать желаемый канал обновлений в [конфигурации](modules/deckhouse/configuration.html#parameters-releasechannel) модуля.

<div id="releases__stale__block" class="releases__info releases__stale__warning" >
  <strong>Внимание!</strong> В этом кластере не используется какой-либо канал обновлений.
</div>

{%- assign channels_sorted = site.data.releases.channels | sort: "stability" %}
{%- assign channels_sorted_reverse = site.data.releases.channels | sort: "stability" | reverse  %}

<div class="page__container page_releases" markdown="0">
<div class="releases__menu">
{%- for channel in channels_sorted_reverse %}
    <div class="releases__menu-item releases__menu--channel--{{ channel.name }}">
        <div class="releases__menu-item-header">
            <div class="releases__menu-item-title releases__menu--channel--{{ channel.name }}">
                {{ channel.title }}
            </div>
        </div>
        <div class="releases__menu-item-description">
            {{ channel.description[page.lang] }}
        </div>
    </div>
{%- endfor %}
</div>
</div>

Для любого из каналов обновлений можно отключить автоматические обновления (за исключением минорных hotfix’ов) или выбрать удобные временные окна (!!!!!link), в которые будут устанавливаться свежие обновления.

Канал обновлений указывается в модуле Deckhouse в параметре `spec.settings.releaseChannel` – если параметр не указан, то автоматическое обновление отключено. Пример конфигурации с каналом `Stable`:

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

При этом, Deckhouse будет каждую минуту проверять данные о релизе на указанном канале обновлений и при появлении нового релиза, скачает его в кластер и создаст Custom Resource `DeckhouseRelease`, после чего начнется обновление согласно установленным правилам.

## Смена канала обновлений

Deckhouse Kubernetes Platform позволяет мягко переключаться между различными каналами обновлений. Чтобы сменить канал, достаточно указать его новое название в настройках модуля Deckhouse. Однако при этом следует учитывать несколько важных моментов.

### Переключение на более стабильный канал

Например, при переходе с канала `Alpha` на `EarlyAccess` Deckhouse:

1. Скачивает данные о релизах из канала `EarlyAccess`.
1. Сравнивает их с уже существующими в кластере Custom Resource `DeckhouseRelease`.

   - Если в кластере есть **более поздние релизы** (например, `v1.68`) со статусом `Pending` (ещё не применены), они будут **удалены**, поскольку в новом канале таких релизов нет.
   - Если **более поздние релизы** уже перешли в статус `Deployed` (успешно установлены), переход на новый релиз не произойдёт сразу. Deckhouse останется на текущем релизе до тех пор, пока на канале `EarlyAccess` не выйдет **ещё более новая версия**, которую можно будет установить.

### Переключение на менее стабильный канал
Например, при переходе с канала `EarlyAccess` на `Alpha` Deckhouse:

1. Скачивает данные о релизах из канала `Alpha`.
1. Сравнивает их с Custom Resource `DeckhouseRelease`, уже существующими в кластере.
1. Далее платформа выполняет обновление в соответствии с **текущими параметрами обновления** (например, режим обновлений `Auto` или `Manual`, окна обновлений и т.д.).

Таким образом, переключение на более стабильный канал может отложить установку новых релизов до тех пор, пока версии в новом канале не станут более свежими, чем уже имеющиеся в кластере. Переключение же на менее стабильный канал сразу задействует релизы, доступные в более быстром (менее стабильном) канале.

### Как установить желаемый канал обновлений?

Чтобы перейти на другой канал обновлений автоматически, нужно в [конфигурации](modules/deckhouse/configuration.html) модуля `deckhouse` изменить (установить) параметр [releaseChannel](modules/deckhouse/configuration.html#parameters-releasechannel).

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

### Отсутствие обновлений из канала обновлений

Если Deckhouse Kubernetes Platform перестал получать обновления из каналов обновлений:

* Проверьте, что [настроен](#как-установить-желаемый-канал-обновлений) нужный канал обновлений.
* Проверьте корректность разрешения DNS-имени хранилища образов Deckhouse.

  Получите и сравните IP-адреса хранилища образов Deckhouse (`registry.deckhouse.ru`) на одном из узлов и в поде Deckhouse. Они должны совпадать.

  Пример получения IP-адреса хранилища образов Deckhouse на узле:

  ```shell
  $ getent ahosts registry.deckhouse.ru
  185.193.90.38    STREAM registry.deckhouse.ru
  185.193.90.38    DGRAM
  185.193.90.38    RAW
  ```

  Пример получения IP-адреса хранилища образов Deckhouse в поде Deckhouse:

  ```shell
  $ kubectl -n d8-system exec -ti svc/deckhouse-leader -c deckhouse -- getent ahosts registry.deckhouse.ru
  185.193.90.38    STREAM registry.deckhouse.ru
  185.193.90.38    DGRAM  registry.deckhouse.ru
  ```

  Если полученные IP-адреса не совпадают, проверьте настройки DNS на узле. В частности, обратите внимание на список доменов в параметре `search` файла `/etc/resolv.conf` (он влияет на разрешение имен в поде Deckhouse). Если в параметре `search` файла `/etc/resolv.conf` указан домен, в котором настроено разрешение wildcard-записей, это может привести к неверному разрешению IP-адреса хранилища образов Deckhouse (см. пример).

{% offtopic title="Пример настроек DNS, которые могут привести к ошибкам в разрешении IP-адреса хранилища образов Deckhouse..." %}

Далее описан пример, когда настройки DNS приводят к различному результату при разрешении имен на узле и в поде Kubernetes:
- Пример файла `/etc/resolv.conf` на узле:

  ```text
  nameserver 10.0.0.10
  search company.my
  ```

  > Обратите внимание, что по умолчанию на узле параметр `ndot` равен 1 (`options ndots:1`). Но в подах Kubernetes параметр `ndot` равен **5**. Таким образом, логика разрешения DNS-имен, имеющих в имени 5 точек и менее, различается на узле и в поде.

- В DNS-зоне `company.my` настроено разрешение wildcard-записей `*.company.my` в адрес `10.0.0.100`. То есть любое DNS-имя в зоне `company.my`, для которого нет конкретной записи в DNS, разрешается в адрес `10.0.0.100`.

Тогда с учетом параметра `search`, указанного в файле `/etc/resolv.conf`, при обращении на адрес `registry.deckhouse.ru` на узле система попробует получить IP-адрес для имени `registry.deckhouse.ru` (так как считает его полностью определенным, учитывая настройку по умолчанию параметра `options ndots:1`).

При обращении же на адрес `registry.deckhouse.ru` **из пода** Kubernetes, учитывая параметры `options ndots:5` (используется в Kubernetes по умолчанию) и `search`, система первоначально попробует получить IP-адрес для имени `registry.deckhouse.ru.company.my`. Имя `registry.deckhouse.ru.company.my` разрешится в IP-адрес `10.0.0.100`, так как в DNS-зоне `company.my` настроено разрешение wildcard-записей `*.company.my` в адрес `10.0.0.100`. В результате к хосту `registry.deckhouse.ru` будет невозможно подключиться и скачать информацию о доступных обновлениях Deckhouse.
{% endofftopic %}
