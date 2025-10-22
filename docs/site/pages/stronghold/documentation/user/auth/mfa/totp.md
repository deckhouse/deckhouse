---
title: "TOTP"
permalink: en/stronghold/documentation/user/auth/mfa/totp.html
---

## Configuring TOTP

Stronghold supports verification of an additional authentication factor using Time-Based One-Time Passwords (TOTP),
which are short-lived one-time codes.
TOTP verification can be configured for a specific user or for the entire authentication method, including enforced mode.

To configure TOTP, follow these steps:

1. Enable the TOTP MFA method and obtain its ID:

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

1. If you need to enable (or recreate) TOTP MFA for a specific user, specify the user's ID:

   ```shell
   ENTITY_ID="f0075fa0-89ca-6235-5b90-b4420134cd36"
   ```

1. Generate a QR code for OTP configuration:

   ```shell
   d8 stronghold write -field=barcode \
       /identity/mfa/method/totp/admin-generate \
       method_id=$TOTP_METHOD_ID entity_id=$ENTITY_ID \
       | base64 -d > /tmp/qr-code.png
   ```

If the user has access to the `identity/mfa/method/totp/generate` endpoint,
they can obtain their own TOTP MFA configuration via the Stronghold UI using the method ID specified above.

## Enabling MFA

As an example, let's configure MFA verification for the Userpass authentication method.

1. Obtain the method's accessor:

   ```shell
   LDAP_ACCESSOR=$(d8 stronghold auth list -format=json \
       --detailed | jq -r '."userpass/".accessor')
   echo $LDAP_ACCESSOR
   ```

1. Enable MFA:

   ```shell
   d8 stronghold write /identity/mfa/login-enforcement/userpass-totp-enforcement \
       mfa_method_ids="$TOTP_METHOD_ID" \
       auth_method_accessors=$LDAP_ACCESSOR
   ```

1. Log in:

   ```shell
   d8 stronghold login -method=userpass username=user password='My-Password-1234'
   Initiating Interactive MFA Validation...
   Enter the passphrase for methodID "22c35aa4-bf37-cf31-4187-c5a676c19aca" of type "totp":
   ```

To disable MFA verification, run:

```shell
d8 stronghold delete identity/mfa/login-enforcement/userpass-totp-enforcement
```
