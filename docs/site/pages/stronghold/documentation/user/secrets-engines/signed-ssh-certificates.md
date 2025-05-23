---
title: Signed SSH certificates
permalink: en/stronghold/documentation/user/secrets-engines/ssh.html
lang: en
description: >-
  The signed SSH certificates is the simplest and most powerful in terms of

  setup complexity and in terms of being platform agnostic. When using this

  type, an SSH CA signing key is generated or configured at the secrets engine's
  mount.

  This key will be used to sign other SSH keys.
---

The signed SSH certificates is the simplest and most powerful in terms of setup
complexity and in terms of being platform agnostic. By leveraging Stronghold's
powerful CA capabilities and functionality built into OpenSSH, clients can SSH
into target hosts using their own local SSH keys.

In this section, the term "**client**" refers to the person or machine
performing the SSH operation. The "**host**" refers to the target machine. If
this is confusing, substitute "client" with "user".

This page will show a quick start for this secrets engine. For detailed documentation
on every path, use `bao path-help` after mounting the secrets engine.

## Client key signing

Before a client can request their SSH key be signed, the Stronghold SSH secrets engine must
be configured. Usually an Stronghold administrator or security team performs these
steps. It is also possible to automate these actions using a configuration
management tool like Chef, Puppet, Ansible, or Salt.

### Signing key &amp; role configuration

The following steps are performed in advance by an Stronghold administrator, security
team, or configuration management tooling.

1. Mount the secrets engine. Like all secrets engines in Stronghold, the SSH secrets engine
    must be mounted before use.

    ```text
    $ d8 stronghold secrets enable -path=ssh-client-signer ssh
    Successfully mounted 'ssh' at 'ssh-client-signer'!
    ```

    This enables the SSH secrets engine at the path "ssh-client-signer". It is
    possible to mount the same secrets engine multiple times using different
    `-path` arguments. The name "ssh-client-signer" is not special - it can be
    any name, but this documentation will assume "ssh-client-signer".

1. Configure Stronghold with a CA for signing client keys using the `/config/ca`
    endpoint. If you do not have an internal CA, Stronghold can generate a keypair for
    you.

    ```text
    $ d8 stronghold write ssh-client-signer/config/ca generate_signing_key=true
    Key             Value
    ---             -----
    public_key      ssh-rsa AAAAB3NzaC1yc2EA...
    ```

    If you already have a keypair, specify the public and private key parts as
    part of the payload:

    ```text
    $ d8 stronghold write ssh-client-signer/config/ca \
        private_key="..." \
        public_key="..."
    ```

    The SSH secrets engine allows multiple Certificate Authority (CA) certificates
    ("issuers") to be configured in a single mount. This feature is designed to
    facilitate CA rotation. When configuring a CA, one issuer is designated as the
    default - its operations will be used whenever no specific issuer is referenced
    during role creation. The default issuer can be changed at any time by either
    generating a new CA or updating it through the configuration endpoint, enabling
    seamless CA rotation.

    Regardless of whether it is generated or uploaded, the client signer public
    key is accessible via the API at the `/public_key` endpoint or the CLI (see next step).

1. Add the public key to all target host's SSH configuration. This process can
    be manual or automated using a configuration management tool. The public key is
    accessible via the API and does not require authentication.

    ```text
    curl -o /etc/ssh/trusted-user-ca-keys.pem http://127.0.0.1:8200/v1/ssh-client-signer/public_key
    ```

    ```text
    d8 stronghold read -field=public_key ssh-client-signer/config/ca > /etc/ssh/trusted-user-ca-keys.pem
    ```

    Add the path where the public key contents are stored to the SSH
    configuration file as the `TrustedUserCAKeys` option.

    ```text
    # /etc/ssh/sshd_config
    # ...
    TrustedUserCAKeys /etc/ssh/trusted-user-ca-keys.pem
    ```

    Restart the SSH service to pick up the changes.

1. Create a named Stronghold role for signing client keys.

    Because of the way some SSH certificate features are implemented, options
    are passed as a map. The following example adds the `permit-pty` extension
    to the certificate, and allows the user to specify their own values for `permit-pty` and `permit-port-forwarding`
    when requesting the certificate.

    ```text
    $ d8 stronghold write ssh-client-signer/roles/my-role -<<"EOH"
    {
      "algorithm_signer": "rsa-sha2-256",
      "allow_user_certificates": true,
      "allowed_users": "*",
      "allowed_extensions": "permit-pty,permit-port-forwarding",
      "default_extensions": {
        "permit-pty": ""
      },
      "key_type": "ca",
      "default_user": "ubuntu",
      "ttl": "30m0s"
    }
    EOH
    ```

