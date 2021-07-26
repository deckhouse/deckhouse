Для непосредственной установки **Deckhouse Platform {% if page.revision == 'ee' %}Enterprise Edition{% else %}Community Edition{% endif %}** {% if page.platform_type == 'baremetal' %}на{% else %}в{% endif %} {{ page.platform_name[page.lang] }} потребуется Docker-образ его установщика. Мы воспользуемся уже готовым официальным образом от проекта. Информацию по самостоятельной сборке образа из исходников можно будет найти в [репозитории проекта](https://github.com/deckhouse/deckhouse).

В результате запуска следующих команд произойдет скачивание Docker-образа установщика Deckhouse Platform {% if page.revision == 'ee' %}Enterprise Edition{% else %}Community Edition{% endif %}, в который будут переданы приватная часть SSH-ключа и файл конфигурации, подготовленные на предыдущем шаге (пути расположения файлов даны по умолчанию). Будет запущен интерактивный терминал в системе образа.

{%- if page.revision == 'ee' %}
   ```shell
docker login -u license-token -p <LICENSE_TOKEN> registry.deckhouse.io
docker run -it -v "$PWD/config.yml:/config.yml" -v "$HOME/.ssh/:/tmp/.ssh/"
{%- if page.platform_type == "existing" %} -v "$PWD/kubeconfig:/kubeconfig" {% endif %}
{%- if page.platform_type == "cloud" %} -v "$PWD/dhctl-tmp:/tmp" {% endif %} registry.deckhouse.io/deckhouse/ee/install:alpha bash
```
{% else %}
   ```shell
docker run -it -v "$PWD/config.yml:/config.yml" -v "$HOME/.ssh/:/tmp/.ssh/"
{%- if page.platform_type == "existing" %} -v "$PWD/kubeconfig:/kubeconfig" {% endif %}
{%- if page.platform_type == "cloud" %} -v "$PWD/dhctl-tmp:/tmp" {% endif %} registry.deckhouse.io/deckhouse/ce/install:alpha bash
```
{% endif %}

{%- if page.platform_type == "existing" %}
Примечания:
-  В kubeconfig необходимо смонтировать kubeconfig с доступом к Kubernetes API.
{% endif %}

Выполните команду для запуска установки:

```shell
{%- if page.platform_type == "existing" %}
dhctl bootstrap-phase install-deckhouse \
  --kubeconfig=/kubeconfig \
  --config=/config.yml
{%- elsif page.platform_type == "baremetal" %}
dhctl bootstrap \
  --ssh-user=<username> \
  --ssh-host=<master_ip> \
  --ssh-agent-private-keys=/tmp/.ssh/id_rsa \
  --config=/config.yml
{%- elsif page.platform_type == "cloud" %}
dhctl bootstrap \
  --ssh-user=<username> \
  --ssh-agent-private-keys=/tmp/.ssh/id_rsa \
  --config=/config.yml
{%- endif %}
```

{%- if page.platform_type == "baremetal" or page.platform_type == "cloud" %}
{%- if page.platform_type == "baremetal" %}
Здесь, переменная `username` — это имя пользователя, от которого генерировался SSH-ключ для установки.
{%- else %}
Здесь, переменная `username` —
{%- if page.platform_code == "openstack" %} имя пользователя по умолчанию для выбранного образа виртуальной машины.
{%- elsif page.platform_code == "azure" %} `azureuser` (для предложенных в этой документации образов).
{%- else %} `ubuntu` (для предложенных в этой документации образов).
{%- endif %}
{%- endif %}

Примечания:
{%- if page.platform_type == "cloud" %}
- Благодаря использованию параметра `-v "$PWD/dhctl-tmp:/tmp"` состояние данных Terraform-инстяллятора будет сохранено во временной директории на хосте запуска, что позволит корректно продолжить установку в случае прерывания работы контейнера с инсталлятором.
{%- endif %}
- В случае возникновения проблем во время разворачивания кластера{% if page.platform_type="cloud" %} в одном из облачных провайдеров{% endif %}, для остановки процесса установки воспользуйтесь следующей командой (файл конфигурации должен совпадать с тем, с которым производилось разворачивание кластера):

  ```shell
dhctl bootstrap-phase abort --config=/config.yml
```
{% endif %}

По окончании установки произойдет возврат к командной строке.

Почти все готово для полноценной работы Deckhouse Platform {% if page.revision == 'ee' %}Enterprise Edition{% else %}Community Edition{% endif %}!

Чтобы можно было использовать любой модуль Deckhouse Platform, в кластер необходимо добавить узлы.
