<script type="text/javascript" src='{{ assets["getting-started.js"].digest_path }}'></script>
<script type="text/javascript" src='{{ assets["getting-started-access.js"].digest_path }}'></script>

{% if page.platform_type == 'cloud' %}
Получите адрес балансировщика. Для этого в кластере от пользователя root выполните команду:
{% if page.platform_code == 'aws' %}
{% snippetcut %}
{% raw %}
```bash
BALANCER_HOSTNAME=$(kubectl -n d8-ingress-nginx get svc -l app=controller,name=nginx \
-o=go-template='{{ (index (index .items 0).status.loadBalancer.ingress 0).hostname }}') ;\
echo "$BALANCER_HOSTNAME"
```
{% endraw %}
{% endsnippetcut %}
{% else %}
{% snippetcut %}
{% raw %}
```bash
BALANCER_IP=$(kubectl -n d8-ingress-nginx get svc -l app=controller,name=nginx \
-o=go-template='{{ (index (index .items 0).status.loadBalancer.ingress 0).ip }}') ;\
echo "$BALANCER_IP"
```
{% endraw %}
{% endsnippetcut %}
{% endif %}
{% endif %}

Подключите домен для сервисов Deckhouse, который вы указали на шаге ["Установка кластера"](./step3.html), одним из следующих способов:
<div markdown="1">
<ul><li><p>Если у вас есть возможность добавить DNS-запись используя DNS-сервер, то мы рекомендуем воспользоваться следующим вариантом — добавьте
{%- if page.platform_code == 'aws' %} CNAME wildcard-запись для <code>*.example.com</code> и адреса балансировщика (<code>BALANCER_HOSTNAME</code>)
{%- else %} wildcard-запись для <code>*.example.com</code> и IP-адреса балансировщика (<code>BALANCER_IP</code>)
{%- endif -%}
  , который вы получили выше.</p></li>
<li><p>Если вы хотите протестировать работу кластера, но не имеете под управлением DNS-сервер, добавьте статические записи соответствия имен конкретных сервисов IP-адресу балансировщика в файл <code>/etc/hosts</code> для Linux (<code>%SystemRoot%\system32\drivers\etc\hosts</code> для Windows).</p>
{% if page.platform_code == 'aws' %}
  <p>Определить IP-адрес балансировщика можно при помощи следующей команды (также выполняемой в кластере):</p>

<div markdown="1">
{% snippetcut %}
```bash
BALANCER_IP=$(dig "$BALANCER_HOSTNAME" +short | head -1); echo "$BALANCER_IP"
```
{% endsnippetcut %}
</div>
{% endif %}

  <p>Для добавления записей в файл `/etc/hosts` локально, выполните например следующие шаги:</p>

<ul><li><p>Экспортируйте переменную BALANCER_IP, указав полученный IP-адрес балансировщика:</p>
{% snippetcut %}
```bash
export BALANCER_IP="<PUT_BALANCER_IP_HERE>"
```
{% endsnippetcut %}
</li>
  <li><p>Добавьте записи сервисов Deckhouse:</p>
{% snippetcut selector="example-hosts" %}
```bash
sudo -E bash -c "cat <<EOF >> /etc/hosts
$BALANCER_IP dashboard.example.com
$BALANCER_IP deckhouse.example.com
$BALANCER_IP kubeconfig.example.com
$BALANCER_IP grafana.example.com
$BALANCER_IP dex.example.com
EOF
"
```
{% endsnippetcut %}
</li>
</ul></li>
</ul>
</div>
