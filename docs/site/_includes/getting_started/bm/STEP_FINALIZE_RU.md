<script type="text/javascript" src='{{ assets["getting-started.js"].digest_path }}'></script>
<script type="text/javascript" src='{{ assets["getting-started-access.js"].digest_path }}'></script>
<script type="text/javascript" src='{{ assets["bcrypt.js"].digest_path }}'></script>

На данном этапе вы создали кластер, который состоит из **единственного** узла — master-узла. На master-узле по умолчанию работает только ограниченный набор системных компонентов. Для полноценной работы кластера необходимо либо добавить в кластер хотя бы один worker-узел, либо разрешить остальным компонентам Deckhouse работать на master-узле.

Выберите ниже один из двух вариантов, для продолжения установки кластера:

<div class="tabs">
        <a id='tab_layout_worker' href="javascript:void(0)" class="tabs__btn tabs__btn_revision active"
        onclick="openTabAndSaveStatus(event, 'tabs__btn_revision', 'tabs__content_worker', 'block_layout_master');
                 openTabAndSaveStatus(event, 'tabs__btn_revision', 'tabs__content_master', 'block_layout_worker');">
        Кластер из нескольких узлов
        </a>
        <a id='tab_layout_master' href="javascript:void(0)" class="tabs__btn tabs__btn_revision"
        onclick="openTabAndSaveStatus(event, 'tabs__btn_revision', 'tabs__content_master', 'block_layout_worker');
                 openTabAndSaveStatus(event, 'tabs__btn_revision', 'tabs__content_worker', 'block_layout_master');">
        Кластер из единственного узла
        </a>
</div>

<div id="block_layout_master" class="tabs__content_master" style="display: none;">
<p>Кластера, состоящего из единственного узла, может быть достаточно, например, для ознакомительных целей.</p>
<ul>
  <li>
<p>Выполните на <strong>master-узле</strong> следующую команду, для того чтобы снять с него <i>taint</i> и разрешить остальным компонентам Deckhouse работать на master-узле:</p>

{% snippetcut %}
```bash
sudo /opt/deckhouse/bin/kubectl patch nodegroup master --type json -p '[{"op": "remove", "path": "/spec/nodeTemplate/taints"}]'
```
{% endsnippetcut %}
  </li>
  <li>
<p>Настройте StorageClass <a href="/documentation/v1/modules/031-local-path-provisioner/cr.html#localpathprovisioner">локального хранилища</a>, выполнив на <strong>master-узле</strong> следующую команду:</p>
{% snippetcut %}
```shell
sudo /opt/deckhouse/bin/kubectl create -f - << EOF
apiVersion: deckhouse.io/v1alpha1
kind: LocalPathProvisioner
metadata:
  name: localpath
spec:
  path: "/opt/local-path-provisioner"
  reclaimPolicy: Delete
EOF
```
{% endsnippetcut %}
  </li>
  <li>
<p>Укажите, что созданный StorageClass должен использоваться как StorageClass по умолчанию. Для этого выполните на <strong>master-узле</strong> следующую команду, чтобы добавить на StorageClass аннотацию <code>storageclass.kubernetes.io/is-default-class='true'</code>:
</p>
{% snippetcut %}
```shell
sudo /opt/deckhouse/bin/kubectl annotate sc localpath storageclass.kubernetes.io/is-default-class='true'
```
{% endsnippetcut %}
  </li>
</ul>
</div>

<div id="block_layout_worker" class="tabs__content_worker">
<p>Добавьте узел в кластер (подробнее о добавлении статического узла в кластер читайте в <a href="/documentation/latest/modules/040-node-manager/examples.html#добавление-статического-узла-в-кластер">документации</a>):</p>

<ul>
  <li>
    Подготовьте <strong>чистую</strong> виртуальную машину, которая будет узлом кластера.
  </li>
  <li>