### Client SSH authentication

The following steps are performed by the client (user) that wants to
authenticate to machines managed by Stronghold. These commands are usually run from
the client's local workstation.

1. Locate or generate the SSH public key. Usually this is `~/.ssh/id_rsa.pub`.
    If you do not have an SSH keypair, generate one:

    ```text
    ssh-keygen -t rsa -C "user@example.com"
    ```

1. Ask Stronghold to sign your **public key**. This file usually ends in `.pub` and
    the contents begin with `ssh-rsa ...`.

    ```text
    $ d8 stronghold write ssh-client-signer/sign/my-role \
        public_key=@$HOME/.ssh/id_rsa.pub

    Key             Value
    ---             -----
    serial_number   c73f26d2340276aa
    signed_key      ssh-rsa-cert-v01@openssh.com AAAAHHNzaC1...
    ```

    The result will include the serial and the signed key. This signed key is
    another public key.

    To customize the signing options, use a JSON payload:

    ```text
    $ d8 stronghold write ssh-client-signer/sign/my-role -<<"EOH"
    {
      "public_key": "ssh-rsa AAA...",
      "valid_principals": "my-user",
      "key_id": "custom-prefix",
      "extensions": {
        "permit-pty": "",
        "permit-port-forwarding": ""
      }
    }
    EOH
    ```

1. Save the resulting signed, public key to disk. Limit permissions as needed.

    ```text
    $ d8 stronghold write -field=signed_key ssh-client-signer/sign/my-role \
        public_key=@$HOME/.ssh/id_rsa.pub > signed-cert.pub
    ```

    If you are saving the certificate directly beside your SSH keypair, suffix
    the name with `-cert.pub` (`~/.ssh/id_rsa-cert.pub`). With this naming
    scheme, OpenSSH will automatically use it during authentication.

1. (Optional) View enabled extensions, principals, and metadata of the signed
    key.

    ```text
    ssh-keygen -Lf ~/.ssh/signed-cert.pub
    ```

1. SSH into the host machine using the signed key. You must supply both the
    signed public key from Stronghold **and** the corresponding private key as
    authentication to the SSH call.

    ```text
    ssh -i signed-cert.pub -i ~/.ssh/id_rsa username@10.0.23.5
    ```

## Host key signing

For an added layer of security, we recommend enabling host key signing. This is
used in conjunction with client key signing to provide an additional integrity
layer. When enabled, the SSH agent will verify the target host is valid and
trusted before attempting to SSH. This will reduce the probability of a user
accidentally SSHing into an unmanaged or malicious machine.

### Signing key configuration

1. Mount the secrets engine. For the most security, mount at a different path from the
    client signer.

    ```text
    $ d8 stronghold secrets enable -path=ssh-host-signer ssh
    Successfully mounted 'ssh' at 'ssh-host-signer'!
    ```

1. Configure Stronghold with a CA for signing host keys using the `/config/ca`
    endpoint. If you do not have an internal CA, Stronghold can generate a keypair for
    you.

    ```text
    $ d8 stronghold write ssh-host-signer/config/ca generate_signing_key=true
    Key             Value
    ---             -----
    public_key      ssh-rsa AAAAB3NzaC1yc2EA...
    ```

    If you already have a keypair, specify the public and private key parts as
    part of the payload:

    ```text
    $ d8 stronghold write ssh-host-signer/config/ca \
        private_key="..." \
        public_key="..."
    ```

    Regardless of whether it is generated or uploaded, the host signer public
    key is accessible via the API at the `/public_key` endpoint.

1. Extend host key certificate TTLs.

    ```text
    d8 stronghold secrets tune -max-lease-ttl=87600h ssh-host-signer
    ```

1. Create a role for signing host keys. Be sure to fill in the list of allowed
    domains, set `allow_bare_domains`, or both.

    ```text
    $ d8 stronghold write ssh-host-signer/roles/hostrole \
        key_type=ca \
        algorithm_signer=rsa-sha2-256 \
        ttl=87600h \
        allow_host_certificates=true \
        allowed_domains="localdomain,example.com" \
        allow_subdomains=true
    ```

