<script type="text/javascript" src='{% javascript_asset_tag getting-started %}[_assets/js/getting-started.js]{% endjavascript_asset_tag %}'></script>
<script type="text/javascript" src='{% javascript_asset_tag getting-started-access %}[_assets/js/getting-started-access.js]{% endjavascript_asset_tag %}'></script>
Для того чтобы получить доступ к веб-интерфейсам компонентов Deckhouse, нужно:
- настроить работу DNS
- указать в параметрах Deckhouse [шаблон DNS-имен](../../documentation/v1/reference/api/global.html#parameters-modules-publicdomaintemplate)

*Шаблон DNS-имен* используется для настройки Ingress-ресурсов системных приложений. Например, за интерфейсом модуля внутренней документации закреплено имя `deckhouse`. Тогда, для шаблона `%s.kube.company.my` Grafana будет доступна по адресу `deckhouse.kube.company.my`, и т.д.

Чтобы упростить настройку, далее будет использоваться сервис [sslip.io](https://sslip.io/).

Выполните следующую команду, чтобы настроить [шаблон DNS-имен](../../documentation/v1/reference/api/global.html#parameters-modules-publicdomaintemplate) сервисов Deckhouse на использование *sslip.io* (укажите публичный IP-адрес узла, где запущен Ingress-контролллер):
<div markdown="1">
{% raw %}
```shell
BALANCER_IP=<INGRESS_CONTROLLER_IP> 
kubectl patch mc global --type merge \
  -p "{\"spec\": {\"settings\":{\"modules\":{\"publicDomainTemplate\":\"%s.${BALANCER_IP}.sslip.io\"}}}}" && echo && \
echo "Domain template is '$(kubectl get mc global -o=jsonpath='{.spec.settings.modules.publicDomainTemplate}')'."
```
{% endraw %}
</div>

Команда также выведет установленный шаблон DNS-имен. Пример вывода:
```text
moduleconfig.deckhouse.io/global patched

Domain template is '%s.1.2.3.4.sslip.io'.
```

{% alert %}
Перегенерация сертификатов после изменения шаблона DNS-имен может занять до 5 минут.
{% endalert %}

{% offtopic title="Другие варианты настройки..." %}
Вместо сервиса *sslip.io* вы можете использовать другие варианты настройки.
{% include getting_started/global/partials/DNS_OPTIONS_RU.liquid %}

Затем, выполните следующую команду, чтобы изменить шаблон DNS-имен в параметрах Deckhouse:
<div markdown="1">
```shell
kubectl patch mc global --type merge -p "{\"spec\": {\"settings\":{\"modules\":{\"publicDomainTemplate\":\"${DOMAIN_TEMPLATE}\"}}}}"
```
</div>
{% endofftopic %}
