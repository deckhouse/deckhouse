---
title: "Поддержка HSM"
permalink: ru/stronghold/documentation/admin/standalone/hsm.html
lang: ru
---

Stronghold поддерживает шифрование Root-ключа с использованием аппаратных модулей защиты (HSM), таких как TPM2, Rutoken ЭЦП 3.0, JaCarta и другие устройства с поддержкой стандарта PKCS11.  
Для целей тестирования и разработки также поддерживается SoftHSM2.  

Для использования автоматического распечатывания через PKCS11 необходимо предварительно создать ключи в HSM и сконфигурировать Stronghold для работы с ними.

## SoftHSM2

1. Установите необходимые пакеты:

   ```shell
   apt install libsofthsm2 opensc
   ```

1. Создайте конфигурацию для SoftHSM2:

   ```shell
   mkdir /home/stronghold/softhsm
   cd softhsm
   echo "directories.tokendir = /home/stronghold/softhsm/" > /home/stronghold/softhsm2.conf
   ```

1. Сгенерируйте ключи в HSM:

   ```shell
   $ export SOFTHSM2_CONF=/home/stronghold/softhsm2.conf
   $ HSMLIB="/usr/lib/x86_64-linux-gnu/softhsm/libsofthsm2.so"
   $ pkcs11-tool --module $HSMLIB --init-token --so-pin 1234 --init-pin --pin 4321 --label my_token --login

   Using slot 0 with a present token (0x0)
   Token successfully initialized
   User PIN successfully initialized

   $ pkcs11-tool --module $HSMLIB -L

   Available slots:
   Slot 0 (0xe6829d3): SoftHSM slot ID 0xe6829d3
     token label        : my_token
     token manufacturer : SoftHSM project
     token model        : SoftHSM v2
     token flags        : login required, rng, token initialized, PIN initialized, other flags=0x20
     hardware version   : 2.6
     firmware version   : 2.6
     serial num         : 6a5468368e6829d3
     pin min/max        : 4/255
   Slot 1 (0x1): SoftHSM slot ID 0x1
     token state:   uninitialized


   $ pkcs11-tool --module $HSMLIB --login --pin 4321 --keypairgen --key-type rsa:4096 --label "vault-rsa-key"

   Using slot 0 with a present token (0xe6829d3)
   Key pair generated:
   Private Key Object; RSA
     label:      vault-rsa-key
     Usage:      decrypt, sign, signRecover, unwrap
     Access:     sensitive, always sensitive, never extractable, local
   Public Key Object; RSA 4096 bits
     label:      vault-rsa-key
     Usage:      encrypt, verify, verifyRecover, wrap
     Access:     local
   ```

   Пример конфигурации Stronhold (`config.hcl`)

   ```console
   api_addr="https://0.0.0.0:8200"
   log_level = "warn"
   ui = true
   listener "tcp" {
     address = "0.0.0.0:8200"
     tls_cert_file = "/home/stronghold/cert.pem"
     tls_key_file  = "/home/stronghold/key.pem"
     #tls_require_and_verify_client_cert = true
     #tls_client_ca_file = "ca.crt"
     tls_disable = "false"
   }
   storage "raft" {
     path = "/home/stronghold/data"
   }

   seal "pkcs11" {
     lib = "/usr/lib/x86_64-linux-gnu/softhsm/libsofthsm2.so"
     token_label = "my_token"
     pin = "4321"
     key_label = "vault-rsa-key"
     rsa_oaep_hash = "sha1"
   }
   ```

1. Запустите Stronghold:

   ```shell
   export SOFTHSM2_CONF=/home/stronghold/softhsm2.conf
   stronghold server -config config.hcl
   ```

## Использование Рутокен ЭЦП 3.0

1. Скачайте и установите библиотеку `librtpkcs11ecp.so` с сайта [https://www.rutoken.ru/](https://www.rutoken.ru/support/download/pkcs/).

1. Сгенерируйте в токене пару ключей (публичный и приватный), которые будут использоваться для шифрования Root-ключа. Эта операция выполняется с помощью утилиты `pkcs11-tool` из пакета `opensc`:

   ```shell
   $ HSMLIB="/usr/lib/librtpkcs11ecp.so"
   $ pkcs11-tool --module $HSMLIB --init-token --so-pin 87654321 \
                 --init-pin --pin 12345678 --label my_token --login
   $ pkcs11-tool --module $HSMLIB --login --pin 12345678 --keypairgen \
                 --key-type rsa:2048 --label "vault-rsa-key"

   Using slot 0 with a present token (0x0)
   Key pair generated:
   Private Key Object; RSA
     label:      vault-rsa-key
     Usage:      decrypt, sign
     Access:     sensitive, always sensitive, never extractable, local
   Public Key Object; RSA 2048 bits
     label:      vault-rsa-key
     Usage:      encrypt, verify
     Access:     local
   ```

1. Добавьте в конфигурацию Stronghold метод распечатки `pkcs11`:

   ```console
   ...
   seal "pkcs11" {
     lib = "/usr/lib/librtpkcs11ecp.so"
     token_label = "my_token"
     pin = "12345678"
     key_label = "vault-rsa-key"
   }
   ```

1. Запустите Stronghold и выполните `init`:

   ```shell
   systemctl start stronghold

   stronghold operator init
   ```

1. Проверьте статус Stronghold:

   ```shell
   stronghold status

   Key                      Value
   ---                      -----
   Recovery Seal Type       shamir
   Initialized              true
   Sealed                   false
   Total Recovery Shares    5
   Threshold                3
   Version                  1.15.2+hsm
   Build Date               2025-04-03T13:06:02Z
   Storage Type             raft
   Cluster Name             stronghold-cluster-6586e287
   Cluster ID               d7552773-2e8a-33b6-9c32-6749a4c9af13
   HA Enabled               false
   ```

## Миграция с Shamir ключей на HSM

1. Измените конфигурацию Stronghold, добавив блок `seal`:

   ```console
   ...
   seal "pkcs11" {
     lib = "/usr/lib/librtpkcs11ecp.so"
     token_label = "my_token"
     pin = "12345678"
     key_label = "vault-rsa-key"
   }
   ```

1. Перезапустите Stronghold. В логах появится сообщение:

   ```console
   2025-04-03T17:08:13.431+0300 [WARN]  core: entering seal migration mode; Stronghold will not automatically unseal even if using an autoseal: from_barrier_type=shamir to_barrier_type=pkcs11
   ```

1. Выполните миграцию, введя ключи распечатки:

   ```shell
   stronghold operator unseal -migrate
   ```

После завершения миграции Stronghold при перезапуске будет автоматически распечатываться с использованием `pkcs11`.

## Миграция с HSM на Shamir ключи

1. Измените конфигурацию, добавив параметр `disabled = "true"` в раздел `seal`:

   ```console
   ...
   seal "pkcs11" {
     lib = "/usr/lib/librtpkcs11ecp.so"
     token_label = "my_token"
     pin = "12345678"
     key_label = "vault-rsa-key"
     disabled = "true"
   }
   ```

1. Перезапустите Stronghold.

1. Выполните миграцию, введя recovery-ключи:

   ```shell
   stronghold operator unseal -migrate
   ```

После завершения миграции при каждом перезапуске Stronghold потребуется вводить ключи распечатки вручную.