<p>Настройте StorageClass <a href="/documentation/v1/modules/031-local-path-provisioner/cr.html#localpathprovisioner">локального хранилища</a>, выполнив на <strong>master-узле</strong> следующую команду:</p>
{% snippetcut %}
```shell
sudo /opt/deckhouse/bin/kubectl create -f - << EOF
apiVersion: deckhouse.io/v1alpha1
kind: LocalPathProvisioner
metadata:
  name: localpath
spec:
  path: "/opt/local-path-provisioner"
  reclaimPolicy: Delete
EOF
```
{% endsnippetcut %}
  </li>
  <li>
<p>Укажите, что созданный StorageClass должен использоваться как StorageClass по умолчанию. Для этого выполните на <strong>master-узле</strong> следующую команду, чтобы добавить на StorageClass аннотацию <code>storageclass.kubernetes.io/is-default-class='true'</code>:</p>
{% snippetcut %}
```shell
sudo /opt/deckhouse/bin/kubectl annotate sc localpath storageclass.kubernetes.io/is-default-class='true'
```
{% endsnippetcut %}
  </li>
  <li>
    <p>Создайте <a href="/documentation/v1/modules/040-node-manager/cr.html#nodegroup">NodeGroup</a> <code>worker</code>. Для этого выполните на <strong>master-узле</strong> следующую команду:</p>
{% snippetcut %}
```bash
sudo /opt/deckhouse/bin/kubectl create -f - << EOF
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: worker
spec:
  nodeType: Static
  staticInstances:
    count: 1
    labelSelector:
      matchLabels:
        role: worker
EOF
```
{% endsnippetcut %}
  </li>
  <li>
    <p>Сгенерируйте на master сервере пару SSH-ключей с пустой парольной фразой:</p>
{% snippetcut %}
```bash
ssh-keygen -t rsa -f /dev/shm/caps-id -C "" -N ""
```
{% endsnippetcut %}
  </li>
  <li>
    <p>Создайте в кластере ресурс <a href="/documentation/v1/modules/040-node-manager/cr.html#sshcredentials">SSHCredentials</a>:</p>
{% snippetcut %}
```bash
kubectl create -f - <<EOF
apiVersion: deckhouse.io/v1alpha1
kind: SSHCredentials
metadata:
  name: caps
spec:
  user: caps
  privateSSHKey: "`cat /dev/shm/caps-id | base64 -w0`"
EOF
```
{% endsnippetcut %}
  </li>
  <li>
    <p>Запомним содержимое публичного ssh ключа:</p>
{% snippetcut %}
```bash
echo "export key='`cat /dev/shm/caps-id.pub`'"
```
{% endsnippetcut %}
  </li>
  <li>
    <p><strong> На подготовленной виртуальной машине</strong> выполните следующую команды для создания пользователя <a href="/documentation/v1/modules/040-node-manager/examples.html#с-помощью-cluster-api-provider-static">caps</a>:</p>
{% snippetcut %}
```bash
export key=....
useradd -m -s /bin/bash caps
usermod -aG sudo caps
echo 'caps ALL=(ALL) NOPASSWD: ALL' | sudo EDITOR='tee -a' visudo
mkdir /home/caps/.ssh
echo $key >> /home/caps/.ssh/authorized_keys
```
{% endsnippetcut %}
  </li>
  <li>
    <p>На <strong>master-узле</strong> создайте <a href="/documentation/v1/modules/040-node-manager/cr.html#staticinstance">StaticInstance</a> для добавляемой ноды:</p>
{% snippetcut %}
```bash
kubectl create -f - <<EOF
apiVersion: deckhouse.io/v1alpha1
kind: StaticInstance
metadata:
  name: d8cluster-worker
  labels:
    role: worker
spec:
  address: "d8cluster-worker-ip"
  credentialsRef:
    kind: SSHCredentials
    name: caps
EOF
```
{% endsnippetcut %}
  </li>
  <li><p>Убедитесь, что все узлы кластера находятся в статусе <code>Ready</code>.</p>
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
</ul>
</div>

<p>Запуск всех компонентов Deckhouse после завершения установки может занять какое-то время.</p>

<ul>
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

Далее нужно создать Ingress-контроллер, создать пользователя для доступа в веб-интерфейсы и настроить DNS.

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
