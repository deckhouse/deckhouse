---
title: "Import Key Wrapping Guide"
permalink: ru/stronghold/documentation/user/secrets-engines/transit/key-wrapping-guide.html
lang: ru
description: |-
  Details about wrapping keys for import into the transit secrets engine.
---

The "bring your own key" (BYOK) functionality for the transit
secrets engine allows users to import keys that were generated
outside of Stronghold into the transit secrets engine.

This document describes the process for wrapping an externally-generated
key (the target key) for import into Stronghold. It describes the processes
for importing a software-stored key using Golang and for importing a key
that is stored in an HSM.

### Mount the secrets engine

```shell-session
$ d8 stronghold secrets enable transit
Success! Enabled the transit secrets engine at: transit/
```

### Retrieve the transit wrapping key

```shell-session
$ d8 stronghold read transit/wrapping_key
```

This returns a 4096-bit RSA key.

The steps after this depend on whether the key is stored using
a software solution or in an HSM.

### Software example (Go)

This example assumes that the key is stored in software using the
variable name `key`. It demonstrates how to wrap the target key using
Golang crypto libraries.

Once you have the wrapping key, you can parse it using the `encoding/pem`
and `crypto/x509` libraries (the example code below assumes that the wrapping
key has been written to a variable called `wrappingKeyString`):

```
keyBlock, _ := pem.Decode([]byte(wrappingKeyString))
parsedKey, err := x509.ParsePKIXPublicKey(keyBlock.Bytes)
if err != nil {
    return err
}
```

Then generate an ephemeral AES key for wrapping the target key.
This example uses Golang's `crypto/rand` library for generating the key:

```
ephemeralAESKey := make([]byte, 32)
_, err := rand.Read(ephemeralAESKey)
if err != nil {
        return err
}
```

{% alert level="warning" %}

**NOTE**: Be sure to securely delete the ephemeral AES key once it
has been used!

{% endalert %}
Google's [tink library](https://pkg.go.dev/github.com/tink-crypto/tink-go/kwp/subtle)
provides a function for performing the key wrap operation:

```
wrapKWP, err := subtle.NewKWP(aesKey)
if err != nil {
        return err
}
wrappedTargetKey, err := wrapKWP.Wrap(key)
if err != nil {
        return err
}
```

Then encrypt the ephemeral AES key using the transit wrapping key:

```
wrappedAESKey, err := rsa.EncryptOAEP(
        sha256.New(),
        rand.Reader,
        wrappingKey,
        ephemeralAESKey,
        []byte{},
)
if err != nil {
        return err
}
```

Note that though this example uses SHA256, Stronghold also supports the use of
SHA1, SHA384, or SHA512. The hash function that was used at this step will
need to be provided as a parameter when importing the key.

Finally, concatenate the wrapped keys into a single byte string.
The leftmost 4096 bits of the string should be the wrapped AES key, and
the remaining bits should be the wrapped target key. Then the resulting
bytes should be base64-encoded.

```
combinedCiphertext := append(wrappedAESKey, wrappedTargetKey...)
base64Ciphertext := base64.StdEncoding.EncodeToString(combinedCiphertext)
```

This is the ciphertext that should be provided to Stronghold when importing a
key into the transit secrets engine.

```shell-session
$ d8 stronghold write transit/keys/test-key/import ciphertext=$CIPHERTEXT hash_function=SHA256 type=$KEY_TYPE
```


### AWS CloudHSM example

This example demonstrates how to import a key into the transit secrets engine from
an AWS CloudHSM cluster. The process and mechanisms used will apply to importing
a key from an HSM in general, but the details will differ between HSMs.

For information on creating and communicating with an AWS CloudHSM cluster, see
the [Getting Started guide in the AWS CloudHSM documentation](https://docs.aws.amazon.com/cloudhsm/latest/userguide/getting-started.html).

Communication with the HSM uses AWS's `key_mgmt_util` tool. For help setting that
up, see the [Getting Started page for key_mgmt_util](https://docs.aws.amazon.com/cloudhsm/latest/userguide/key_mgmt_util-getting-started.html).

The first step is writing the transit wrapping key to the HSM. This involves
creating a new RSA public key object with the key returned by transit's
`wrapping_key` endpoint.

```shell-session
$ importPubKey -f wrapping_key.pem -l "my-transit-wrapping-key"
```

This will create the public key in the HSM with all of the necessary permissions.
If you're using a different tool, make sure that the usage for the wrapping key
includes the attribute `CKA_WRAP`.

The next step is wrapping the target key using the wrapping key. If the
ID of the target key is `1` and the wrapping key is `2`, the command looks like this:

```shell-session
$ wrapKey -noheader -k 1 -w 2 -t 3 -m 7 -out ciphertext.key
```

The `-m 7` flag specifies the mechanism to use for the key wrapping. For AWS CloudHSM,
7 corresponds to the PKCS11 mechanism `CKM_AES_RSA_KEY_WRAP` ([see the AWS documentation for details](https://docs.aws.amazon.com/cloudhsm/latest/userguide/key_mgmt_util-wrapKey.html)).
The `-t 3` flag specifies `SHA256` as the hash function. The result is written to a
file called `ciphertext.key`. The `noheader` flag ensures that the ciphertext does
not include an AWS-specific header.

The output from this is a binary file, which needs to be base64-encoded when it
is provided to Stronghold.

```shell-session
$ export CIPHERTEXT=$(base64 ciphertext.key)
$ d8 stronghold write transit/keys/test-key/import ciphertext=$CIPHERTEXT hash_function=SHA256 type=$KEY_TYPE
```

Once the key has been imported, it can be used like any other transit key.
