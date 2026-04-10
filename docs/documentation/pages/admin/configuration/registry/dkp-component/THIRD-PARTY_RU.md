---
title: "Managed Kubernetes: сторонний registry"
permalink: ru/admin/configuration/registry/dkp-component/third-party.html
description: "Как переключить Managed Kubernetes-кластер на сторонний registry для образов Deckhouse Kubernetes Platform."
lang: ru
search: managed kubernetes, third-party registry, change-registry, registry migration, сторонний registry
---

{% alert level="warning" %}
Эта инструкция подходит только для **Managed Kubernetes-кластеров**.

В таких кластерах нельзя управлять registry через модуль [`registry`](/modules/registry/).  
Если кластер полностью управляется DKP, используйте раздел [«Кластер, управляемый DKP»](../managing-interaction).
{% endalert %}

{% alert level="warning" %}
Использовать registry, отличные от `registry.deckhouse.io` и `registry.deckhouse.ru`, можно только в **коммерческих редакциях** DKP.
{% endalert %}

{% alert level="warning" %}
Если после переключения образ какого-либо модуля не загрузился заново и модуль не переустановился, воспользуйтесь [инструкцией из FAQ](../../../../faq.html#что-делать-если-образ-модуля-не-скачался-и-модуль-не-переустанов).
{% endalert %}

Эта инструкция помогает переключить Managed Kubernetes-кластер на другой registry для образов DKP.

## Когда нужна эта инструкция

Используйте этот сценарий, если хотите:

- перевести DKP на другой registry;
- использовать приватный registry вместо `registry.deckhouse.io` или `registry.deckhouse.ru`;
- настроить доступ к registry с логином, паролем или собственным CA-сертификатом.

## Как переключить кластер на сторонний registry

1. Запустите `helper change-registry` из pod'а Deckhouse.

   Пример:

   ```bash
   d8 k -n d8-system exec -ti svc/deckhouse-leader -c deckhouse -- \
     deckhouse-controller helper change-registry \
     --user MY-USER \
     --password MY-PASSWORD \
     registry.example.com/deckhouse/ee
   ```

1. Если registry использует самоподписанный сертификат, передайте CA-сертификат через `--ca-file`.

   Пример:

   ```bash
   CA_CONTENT=$(cat <<EOF
   -----BEGIN CERTIFICATE-----
   CERTIFICATE
   -----END CERTIFICATE-----
   -----BEGIN CERTIFICATE-----
   CERTIFICATE
   -----END CERTIFICATE-----
   EOF
   )

   d8 k -n d8-system exec svc/deckhouse-leader -c deckhouse -- bash -c \
   "echo '$CA_CONTENT' > /tmp/ca.crt && \
   deckhouse-controller helper change-registry \
     --ca-file /tmp/ca.crt \
     --user MY-USER \
     --password MY-PASSWORD \
     registry.example.com/deckhouse/ee"
   ```

1. Посмотрите доступные параметры команды, если нужны дополнительные настройки:

   ```bash
   d8 k -n d8-system exec -ti svc/deckhouse-leader -c deckhouse -- \
     deckhouse-controller helper change-registry --help
   ```

   Основные параметры:
   - `--user` — логин для доступа к registry;
   - `--password` — пароль или токен;
   - `--ca-file` — путь к CA-сертификату;
   - `--scheme` — схема подключения: `http` или `https`;
   - `--dry-run` — показать изменения без применения;
   - `--new-deckhouse-tag` — задать новый tag для образа Deckhouse.

1. Дождитесь, пока pod registry перейдёт в статус `Ready`.

   Если pod ушёл в `ImagePullBackOff`, перезапустите его и снова проверьте статус.

1. Дождитесь, пока новые настройки применятся на master-узле.

   Проверьте журнал `bashible`:

   ```bash
   journalctl -u bashible -n 20
   ```

   В журнале должна появиться строка:

   ```text
   Configuration is in sync, nothing to do
   ```

1. Проверьте, не осталось ли в кластере pod'ов со старым адресом registry:

   ```bash
   d8 k get pods -A -o json | jq -r '.items[] | select(.spec.containers[]
     | select(.image | startswith("registry.deckhouse"))) | .metadata.namespace + "\t" + .metadata.name' | sort | uniq
   ```

## Если нужно отключить автообновление

Если после переключения вы не хотите автоматически получать обновления через новый registry, удалите параметр `releaseChannel` из конфигурации модуля `deckhouse`.

## Что может пойти не так

### Pod registry не переходит в `Ready`

Проверьте:

- правильный ли логин и пароль вы передали;
- доступен ли registry из кластера;
- корректен ли CA-сертификат;
- не ушёл ли pod в `ImagePullBackOff`.

### В кластере остались pod'ы со старым registry

Это значит, что часть workload'ов ещё не перешла на новый адрес образов. Дождитесь обновления или перезапустите проблемные pod'ы по инструкции из FAQ.

### Образ модуля не скачался

Используйте [инструкцию из FAQ](../../../faq.html#что-делать-если-образ-модуля-не-скачался-и-модуль-не-переустанов).

## Что дальше

- Если кластер полностью управляется DKP, используйте раздел [«Кластер, управляемый DKP»](../managing-interaction).
- Если нужно восстановить доступ к registry, перейдите в раздел [«Восстановление подключения к registry»](../restore-token).
- Если нужно хранить образы приложений внутри кластера, откройте [Payload registry](../../custom_image_storage/payload-registry).
