---
title: Переключение работающего кластера DKP на использование стороннего registry
permalink: ru/admin/configuration/registry/third-party.html
description: "Переключение Deckhouse Kubernetes Platform на использование стороннего container registry. Настройка внешних registry и миграция с официального registry."
lang: ru
---

{% alert level="warning" %}
При использовании модуля [`registry`](/modules/registry/) смена адреса и параметров registry выполняется в секции [registry](/modules/deckhouse/configuration.html#parameters-registry) конфигурации модуля `deckhouse`. Пример настройки приведен в документации модуля [`registry`](/modules/registry/examples.html).
{% endalert %}

{% alert level="warning" %}
Использование registries, отличных от `registry.deckhouse.io` и `registry.deckhouse.ru`, доступно только в коммерческих редакциях Deckhouse Kubernetes Platform.
{% endalert %}

Для переключения кластера на использование стороннего registry выполните следующие действия:

1. Выполните команду `deckhouse-controller helper change-registry` из пода DKP с параметрами нового registry.
   Пример запуска:

   ```shell
   d8 k -n d8-system exec -ti svc/deckhouse-leader -c deckhouse -- deckhouse-controller helper change-registry --user MY-USER --password MY-PASSWORD registry.example.com/deckhouse/ee
   ```

1. Если registry использует самоподписанные сертификаты, положите корневой сертификат соответствующего сертификата registry в файл `/tmp/ca.crt` в поде DKP и добавьте к вызову опцию `--ca-file /tmp/ca.crt` или вставьте содержимое CA в переменную, как в примере ниже:

   ```shell
   CA_CONTENT=$(cat <<EOF
   -----BEGIN CERTIFICATE-----
   CERTIFICATE
   -----END CERTIFICATE-----
   -----BEGIN CERTIFICATE-----
   CERTIFICATE
   -----END CERTIFICATE-----
   EOF
   )
   d8 k -n d8-system exec svc/deckhouse-leader -c deckhouse -- bash -c "echo '$CA_CONTENT' > /tmp/ca.crt && deckhouse-controller helper change-registry --ca-file /tmp/ca.crt --user MY-USER --password MY-PASSWORD registry.example.com/deckhouse/ee"
   ```

   Просмотреть список доступных ключей команды `deckhouse-controller helper change-registry` можно, выполнив следующую команду:

   ```shell
   d8 k -n d8-system exec -ti svc/deckhouse-leader -c deckhouse -- deckhouse-controller helper change-registry --help
   ```

   Пример вывода:

   ```console
   usage: deckhouse-controller helper change-registry [<flags>] <new-registry>

   Change registry for deckhouse images.

   Flags:
     --help               Show context-sensitive help (also try --help-long and --help-man).
     --user=USER          User with pull access to registry.
     --password=PASSWORD  Password/token for registry user.
     --ca-file=CA-FILE    Path to registry CA.
     --scheme=SCHEME      Used scheme while connecting to registry, http or https.
     --dry-run            Don't change deckhouse resources, only print them.
     --new-deckhouse-tag=NEW-DECKHOUSE-TAG
                         New tag that will be used for deckhouse deployment image (by default
                         current tag from deckhouse deployment will be used).

   Args:
     <new-registry>  Registry that will be used for deckhouse images (example:
                     registry.deckhouse.io/deckhouse/ce). By default, https will be used, if you need
                     http - provide '--scheme' flag with http value
   ```

1. Дождитесь перехода пода registry в статус `Ready`. Если под находится в статусе `ImagePullBackoff`, перезапустите его.
1. Дождитесь применения новых настроек на master-узле.

   Проверьте журнал системного сервиса bashible на master-узле, например, с помощью следующей команды:

   ```shell
   journalctl -u bashible -n 20
   ```

   В журнале должно появится сообщение `Configuration is in sync, nothing to do`.

   Пример вывода при просмотре журнала сервиса bashible:

   ```console
   $ journalctl -u bashible -n 20
   ...
   Aug 13 05:03:08 kube-master-0 systemd[1]: Started Bashible service.
   Aug 13 05:03:10 kube-master-0 bash[1847265]: Configuration is in sync, nothing to do.   <--
   Aug 13 05:03:10 kube-master-0 systemd[1]: bashible.service: Deactivated successfully.
   Aug 13 05:03:10 kube-master-0 systemd[1]: bashible.service: Consumed 1.075s CPU time.
   ```

1. Если необходимо отключить автоматическое обновление registry через сторонний registry, удалите параметр `releaseChannel` из конфигурации модуля `deckhouse`.

1. Проверьте, не осталось ли в кластере подов с оригинальным адресом registry:

   ```shell
   d8 k get pods -A -o json | jq -r '.items[] | select(.spec.containers[]
     | select(.image | startswith("registry.deckhouse"))) | .metadata.namespace + "\t" + .metadata.name' | sort | uniq
   ```
