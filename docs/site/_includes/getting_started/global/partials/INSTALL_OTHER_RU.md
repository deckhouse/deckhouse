{% assign revision=include.revision %}

Для установки **Deckhouse Platform** воспользуемся готовым Docker-образом. В образ необходимо передать конфигурационные файлы, а так же ssh-ключи для доступа на master-узлы:

{%- if revision == 'ee' %}
{% snippetcut selector="docker-login" %}
```shell
docker login -u license-token -p <LICENSE_TOKEN> registry.deckhouse.io
docker run -it -v "$PWD/config.yml:/config.yml" -v "$HOME/.ssh/:/tmp/.ssh/" \
{% if page.platform_type == "existing" %} -v "$PWD/kubeconfig:/kubeconfig" \
{% endif %}{% if page.platform_type == "cloud" %} -v "$PWD/resources.yml:/resources.yml" -v "$PWD/dhctl-tmp:/tmp" {% endif %} registry.deckhouse.io/deckhouse/ee/install:stable bash
```
{% endsnippetcut %}
{% else %}
{% snippetcut %}
```shell
docker run -it -v "$PWD/config.yml:/config.yml" -v "$HOME/.ssh/:/tmp/.ssh/" \
{% if page.platform_type == "existing" %} -v "$PWD/kubeconfig:/kubeconfig" \
{% endif %}{% if page.platform_type == "cloud" %} -v "$PWD/resources.yml:/resources.yml" -v "$PWD/dhctl-tmp:/tmp" {% endif %} registry.deckhouse.io/deckhouse/ce/install:stable bash
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

{%- if page.platform_type == "cloud" %}
Примечания:
<ul>
<li>
<div markdown="1">
Благодаря использованию параметра `-v "$PWD/dhctl-tmp:/tmp/dhctl"` состояние данных Terraform-инстяллятора будет сохранено во временной директории на хосте запуска, что позволит корректно продолжить установку в случае прерывания работы контейнера с инсталлятором.
</div>
</li>
<li><p>В случае возникновения проблем во время разворачивания кластера{% if page.platform_type="cloud" %} в одном из облачных провайдеров{% endif %}, для остановки процесса установки воспользуйтесь следующей командой (файл конфигурации должен совпадать с тем, с которым производилось разворачивание кластера):</p>
<div markdown="0">
{% snippetcut %}
```shell
dhctl bootstrap-phase abort --config=/config.yml
```
{% endsnippetcut %}
</div></li>
</ul>
{%- endif %}
{%- endif %}

По окончании установки произойдет возврат к командной строке.

Почти все готово для полноценной работы Deckhouse Platform!
