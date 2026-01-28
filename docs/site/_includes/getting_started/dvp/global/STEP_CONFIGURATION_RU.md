<script type="text/javascript" src='{% javascript_asset_tag getting-started %}[_assets/js/getting-started.js]{% endjavascript_asset_tag %}'></script>
<script type="text/javascript" src='{% javascript_asset_tag getting-started-access %}[_assets/js/getting-started-access.js]{% endjavascript_asset_tag %}'></script>

## Дополнительная конфигурация кластера

На master-узле кластера создайте файл `additional_configuration.yml`:

{% capture includePath %}_includes/getting_started/dvp/bm/partials/config.ru.yml.other.ce.inc{% endcapture %}
{% include_file "{{ includePath }}" syntax="yaml" %}

После этого примените файл конфигурации, выполнив команду:

```console
sudo -i d8 k apply -f additional_configuration.yml
```

## Настройка ingress-контроллера

Прежде чем продолжить убедитесь что Ingress-контроллер запустился (выполните на <code>master-узле</code>):</p>

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

## Проверка работоспособности всех компонентов

### Проверка доступности узлов кластера

Выведите список всех узлов кластера, выполнив на master-узле следующую команду:

```bash
sudo -i d8 k get no
```

Убедитесь, что все узлы находятся в состоянии `Ready`. Пример корректного вывода:

```console
NAME     STATUS   ROLES           AGE   VERSION
master   Ready    control-plane   15m   v1.30.0
worker   Ready    <none>          12m   v1.30.0
```

### Проверка работоспособности хранилища NFS

1. Убедитесь, что модуль `csi-nfs` находится в состоянии `Ready`:

   ```bash
   sudo -i d8 k get module csi-nfs -w
   ```

1. Проверьте, что NFSStorageClass создан успешно:

   ```bash
   sudo -i d8 k get nfsstorageclass
   ```

1. Проверьте, что StorageClass установлен как используемый по умолчанию:

   ```bash
   sudo -i d8 k get storageclass
   ```

   В колонке `DEFAULT` у `nfs-storage-class` должна быть отметка.

### Проверка работоспособности модуля `virtualization`

Дождитесь, пока все поды модуля `virtualization` не перейдут в статус `Running`:

```bash
sudo -i d8 k get po -n d8-virtualization
```

Пример вывода:

```console
NAME                                         READY   STATUS    RESTARTS      AGE
cdi-apiserver-858786896d-rsfjw               3/3     Running   0             10m
cdi-deployment-6d9b646b5b-8dgmj              3/3     Running   0             10m
cdi-operator-5fdc989d9f-zmk55                3/3     Running   0             10m
dvcr-74dc9c94b-pczhx                         2/2     Running   0             10m
virt-api-78d49dcbbf-qwggw                    3/3     Running   0             10m
virt-controller-6f8fff445f-w866w             3/3     Running   0             10m
virt-handler-g6l9h                           4/4     Running   0             10m
virt-handler-t5fgb                           4/4     Running   0             10m
virt-handler-ztj77                           4/4     Running   0             10m
virt-operator-58dc5459d5-hpps8               3/3     Running   0             10m
virtualization-api-5d69f55947-k6h9n          1/1     Running   0             10m
virtualization-controller-69647d98c6-9rkht   3/3     Running   0             10m
vm-route-forge-288z7                         1/1     Running   0             10m
vm-route-forge-829wm                         1/1     Running   0             10m
vm-route-forge-nq9xr                         1/1     Running   0             10m
```

### Проверка доступа к кластеру DVP

Для доступа к веб-интерфейсу Deckhouse Virtualization Platform выполните следующие действия:

1. Откройте в браузере адрес `console.domain.my`;
1. Введите учетные данные администратора, которые были созданы на этапе настройки доступа.
1. Убедитесь, что веб-интерфейс открывается и отображается корректно.

Поздравляем, ваш кластер готов к работе. Вы успешно настроили:

- master-узел с развернутой Deckhouse Virtualization Platform;
- worker-узел для запуска рабочих нагрузок;
- NFS-хранилище для данных;
- модуль `virtualization` для создания виртуальных машин;
- Веб-интерфейс для управления платформой;
- Ingress-контроллер для доступа к веб-интерфейсу и к виртуальным машинам.

Далее рассмотрим дальнейшие возможности использования Deckhouse Virtualization Platform.
