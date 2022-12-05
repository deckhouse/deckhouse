<script type="text/javascript" src='{{ assets["getting-started.js"].digest_path }}'></script>
<script type="text/javascript" src='{{ assets["getting-started-access.js"].digest_path }}'></script>

# Доступ к кластеру через Kubernetes API
Deckhouse только что завершил процесс установки вашего кластера. Теперь вы можете подключиться к master-узлу, используя ssh.
Для этого необходимо получить IP-адрес master-узла либо из логов dhctl, либо из web интерфейса/cli утилиты облачного провайдера.
{% snippetcut %}
```shell
ssh {% if page.platform_code == "azure" %}azureuser{% elsif page.platform_code == "gcp" %}user{% else %}ubuntu{% endif %}@<MASTER_IP>
```
{% endsnippetcut %}
Вы можете запускать kubectl на master-узле от пользователя root. Это не безопасный способ, и мы рекомендуем настроить [внешний доступ](/documentation/v1/modules/150-user-authn/faq.html#как-я-могу-сгенерировать-kubeconfig-для-доступа-к-kubernetes-api) к Kubernetes API позже.
{% snippetcut %}
```shell
sudo -i
kubectl get nodes
```
{% endsnippetcut %}

# Доступ к кластеру через NGINX Ingress
[IngressNginxController](/documentation/v1/modules/402-ingress-nginx/cr.html#ingressnginxcontroller) был создан во время процесса установки кластера.
Теперь осталось настроить доступ к веб-интерфейсам компонентов, которые уже установлены в кластере, таким как Grafana, Prometheus, Dashboard и так далее.
{% if page.platform_type == 'cloud' and page.platform_code != 'vsphere' %}
LoadBalancer уже создан и вам остаётся только направить DNS-домен на него.
В первую очередь необходимо подключиться к master-узлу, как это описано [выше](#доступ-к-кластеру-через-kubernetes-api).

Получите IP адрес балансировщика. Для этого, на **master-узле** от пользователя `root` выполните команду:
{% if page.platform_code == 'aws' %}
{% snippetcut %}
{% raw %}
```shell
BALANCER_HOSTNAME=$(kubectl -n d8-ingress-nginx get svc nginx-load-balancer -o json | jq -r '.status.loadBalancer.ingress[0].hostname')
echo "$BALANCER_HOSTNAME"
```
{% endraw %}
{% endsnippetcut %}
{% else %}
{% snippetcut %}
{% raw %}
```shell
BALANCER_IP=$(kubectl -n d8-ingress-nginx get svc nginx-load-balancer -o json | jq -r '.status.loadBalancer.ingress[0].ip')
echo "$BALANCER_IP"
```
{% endraw %}
{% endsnippetcut %}
{% endif %}
{% endif %}

Настройте домен для сервисов Deckhouse, который вы указали на шаге «[Установка кластера](./step3.html)», одним из следующих способов:
<ul>
  <li>Если у вас есть возможность добавить DNS-запись используя DNS-сервер:
    <ul>
      <li>Если ваш шаблон DNS-имен кластера является <a href="https://en.wikipedia.org/wiki/Wildcard_DNS_record">wildcard
        DNS-шаблоном</a> (например, <code>%s.kube.my</code>), то добавьте
        {%- if page.platform_code == 'aws' %} соответствующую wildcard CNAME-запись со значением адреса балансировщика (<code>BALANCER_HOSTNAME</code>)
        {%- else %} соответствующую wildcard A-запись со значением IP-адреса {% if page.platform_code == 'vsphere' %}master-узла, который вы получили выше (если настроены выделенные frontend-узлы, то используйте их IP-адреса вместо IP-адреса master-узла){% else %}балансировщика (<code>BALANCER_IP</code>), который вы получили выше{% endif %}{%- endif -%}.
      </li>
      <li>
        Если ваш шаблон DNS-имен кластера <strong>НЕ</strong> является <a
              href="https://en.wikipedia.org/wiki/Wildcard_DNS_record">wildcard DNS-шаблоном</a> (например, <code>%s-kube.company.my</code>),
        то добавьте А или CNAME-записи со значением IP-адреса {% if page.platform_code == 'vsphere' %}master-узла, который вы получили выше (если настроены выделенные frontend-узлы, то используйте их IP-адреса вместо IP-адреса master-узла){% else %}балансировщика (<code>BALANCER_IP</code>), который вы получили выше{% endif %}, для следующих DNS-имен сервисов Deckhouse в вашем кластере:
        <div class="highlight">
<pre class="highlight">
<code example-hosts>api.example.com
dashboard.example.com
deckhouse.example.com
dex.example.com
grafana.example.com
kubeconfig.example.com
status.example.com
upmeter.example.com</code>
</pre>
        </div>
      </li>
    </ul>
  </li>
  <li><p>Если вы не имеете под управлением DNS-сервер, то на компьютере, с которого необходим доступ к сервисам Deckhouse, добавьте статические записи в файл <code>/etc/hosts</code> для Linux или <code>%SystemRoot%\system32\drivers\etc\hosts</code> для Windows.</p>
{% if page.platform_code == 'aws' %}
  <p>Определить IP-адрес балансировщика можно при помощи следующей команды, выполняемой на <strong>master-узле</strong>:</p>

<div markdown="1">
{% snippetcut %}
```bash
BALANCER_IP=$(dig "$BALANCER_HOSTNAME" +short | head -1); echo "$BALANCER_IP"
```
{% endsnippetcut %}
</div>
{% endif %}

  <p>Для добавления записей в файл <code>/etc/hosts</code> на Linux-компьютере, с которого необходим доступ к сервисам Deckhouse, выполните следующие шаги:</p>

<ul>
{%- if page.platform_code != 'vsphere' %}
<li><p>Экспортируйте переменную <code>BALANCER_IP</code>, указав полученный IP-адрес балансировщика:</p>
{% snippetcut %}
```bash
export BALANCER_IP="<BALANCER_IP>"
```
{% endsnippetcut %}
</li>
{%- else %}
<li><p>Экспортируйте переменную <code>BALANCER_IP</code>, указав полученный IP-адрес <strong>master-узла</strong> (если настроены выделенные frontend-узлы, то используйте их IP-адреса вместо IP-адреса master-узла):</p>
{% snippetcut %}
```bash
export BALANCER_IP="<MASTER_OR_FRONT_IP>"
```
{% endsnippetcut %}
</li>
{%- endif %}
  <li><p>Добавьте DNS-записи для веб-интерфейсов Deckhouse:</p>
{% snippetcut selector="example-hosts" %}
```bash
sudo -E bash -c "cat <<EOF >> /etc/hosts
$BALANCER_IP api.example.com
$BALANCER_IP dashboard.example.com
$BALANCER_IP deckhouse.example.com
$BALANCER_IP dex.example.com
$BALANCER_IP grafana.example.com
$BALANCER_IP kubeconfig.example.com
$BALANCER_IP status.example.com
$BALANCER_IP upmeter.example.com
EOF
"
```
{% endsnippetcut %}
</li>
</ul></li>
</ul>
