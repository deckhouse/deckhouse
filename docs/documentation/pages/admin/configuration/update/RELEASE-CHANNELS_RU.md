---
title: Каналы обновлений
permalink: ru/admin/configuration/update/release-channels.html
lang: ru
---

{% capture asset_url %}{%- css_asset_tag releases %}[_assets/css/releases.css]{% endcss_asset_tag %}{% endcapture %}
<link rel="stylesheet" type="text/css" href='{{ asset_url | strip_newlines  | true_relative_url }}' />

{%- assign releases = site.data.releases.channels | sort: "stability" -%}

{% alert level="info" %}
Актуальная информация о версиях DKP на разных каналах обновлений доступна на сайте [releases.deckhouse.ru](https://releases.deckhouse.ru).
{% endalert %}

Deckhouse Kubernetes Platform (DKP) использует **пять каналов обновлений**, обеспечивающих поэтапное развертывание новых версий.
Каждая новая версия DKP сначала публикуется в канале **Alpha** и постепенно продвигается к **Rock Solid**.
Обновления из менее стабильных каналов доступны небольшому числу пользователей, что позволяет выявлять и устранять потенциальные проблемы, прежде чем они затронут production-среды.

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

## Проверка текущего канала обновлений

Чтобы узнать, какой канал обновлений используется в кластере, выполните следующую команду:

```shell
sudo -i d8 k get mc deckhouse -o yaml | grep releaseChannel
```

Пример вывода:

```console
    releaseChannel: Stable
```

## Смена канала обновлений

Чтобы сменить канал обновлений, укажите его название в параметре `settings.releaseChannel` модуля `deckhouse`(#TODO).

Пример конфигурации с каналом `Stable`:

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

### Процесс переключения на более стабильный канал

При переключении на более стабильный канал (например, с `Alpha` на `EarlyAccess`):

1. DKP скачивает данные о релизах из канала `EarlyAccess`.
1. Сравнивает их с данными из существующих в кластере кастомных ресурсов DeckhouseRelease.
   - Если в кластере есть более поздние релизы со статусом `Pending` (ещё не применены), они будут **удалены**, поскольку они отсутствуют на новом канале.
   - Если более поздние релизы уже перешли в статус `Deployed` (успешно установлены), переход на новый релиз произойдёт не сразу. DKP останется на текущем релизе до тех пор, пока на канале `EarlyAccess` не выйдет более поздняя версия, которую можно будет установить.

### Процесс переключения на менее стабильный канал

При переключении на менее стабильный канал (например, с `EarlyAccess` на `Alpha`):

1. DKP скачивает данные о релизах из канала `Alpha`.
1. Сравнивает их с данными из существующих в кластере кастомных ресурсов DeckhouseRelease.
1. Выполняет обновление в соответствии [с настроенными параметрами обновления](configuration.html).
