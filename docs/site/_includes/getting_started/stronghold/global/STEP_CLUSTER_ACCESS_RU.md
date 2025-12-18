<script type="text/javascript" src='{% javascript_asset_tag getting-started %}[_assets/js/getting-started.js]{% endjavascript_asset_tag %}'></script>
<script type="text/javascript" src='{% javascript_asset_tag getting-started-access %}[_assets/js/getting-started-access.js]{% endjavascript_asset_tag %}'></script>
<script type="text/javascript" src='{% javascript_asset_tag getting-started-finish %}[_assets/js/getting-started-finish.js]{% endjavascript_asset_tag %}'></script>
<script type="text/javascript" src='{% javascript_asset_tag bcrypt %}[_assets/js/bcrypt.js]{% endjavascript_asset_tag %}'></script>

## Подключение к master-узлу
Deckhouse завершил процесс установки кластера. Осталось выполнить некоторые настройки, для чего необходимо подключиться к **master-узлу**.

Подключитесь к master-узлу по SSH (IP-адрес master-узла был выведен инсталлятором по завершении установки, но вы также можете найти его используя веб-интерфейс или CLI&#8209;утилиты облачного провайдера):
{% snippetcut %}
```shell
ssh {% if page.platform_code == "azure" %}azureuser{% elsif page.platform_code == "gcp" or page.platform_code == "dynamix" %}user{% else %}ubuntu{% endif %}@<MASTER_IP>
```
{% endsnippetcut %}

Проверьте работу kubectl, выведя список узлов кластера:
{% snippetcut %}
```shell
sudo -i d8 k get nodes
```
{% endsnippetcut %}

{% offtopic title="Пример вывода..." %}
```
$ sudo -i d8 k get nodes
NAME                                     STATUS   ROLES                  AGE   VERSION
cloud-demo-master-0                      Ready    control-plane,master   12h   v1.23.9
cloud-demo-worker-01a5df48-84549-jwxwm   Ready    worker                 12h   v1.23.9
```
{%- endofftopic %}

Запуск Ingress-контроллера после завершения установки Deckhouse может занять какое-то время. Прежде чем продолжить убедитесь что Ingress-контроллер запустился:

{% snippetcut %}
```shell
sudo -i d8 k -n d8-ingress-nginx get po
```
{% endsnippetcut %}

Дождитесь перехода Pod'ов в статус `Ready`.

{% offtopic title="Пример вывода..." %}
```
$ sudo -i d8 k -n d8-ingress-nginx get po
NAME                                       READY   STATUS    RESTARTS   AGE
controller-nginx-r6hxc                     3/3     Running   0          16h
kruise-controller-manager-78786f57-82wph   3/3     Running   0          16h
```
{%- endofftopic %}

{% if page.platform_type == 'cloud' and page.platform_code != 'vsphere' %}
Также дождитесь готовности балансировщика:
{% snippetcut %}
```shell
sudo -i d8 k -n d8-ingress-nginx get svc nginx-load-balancer
```
{% endsnippetcut %}

Значение `EXTERNAL-IP` должно быть заполнено публичным IP-адресом или DNS-именем.

{% offtopic title="Пример вывода..." %}
```
$ sudo -i d8 k -n d8-ingress-nginx get svc nginx-load-balancer
NAME                  TYPE           CLUSTER-IP      EXTERNAL-IP     PORT(S)                      AGE
nginx-load-balancer   LoadBalancer   10.222.91.204   1.2.3.4         80:30493/TCP,443:30618/TCP   1m
```
{%- endofftopic %}
{% endif %}

## DNS

Для того чтобы получить доступ к веб-интерфейсам компонентов Deckhouse, нужно:

- настроить работу DNS
- указать в параметрах Deckhouse [шаблон DNS-имен](/products/kubernetes-platform/documentation/v1/reference/api/global.html#parameters-modules-publicdomaintemplate)

*Шаблон DNS-имен* используется для настройки Ingress-ресурсов системных приложений. Например, за интерфейсом Grafana закреплено имя `grafana`. Тогда, для шаблона `%s.kube.company.my` Grafana будет доступна по адресу `grafana.kube.company.my`, и т.д.

{% if page.platform_type == 'cloud' and page.platform_code != 'vsphere' %}
Чтобы упростить настройку, далее будет использоваться сервис [sslip.io](https://sslip.io/).

На **master-узле** выполните следующую команду, чтобы получить IP-адрес балансировщика и настроить [шаблон DNS-имен](../../documentation/v1/reference/api/global.html#parameters-modules-publicdomaintemplate) сервисов Deckhouse на использование *sslip.io*:
{% if page.platform_code == 'aws' %}
{% snippetcut %}
{% raw %}
```shell
BALANCER_IP=$(dig $(sudo -i d8 k -n d8-ingress-nginx get svc nginx-load-balancer -o json | jq -r '.status.loadBalancer.ingress[0].hostname') +short | head -1) && \
echo "Balancer IP is '${BALANCER_IP}'." && sudo -i d8 k patch mc global --type merge \
  -p "{\"spec\": {\"settings\":{\"modules\":{\"publicDomainTemplate\":\"%s.${BALANCER_IP}.sslip.io\"}}}}" && echo && \
echo "Domain template is '$(sudo -i d8 k get mc global -o=jsonpath='{.spec.settings.modules.publicDomainTemplate}')'."
```
{% endraw %}
{% endsnippetcut %}
{% else %}
{% snippetcut %}
{% raw %}
```shell
BALANCER_IP=$(sudo -i d8 k -n d8-ingress-nginx get svc nginx-load-balancer -o json | jq -r '.status.loadBalancer.ingress[0].ip') && \
echo "Balancer IP is '${BALANCER_IP}'." && sudo -i d8 k patch mc global --type merge \
  -p "{\"spec\": {\"settings\":{\"modules\":{\"publicDomainTemplate\":\"%s.${BALANCER_IP}.sslip.io\"}}}}" && echo && \
echo "Domain template is '$(sudo -i d8 k get mc global -o=jsonpath='{.spec.settings.modules.publicDomainTemplate}')'."
```
{% endraw %}
{% endsnippetcut %}
{% endif %}

Команда также выведет установленный шаблон DNS-имен. Пример вывода:
```text
Balancer IP is '1.2.3.4'.
moduleconfig.deckhouse.io/global patched

Domain template is '%s.1.2.3.4.sslip.io'.
```

{% alert %}
Перегенерация сертификатов после изменения шаблона DNS-имен может занять до 5 минут.
{% endalert %}

{% offtopic title="Другие варианты настройки..." %}
Вместо сервиса *sslip.io* вы можете использовать другие варианты настройки.
{% include getting_started/global/partials/DNS_OPTIONS_RU.liquid %}

Затем, на **master-узле** выполните следующую команду (укажите используемый шаблон DNS-имен в переменной <code>DOMAIN_TEMPLATE</code>):
<div markdown="0">
{% snippetcut %}
```shell
DOMAIN_TEMPLATE='<DOMAIN_TEMPLATE>'
sudo -i d8 k patch mc global --type merge -p "{\"spec\": {\"settings\":{\"modules\":{\"publicDomainTemplate\":\"${DOMAIN_TEMPLATE}\"}}}}"
```
{% endsnippetcut %}
</div>
{% endofftopic %}
{% endif %}

{% if page.platform_type == 'cloud' and page.platform_code == 'vsphere' %} 
Настройте DNS для сервисов Deckhouse одним из следующих способов:

{% include getting_started/global/partials/DNS_OPTIONS_RU.liquid %}

Затем, на **master-узле** выполните следующую команду (укажите используемый шаблон DNS-имен в переменной <code>DOMAIN_TEMPLATE</code>):
{% snippetcut %}
{% raw %}
```shell
DOMAIN_TEMPLATE='<DOMAIN_TEMPLATE>'
sudo -i d8 k patch mc global --type merge -p "{\"spec\": {\"settings\":{\"modules\":{\"publicDomainTemplate\":\"${DOMAIN_TEMPLATE}\"}}}}"
```
{% endraw %}
{% endsnippetcut %}
{% endif %}

## Настройте удаленный доступ к кластеру 

На **персональном компьютере** выполните следующие шаги, для того чтобы настроить подключение `kubectl` к кластеру:
- Откройте веб-интерфейс сервиса *Kubeconfig Generator*. Для него зарезервировано имя `kubeconfig`, и адрес для доступа формируется согласно шаблона DNS-имен (который вы установили ранее). Например, для шаблона DNS-имен `%s.1.2.3.4.sslip.io`, веб-интерфейс *Kubeconfig Generator* будет доступен по адресу `https://kubeconfig.1.2.3.4.sslip.io`.
- Авторизуйтесь под пользователем `admin@deckhouse.io`. Пароль пользователя, сгенерированный на предыдущем шаге, — `<GENERATED_PASSWORD>` (вы также можете найти его в CustomResource `User` в файле `resource.yml`).
- Выберите вкладку с ОС персонального компьютера.
- Последовательно скопируйте и выполните команды, приведенные на странице.
- Проверьте корректную работу `kubectl` (например, выполнив команду `kubectl get no`).
