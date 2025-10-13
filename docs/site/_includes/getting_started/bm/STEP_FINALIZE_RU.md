<script type="text/javascript" src='{% javascript_asset_tag getting-started %}[_assets/js/getting-started.js]{% endjavascript_asset_tag %}'></script>
<script type="text/javascript" src='{% javascript_asset_tag getting-started-access %}[_assets/js/getting-started-access.js]{% endjavascript_asset_tag %}'></script>
<script type="text/javascript" src='{% javascript_asset_tag bcrypt %}[_assets/js/bcrypt.js]{% endjavascript_asset_tag %}'></script>

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
<div markdown="1">
```bash
sudo -i d8 k patch nodegroup master --type json -p '[{"op": "remove", "path": "/spec/nodeTemplate/taints"}]'
```
</div>
  </li>
  <li>
<p>Настройте StorageClass <a href="/modules/local-path-provisioner/cr.html#localpathprovisioner">локального хранилища</a>, выполнив на <strong>master-узле</strong> следующую команду:</p>
<div markdown="1">
```shell
sudo -i d8 k create -f - << EOF
apiVersion: deckhouse.io/v1alpha1
kind: LocalPathProvisioner
metadata:
  name: localpath
spec:
  path: "/opt/local-path-provisioner"
  reclaimPolicy: Delete
EOF
```
</div>
  </li>
  <li>
<p>Укажите, что созданный StorageClass должен использоваться как StorageClass по умолчанию. Для этого выполните на <strong>master-узле</strong> следующую команду:</p>
<div markdown="1">
```shell
sudo -i d8 k patch mc global --type merge \
  -p "{\"spec\": {\"settings\":{\"defaultClusterStorageClass\":\"localpath\"}}}"
```
</div>
  </li>
</ul>
</div>

<div id="block_layout_worker" class="tabs__content_worker">
<p>Добавьте узел в кластер (подробнее о добавлении статического узла в кластер читайте в <a href="/modules/node-manager/examples.html#добавление-статического-узла-в-кластер">документации</a>):</p>

<ul>
  <li>
    Подготовьте <strong>чистую</strong> виртуальную машину, которая будет узлом кластера.
  </li>
  <li>
<p>Настройте StorageClass <a href="/modules/local-path-provisioner/cr.html#localpathprovisioner">локального хранилища</a>, выполнив на <strong>master-узле</strong> следующую команду:</p>
<div markdown="1">
```shell
sudo -i d8 k create -f - << EOF
apiVersion: deckhouse.io/v1alpha1
kind: LocalPathProvisioner
metadata:
  name: localpath
spec:
  path: "/opt/local-path-provisioner"
  reclaimPolicy: Delete
EOF
```
</div>
  </li>
  <li>
<p>Укажите, что созданный StorageClass должен использоваться как StorageClass по умолчанию. Для этого выполните на <strong>master-узле</strong> следующую команду:</p>
<div markdown="1">
```shell
sudo -i d8 k patch mc global --type merge \
  -p "{\"spec\": {\"settings\":{\"defaultClusterStorageClass\":\"localpath\"}}}"
```
</div>
  </li>
  <li>
    <p>Создайте <a href="/modules/node-manager/cr.html#nodegroup">NodeGroup</a> <code>worker</code> и добавьте узел с помощью с помощью Cluster API Provider Static (CAPS) или вручную — с помощью bootstrap-скрипта.</p>
    
<div class="tabs">
        <a id='tab_block_caps' href="javascript:void(0)" class="tabs__btn tabs__btn_caps_bootstrap active"
        onclick="openTabAndSaveStatus(event, 'tabs__btn_caps_bootstrap', 'tabs__caps', 'block_bootstrap');
                 openTabAndSaveStatus(event, 'tabs__btn_caps_bootstrap', 'tabs__bootstrap', 'block_caps');">
        CAPS
        </a>
        <a id='tab_block_bootstrap' href="javascript:void(0)" class="tabs__btn tabs__btn_caps_bootstrap"
        onclick="openTabAndSaveStatus(event, 'tabs__btn_caps_bootstrap', 'tabs__bootstrap', 'block_caps');
                 openTabAndSaveStatus(event, 'tabs__btn_caps_bootstrap', 'tabs__caps', 'block_bootstrap');">
        Bootstrap-скрипт
        </a>
</div>

  <div id="block_bootstrap" class="tabs__bootstrap" style="display: none;">
  <ul>
  <li><p>Создайте NodeGroup с именем <code>worker</code>, выполнив на <strong>master-узле</strong> следующую команду:</p>
