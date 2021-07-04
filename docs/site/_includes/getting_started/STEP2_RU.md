## Шаг 2. Установка

Для непосредственной установки потребуется Docker-образ установщика Deckhouse. Мы воспользуемся уже готовым официальным образом от проекта. Информацию по самостоятельной сборке образа из исходников можно будет найти в [репозитории проекта](https://github.com/deckhouse/deckhouse).

В результате запуска следующих команд произойдет скачивание Docker-образа установщика Deckhouse, в который будут передана приватная часть SSH-ключа и файл конфигурации, подготовленные на прошлом шаге (пути расположения файлов даны по умолчанию). Будет запущен интерактивный терминал в системе образа.

-  Для редакции CE:

   ```shell
docker run -it -v "$(pwd)/config.yml:/config.yml" -v "$HOME/.ssh/:/tmp/.ssh/"
{%- if include.mode == "existing" %} -v "$(pwd)/kubeconfig:/kubeconfig" {% endif %}
{%- if include.mode == "cloud" %} -v "$(pwd)/dhctl-tmp:/tmp" {% endif %} registry.deckhouse.io/deckhouse/ce/install:beta bash
```

-  Для редакции EE:

   ```shell
docker login -u license-token -p <LICENSE_TOKEN> registry.deckhouse.io
docker run -it -v "$(pwd)/config.yml:/config.yml" -v "$HOME/.ssh/:/tmp/.ssh/"
{%- if include.mode == "existing" %} -v "$(pwd)/kubeconfig:/kubeconfig" {% endif %}
{%- if include.mode == "cloud" %} -v "$(pwd)/dhctl-tmp:/tmp" {% endif %} registry.deckhouse.io/deckhouse/ee/install:beta bash
```

{%- if include.mode == "existing" %}
Примечания:
-  В kubeconfig необходимо смонтировать kubeconfig с доступом к Kubernetes API.
{% endif %}

Далее для запуска установки необходимо выполнить команду:

```shell
{%- if include.mode == "existing" %}
dhctl bootstrap-phase install-deckhouse \
  --kubeconfig=/kubeconfig \
  --config=/config.yml
{%- elsif include.mode == "baremetal" %}
dhctl bootstrap \
  --ssh-user=<username> \
  --ssh-host=<master_ip> \
  --ssh-agent-private-keys=/tmp/.ssh/id_rsa \
  --config=/config.yml
{%- elsif include.mode == "cloud" %}
dhctl bootstrap \
  --ssh-user=<username> \
  --ssh-agent-private-keys=/tmp/.ssh/id_rsa \
  --config=/config.yml
{% endif %}
```

{%- if include.mode == "baremetal" or include.mode == "cloud" %}
{%- if include.mode == "baremetal" %}
Здесь переменная `username` — это имя пользователя, от которого генерировался SSH-ключ для установки.
{%- else %}
Здесь переменная `username` —
{%- if include.provider == "openstack" %} имя пользователя по умолчанию для выбранного образа виртуальной машины.
{%- elsif include.provider == "azure" %} `azureuser` (для предложенных в этой документации образов).
{%- else %} `ubuntu` (для предложенных в этой документации образов).
{%- endif %}
{%- endif %}

Примечания:
{%- if include.mode == "cloud" %}
- Благодаря использованию параметра `-v "$(pwd)/dhctl-tmp:/tmp"` состояние данных Terraform-инстяллятора будет сохранено во временной директории на хосте запуска, что позволит корректно продолжить установку в случае прерывания работы контейнера с инсталлятором.
{%- endif %}
- В случае возникновения проблем во время разворачивания кластера {% if include.mode="cloud" %}в одном из облачных провайдеров {% endif %}для остановки процесса установки следует воспользоваться следующей командой (файл конфигурации должен совпадать с тем, с которым производилось разворачивание кластера):

  ```shell
dhctl bootstrap-phase abort --config=/config.yml
```
{% endif %}

По окончании установки произойдет возврат к командной строке. Deckhouse готов к работе! Можно управлять дополнительными модулями, разворачивать ваши приложения и т.п.