1. Sign the host's SSH public key.

    ```text
    $ d8 stronghold write ssh-host-signer/sign/hostrole \
        cert_type=host \
        public_key=@/etc/ssh/ssh_host_rsa_key.pub
    Key             Value
    ---             -----
    serial_number   3746eb17371540d9
    signed_key      ssh-rsa-cert-v01@openssh.com AAAAHHNzaC1y...
    ```

1. Set the resulting signed certificate as `HostCertificate` in the SSH
    configuration on the host machine.

    ```text
    $ d8 stronghold write -field=signed_key ssh-host-signer/sign/hostrole \
        cert_type=host \
        public_key=@/etc/ssh/ssh_host_rsa_key.pub > /etc/ssh/ssh_host_rsa_key-cert.pub
    ```

    Set permissions on the certificate to be `0640`:

    ```text
    chmod 0640 /etc/ssh/ssh_host_rsa_key-cert.pub
    ```

    Add host key and host certificate to the SSH configuration file.

    ```text
    # /etc/ssh/sshd_config
    # ...

    # For client keys
    TrustedUserCAKeys /etc/ssh/trusted-user-ca-keys.pem

    # For host keys
    HostKey /etc/ssh/ssh_host_rsa_key
    HostCertificate /etc/ssh/ssh_host_rsa_key-cert.pub
    ```

    Restart the SSH service to pick up the changes.

### Client-Side host verification

1. Retrieve the host signing CA public key to validate the host signature of
    target machines.

    ```text
    curl http://127.0.0.1:8200/v1/ssh-host-signer/public_key
    ```

    ```text
    d8 stronghold read -field=public_key ssh-host-signer/config/ca
    ```

1. Add the resulting public key to the `known_hosts` file with authority.

    ```text
    # ~/.ssh/known_hosts
    @cert-authority *.example.com ssh-rsa AAAAB3NzaC1yc2EAAA...
    ```

1. SSH into target machines as usual.

## Troubleshooting

When initially configuring this type of key signing, enable `VERBOSE` SSH
logging to help annotate any errors in the log.

```text
# /etc/ssh/sshd_config
# ...
LogLevel VERBOSE
```

Restart SSH after making these changes.

By default, SSH logs to `/var/log/auth.log`, but so do many other things. To
extract just the SSH logs, use the following:

```shell-session
tail -f /var/log/auth.log | grep --line-buffered "sshd"
```

If you are unable to make a connection to the host, the SSH server logs may
provide guidance and insights.

### Name is not a listed principal

If the `auth.log` displays the following messages:

```text
# /var/log/auth.log
key_cert_check_authority: invalid certificate
Certificate invalid: name is not a listed principal
```

