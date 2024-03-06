<script type="text/javascript" src='{{ assets["getting-started.js"].digest_path }}'></script>
<script type="text/javascript" src='{{ assets["getting-started-access.js"].digest_path }}'></script>
<script type="text/javascript" src='{{ assets["bcrypt.js"].digest_path }}'></script>

На данном этапе вы создали кластер, который состоит из **единственного** узла — master-узла. Так как на master-узле по умолчанию работает только ограниченный набор системных компонентов, для полноценной работы кластера необходимо <a href="/documentation/latest/modules/040-node-manager/examples.html#добавление-статического-узла-в-кластер">добавить в кластер</a> хотя бы один worker-узел.

{% offtopic title="Если вам достаточно одного master-узла..." %}
<div>
<p>Если вы развернули кластер <strong>для ознакомительных целей</strong>, то кластера из одного узла может быть достаточно. Для того чтобы разрешить остальным компонентам Deckhouse работать на master-узле, необходимо снять с master-узла taint, выполнив на нем следующую команду:</p>

<div markdown="0">
{% snippetcut %}
```bash
sudo /opt/deckhouse/bin/kubectl patch nodegroup master --type json -p '[{"op": "remove", "path": "/spec/nodeTemplate/taints"}]'
```
{% endsnippetcut %}
</div>

<p><strong>Выполнять дальнейшие шаги по добавлению нового узла в кластер не нужно!</strong></p>
</div>
{%- endofftopic %}

Добавьте узел в кластер:

<ul>
  <li>
    Подготовьте <strong>чистую</strong> виртуальную машину, которая будет узлом кластера.
  </li>
  <li>
  Настройте StorageClass <a href="/documentation/v1/modules/031-local-path-provisioner/cr.html#localpathprovisioner">локального хранилища</a>, выполнив на <strong>master-узле</strong> следующую команду:
{% snippetcut %}
```shell
sudo /opt/deckhouse/bin/kubectl create -f - << EOF
apiVersion: deckhouse.io/v1alpha1
kind: LocalPathProvisioner
metadata:
  name: localpath-deckhouse
spec:
  nodeGroups:
  - worker
  path: "/opt/local-path-provisioner"
EOF
```
{% endsnippetcut %}
  </li>
  <li>
  Укажите что созданный StorageClass должен использоваться как StorageClass по умолчанию, добавив аннотацию <code>storageclass.kubernetes.io/is-default-class='true'</code>:
{% snippetcut %}
```shell
sudo /opt/deckhouse/bin/kubectl annotate sc localpath-deckhouse storageclass.kubernetes.io/is-default-class='true'
```
{% endsnippetcut %}
  </li>
  <li>
    Создайте <a href="/documentation/v1/modules/040-node-manager/cr.html#nodegroup">NodeGroup</a> <code>worker</code>. Для этого выполните на <strong>master-узле</strong> следующую команду:
{% snippetcut %}
```bash
sudo /opt/deckhouse/bin/kubectl create -f - << EOF
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: worker
spec:
  nodeType: Static
EOF
```
{% endsnippetcut %}
  </li>
  <li>
    Deckhouse подготовит скрипт, необходимый для настройки будущего узла и включения его в кластер. Выведите его содержимое в формате Base64 (оно понадобится на следующем шаге):
{% snippetcut %}
```bash
sudo /opt/deckhouse/bin/kubectl -n d8-cloud-instance-manager get secret manual-bootstrap-for-worker -o json | jq '.data."bootstrap.sh"' -r
```
{% endsnippetcut %}
  </li>
  <li>
    <strong> На подготовленной виртуальной машине</strong> выполните следующую команду, вставив код скрипта, полученный на предыдущем шаге:
{% snippetcut %}
```bash
echo <Base64-КОД-СКРИПТА> | base64 -d | sudo bash
```
{% endsnippetcut %}
  </li>
</ul>

<p>Запуск всех компонентов Deckhouse после завершения установки может занять какое-то время.</p>

Прежде чем продолжить:
<ul><li><p>Если вы добавляли дополнительные узлы в кластер, убедитесь что они находятся в статусе <code>Ready</code>.</p>
<p>Выполните на <strong>master-узле</strong> следующую команду, чтобы получить список узлов кластера:</p>
{% snippetcut %}
```shell
sudo /opt/deckhouse/bin/kubectl get no
```
{% endsnippetcut %}

{% offtopic title="Пример вывода..." %}
```
$ sudo /opt/deckhouse/bin/kubectl get no
NAME               STATUS   ROLES                  AGE    VERSION
d8cluster          Ready    control-plane,master   30m   v1.23.17
d8cluster-worker   Ready    worker                 10m   v1.23.17
```
{%- endofftopic %}
</li>
<li><p>Убедитесь, что под Kruise controller manager модуля <a href="/documentation/v1/modules/402-ingress-nginx/">ingress-nginx</a> запустился и находится в статусе <code>Ready</code>.</p>
<p>Выполните на <strong>master-узле</strong> следующую команду:</p>

