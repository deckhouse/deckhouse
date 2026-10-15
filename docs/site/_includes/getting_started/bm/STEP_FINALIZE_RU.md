<script type="text/javascript" src='{% javascript_asset_tag getting-started %}[_assets/js/getting-started.js]{% endjavascript_asset_tag %}'></script>
<script type="text/javascript" src='{% javascript_asset_tag getting-started-access %}[_assets/js/getting-started-access.js]{% endjavascript_asset_tag %}'></script>
<script type="text/javascript" src='{% javascript_asset_tag bcrypt %}[_assets/js/bcrypt.js]{% endjavascript_asset_tag %}'></script>

<style>
  #block_layout_master, #block_layout_worker { display: none; }
</style>

<div id="block_layout_master" class="tabs__content_master" style="display: none;">
<p>Кластера, состоящего из единственного узла, может быть достаточно, например, для ознакомительных целей.</p>
<ul>
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
    
<div class="tabs-block">
  <ul class="tabs__container tabs__container--title">
    <li id='tab_block_caps' href="javascript:void(0)" class="tabs__item tabs__item--title active"
    onclick="openTabAndSaveStatus(event, 'tabs__item--title', 'tabs__caps', 'block_bootstrap');
              openTabAndSaveStatus(event, 'tabs__item--title', 'tabs__bootstrap', 'block_caps');">
    CAPS
    </li>
    <li id='tab_block_bootstrap' href="javascript:void(0)" class="tabs__item tabs__item--title"
    onclick="openTabAndSaveStatus(event, 'tabs__item--title', 'tabs__bootstrap', 'block_caps');
              openTabAndSaveStatus(event, 'tabs__item--title', 'tabs__caps', 'block_bootstrap');">
    Bootstrap-скрипт
    </li>
  </ul>

  <div id="block_bootstrap" class="tabs__container tabs__container--descr tabs__bootstrap">
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
  <div id="block_caps" class="tabs__container tabs__container--descr tabs__caps active">
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
ssh-keygen -t ed25519 -f /dev/shm/caps-id -C "" -N ""
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
{% tabs os %}
{% tab "ОС на базе Ubuntu" %}
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
{% endtab %}
{% tab "Для CentOS, Rocky Linux, ALT Linux, РОСА Сервер, РЕД ОС, МОС ОС" %}
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
{% endtab %}
{% endtabs %}
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
{% alert type="info" %}
{% offtopic title="Добавление двух и более worker-узлов..." %}
**Bootstrap-скрипт**:  
Используйте тот же ресурс NodeGroup `worker` и тот же bootstrap-скрипт, которые вы использовали при добавлении первого узла. 
Для каждого дополнительного узла:

1. Подготовьте чистую виртуальную машину.
1. На этой ВМ выполните команду с тем же Base64-кодом скрипта, который вы использовали при добавлении первого узла:

   ```shell
   echo <Base64-КОД-СКРИПТА> | base64 -d | bash
   ```

**CAPS**:  
Если вы используете CAPS и хотите добавить статические узлы в NodeGroup `worker`:

1. Увеличьте значение <a href="/modules/node-manager/cr.html#nodegroup-v1-spec-staticinstances-count">spec.staticInstances.count</a> в ресурсе NodeGroup `worker` до нужного количества узлов.  
1. Создайте по одному ресурсу StaticInstance для каждого нового узла, указав:  
   - лейбл `role: worker`;
   - IP-адрес этого узла в поле <a href="/modules/node-manager/cr.html#staticinstance-v1alpha2-spec-address">spec.address</a>;
   - ссылку на <a href="/modules/node-manager/cr.html#sshcredentials">SSHCredentials</a> в <a href="/modules/node-manager/cr.html#staticinstance-v1alpha2-spec-credentialsref">spec.credentialsRef</a>.
{%- endofftopic %}
{% endalert %}

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

{% alert level="info" %}
Если в качестве шаблона DNS-имён кластера вы используете `sslip.io` или аналогичный wildcard-DNS-сервис, создавать DNS-записи вручную не требуется.
{% endalert %}
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
