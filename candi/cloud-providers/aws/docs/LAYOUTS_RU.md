---
title: "Cloud provider — AWS: схемы размещения"
---

Поддерживаются три схемы размещения. Ниже подробнее о каждой их них.

## WithoutNAT

**Рекомендованная схема размещения.**

Каждому узлу присваивается публичный IP (ElasticIP). NAT не используется.

![resources](https://docs.google.com/drawings/d/e/2PACX-1vQDR2iRcFO3Ra3hmdrYCuoHPP6m3DCArtZjmbQGMJL00xmR-F94IMJKx2jKqeiwe-KvbykqtCEjsR9c/pub?w=812&h=655)
<!--- Исходник: https://docs.google.com/drawings/d/1JDmeSY12EoZ3zBfanEDY-QvSgLekzw6Tzjj2pgY8giM/edit --->

Пример конфигурации схемы размещения:

```yaml
apiVersion: deckhouse.io/v1
kind: AWSClusterConfiguration
layout: WithoutNAT
provider:
  providerAccessKeyId: MYACCESSKEY
  providerSecretAccessKey: mYsEcReTkEy
  region: eu-central-1
masterNodeGroup:
  # Количество master-узлов.
  # Если указано больше одного master-узла, то etcd-кластер соберётся автоматически.
  replicas: 1
  instanceClass:
    instanceType: m5.xlarge
    ami: ami-03818140b4ac9ae2b
nodeGroups:
  - name: mydb
    nodeTemplate:
      labels:
        node-role.kubernetes.io/mydb: ""
    replicas: 2
    instanceClass:
      instanceType: t2.medium
      ami: ami-03818140b4ac9ae2b
    additionalTags:
      backup: me
vpcNetworkCIDR: "10.241.0.0/16"
nodeNetworkCIDR: "10.241.32.0/20"
sshPublicKey: "ssh-rsa <SSH_PUBLIC_KEY>"
tags:
  team: torpedo
```

## WithNAT

>**Важно!** В данной схеме размещения необходим bastion-хост для доступа к узлам (его можно создать вместе с кластером, указав параметры в секции `withNAT.bastionInstance`).

Виртуальные машины выходят в интернет через NAT Gateway с общим и единственным IP-адресом.

![resources](https://docs.google.com/drawings/d/e/2PACX-1vRS95L6rJr_SswWphLYYHN9GZLC3I0jpbKXbjr3935kqJdaeBIxmJyejKCOUdLPaKlY2Fk_zzNaGmE9/pub?w=1422&h=997)
<!--- Исходник: https://docs.google.com/drawings/d/1UPzygO3w8wsRNHEna2uoYB-69qvW6zDYB5s1OumUOes/edit --->

Пример конфигурации схемы размещения:

```yaml
apiVersion: deckhouse.io/v1
kind: AWSClusterConfiguration
layout: WithNAT
provider:
  providerAccessKeyId: MYACCESSKEY
  providerSecretAccessKey: mYsEcReTkEy
  region: eu-central-1
withNAT:
  bastionInstance:
    zone: eu-central-1a
    instanceClass:
      instanceType: m5.large
      ami: ami-09a4a23815cdb5e06
masterNodeGroup:
  # Количество master-узлов.
  # Если указано больше одного master-узла, то etcd-кластер соберётся автоматически.
  replicas: 1
  instanceClass:
    instanceType: m5.xlarge
    ami: ami-03818140b4ac9ae2b
nodeGroups:
  - name: mydb
    nodeTemplate:
      labels:
        node-role.kubernetes.io/mydb: ""
    replicas: 2
    instanceClass:
      instanceType: t2.medium
      ami: ami-03818140b4ac9ae2b
    additionalTags:
      backup: me
vpcNetworkCIDR: "10.241.0.0/16"
nodeNetworkCIDR: "10.241.32.0/20"
sshPublicKey: "ssh-rsa <SSH_PUBLIC_KEY>"
tags:
  team: torpedo
```
