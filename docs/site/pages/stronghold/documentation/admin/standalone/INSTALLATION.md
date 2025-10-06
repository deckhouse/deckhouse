---
title: "Installation"
permalink: en/stronghold/documentation/admin/standalone/installation.html
---

Stronghold supports a multi-server mode for high availability (`HA`). This mode is automatically enabled when using a storage backend that supports it, and it protects the system from failures by utilizing multiple Stronghold servers.

How to determine if your storage backend supports high availability?
Run the server and check if the message `HA available` is displayed next to the storage information. If yes, Stronghold will automatically use the HA mode.

To ensure high availability, one of the Stronghold nodes acquires a lock in the storage system. This node then becomes active, while the remaining nodes enter standby mode. If the standby nodes receive requests, they either redirect them or forward the clients according to the settings and the current state of the cluster.

To deploy Stronghold in HA mode with the integrated Raft storage backend, you will need at least three Stronghold servers. Without this, you will not be able to achieve quorum and unseal the storage.

Prerequisites:

* A supported OS (Ubuntu, RedOS, Astra Linux) is installed on the server.
* The Stronghold distribution has been copied to the server.
* A `systemd-unit` has been created.
* Certificates for each node in the Raft cluster, as well as the root CA certificate, are available.

## Infrastructure preparation

The following scenario describes the process of setting up a Stronghold cluster consisting of three nodes — one active and two standby.

### Starting with systemd-unit

{% alert level="warning" %}
All examples assume that there is a `stronghold` user and the service runs under it. If you want to run the service under another user, replace the username with the desired one.
{% endalert %}

Create the file `/etc/systemd/system/stronghold.service`:

```hcl
[Unit]
Description=Stronghold service
Documentation=https://deckhouse.ru/products/stronghold/
After=network.target

[Service]
Type=simple
ExecStart=/opt/stronghold/stronghold server -config=/opt/stronghold/config.hcl
ExecReload=/bin/kill -HUP $MAINPID
KillMode=process
Restart=on-failure
RestartSec=5
User=stronghold
Group=stronghold
LimitNOFILE=65536
CapabilityBoundingSet=CAP_IPC_LOCK
AmbientCapabilities=CAP_IPC_LOCK
SecureBits=noroot

[Install]
WantedBy=multi-user.target
```

Run the `systemctl daemon-reload` command.

Enable the service to start automatically with `systemctl enable stronghold.service`.

Create the directory `/opt/stronghold/data` and set the appropriate permissions:

```shell
mkdir -p /opt/stronghold/data
chown stronghold:stronghold /opt/stronghold/data
chmod 0700 /opt/stronghold/data
```

### Preparing the required certificates

For TLS setup, the following set of certificates and keys should be placed in the `/opt/stronghold/tls` directory.

- The root certificate authority (CA) certificate that signed the Stronghold TLS certificate. In this scenario, its name is `stronghold-ca.pem`.
- Raft node certificates. In the current scenario, three nodes will be added to the cluster, and the following certificates will be created:
  - node-1-cert.pem
  - node-2-cert.pem
  - node-3-cert.pem
- Private keys for the node certificates:
  - node-1-key.pem
  - node-2-key.pem
  - node-3-key.pem

In this example, we will create a root certificate and a set of self-signed certificates for each node.

Although self-signed certificates are suitable for experimentation with deployment and running Stronghold, we highly recommend using certificates created and signed by an appropriate certificate authority.

### Steps

On the first node, navigate to the `/opt/stronghold/tls/` directory. If the directory doesn't exist yet, create it:

```shell
mkdir -p /opt/stronghold/tls
cd /opt/stronghold/tls/

```shell
mkdir -p /opt/stronghold/tls
cd /opt/stronghold/tls/
```

Generate the key for the root certificate:

```shell
openssl genrsa 2048 > stronghold-ca-key.pem
```

Issue the root certificate:

```console
openssl req -new -x509 -nodes -days 3650 -key stronghold-ca-key.pem -out stronghold-ca.pem