{% snippetcut %}
```shell
sudo /opt/deckhouse/bin/kubectl -n d8-ingress-nginx get po -l app=kruise
```
{% endsnippetcut %}

{% offtopic title="Пример вывода..." %}
```
$ sudo /opt/deckhouse/bin/kubectl -n d8-ingress-nginx get po -l app=kruise
NAME                                         READY   STATUS    RESTARTS    AGE
kruise-controller-manager-7dfcbdc549-b4wk7   3/3     Running   0           15m
```
{%- endofftopic %}
</li></ul>

Далее нужно создать Ingress-контроллер, Storage Class для хранения данных, пользователя для доступа в веб-интерфейсы и настроить DNS.

<ul><li><p><strong>Установка Ingress-контроллера</strong></p>
<p>Создайте на <strong>master-узле</strong> файл <code>ingress-nginx-controller.yml</code> содержащий конфигурацию Ingress-контроллера:</p>
{% snippetcut name="ingress-nginx-controller.yml" selector="ingress-nginx-controller-yml" %}
{% include_file "_includes/getting_started/{{ page.platform_code }}/partials/ingress-nginx-controller.yml.inc" syntax="yaml" %}
{% endsnippetcut %}
<p>Примените его, выполнив на <strong>master-узле</strong> следующую команду:</p>
{% snippetcut %}
```shell
sudo /opt/deckhouse/bin/kubectl create -f ingress-nginx-controller.yml
```
{% endsnippetcut %}

Запуск Ingress-контроллера после завершения установки Deckhouse может занять какое-то время. Прежде чем продолжить убедитесь что Ingress-контроллер запустился (выполните на <code>master-узле</code>):

{% snippetcut %}
```shell
sudo /opt/deckhouse/bin/kubectl -n d8-ingress-nginx get po -l app=controller
```
{% endsnippetcut %}

Дождитесь перехода подов Ingress-контроллера в статус <code>Ready</code>.

{% offtopic title="Пример вывода..." %}
```
$ sudo /opt/deckhouse/bin/kubectl -n d8-ingress-nginx get po -l app=controller
NAME                                       READY   STATUS    RESTARTS   AGE
controller-nginx-r6hxc                     3/3     Running   0          5m
```
{%- endofftopic %}
</li>
<li><p><strong>Создание пользователя</strong> для доступа в веб-интерфейсы кластера</p>
<p>Создайте на <strong>master-узле</strong> файл <code>user.yml</code> содержащий описание учетной записи пользователя и прав доступа:</p>
{% snippetcut name="user.yml" selector="user-yml" %}
{% include_file "_includes/getting_started/{{ page.platform_code }}/partials/user.yml.inc" syntax="yaml" %}
{% endsnippetcut %}
<p>Примените его, выполнив на <strong>master-узле</strong> следующую команду:</p>
{% snippetcut %}
```shell
sudo /opt/deckhouse/bin/kubectl create -f user.yml
```
{% endsnippetcut %}
</li>
<li><strong>Создание DNS-записи</strong>, для доступа в веб-интерфейсы кластера
  <ul><li>Выясните публичный IP-адрес узла, на котором работает Ingress-контроллер.</li>
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
cdi-uploadproxy.example.com
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
    </ul>
  </li>

    <li><p>Если вы <strong>не</strong> имеете под управлением DNS-сервер: добавьте статические записи соответствия имен конкретных сервисов публичному IP-адресу узла, на котором работает Ingress-контроллер.</p><p>Например, на персональном Linux-компьютере, с которого необходим доступ к сервисам Deckhouse, выполните следующую команду (укажите ваш публичный IP-адрес в переменной <code>PUBLIC_IP</code>) для добавления записей в файл <code>/etc/hosts</code> (для Windows используйте файл <code>%SystemRoot%\system32\drivers\etc\hosts</code>):</p>
{% snippetcut selector="example-hosts" %}
```bash
export PUBLIC_IP="<PUBLIC_IP>"
sudo -E bash -c "cat <<EOF >> /etc/hosts
$PUBLIC_IP api.example.com
$PUBLIC_IP argocd.example.com
$PUBLIC_IP cdi-uploadproxy.example.com
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
{% endsnippetcut %}
</li></ul>
</li>
</ul>

<script type="text/javascript">
$(document).ready(function () {
    generate_password(true);
    update_parameter('dhctl-user-password-hash', 'password', '<GENERATED_PASSWORD_HASH>', null, null);
    update_parameter('dhctl-user-password-hash', null, '<GENERATED_PASSWORD_HASH>', null, '[user-yml]');
    update_parameter('dhctl-user-password', null, '<GENERATED_PASSWORD>', null, '[user-yml]');
    update_parameter('dhctl-user-password', null, '<GENERATED_PASSWORD>', null, 'code span.c1');
    update_domain_parameters();
    config_highlight();
});

</script>
