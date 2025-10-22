---
title: "Управление DNS"
permalink: ru/stronghold/documentation/admin/platform-management/network/dns.html
lang: ru
---

Для установки компонентов CoreDNS и управления DNS можно воспользоваться функциональностью модуля kube-dns.

{% alert level="info" %}
Модуль удаляет ранее созданные с помощью kubeadm ресурсы: Deployment, ConfigMap и RBAC для CoreDNS.
{% endalert %}

Чтобы включить модуль kube-dns с настройками по умолчанию, примените ресурс `ModuleConfig`:

```yaml
d8 k apply -f - <<EOF
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: kube-dns
spec:
  enabled: true
EOF
```

Подробности о возможностях настроек модуля описаны по [ссылке](/modules/kube-dns/configuration.html).

## Пример конфигурации DNS

Пример конфигурации модуля kube-dns с помощью ресурса ModuleConfig:

```yaml
d8 k apply -f - <<EOF
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: kube-dns
spec:
  version: 1
  enabled: true
  settings:
    # Список IP-адресов рекурсивных DNS-серверов, которые CoreDNS будет использовать для разрешения внешних доменов.
    # По умолчанию используется список из /etc/resolv.conf.
    upstreamNameservers:
      - 8.8.8.8
      - 8.8.4.4
    # Статический список хостов в стиле /etc/hosts:
    hosts:
      - domain: one.example.com
        ip: 192.168.0.1
      - domain: two.another.example.com
        ip: 10.10.0.128
    # Список дополнительных зон для обслуживания CoreDNS.
    stubZones:
      - zone: consul.local
        upstreamNameservers:
          - 10.150.0.1
    # Список альтернативных доменов кластера, разрешаемых наравне с global.discovery.clusterDomain.
    clusterDomainAliases:
      - foo.bar
      - baz.qux
EOF
```

Подробности о возможностях конфигурации модуля kube-dns описаны по [ссылке](/modules/kube-dns/configuration.html).

## Изменение домена кластера

Чтобы изменить домен кластера с минимальным простоем, выполните следующие шаги:

1. Отредактируйте настройки модуля control-plane manager, который отвечает за конфигурацию Deckhouse.

    Внесите изменения в секции по шаблону ниже:

    ```yaml
    apiVersion: deckhouse.io/v1alpha1
    kind: ModuleConfig
    metadata:
      name: control-plane-manager
    spec:
      version: 1
      enabled: true
      settings:
        apiserver:
          # Список опций сертификата SANs, с которыми будет сгенерирован сертификат API-сервера.
          certSANs:
           - kubernetes.default.svc.<старый clusterDomain>
           - kubernetes.default.svc.<новый clusterDomain>
          serviceAccount:
            # Список API audience’ов, которые следует добавить при создании токенов ServiceAccount.
            additionalAPIAudiences:
            - https://kubernetes.default.svc.<старый clusterDomain>
            - https://kubernetes.default.svc.<новый clusterDomain>
            # Список дополнительных издателей API токенов ServiceAccount, которые нужно включить при их создании.
            additionalAPIIssuers:
            - https://kubernetes.default.svc.<старый clusterDomain>
            - https://kubernetes.default.svc.<новый clusterDomain>
    ```

1. Укажите список альтернативных доменов кластера в настройках модуля kube-dns:

    ```yaml
    apiVersion: deckhouse.io/v1alpha1
    kind: ModuleConfig
    metadata:
      name: kube-dns
    spec:
      version: 1
      enabled: true
      settings:
        clusterDomainAliases:
          - <старый clusterDomain>
          - <новый clusterDomain>
    ```

1. Дождитесь перезапуска `kube-apiserver`.
1. Поменяйте `clusterDomain` на новый домен в `dhctl config edit cluster-configuration`.