<div markdown="1">
```bash
sudo -i d8 k create -f - << EOF
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: worker
spec:
  nodeType: Static
EOF
```
</div>
  </li>
  <li><p>Получите код скрипта для добавления и настройки узла в кодировке Base64.</p>
  <p>Для этого выполните на <strong>master-узле</strong> следующую команду:</p>
<div markdown="1">
```shell
NODE_GROUP=worker
sudo -i d8 k -n d8-cloud-instance-manager get secret manual-bootstrap-for-${NODE_GROUP} -o json | jq '.data."bootstrap.sh"' -r
```
</div>
  </li>
  <li><p><strong>На подготовленной виртуальной машине</strong> выполните следующую команду, вставив полученный на предыдущем шаге код скрипта в кодировке Base64:</p>
<div markdown="1">
```shell
echo <Base64-КОД-СКРИПТА> | base64 -d | bash
```
  </div>
  </li>
  </ul>
  </div>
  <div id="block_caps" class="tabs__caps">
  <ul>
<li><p>Создайте NodeGroup с именем <code>worker</code>, выполнив на <strong>master-узле</strong> следующую команду:</p>
<div markdown="1">
```bash
sudo -i d8 k create -f - << EOF
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
</div>
</li>
  <li>
    <p>Сгенерируйте SSH-ключ с пустой парольной фразой. Для этого выполните на <strong>master-узле</strong> следующую команду:</p>
<div markdown="1">
```bash
ssh-keygen -t rsa -f /dev/shm/caps-id -C "" -N ""
```
</div>
  </li>
  <li>
    <p>Создайте в кластере ресурс <a href="/modules/node-manager/cr.html#sshcredentials">SSHCredentials</a>. Для этого выполните на <strong>master-узле</strong> следующую команду:</p>
<div markdown="1">
```bash
sudo -i d8 k create -f - <<EOF
apiVersion: deckhouse.io/v1alpha2
kind: SSHCredentials
metadata:
  name: caps
spec:
  user: caps
  privateSSHKey: "`cat /dev/shm/caps-id | base64 -w0`"
EOF
```
</div>
  </li>
  <li>
    <p>Выведите публичную часть сгенерированного ранее SSH-ключа (он понадобится на следующем шаге). Для этого выполните на <strong>master-узле</strong> следующую команду:</p>
<div markdown="1">
```bash
cat /dev/shm/caps-id.pub
```
</div>
  </li>
  <li>
    <p><strong>На подготовленной виртуальной машине</strong> создайте пользователя <code>caps</code>. Для этого выполните следующую команду, указав публичную часть SSH-ключа, полученную на предыдущем шаге:</p>
{% offtopic title="Если у вас CentOS, Rocky Linux, ALT Linux, РОСА Сервер, РЕД ОС или МОС ОС..." %}
В операционных системах на базе RHEL (Red Hat Enterprise Linux) пользователя caps нужно добавлять в группу wheel. Для этого выполните следующую команду, указав публичную часть SSH-ключа, полученную на предыдущем шаге:
<div markdown="1">
```bash
# Укажите публичную часть SSH-ключа пользователя.
export KEY='<SSH-PUBLIC-KEY>'
useradd -m -s /bin/bash caps
usermod -aG wheel caps
echo 'caps ALL=(ALL) NOPASSWD: ALL' | sudo EDITOR='tee -a' visudo
mkdir /home/caps/.ssh
echo $KEY >> /home/caps/.ssh/authorized_keys
chown -R caps:caps /home/caps
chmod 700 /home/caps/.ssh
chmod 600 /home/caps/.ssh/authorized_keys
```
</div>
Далее перейдите к следующему шагу, **выполнять команду ниже не нужно**.
{% endofftopic %}
<div markdown="1">
```bash
# Укажите публичную часть SSH-ключа пользователя.
export KEY='<SSH-PUBLIC-KEY>'
useradd -m -s /bin/bash caps
usermod -aG sudo caps
echo 'caps ALL=(ALL) NOPASSWD: ALL' | sudo EDITOR='tee -a' visudo
mkdir /home/caps/.ssh
echo $KEY >> /home/caps/.ssh/authorized_keys
chown -R caps:caps /home/caps
chmod 700 /home/caps/.ssh
chmod 600 /home/caps/.ssh/authorized_keys
```
</div>
  </li>
  <li>
    <p><strong>В операционных системах семейства Astra Linux</strong>, при использовании модуля мандатного контроля целостности Parsec, сконфигурируйте максимальный уровень целостности для пользователя <code>caps</code>:</p>
<div markdown="1">
```bash
pdpl-user -i 63 caps
```
</div>
  </li>
  <li>
    <p>Создайте <a href="/modules/node-manager/cr.html#staticinstance">StaticInstance</a> для добавляемого узла. Для этого выполните на <strong>master-узле</strong> следующую команду, указав IP-адрес добавляемого узла:</p>
<div markdown="1">
```bash
# Укажите IP-адрес узла, который необходимо подключить к кластеру.
export NODE=<NODE-IP-ADDRESS>
sudo -i d8 k create -f - <<EOF
apiVersion: deckhouse.io/v1alpha2
kind: StaticInstance
metadata:
  name: d8cluster-worker
  labels:
    role: worker