The certificate does not permit the username as a listed principal for
authenticating to the system. This is most likely due to an OpenSSH bug (see
[known issues](#known-issues) for more information). This bug does not respect
the `allowed_users` option value of "\*". Here are ways to work around this
issue:

1. Set `default_user` in the role. If you are always authenticating as the same
    user, set the `default_user` in the role to the username you are SSHing into the
    target machine:

    ```text
    $ d8 stronghold write ssh/roles/my-role -<<"EOH"
    {
      "default_user": "YOUR_USER",
      // ...
    }
    EOH
    ```

1. Set `valid_principals` during signing. In situations where multiple users may
    be authenticating to SSH vian Stronghold, set the list of valid principles during key
    signing to include the current username:

    ```text
    $ d8 stronghold write ssh-client-signer/sign/my-role -<<"EOH"
    {
      "valid_principals": "my-user"
      // ...
    }
    EOH
    ```

### No prompt after login

If you do not see a prompt after authenticating to the host machine, the signed
certificate may not have the `permit-pty` extension. There are two ways to add
this extension to the signed certificate.

- As part of the role creation

  ```text
  $ d8 stronghold write ssh-client-signer/roles/my-role -<<"EOH"
  {
    "default_extensions": {
      "permit-pty": ""
    }
    // ...
  }
  EOH
  ```

- As part of the signing operation itself:

  ```text
  $ d8 stronghold write ssh-client-signer/sign/my-role -<<"EOH"
  {
    "extensions": {
      "permit-pty": ""
    }
    // ...
  }
  EOH
  ```

### No port forwarding

If port forwarding from the guest to the host is not working, the signed
certificate may not have the `permit-port-forwarding` extension. Add the
extension as part of the role creation or signing process to enable port
forwarding. See [no prompt after login](#no-prompt-after-login) for examples.

```json
{
  "default_extensions": {
    "permit-port-forwarding": ""
  }
}
```

### No x11 forwarding

If X11 forwarding from the guest to the host is not working, the signed
certificate may not have the `permit-X11-forwarding` extension. Add the
extension as part of the role creation or signing process to enable X11
forwarding. See [no prompt after login](#no-prompt-after-login) for examples.

```json
{
  "default_extensions": {
    "permit-X11-forwarding": ""
  }
}
```

### No agent forwarding

If agent forwarding from the guest to the host is not working, the signed
certificate may not have the `permit-agent-forwarding` extension. Add the
extension as part of the role creation or signing process to enable agent
forwarding. See [no prompt after login](#no-prompt-after-login) for examples.

```json
{
  "default_extensions": {
    "permit-agent-forwarding": ""
  }
}
```

### Key comments

There are additional steps needed to preserve [comment attributes](https://www.rfc-editor.org/rfc/rfc4716#section-3.3.2)
in keys which ought to be considered if they are required. Private and public
key may have comments applied to them and for example where `ssh-keygen` is used
with its `-C` parameter - similar to:

```shell-session
ssh-keygen -C "...Comments" -N "" -t rsa -b 4096 -f host-ca
```

Adapted key values containing comments must be provided with the key related
parameters as per the Stronghold CLI and API steps demonstrated below.

```shell-extension
# Using CLI:
d8 stronghold secrets enable -path=hosts-ca ssh
KEY_PRI=$(cat ~/.ssh/id_rsa | sed -z 's/\n/\\n/g')
KEY_PUB=$(cat ~/.ssh/id_rsa.pub | sed -z 's/\n/\\n/g')
# Create / update keypair in Stronghold
d8 stronghold write ssh-client-signer/config/ca \
  generate_signing_key=false \
  private_key="${KEY_PRI}" \
  public_key="${KEY_PUB}"
```

```shell-extension
# Using API:
curl -X POST -H "X-Vault-Token: ..." -d '{"type":"ssh"}' http://127.0.0.1:8200/v1/sys/mounts/hosts-ca
KEY_PRI=$(cat ~/.ssh/id_rsa | sed -z 's/\n/\\n/g')
KEY_PUB=$(cat ~/.ssh/id_rsa.pub | sed -z 's/\n/\\n/g')
tee payload.json <<EOF
{
  "generate_signing_key" : false,
  "private_key"          : "${KEY_PRI}",
  "public_key"           : "${KEY_PUB}"
}
EOF
# Create / update keypair in Stronghold
curl -X POST -H "X-Vault-Token: ..." -d @payload.json http://127.0.0.1:8200/v1/hosts-ca/config/ca
```

:::warning

**IMPORTANT:** Do NOT add a private key password since Stronghold can't decrypt it.
Destroy the keypair and `payload.json` from your hosts immediately after they have been confirmed as successfully uploaded.

:::

### Known issues

- On SELinux-enforcing systems, you may need to adjust related types so that the
  SSH daemon is able to read it. For example, adjust the signed host certificate
  to be an `sshd_key_t` type.

- On some versions of SSH, you may get the following error:

  ```text
  no separate private key for certificate
  ```

  This is a bug introduced in OpenSSH version 7.2 and fixed in 7.5. See
  [OpenSSH bug 2617](https://bugzilla.mindrot.org/show_bug.cgi?id=2617) for
  details.

- On some versions of SSH, you may get the following error on target host:

  ```text
  userauth_pubkey: certificate signature algorithm ssh-rsa: signature algorithm not supported [preauth]
  ```

  Fix is to add below line to /etc/ssh/sshd_config

  ```text
  CASignatureAlgorithms ^ssh-rsa
  ```

  The ssh-rsa algorithm is no longer supported in [OpenSSH 8.2](https://www.openssh.com/txt/release-8.2)

## API

The SSH secrets engine has a full HTTP API. Please see the
[SSH secrets engine API](/api-docs/secret/ssh) for more
details.
