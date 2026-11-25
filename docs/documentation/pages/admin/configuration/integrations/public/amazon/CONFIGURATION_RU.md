---
title: Настройка интеграции с Amazon Web Services
permalink: ru/admin/integrations/public/amazon/configuration.html
lang: ru
---

## Назначение AWSClusterConfiguration

[Ресурс AWSClusterConfiguration](/modules/cloud-provider-aws/cluster_configuration.html#awsclusterconfiguration) описывает параметры кластера и используется Deckhouse Platform для:

- задания схемы размещения и сетевых CIDR;
- конфигурации master- и рабочих узлов;
- указания параметров подключения к AWS API (ключи доступа, регион);
- назначения общих и специфичных тегов;
- описания настроек bastion-хоста (в схеме WithNAT).

Обязательные поля:

- `apiVersion` — должен быть `deckhouse.io/v1`;
- `kind` — всегда AWSClusterConfiguration.

Пример заголовка ресурса:

```yaml
apiVersion: deckhouse.io/v1
kind: AWSClusterConfiguration
```

Чтобы отредактировать этот ресурс в работающем кластере, выполните команду:

```shell
d8 system edit provider-cluster-configuration
```

После внесения изменений их необходимо применить с помощью команды:

```shell
dhctl converge
```
