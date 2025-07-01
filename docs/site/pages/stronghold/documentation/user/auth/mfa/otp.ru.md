---
title: "TOTP"
permalink: ru/stronghold/documentation/user/auth/mfa/totp.html
lang: ru
---

## Настройка TOTP

Stronghold поддерживает проверку дополнительного фактора при аутентификации с использованием
Time-Based One-Time Password (TOTP) - одноразовых короткоживущих кодов.
Проверка TOTP может быть установлена как для конкретного пользователя, так и для метода
аутентификации целиком, в том числе принудительно.

Потребуется включить метод MFA TOTP и получить его идентификатор:

```shell
TOTP_METHOD_ID=$(d8 stronghold write identity/mfa/method/totp \
    -format=json \
    generate=true \
    issuer=MyTOTP \
    period=30 \
    key_size=30 \
    algorithm=SHA256 \
    digits=6 | jq -r '.data.method_id')
echo $TOTP_METHOD_ID
```

Если администратору требуется включить (или пересоздать) TOTP MFA для конкретного пользователя,
потребуется указать его идентификатор:

```shell
ENTITY_ID="f0075fa0-89ca-6235-5b90-b4420134cd36"
```

После чего сгенерировать QR-код для настройки OTP:

```shell
d8 stronghold write -field=barcode \
    /identity/mfa/method/totp/admin-generate \
    method_id=$TOTP_METHOD_ID entity_id=$ENTITY_ID \
    | base64 -d > /tmp/qr-code.png
```

Если у пользователя есть доступ к эндпойнту `identity/mfa/method/totp/generate`,
тогда пользователь сам сможет получить настройки TOTP MFA через UI Stronghold,
используя этот идентификатор.

## Включение MFA

В качестве примера разберём проверку MFA для метода аутентификации Userpass.
Для начала потребуется идентификатор метода:

```shell
LDAP_ACCESSOR=$(d8 stronghold auth list -format=json \
    --detailed | jq -r '."userpass/".accessor')
echo $LDAP_ACCESSOR
```

Включите MFA:

```shell
d8 stronghold write /identity/mfa/login-enforcement/userpass-totp-enforcement \
    mfa_method_ids="$TOTP_METHOD_ID" \
    auth_method_accessors=$LDAP_ACCESSOR
```

Выполните вход:

```shell
d8 stronghold login -method=userpass username=user password='My-Password-1234'
Initiating Interactive MFA Validation...
Enter the passphrase for methodID "22c35aa4-bf37-cf31-4187-c5a676c19aca" of type "totp":
```

Чтобы отключить проверку MFA, выполните:

```shell
d8 stronghold delete identity/mfa/login-enforcement/userpass-totp-enforcement
```
