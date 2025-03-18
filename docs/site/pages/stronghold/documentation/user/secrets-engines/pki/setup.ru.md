---
title: "Механизм секретов PKI - настройка и использование"
permalink: ru/stronghold/documentation/user/secrets-engines/pki/setup.html
lang: ru
description: The PKI secrets engine for Stronghold generates TLS certificates.
---

Эта секция содержит краткий обзор настройки и использования движка секретов PKI.

## Настройка

Большинство механизмов секретов должно быть настроено заранее, чтобы они могли выполнять свои функции. Эти шаги обычно выполняются оператором или инструментом управления конфигурацией.

1. Включите механизм секретов PKI:

    ```shell
    $ d8 stronghold secrets enable pki
    Success! Enabled the pki secrets engine at: pki/
    ```

    По умолчанию механизм секретов будет установлен с именем движка. Чтобы включить механизм секретов по другому пути, используйте аргумент `-path`.

1. Увеличьте TTL, настроив механизм секретов. Значение по умолчанию в 30 дней может быть слишком коротким, поэтому увеличьте его до 1 года:

    ```shell
    $ d8 stronghold secrets tune -max-lease-ttl=8760h pki
    Success! Tuned the secrets engine at: pki/
    ```

    Обратите внимание, что отдельные роли могут ограничивать это значение до более короткого на основе каждого сертификата. Это лишь настраивает глобальное максимальное значение для этого механизма секретов.

1. Настройте сертификат CA и приватный ключ. Stronghold может использовать уже существующую пару ключей или сгенерировать собственный самоподписанный корневой сертификат. В общем случае, мы рекомендуем поддерживать ваш корневой CA вне Stronghold и предоставлять Stronghold подписанный промежуточный CA.

    ```shell
    $ d8 stronghold write pki/root/generate/internal \
        common_name=my-website.ru \
        ttl=8760h

    Key              Value
    ---              -----
    certificate      -----BEGIN CERTIFICATE-----...
    expiration       1756317679
    issuing_ca       -----BEGIN CERTIFICATE-----...
    serial_number    fc:f1:fb:2c:6d:4d:99:1e:82:1b:08:0a:81:ed:61:3e:1d:fa:f5:29
    ```

    Возвращаемый сертификат является чисто информативным. Закрытый ключ безопасно хранится внутри Stronghold.

1. Обновите местоположение CRL и выпускающие сертификаты. Эти значения могут быть обновлены в будущем.

    ```shell
    $ d8 stronghold write pki/config/urls \
        issuing_certificates="http://127.0.0.1:8200/v1/pki/ca" \
        crl_distribution_points="http://127.0.0.1:8200/v1/pki/crl"
    Success! Data written to: pki/config/urls
    ```

1. Настройте роль, которая сопоставляет имя в Stronghold с процедурой генерации сертификата. Когда пользователи или машины генерируют учетные данные, они генерируются для этой роли:

    ```shell
    $ d8 stronghold write pki/roles/example-dot-ru \
        allowed_domains=my-website.ru \
        allow_subdomains=true \
        max_ttl=72h
    Success! Data written to: pki/roles/example-dot-ru
    ```

## Использование

После того как механизм секретов настроен и у пользователя/машины есть Stronghold-токен с соответствующими правами, можно генерировать учетные данные.

1. Сгенерируйте новые учетные данные, записав их в путь `/issue` с именем роли:

    ```shell
    $ d8 stronghold write pki/issue/example-dot-ru \
        common_name=www.my-website.ru

    Key                 Value
    ---                 -----
    certificate         -----BEGIN CERTIFICATE-----...
    issuing_ca          -----BEGIN CERTIFICATE-----...
    private_key         -----BEGIN RSA PRIVATE KEY-----...
    private_key_type    rsa
    serial_number       1d:2e:c6:06:45:18:60:0e:23:d6:c5:17:43:c0:fe:46:ed:d1:50:be
    ```

    Вывод будет включать динамически сгенерированный закрытый ключ и сертификат, который соответствует данной роли и истекает через 72 часа (как указано в нашем определении роли). Также возвращаются выпускающий CA и цепочка доверия для упрощения автоматизации.

## API

У механизма секретов PKI есть полный HTTP API. Пожалуйста, ознакомьтесь с [API](api_guide.html) документацией.