spec:
  address: "$NODE"
  credentialsRef:
    kind: SSHCredentials
    name: caps
EOF
```
</div>
  </li>
  </ul>
  </div>
  </li>
  <li><p>Убедитесь, что все узлы кластера находятся в статусе <code>Ready</code>.</p>
<p>Выполните на <strong>master-узле</strong> следующую команду, чтобы получить список узлов кластера:</p>
<div markdown="1">
```shell
sudo -i d8 k get no
```
</div>

{% offtopic title="Пример вывода..." %}
```
$ sudo -i d8 k get no
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
<li><p>Убедитесь, что под Kruise controller manager модуля <a href="/modules/ingress-nginx/">ingress-nginx</a> запустился и находится в статусе <code>Ready</code>.</p>
<p>Выполните на <strong>master-узле</strong> следующую команду:</p>

<div markdown="1">
```shell
sudo -i d8 k -n d8-ingress-nginx get po -l app=kruise
```
</div>

{% offtopic title="Пример вывода..." %}
```
$ sudo -i d8 k -n d8-ingress-nginx get po -l app=kruise
NAME                                         READY   STATUS    RESTARTS    AGE
kruise-controller-manager-7dfcbdc549-b4wk7   3/3     Running   0           15m
```
{%- endofftopic %}
</li></ul>

Далее нужно создать Ingress-контроллер, создать пользователя для доступа в веб-интерфейсы и настроить DNS.

<ul><li><p><strong>Установка Ingress-контроллера</strong></p>
<p>Создайте на <strong>master-узле</strong> файл <code>ingress-nginx-controller.yml</code> содержащий конфигурацию Ingress-контроллера:</p>
{% capture includePath %}_includes/getting_started/{{ page.platform_code }}/partials/ingress-nginx-controller.yml.inc{% endcapture %}
<div markdown="1">
{% include_file "{{ includePath }}" syntax="yaml" %}
</div>
<p>Примените его, выполнив на <strong>master-узле</strong> следующую команду:</p>

<div markdown="1">
```shell
sudo -i d8 k create -f $PWD/ingress-nginx-controller.yml
```
</div>

<p>Запуск Ingress-контроллера после завершения установки Deckhouse может занять какое-то время. Прежде чем продолжить убедитесь что Ingress-контроллер запустился (выполните на <code>master-узле</code>):</p>

<div markdown="1">
```shell
sudo -i d8 k -n d8-ingress-nginx get po -l app=controller
```
</div>

<p>Дождитесь перехода подов Ingress-контроллера в статус <code>Ready</code>.</p>

{% offtopic title="Пример вывода..." %}
```
$ sudo -i d8 k -n d8-ingress-nginx get po -l app=controller
NAME                                       READY   STATUS    RESTARTS   AGE
controller-nginx-r6hxc                     3/3     Running   0          5m
```
{%- endofftopic %}
</li>
<li><p><strong>Создание пользователя</strong> для доступа в веб-интерфейсы кластера</p>
<p>Создайте на <strong>master-узле</strong> файл <code>user.yml</code> содержащий описание учетной записи пользователя и прав доступа:</p>
{% capture includePath %}_includes/getting_started/{{ page.platform_code }}/partials/user.yml.inc{% endcapture %}
<div markdown="1">
{% include_file "{{ includePath }}" syntax="yaml" %}
</div>
<p>Примените его, выполнив на <strong>master-узле</strong> следующую команду:</p>
<div markdown="1">
```shell
sudo -i d8 k create -f $PWD/user.yml
```
</div>
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
