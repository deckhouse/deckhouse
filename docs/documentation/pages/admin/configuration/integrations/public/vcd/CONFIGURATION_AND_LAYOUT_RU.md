---
title: Схемы размещения и настройка
permalink: ru/admin/integrations/public/vcd/vcd-layout.html
lang: ru
---

Deckhouse поддерживает одну схему размещения в VMware Cloud Director — Standard. Она обеспечивает изолированную внутреннюю сеть с возможностью назначения статических IP-адресов и подключения Elastic IP (через DNAT).

## Standard

Схема Standard подразумевает:

- наличие внутренней изолированной сети (CIDR);
- использование DHCP для назначения IP-адресов узлам;
- возможность назначения статических IP-адресов для системных узлов (например, master и frontend);
- проброс трафика снаружи через Edge Gateway с DNAT и firewall;
- возможность назначить Elastic IP-адреса узлам через внешнюю настройку;
- использование vApp и сети, заранее настроенных в VMware Cloud Director.

![resources](../../../../images/cloud-provider-vcd/vcd-standard.png)
<!--- Исходник: https://www.figma.com/design/T3ycFB7P6vZIL359UJAm7g/%D0%98%D0%BA%D0%BE%D0%BD%D0%BA%D0%B8-%D0%B8-%D1%81%D1%85%D0%B5%D0%BC%D1%8B?node-id=995-11247&t=Qb5yyWumzPiTBtfL-0 --->

Пример конфигурации:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: VCDClusterConfiguration
layout: Standard
sshPublicKey: "ssh-rsa AAAABBBBB"
organization: deckhouse
virtualDataCenter: MSK-1
virtualApplicationName: deckhouse
internalNetworkCIDR: 192.168.199.0/24
provider:
  server: '<SERVER>'
  username: '<USERNAME>'
  password: '<PASSWORD>'
  insecure: true
masterNodeGroup:
  replicas: 1
  instanceClass:
    storageProfile: "Fast vHDD"
    sizingPolicy: 4cpu8mem
    template: "catalog/Ubuntu 22.04 Server"
    mainNetwork: internal
    mainNetworkIPAddresses:
      - 192.168.199.10
```

> Убедитесь, что CIDR-сеть не пересекается с другими используемыми сетями, если в кластере предполагается пиринг или внешние подключения.
>
> Для каждого master-узла можно задать свой IP-адрес через поле `mainNetworkIPAddresses`.
>
> Если используется DHCP, то параметр `mainNetworkIPAddresses` можно опустить — IP будет выдан автоматически.
