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

   Добавьте DNS-записи:

   - **Wildcard-шаблон** (например `%s.domain.my`) — одна wildcard A-запись на IP узла с Ingress.
   - **Шаблон без wildcard** (например `%s-kube.company.my`) — A- или CNAME-записи для каждого имени:

     ```bash
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
     registry.domain.my
     prometheus.domain.my
     status.domain.my
     tools.domain.my
     ```

   Если DNS-сервера нет, добавьте записи на **рабочей станции** (в Windows — `%SystemRoot%\system32\drivers\etc\hosts`):

   ```bash
   export PUBLIC_IP="<MASTER_IP>"
   sudo -E bash -c "cat <<EOF >> /etc/hosts
   $PUBLIC_IP api.domain.my
   $PUBLIC_IP code.domain.my
   $PUBLIC_IP commander.domain.my
   $PUBLIC_IP console.domain.my
   $PUBLIC_IP dex.domain.my
   $PUBLIC_IP documentation.domain.my
   $PUBLIC_IP grafana.domain.my
   $PUBLIC_IP hubble.domain.my
   $PUBLIC_IP istio.domain.my
   $PUBLIC_IP istio-api-proxy.domain.my
   $PUBLIC_IP kubeconfig.domain.my
   $PUBLIC_IP openvpn-admin.domain.my
   $PUBLIC_IP registry.domain.my
   $PUBLIC_IP prometheus.domain.my
   $PUBLIC_IP status.domain.my
   $PUBLIC_IP tools.domain.my
   EOF
   "
   ```

Кластер DVP развёрнут и готов к работе.