Country Name (2 letter code) [XX]:RU
State or Province Name (full name) []:
Locality Name (eg, city) [Default City]:Moscow
Organization Name (eg, company) [Default Company Ltd]:MyOrg
Organizational Unit Name (eg, section) []:
Common Name (eg, your name or your server hostname) []:demo.tld
```

The certificate attributes are provided as an example. To issue node certificates, create configuration files that include `subjectAltName` (SAN). For example, the file for the raft-node-1 node will look like this:

```shell
cat << EOF > node-1.cnf
[v3_ca]
subjectAltName = @alt_names
[alt_names]
DNS.1 = raft-node-1.demo.tld
IP.1 = 10.20.30.10
IP.2 = 127.0.0.1
EOF
```

Each node must have a correct FQDN and IP address. The subjectAltName field in the certificate should contain the corresponding values for the specific node.

You will also need to create a configuration file for each node you plan to add to the cluster.

For each node, generate the certificate signing request (CSR) file:

```shell
openssl req -newkey rsa:2048 -nodes -keyout node-1-key.pem -out node-1-csr.pem -subj "/CN=raft-node-1.demo.tld"
openssl req -newkey rsa:2048 -nodes -keyout node-2-key.pem -out node-2-csr.pem -subj "/CN=raft-node-2.demo.tld"
openssl req -newkey rsa:2048 -nodes -keyout node-3-key.pem -out node-3-csr.pem -subj "/CN=raft-node-3.demo.tld"
```

Issue the certificates based on the requests:

```shell
openssl x509 -req -set_serial 01 -days 3650 -in node-1-csr.pem -out node-1-cert.pem -CA stronghold-ca.pem -CAkey stronghold-ca-key.pem -extensions v3_ca -extfile ./node-1.cnf
openssl x509 -req -set_serial 01 -days 3650 -in node-2-csr.pem -out node-2-cert.pem -CA stronghold-ca.pem -CAkey stronghold-ca-key.pem -extensions v3_ca -extfile ./node-2.cnf
openssl x509 -req -set_serial 01 -days 3650 -in node-3-csr.pem -out node-3-cert.pem -CA stronghold-ca.pem -CAkey stronghold-ca-key.pem -extensions v3_ca -extfile ./node-3.cnf
```

To automatically connect the nodes, copy the following to each of them:

- The certificate file for this node.
- The key file for this node.
- The root certificate file.

For example:

```shell
scp ./node-2-key.pem ./node-2-cert.pem ./stronghold-ca.pem  raft-node-2.demo.tld:/opt/stronghold/tls
scp ./node-3-key.pem ./node-3-cert.pem ./stronghold-ca.pem  raft-node-3.demo.tld:/opt/stronghold/tls
```

If the `/opt/stronghold/tls` directory does not exist on the target nodes, create it.

## Deploying a Raft cluster

Connect to the first server where the Stronghold cluster initialization will take place.

Add the necessary firewall rules for TCP ports 8200 and 8201. Here's an example for `firewalld`:

```console
firewall-cmd --add-port=8200/tcp --permanent
firewall-cmd --add-port=8201/tcp --permanent
firewall-cmd --reload
```

You can use any other ports by specifying them in the configuration file `/opt/stronghold/config.hcl`.

Create the file `/opt/stronghold/config.hcl` for the Raft configuration. If the `/etc/stronghold/` directory does not exist, create it. Add the following content to the file, replacing the values of the parameters with your own:

```hcl
ui = true
cluster_addr  = "https://10.20.30.10:8201"
api_addr      = "https://10.20.30.10:8200"
disable_mlock = true

listener "tcp" {
  address       = "0.0.0.0:8200"
  tls_cert_file      = "/opt/stronghold/tls/node-1-cert.pem"
  tls_key_file       = "/opt/stronghold/tls/node-1-key.pem"
}

storage "raft" {
  path = "/opt/stronghold/data"
  node_id = "raft-node-1"

  retry_join {
    leader_tls_servername   = "raft-node-1.demo.tld"
    leader_api_addr         = "https://10.20.30.10:8200"
    leader_ca_cert_file     = "/opt/stronghold/tls/stronghold-ca.pem"
    leader_client_cert_file = "/opt/stronghold/tls/node-1-cert.pem"
    leader_client_key_file  = "/opt/stronghold/tls/node-1-key.pem"
  }
  retry_join {
    leader_tls_servername   = "raft-node-2.demo.tld"
    leader_api_addr         = "https://10.20.30.11:8200"
    leader_ca_cert_file     = "/opt/stronghold/tls/stronghold-ca.pem"
    leader_client_cert_file = "/opt/stronghold/tls/node-1-cert.pem"
    leader_client_key_file  = "/opt/stronghold/tls/node-1-key.pem"
  }
  retry_join {
    leader_tls_servername   = "raft-node-3.demo.tld"
    leader_api_addr         = "https://10.20.30.12:8200"
    leader_ca_cert_file     = "/opt/stronghold/tls/stronghold-ca.pem"
    leader_client_cert_file = "/opt/stronghold/tls/node-1-cert.pem"
    leader_client_key_file  = "/opt/stronghold/tls/node-1-key.pem"
  }
}
```

Start the service:

```shell
systemctl start stronghold
```

Initialize the cluster:

```shell
stronghold operator init -ca-cert /opt/stronghold/tls/stronghold-ca.pem
```

You can pass the `-key-shares` and `-key-threshold` parameters to specify how many parts the key will be split into and how many of them will be required to unseal the storage. By default, `key-shares=5` and `key-threshold=3`.

{% alert level="warning" %}
After the initialization is complete, all key parts and the root token will be displayed in the terminal. Be sure to save this information in a secure location. The key parts and initial root token are extremely important. If you lose part of the key, you will not be able to access Stronghold's data.
{% endalert %}

Next, you need to unseal the cluster. To do this, execute the following command the necessary number of times:

```shell
stronghold operator unseal -ca-cert /opt/stronghold/tls/stronghold-ca.pem
```

Enter the unseal keys that were obtained in the previous step. If you did not modify the `-key-threshold` parameter, you will need to enter 3 key parts.

Repeat the setup on the other nodes in the cluster. For this, specify the corresponding IP addresses of the nodes in the `cluster_addr` and `api_addr` parameters in the `/opt/stronghold/config.hcl` file. Skip the initialization step and proceed directly to unsealing the cluster.

Now, you just need to verify the cluster's operation:

```console
stronghold status -ca-cert /opt/stronghold/tls/stronghold-ca.pem
Key                     Value
---                     -----
Seal Type               shamir
Initialized             true
Sealed                  false
Total Shares            5
Threshold               3
Version                 1.15.2
Build Date              2025-03-07T16:10:46Z
Storage Type            raft
Cluster Name            stronghold-cluster-a3fcc270
Cluster ID              f682968d-5e6c-9ad4-8303-5aecb259ca0b
HA Enabled              true
HA Cluster              https://10.20.30.10:8201
HA Mode                 active
Active Node Address     https://10.20.30.10:8200
Raft Committed Index    40
Raft Applied Index      40
```
