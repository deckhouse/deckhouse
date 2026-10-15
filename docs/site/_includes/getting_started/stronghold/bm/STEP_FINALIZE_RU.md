<script type="text/javascript" src='{% javascript_asset_tag getting-started %}[_assets/js/getting-started.js]{% endjavascript_asset_tag %}'></script>
<script type="text/javascript" src='{% javascript_asset_tag getting-started-access %}[_assets/js/getting-started-access.js]{% endjavascript_asset_tag %}'></script>
<script type="text/javascript" src='{% javascript_asset_tag bcrypt %}[_assets/js/bcrypt.js]{% endjavascript_asset_tag %}'></script>

На данном этапе вы создали кластер, который состоит из **единственного** узла — master-узла. На master-узле по умолчанию работает только ограниченный набор системных компонентов. Для полноценной работы кластера необходимо либо добавить в кластер хотя бы один worker-узел, либо разрешить остальным компонентам Deckhouse работать на master-узле.

Выберите ниже один из двух вариантов, для продолжения установки кластера:

<div class="tabs-block">
  <ul class="tabs__container tabs__container--title">
    <li id='tab_layout_worker' href="javascript:void(0)" class="tabs__item tabs__item--title active"
    onclick="openTabAndSaveStatus(event,'tabs__item--title','tabs__container--descr','block_layout_worker');">
    Кластер из нескольких узлов
    </li>
    <li id='tab_layout_master' href="javascript:void(0)" class="tabs__item tabs__item--title"
    onclick="openTabAndSaveStatus(event,'tabs__item--title','tabs__container--descr','block_layout_master');">
    Кластер из единственного узла
    </li>
  </ul>

<div id="block_layout_master" class="tabs__container tabs__container--descr">
<p>Кластера, состоящего из единственного узла, может быть достаточно, например, для ознакомительных целей.</p>
<ul>
  <li>
<p>Выполните на <strong>master-узле</strong> следующую команду, для того чтобы снять с него <i>taint</i> и разрешить остальным компонентам Deckhouse работать на master-узле:</p>

{% snippetcut %}
```bash
sudo -i d8 k patch nodegroup master --type json -p '[{"op": "remove", "path": "/spec/nodeTemplate/taints"}]'
```
{% endsnippetcut %}
  </li>
  <li>
<p>Настройте StorageClass <a href="/modules/local-path-provisioner/cr.html#localpathprovisioner">локального хранилища</a>, выполнив на <strong>master-узле</strong> следующую команду:</p>
{% snippetcut %}
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
{% endsnippetcut %}
  </li>
  <li>
<p>Укажите, что созданный StorageClass должен использоваться как StorageClass по умолчанию. Для этого выполните на <strong>master-узле</strong> следующую команду:</p>
{% snippetcut %}
```shell
sudo -i d8 k patch mc global --type merge \
  -p "{\"spec\": {\"settings\":{\"defaultClusterStorageClass\":\"localpath\"}}}"
```
{% endsnippetcut %}
  </li>
</ul>
</div>

<div id="block_layout_worker" class="tabs__container tabs__container--descr active">
<p>Добавьте узел в кластер (подробнее о добавлении статического узла в кластер читайте в <a href="/modules/node-manager/examples.html#добавление-статического-узла-в-кластер">документации</a>):</p>

<ul>
  <li>
    Подготовьте <strong>чистую</strong> виртуальную машину, которая будет узлом кластера.
  </li>
  <li>
<p>Настройте StorageClass <a href="/modules/local-path-provisioner/cr.html#localpathprovisioner">локального хранилища</a>, выполнив на <strong>master-узле</strong> следующую команду:</p>
{% snippetcut %}
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
{% endsnippetcut %}
  </li>
  <li>
