Такой вариант установки подходит для локального знакомства с Deckhouse Stronghold: он запускается в режиме разработки (dev), хранит данные в оперативной памяти и не требует кластера Kubernetes.

{% alert level="warning" %}
Режим разработки предназначен только для ознакомления и тестирования. Не используйте его в production-окружении: при остановке процесса все данные будут потеряны.
{% endalert %}

## Загрузка архива

Для загрузки архива со Stronghold для ОС Linux потребуется лицензионный ключ Deckhouse Stronghold редакции Enterprise Edition или Certified Security Edition. Выберите редакцию, введите ключ, выберите версию и нажмите «Скачать»: браузер сохранит архив `stronghold-<версия>.tar` с исполняемым файлом для Linux (amd64).

{% include getting_started/stronghold/linux/partials/download.html.liquid %}

Распакуйте скачанный архив, указав его фактическое имя (оно содержит версию Stronghold):

<div id="stronghold-unpack" markdown="1">
```bash
tar -xf stronghold-v1.19.3.tar
```
</div>

## Запуск Stronghold

Запустите Stronghold в режиме разработки, указав корневой токен:

```bash
./stronghold server -dev -dev-root-token-id root
```

Stronghold будет доступен по адресу `http://127.0.0.1:8200`, процесс остается запущенным в текущем терминале.
