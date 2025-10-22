<script type="text/javascript" src='{% javascript_asset_tag getting-started %}[_assets/js/getting-started.js]{% endjavascript_asset_tag %}'></script>
<script type="text/javascript" src='{% javascript_asset_tag getting-started-access %}[_assets/js/getting-started-access.js]{% endjavascript_asset_tag %}'></script>
Убедитесь, что под Kruise controller manager запустился и находится в статусе `Running`.
  Выполните на **master-узле** следующую команду:

```shell
sudo -i d8 k -n d8-ingress-nginx get po -l app=kruise
```


Настройте Ingress-контроллер и DNS.

<ol>
  <li><p><strong>Установка Ingress-контроллера</strong></p>
<div markdown="1">
```shell
sudo -i d8 k apply -f - <<EOF
# Параметры контроллера NGINX Ingress.
# https://deckhouse.ru/modules/ingress-nginx/cr.html#ingressnginxcontroller
apiVersion: deckhouse.io/v1
kind: IngressNginxController
metadata:
  name: nginx
spec:
  ingressClass: nginx
  # Способ поступления трафика из внешнего мира.
  inlet: HostPort
  hostPort:
    httpPort: 80
    httpsPort: 443
  # Описывает, на каких узлах будет находиться Ingress-контроллер.
  # Возможно, захотите изменить.
  nodeSelector:
    node-role.kubernetes.io/control-plane: ""
  tolerations:
  - effect: NoSchedule
    key: node-role.kubernetes.io/control-plane
    operator: Exists
EOF
```
</div>
<p>
Запуск Ingress-контроллера после завершения установки Deckhouse может занять какое-то время. Прежде чем продолжить убедитесь что Ingress-контроллер запустился (выполните на <code>master-узле</code>):</p>

<div markdown="1">
```shell
sudo -i d8 k -n d8-ingress-nginx get po -l app=controller
```
</div>

<p>Дождитесь перехода подов Ingress-контроллера в статус <code>Running</code>.</p>

{% offtopic title="Пример вывода..." %}
```console
$ sudo -i d8 k -n d8-ingress-nginx get po -l app=controller
NAME                                       READY   STATUS    RESTARTS   AGE
controller-nginx-r6hxc                     3/3     Running   0          5m
```
{% endofftopic %}
</li>
<li><strong>Создание DNS-записи</strong>, для доступа в веб-интерфейсы кластера:
  <ul>
  <li>Выясните публичный IP-адрес узла, на котором работает Ingress-контроллер.</li>
  <li>Если у вас есть возможность добавить DNS-запись используя DNS-сервер:
    <ul>
      <li>Если ваш шаблон DNS-имен кластера является <a href="https://en.wikipedia.org/wiki/Wildcard_DNS_record">wildcard
        DNS-шаблоном</a> (например, <code>%s.kube.my</code>), то добавьте соответствующую wildcard A-запись со значением публичного IP-адреса, который вы получили выше.
      </li>
      <li>
        Если ваш шаблон DNS-имен кластера <strong>НЕ</strong> является <a
              href="https://en.wikipedia.org/wiki/Wildcard_DNS_record">wildcard DNS-шаблоном</a> (например, <code>%s-kube.company.my</code>),
        то добавьте А или CNAME-записи со значением публичного IP-адреса, который вы
        получили выше, для следующих DNS-имен сервисов Deckhouse в вашем кластере:
        <div class="highlight">
<pre class="highlight">
<code example-hosts>api.example.com
argocd.example.com
dashboard.example.com
documentation.example.com
dex.example.com
grafana.example.com
hubble.example.com
istio.example.com
istio-api-proxy.example.com
kubeconfig.example.com
openvpn-admin.example.com
prometheus.example.com
status.example.com
upmeter.example.com</code>
</pre>
        </div>
      </li>
      <li><strong>Важно:</strong> Домен, используемый в шаблоне, не должен совпадать с доменом, указанным в параметре clusterDomain и внутренней сервисной зоне сети. Например, если используется <code>clusterDomain: cluster.local</code> (значение по умолчанию), а сервисная зона сети — ru-central1.internal, то publicDomainTemplate не может быть <code>%s.cluster.local</code> или <code>%s.ru-central1.internal</code>.
      </li>
    </ul>
  </li>
  <li><p>Если вы <strong>не</strong> имеете под управлением DNS-сервер: добавьте статические записи соответствия имен конкретных сервисов публичному IP-адресу узла, на котором работает Ingress-контроллер.</p><p>Например, на персональном Linux-компьютере, с которого необходим доступ к сервисам Deckhouse, выполните следующую команду (укажите ваш публичный IP-адрес в переменной <code>PUBLIC_IP</code>) для добавления записей в файл <code>/etc/hosts</code> (для Windows используйте файл <code>%SystemRoot%\system32\drivers\etc\hosts</code>):</p>
<div markdown="1">
```bash
export PUBLIC_IP="<PUBLIC_IP>"
sudo -E bash -c "cat <<EOF >> /etc/hosts
$PUBLIC_IP api.example.com
$PUBLIC_IP argocd.example.com
$PUBLIC_IP dashboard.example.com
$PUBLIC_IP documentation.example.com
$PUBLIC_IP dex.example.com
$PUBLIC_IP grafana.example.com
$PUBLIC_IP hubble.example.com
$PUBLIC_IP istio.example.com
$PUBLIC_IP istio-api-proxy.example.com
$PUBLIC_IP kubeconfig.example.com
$PUBLIC_IP openvpn-admin.example.com
$PUBLIC_IP prometheus.example.com
$PUBLIC_IP status.example.com
$PUBLIC_IP upmeter.example.com
EOF
"
```
</div>
</li>
</ul>
</li>
</ol>
