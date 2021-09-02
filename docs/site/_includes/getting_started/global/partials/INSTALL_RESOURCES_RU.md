{% assign revision=include.revision %}

{% if revision == 'ee' %}
{% include getting_started/global/EE_ACCESS_RU.md %}
{% endif %}

Ниже сгенерированы рекомендованные настройки для установки Deckhouse Platform {% if revision == 'ee' %}Enterprise Edition{% else %}Community Edition{% endif %}:
- `config.yml` — файл первичной конфигурации кластера. Содержит параметры инсталлятора{% if page.platform_type=='cloud' %}, параметры доступа облачного проавайдера{% endif %} и начальные параметры кластера.
- `resources.yml` — описание ресурсов для создания после установки (настройки узлов и ingress-контроллера).

**Обратите внимание**:
- <span class="mustChange">обязательные</span> для самостоятельного заполнения параметры.
- <span class="mightChange">опциональные</span> параметры.

> Полное описание параметров конфигурации cloud-провайдеров вы можете найти [документации](https://deckhouse.io/ru/documentation/v1/kubernetes.html).
>
> Подробнее о каналах обновления Deckhouse Platform (release channels) можно почитать в [документации](/ru/documentation/v1/deckhouse-release-channels.html).

{% snippetcut name="config.yml" selector="config-yml" %}
{% include_file "_includes/getting_started/{{ page.platform_code }}/partials/config.yml.{{ include.layout }}.{{ revision }}.inc" syntax="yaml" %}
{% endsnippetcut %}

<!-- TODO -->
{% snippetcut name="resources.yml" selector="resources-yml" %}
{% include_file "_includes/getting_started/{{ page.platform_code }}/partials/resources.yml.minimal.inc" syntax="yaml" %}
{% endsnippetcut %}

Для установки **Deckhouse Platform** воспользуемся готовым Docker-образом. В образ необходимо передать конфигурационные файлы, а так же ssh-ключи для доступа на master-узлы:

{%- if revision == 'ee' %}
{% snippetcut selector="docker-login" %}
```shell
docker login -u license-token -p <LICENSE_TOKEN> registry.deckhouse.io
docker run -it -v "$PWD/config.yml:/config.yml" -v "$PWD/resources.yml:/resources.yml" -v "$HOME/.ssh/:/tmp/.ssh/" \
{% if page.platform_type == "existing" %} -v "$PWD/kubeconfig:/kubeconfig" \
{% endif %}{% if page.platform_type == "cloud" %} -v "$PWD/dhctl-tmp:/tmp/dhctl" {% endif %} registry.deckhouse.io/deckhouse/ee/install:stable bash
```
{% endsnippetcut %}
{% else %}
{% snippetcut %}
```shell
docker run -it -v "$PWD/config.yml:/config.yml" -v "$PWD/resources.yml:/resources.yml" -v "$HOME/.ssh/:/tmp/.ssh/" \
{% if page.platform_type == "existing" %} -v "$PWD/kubeconfig:/kubeconfig" \
{% endif %}{% if page.platform_type == "cloud" %} -v "$PWD/dhctl-tmp:/tmp/dhctl" {% endif %} registry.deckhouse.io/deckhouse/ce/install:stable bash
```
{% endsnippetcut %}
{% endif %}

{%- if page.platform_type == "existing" %}
Примечания:
- В kubeconfig необходимо смонтировать kubeconfig с доступом к Kubernetes API.
{% endif %}

Внутри контейнера выполните команду:

{% snippetcut %}
```shell
{%- if page.platform_type == "existing" %}
dhctl bootstrap-phase install-deckhouse \
  --kubeconfig=/kubeconfig \
  --config=/config.yml \
  --resources=/resources.yml
{%- elsif page.platform_type == "baremetal" %}
dhctl bootstrap \
  --ssh-user=<username> \
  --ssh-host=<master_ip> \
  --ssh-agent-private-keys=/tmp/.ssh/id_rsa \
  --config=/config.yml \
  --resources=/resources.yml
{%- elsif page.platform_type == "cloud" %}
dhctl bootstrap \
  --ssh-user=<username> \
  --ssh-agent-private-keys=/tmp/.ssh/id_rsa \
  --config=/config.yml \
  --resources=/resources.yml
{%- endif %}
```
{% endsnippetcut %}

{%- if page.platform_type == "baremetal" or page.platform_type == "cloud" %}
{%- if page.platform_type == "baremetal" %}
Здесь, переменная `username` — это имя пользователя, от которого генерировался SSH-ключ для установки.
{%- else %}
Здесь, переменная `username` —
{%- if page.platform_code == "openstack" %} имя пользователя по умолчанию для выбранного образа виртуальной машины.
{%- elsif page.platform_code == "azure" %} `azureuser` (для предложенных в этой документации образов).
{%- elsif page.platform_code == "gcp" %} `user` (для предложенных в этой документации образов).
{%- else %} `ubuntu` (для предложенных в этой документации образов).
{%- endif %}
{%- endif %}

Примечания:
<ul>
{%- if page.platform_type == "cloud" %}
<li>
<div markdown="1">
Благодаря использованию параметра `-v "$PWD/dhctl-tmp:/tmp/dhctl"` состояние данных Terraform-инстяллятора будет сохранено во временной директории на хосте запуска, что позволит корректно продолжить установку в случае прерывания работы контейнера с инсталлятором.
</div>
</li>
{%- endif %}
<li><p>В случае возникновения проблем во время разворачивания кластера{% if page.platform_type="cloud" %} в одном из облачных провайдеров{% endif %}, для остановки процесса установки воспользуйтесь следующей командой (файл конфигурации должен совпадать с тем, с которым производилось разворачивание кластера):</p>
<div markdown="0">
{% snippetcut %}
```shell
dhctl bootstrap-phase abort \
  --ssh-user=<username> \
  --ssh-agent-private-keys=/tmp/.ssh/id_rsa \
  --config=/config.yml
```
{% endsnippetcut %}
</div></li>
{% endif %}
</ul>

По окончании установки произойдет возврат к командной строке.

Почти все готово для полноценной работы Deckhouse Platform!

Чтобы можно было использовать любой модуль Deckhouse Platform, в кластер необходимо добавить узлы.
