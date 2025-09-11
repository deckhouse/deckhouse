---
title: "HSM support"
permalink: en/stronghold/documentation/admin/standalone/hsm.html
---

Stronghold supports Root key encryption using hardware security modules (HSM) such as TPM2, Rutoken ECP 3.0, JaCarta, and other devices that support the PKCS11 standard.  
For testing and development purposes, SoftHSM2 is also supported.  

To use automatic unsealing via PKCS11, you must first create keys in the HSM and configure Stronghold to use them.

## SoftHSM2

1. Install the required packages:

   ```shell
   apt install libsofthsm2 opensc
   ```

1. Create a configuration for SoftHSM2:

   ```shell
   mkdir /home/stronghold/softhsm
   cd softhsm
   echo "directories.tokendir = /home/stronghold/softhsm/" > /home/stronghold/softhsm2.conf
   ```

1. Generate keys in the HSM:

   ```console
   export SOFTHSM2_CONF=/home/stronghold/softhsm2.conf
   HSMLIB="/usr/lib/x86_64-linux-gnu/softhsm/libsofthsm2.so"
   pkcs11-tool --module $HSMLIB --init-token --so-pin 1234 --init-pin --pin 4321 --label my_token --login

   Using slot 0 with a present token (0x0)
   Token successfully initialized
   User PIN successfully initialized

   pkcs11-tool --module $HSMLIB -L

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

   pkcs11-tool --module $HSMLIB --login --pin 4321 --keypairgen --key-type rsa:4096 --label "vault-rsa-key"

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

1. Example Stronghold configuration (`config.hcl`):

   ```console
   api_addr="https://0.0.0.0:8200"
   log_level = "warn"
   ui = true

   listener "tcp" {
     address          = "0.0.0.0:8200"
     tls_cert_file    = "/home/stronghold/cert.pem"
     tls_key_file     = "/home/stronghold/key.pem"
     #tls_require_and_verify_client_cert = true
     #tls_client_ca_file = "ca.crt"
     tls_disable      = "false"
   }

   storage "raft" {
     path = "/home/stronghold/data"
   }

   seal "pkcs11" {
     lib           = "/usr/lib/x86_64-linux-gnu/softhsm/libsofthsm2.so"
     token_label   = "my_token"
     pin           = "4321"
     key_label     = "vault-rsa-key"
     rsa_oaep_hash = "sha1"
   }
   ```

1. Start Stronghold:

   ```shell
   export SOFTHSM2_CONF=/home/stronghold/softhsm2.conf
   stronghold server -config config.hcl
   ```

## Using Rutoken ECDSA 3.0

1. Download and install the `librtpkcs11ecp.so` library from [https://www.rutoken.ru/](https://www.rutoken.ru/support/download/pkcs/).

1. Generate a key pair (public and private) in the token, which will be used for Root key encryption.  
   This operation is performed with the `pkcs11-tool` utility from the `opensc` package:

   ```console
   HSMLIB="/usr/lib/librtpkcs11ecp.so"
   pkcs11-tool --module $HSMLIB --init-token --so-pin 87654321 \
               --init-pin --pin 12345678 --label my_token --login
   pkcs11-tool --module $HSMLIB --login --pin 12345678 --keypairgen \
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

1. Add the `pkcs11` seal method to the Stronghold configuration:

   ```console
   ...
   seal "pkcs11" {
     lib         = "/usr/lib/librtpkcs11ecp.so"
     token_label = "my_token"
     pin         = "12345678"
     key_label   = "vault-rsa-key"
   }
   ```

1. Start Stronghold and run `init`:

   ```shell
   systemctl start stronghold

   stronghold operator init
   ```

1. Check the Stronghold status:

   ```console
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

## Migration from Shamir Keys to HSM

1. Modify the Stronghold configuration by adding the `seal` block:

   ```console
   ...
   seal "pkcs11" {
     lib         = "/usr/lib/librtpkcs11ecp.so"
     token_label = "my_token"
     pin         = "12345678"
     key_label   = "vault-rsa-key"
   }
   ```

1. Restart Stronghold. The logs will show a message:

   ```console
   2025-04-03T17:08:13.431+0300 [WARN]  core: entering seal migration mode; Stronghold will not automatically unseal even if using an autoseal: from_barrier_type=shamir to_barrier_type=pkcs11
   ```

1. Perform the migration by entering the unseal keys:

   ```shell
   stronghold operator unseal -migrate
   ```

After the migration is complete, Stronghold will automatically unseal using `pkcs11` on restart.

## Migration from HSM to Shamir keys

1. Modify the configuration by adding the parameter `disabled = "true"` to the `seal` block:

   ```console
   ...
   seal "pkcs11" {
     lib         = "/usr/lib/librtpkcs11ecp.so"
     token_label = "my_token"
     pin         = "12345678"
     key_label   = "vault-rsa-key"
     disabled    = "true"
   }
   ```

1. Restart Stronghold.

1. Perform the migration by entering the recovery keys:

   ```shell
   stronghold operator unseal -migrate
   ```

After the migration is complete, Stronghold will require manual entry of unseal keys on each restart.
