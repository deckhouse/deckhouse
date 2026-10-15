{%- include getting_started/dvp/global/partials/gs_scripts.liquid step='access' -%}

Установка платформы завершена. Проверьте кластер и настройте DNS для доступа к веб-интерфейсам DVP.

## Проверка кластера

1. Подключитесь к **master-узлу**:

   ```shell
   ssh ubuntu@<MASTER_IP>
   ```

1. Убедитесь, что все узлы в статусе `Ready`:

   ```shell
   sudo -i d8 k get nodes
   ```

   {% offtopic title="Пример вывода" %}
   <!-- markdownlint-disable MD031 -->
   ```console
   NAME            STATUS   ROLES                  AGE   VERSION
   dvp-master-0    Ready    control-plane,master   30m   v1.29.x
   dvp-worker-1    Ready    worker                 5m    v1.29.x
   ```
   {: .nowrap-default }
   <!-- markdownlint-enable MD031 -->
   {% endofftopic %}

   Запуск компонентов DVP после установки может занять некоторое время.

1. Дождитесь, пока платформа завершит настройку — очередь задач должна опустеть:

   ```shell
   sudo -i d8 system queue list
   ```

   {% offtopic title="Пример вывода" %}
   <!-- markdownlint-disable MD031 -->
   ```console
   Summary:
   - 'main' queue: empty.
   - 132 other queues (0 active, 132 empty): 0 tasks.
   - no tasks to handle.
   ```
   <!-- markdownlint-enable MD031 -->
   {% endofftopic %}

   Пока в очереди есть задачи, компоненты платформы ещё разворачиваются. Сразу после установки это нормально и может занять до 15 минут.

1. Убедитесь, что все включённые модули запущены:

   ```shell
   sudo -i d8 k get module | grep -E "True[[:space:]]+False"
   ```

   Команда выводит модули, которые включены, но ещё не готовы. Пустой вывод означает, что готовы все.

1. Убедитесь, что платформа готова к запуску ВМ:

   ```shell
   sudo -i d8 k get vmclass generic
   ```

   Дождитесь статуса `Ready`.

   {% alert level="info" %}
   На ОС семейства RHEL (AlmaLinux, Rocky Linux, CentOS) при первой настройке узлов загружаются дополнительные пакеты — пока это происходит, класс `generic` остаётся в статусе `Pending`.
   {% endalert %}

## Настройка веб-интерфейсов DVP

Убедитесь, что кластер работает, и настройте DNS для доступа к веб-интерфейсам DVP с рабочей станции.

1. На **master-узле** убедитесь, что поды [`ingress-nginx`](/modules/ingress-nginx/) запущены:

   ```shell
   sudo -i d8 k -n d8-ingress-nginx get po -l app=kruise
   sudo -i d8 k -n d8-ingress-nginx get po -l app=controller
   ```

   Дождитесь статуса `Ready` у подов Ingress-контроллера.

   {% offtopic title="Пример вывода" %}
   <!-- markdownlint-disable MD031 -->
   ```console
   NAME                                         READY   STATUS    RESTARTS   AGE
   kruise-controller-manager-7dfcbdc549-b4wk7   3/3     Running   0          15m

   NAME                   READY   STATUS    RESTARTS   AGE
   controller-nginx-r6hxc   3/3     Running   0          5m
   ```
   {: .nowrap-default }
   <!-- markdownlint-enable MD031 -->
   {% endofftopic %}

1. Настройте DNS для веб-интерфейсов DVP. [Шаблон DNS-имён](/products/kubernetes-platform/documentation/v1/reference/api/global.html#parameters-modules-publicdomaintemplate) (`publicDomainTemplate`) задаёт имена для Ingress: для `%s.domain.my` Grafana — `grafana.domain.my`, веб-интерфейс DVP — `console.domain.my`.

   {% alert level="warning" %}
   Домен в шаблоне не должен совпадать с `clusterDomain` (например `cluster.local`) или внутренними сервисными зонами из конфигурации кластера.
   {% endalert %}

   На **master-узле** проверьте шаблон и IP-адрес узла с Ingress:

   ```shell
   sudo -i d8 k get mc global -ojsonpath='{.spec.settings.modules.publicDomainTemplate}{"\n"}'
   sudo -i d8 k get pods -n d8-ingress-nginx -o=jsonpath='{range .items[*]}{.metadata.name}{"\t"}{.status.hostIP}{"\n"}{end}' | grep '^controller' | awk '{print $2}'
   ```

   Настройте DNS одним из следующих способов:

   {% tabs dns_options %}
   {% tab "Wildcard DNS-запись" %}
   Используйте этот вариант, если у вас есть DNS-сервер, а шаблон DNS-имён кластера — [wildcard-шаблон](https://en.wikipedia.org/wiki/Wildcard_DNS_record) (например, `%s.domain.my`).

   Добавьте на DNS-сервере wildcard A-запись со значением IP-адреса узла с Ingress-контроллером.
   {% endtab %}
   {% tab "Отдельные DNS-записи" %}
   Используйте этот вариант, если у вас есть DNS-сервер, а шаблон DNS-имён кластера — **не** [wildcard-шаблон](https://en.wikipedia.org/wiki/Wildcard_DNS_record) (например, `%s-kube.company.my`).

   Добавьте на DNS-сервере A- или CNAME-записи с IP-адресом узла с Ingress-контроллером для следующих DNS-имён:

   ```text
   api.domain.my
   code.domain.my
   commander.domain.my
   console.domain.my
   dex.domain.my
   documentation.domain.my
   grafana.domain.my
   hubble.domain.my
   istio.domain.my
   istio-api-proxy.domain.my
   kubeconfig.domain.my
   openvpn-admin.domain.my
   prometheus.domain.my
   registry.domain.my
   status.domain.my
   tools.domain.my
   upmeter.domain.my
   virtualization.domain.my
   ```
   {% endtab %}
   {% tab "Файл hosts" %}
   Используйте этот вариант, если у вас нет DNS-сервера. Добавьте статические записи в hosts-файл на **рабочей станции** (`/etc/hosts` для Linux, `%SystemRoot%\system32\drivers\etc\hosts` для Windows).

   Чтобы получить команду для добавления записей, воспользуйтесь [генератором данных для hosts-файла](/products/kubernetes-platform/documentation/latest/reference/public-domains.html#генератор-данных-для-hosts-файла): укажите шаблон DNS-имён и IP-адрес узла с Ingress-контроллером. Оба значения выводят команды выше.
   {% endtab %}
   {% endtabs %}

Кластер DVP развёрнут и готов к работе.
