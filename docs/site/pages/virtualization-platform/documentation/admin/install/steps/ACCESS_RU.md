---
title: "Deckhouse Virtualization Platform"
permalink: ru/virtualization-platform/documentation/admin/install/steps/access.html
lang: ru
---

## Подключение к master-узлу

Deckhouse завершил процесс установки кластера. Осталось выполнить некоторые настройки, для чего необходимо подключиться к master-узлу.

Подключитесь к master-узлу по SSH (IP-адрес master-узла был выведен инсталлятором по завершении установки, но вы также можете найти его используя веб-интерфейс или CLI‑утилиты облачного провайдера):

```bash
ssh ubuntu@<MASTER_IP>
```

Проверьте работу kubectl, выведя список узлов кластера:

```bash
sudo /opt/deckhouse/bin/kubectl get nodes
```

Запуск Ingress-контроллера после завершения установки Deckhouse может занять какое-то время. Прежде чем продолжить убедитесь что Ingress-контроллер запустился:

```bash
sudo /opt/deckhouse/bin/kubectl -n d8-ingress-nginx get po
```

Дождитесь перехода Pod’ов в статус `Ready`.

Также дождитесь готовности балансировщика:

```bash
sudo /opt/deckhouse/bin/kubectl -n d8-ingress-nginx get svc nginx-load-balancer
```

Значение `EXTERNAL-IP` должно быть заполнено публичным IP-адресом или DNS-именем.

## DNS

Для того чтобы получить доступ к веб-интерфейсам компонентов Deckhouse, необходимо:

1. Настроить работу DNS.
2. Указать в параметрах Deckhouse шаблон DNS-имен.

Шаблон DNS-имен используется для настройки Ingress-ресурсов системных приложений. Например, интерфейсу Grafana закреплено имя `grafana`. Тогда, для шаблона `%s.kube.company.my`, Grafana будет доступна по адресу `grafana.kube.company.my`, и т.д.

Чтобы упростить настройку, будет использоваться сервис `sslip.io`. 

На master-узле выполните следующую команду, чтобы получить IP-адрес балансировщика и настроить шаблон DNS-имен сервисов Deckhouse на использование `sslip.io`:

```bash
BALANCER_IP=$(sudo /opt/deckhouse/bin/kubectl -n d8-ingress-nginx get svc nginx-load-balancer -o json | jq -r '.status.loadBalancer.ingress[0].ip') && \
echo "Balancer IP is '${BALANCER_IP}'." && sudo /opt/deckhouse/bin/kubectl patch mc global --type merge \
  -p "{\"spec\": {\"settings\":{\"modules\":{\"publicDomainTemplate\":\"%s.${BALANCER_IP}.sslip.io\"}}}}" && echo && \
echo "Domain template is '$(sudo /opt/deckhouse/bin/kubectl get mc global -o=jsonpath='{.spec.settings.modules.publicDomainTemplate}')'."
```

Команда также выведет установленный шаблон DNS-имен. Пример вывода:

```bash
Balancer IP is '1.2.3.4'.
moduleconfig.deckhouse.io/global patched

Domain template is '%s.1.2.3.4.sslip.io'.
```

## Настройте удаленный доступ к кластеру

На персональном компьютере выполните следующие шаги, для того чтобы настроить подключение `kubectl` к кластеру:

- Откройте веб-интерфейс сервиса Kubeconfig Generator. Для него зарезервировано имя `kubeconfig`, и адрес для доступа формируется согласно шаблона DNS-имен (который вы установили ранее). Например, для шаблона DNS-имен `%s.1.2.3.4.sslip.io`, веб-интерфейс Kubeconfig Generator будет доступен по адресу `https://kubeconfig.1.2.3.4.sslip.io`.
- Авторизуйтесь под пользователем `admin@deckhouse.io`. Пароль пользователя, сгенерированный на предыдущем шаге, — `035hduuvo7` (вы также можете найти его в CustomResource `User` в файле `resource.yml`).
- Выберите вкладку с ОС персонального компьютера.
- Последовательно скопируйте и выполните команды, приведенные на странице.
- Проверьте корректную работу `kubectl` (например, выполнив команду `kubectl get no`).