<p>Укажите, что созданный StorageClass должен использоваться как StorageClass по умолчанию. Для этого выполните на <strong>master-узле</strong> следующую команду:</p>
{% snippetcut %}
```shell
sudo -i d8 k patch mc global --type merge \
  -p "{\"spec\": {\"settings\":{\"defaultClusterStorageClass\":\"localpath\"}}}"
```
{% endsnippetcut %}
  </li>
  <li>
    <p>Создайте <a href="/modules/node-manager/cr.html#nodegroup">NodeGroup</a> <code>worker</code>. Для этого выполните на <strong>master-узле</strong> следующую команду:</p>
{% snippetcut %}
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
{% endsnippetcut %}
  </li>
  <li>
    <p>Сгенерируйте SSH-ключ с пустой парольной фразой. Для этого выполните на <strong>master-узле</strong> следующую команду:</p>
{% snippetcut %}
```bash
ssh-keygen -t ed25519 -f /dev/shm/caps-id -C "" -N ""
```
{% endsnippetcut %}
  </li>
  <li>
    <p>Создайте в кластере ресурс <a href="/modules/node-manager/cr.html#sshcredentials">SSHCredentials</a>. Для этого выполните на <strong>master-узле</strong> следующую команду:</p>
{% snippetcut %}
```bash
sudo -i d8 k -f - <<EOF
apiVersion: deckhouse.io/v1alpha2
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
    <p>Выведите публичную часть сгенерированного ранее SSH-ключа (он понадобится на следующем шаге). Для этого выполните на <strong>master-узле</strong> следующую команду:</p>
{% snippetcut %}
```bash
cat /dev/shm/caps-id.pub
```
{% endsnippetcut %}
  </li>
  <li>
    <p><strong>На подготовленной виртуальной машине</strong> создайте пользователя <code>caps</code>. Для этого выполните следующую команду, указав публичную часть SSH-ключа, полученную на предыдущем шаге:</p>
{% snippetcut %}
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
{% endsnippetcut %}
  </li>
  <li>
    <p><strong>В операционных системах семейства Astra Linux</strong>, при использовании модуля мандатного контроля целостности Parsec, сконфигурируйте максимальный уровень целостности для пользователя <code>caps</code>:</p>
{% snippetcut %}
```bash
pdpl-user -i 63 caps
```
{% endsnippetcut %}
  </li>
  <li>
    <p>Создайте <a href="/modules/node-manager/cr.html#staticinstance">StaticInstance</a> для добавляемого узла. Для этого выполните на <strong>master-узле</strong> следующую команду, указав IP-адрес добавляемого узла:</p>
{% snippetcut %}
```bash
# Укажите IP-адрес узла, который необходимо подключить к кластеру.
export NODE=<NODE-IP-ADDRESS>
sudo -i d8 k -f - <<EOF
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
{% endsnippetcut %}
  </li>
  <li><p>Убедитесь, что все узлы кластера находятся в статусе <code>Ready</code>.</p>
<p>Выполните на <strong>master-узле</strong> следующую команду, чтобы получить список узлов кластера:</p>
{% snippetcut %}
```shell
sudo -i d8 k get no
```
{% endsnippetcut %}

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
</div>

<p>Запуск всех компонентов Deckhouse после завершения установки может занять какое-то время.</p>

<ul>
<li><p>Убедитесь, что под Kruise controller manager модуля <a href="/modules/ingress-nginx/">ingress-nginx</a> запустился и находится в статусе <code>Ready</code>.</p>
<p>Выполните на <strong>master-узле</strong> следующую команду:</p>

{% snippetcut %}
```shell
sudo -i d8 k -n d8-ingress-nginx get po -l app=kruise
```
{% endsnippetcut %}

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
{% snippetcut name="ingress-nginx-controller.yml" selector="ingress-nginx-controller-yml" %}
{% include_file "_includes/getting_started/{{ page.platform_code }}/partials/ingress-nginx-controller.yml.inc" syntax="yaml" %}
{% endsnippetcut %}
<p>Примените его, выполнив на <strong>master-узле</strong> следующую команду:</p>
{% snippetcut %}
```shell
sudo -i d8 k create -f $PWD/ingress-nginx-controller.yml
```
{% endsnippetcut %}

Запуск Ingress-контроллера после завершения установки Deckhouse может занять какое-то время. Прежде чем продолжить убедитесь что Ingress-контроллер запустился (выполните на <code>master-узле</code>):

{% snippetcut %}
```shell
sudo -i d8 k -n d8-ingress-nginx get po -l app=controller
```
{% endsnippetcut %}

Дождитесь перехода подов Ingress-контроллера в статус <code>Ready</code>.

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
{% snippetcut name="user.yml" selector="user-yml" %}
{% include_file "_includes/getting_started/stronghold/{{ page.platform_code }}/partials/user.yml.inc" syntax="yaml" %}
{% endsnippetcut %}
<p>Примените его, выполнив на <strong>master-узле</strong> следующую команду:</p>
{% snippetcut %}
```shell
sudo -i d8 k create -f $PWD/user.yml
```
{% endsnippetcut %}
</li>
<li><strong>Создание DNS-записи</strong>, для доступа в веб-интерфейсы кластера
{% alert level="warning" %}Домен в шаблоне не должен совпадать с доменом из параметра `clusterDomain` и с внутренней сервисной зоной сети. Пример: `clusterDomain: cluster.local` (значение по умолчанию), сервисная зона сети — `ru-central1.internal`. Тогда `publicDomainTemplate` не может быть `%s.cluster.local` или `%s.ru-central1.internal`.{% endalert %}
<p>Узнайте публичный IP-адрес узла, на котором работает Ingress-контроллер. Настройте DNS одним из следующих способов:</p>
{% tabs dns_options %}
{% tab "Wildcard DNS-запись" %}
Используйте этот вариант, если у вас есть DNS-сервер, а шаблон DNS-имён кластера — [wildcard-шаблон](https://en.wikipedia.org/wiki/Wildcard_DNS_record) (например, `%s.kube.company.my`).

Добавьте на DNS-сервере wildcard A-запись со значением этого публичного IP-адреса.
{% endtab %}
{% tab "Отдельные DNS-записи" %}
Используйте этот вариант, если у вас есть DNS-сервер, а шаблон DNS-имён кластера — **не** [wildcard-шаблон](https://en.wikipedia.org/wiki/Wildcard_DNS_record) (например, `%s-kube.company.my`).

Добавьте на DNS-сервере A- или CNAME-записи со значением этого публичного IP-адреса для DNS-имён из списка. Имена в списке построены по шаблону DNS-имён, который вы указали при настройке установки:

<div class="highlight">
<pre class="highlight">
<code example-hosts>{%- assign domains = site.data.reference.public-domains %}
{%- for domain in domains %}
  {%- if domain.gs %}{{ domain.name }}.example.com
{% endif -%}
{%- endfor %}</code>
</pre>
</div>

{% alert type="info" %}
В списке — DNS-имена, которые нужны кластеру, развёрнутому по этому руководству. Полный список приведён в разделе [«Список зарезервированных DNS-имён Deckhouse Platform»](/products/kubernetes-platform/documentation/latest/reference/public-domains.html) документации.
{% endalert %}
{% endtab %}
{% tab "Файл hosts" %}
Используйте этот вариант, если у вас нет DNS-сервера. На компьютере, с которого нужен доступ к сервисам Deckhouse, добавьте статические записи в hosts-файл (`/etc/hosts` для Linux, `%SystemRoot%\system32\drivers\etc\hosts` для Windows). Каждая запись связывает имя сервиса с публичным IP-адресом узла, на котором работает Ingress-контроллер.

Чтобы получить команду для добавления записей, воспользуйтесь [генератором данных для hosts-файла](/products/kubernetes-platform/documentation/latest/reference/public-domains.html#генератор-данных-для-hosts-файла): укажите шаблон DNS-имён и публичный IP-адрес узла, на котором работает Ingress-контроллер.
{% endtab %}
{% endtabs %}

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
