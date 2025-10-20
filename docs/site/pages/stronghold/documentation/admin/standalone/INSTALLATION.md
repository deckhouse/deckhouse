---
title: "Installation"
permalink: en/stronghold/documentation/admin/standalone/installation.html
---

Stronghold supports multi-server mode to ensure high availability (HA). This mode is automatically enabled when using a compatible data storage backend and protects the system from failures by running multiple Stronghold servers.

To check if your storage supports high availability mode, start the server and make sure the message `HA available` is displayed next to the storage information. In this case, Stronghold will automatically use HA mode.

To provide high availability, one of the Stronghold nodes acquires a lock in the storage system and becomes active, while the other nodes switch to standby mode. If standby nodes receive requests, they either forward them or redirect clients according to the configuration and the current state of the cluster.

To run Stronghold in high availability (HA) mode with the integrated Raft storage, you need at least three Stronghold servers. This requirement is necessary to achieve quorum — without it, the cluster cannot operate with the storage.

Prerequisites:

* A supported OS is installed on the server (Ubuntu, RedOS, Astra Linux).
* The Stronghold distribution is copied to the server.
* A systemd unit is created to manage the service.
* Individual certificates are issued for each node in the Raft cluster.
* A root certificate authority (CA) certificate is prepared.

## Infrastructure preparation

The following scenario describes the deployment of a Stronghold cluster consisting of three nodes: one active and two standby. Such a cluster provides high availability (HA).

### Launch via systemd unit

{% alert level="warning" %}
All examples assume that a system user `stronghold` has been created and the service runs under this user.
If you need to use another user, replace `stronghold` with the appropriate name.
{% endalert %}

1. Create the file `/etc/systemd/system/stronghold.service` with the following content:

   ```console
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

1. Apply the systemd configuration changes:

   ```shell
   systemctl daemon-reload
   ```

1. Enable the service to start automatically:

   ```shell
   systemctl enable stronghold.service
   ```

1. Create the `/opt/stronghold/data` directory and set the appropriate permissions:

   ```shell
   mkdir -p /opt/stronghold/data
   chown stronghold:stronghold /opt/stronghold/data
   chmod 0700 /opt/stronghold/data
   ```

### Preparing the required certificates

To configure TLS, you need a set of certificates and keys that must be placed in the `/opt/stronghold/tls` directory:

- Root certificate authority (CA).  
  `stronghold-ca.pem` — the certificate used to sign Stronghold TLS certificates.
- Raft node certificates. In this scenario, the cluster will include three nodes, for which the following certificates will be created:
  - `node-1-cert.pem`;
  - `node-2-cert.pem`;
  - `node-3-cert.pem`.
- Private keys of the node certificates:
  - `node-1-key.pem`;
  - `node-2-key.pem`;
  - `node-3-key.pem`.

In this example, a root certificate and a set of self-signed certificates for each node will be created.

{% alert level="warning" %}
Self-signed certificates are suitable only for testing and experimentation.  
For production use, it is strongly recommended to use certificates issued and signed by a trusted certificate authority (CA).
{% endalert %}

### Procedure

1. On the first node, create a directory for storing certificates (if it does not already exist) and switch to it:

   ```shell
   mkdir -p /opt/stronghold/tls
   cd /opt/stronghold/tls/
   ```

1. Generate a key for the root certificate:

   openssl genrsa 2048 > stronghold-ca-key.pem

1. Issue the root certificate:

   ```console
   openssl req -new -x509 -nodes -days 3650 -key stronghold-ca-key.pem -out stronghold-ca.pem

   Country Name (2 letter code) [XX]:RU
   Locality Name (eg, city) [Default City]:Moscow
   Organization Name (eg, company) [Default Company Ltd]:MyOrg
   Common Name (eg, your name or your server hostname) []:demo.tld
   ```

   > The certificate attributes are provided as an example.

1. To issue node certificates, create configuration files that contain the `subjectAltName` (SAN) parameter.  
   For example, for the `raft-node-1` node:

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

   Each node must have valid FQDN and IP addresses.  
   The `subjectAltName` field in the certificate must contain values relevant to the specific node.  
   Similarly, create a separate configuration file for each node.

1. Generate certificate signing requests (CSRs) and keys for the nodes:

   ```shell
   openssl req -newkey rsa:2048 -nodes -keyout node-1-key.pem -out node-1-csr.pem -subj "/CN=raft-node-1.demo.tld"
   openssl req -newkey rsa:2048 -nodes -keyout node-2-key.pem -out node-2-csr.pem -subj "/CN=raft-node-2.demo.tld"
   openssl req -newkey rsa:2048 -nodes -keyout node-3-key.pem -out node-3-csr.pem -subj "/CN=raft-node-3.demo.tld"
   ```

1. Issue certificates based on the created CSRs:

   ```shell
   openssl x509 -req -set_serial 01 -days 3650 -in node-1-csr.pem -out node-1-cert.pem -CA stronghold-ca.pem -CAkey stronghold-ca-key.pem -extensions v3_ca -extfile ./node-1.cnf
   openssl x509 -req -set_serial 02 -days 3650 -in node-2-csr.pem -out node-2-cert.pem -CA stronghold-ca.pem -CAkey stronghold-ca-key.pem -extensions v3_ca -extfile ./node-2.cnf
   openssl x509 -req -set_serial 03 -days 3650 -in node-3-csr.pem -out node-3-cert.pem -CA stronghold-ca.pem -CAkey stronghold-ca-key.pem -extensions v3_ca -extfile ./node-3.cnf
   ```

   > It is recommended to use unique `-set_serial` values for each certificate.

1. Copy the required files to each node:

   - Node certificate;
   - Node private key;
   - Root certificate.

   For example, for the `raft-node-2` and `raft-node-3` nodes:

   ```shell
   scp ./node-2-key.pem ./node-2-cert.pem ./stronghold-ca.pem raft-node-2.demo.tld:/opt/stronghold/tls
   scp ./node-3-key.pem ./node-3-cert.pem ./stronghold-ca.pem raft-node-3.demo.tld:/opt/stronghold/tls
   ```

   > If the `/opt/stronghold/tls` directory does not exist on the target nodes, create it.

## Deploying a Raft cluster

1. Connect to the first server where the Stronghold cluster initialization will be performed.

1. Allow network connections for TCP ports 8200 and 8201. Example for firewalld:

   ```shell
   firewall-cmd --add-port=8200/tcp --permanent
   firewall-cmd --add-port=8201/tcp --permanent
   firewall-cmd --reload
   ```

   > If necessary, you can use other ports by specifying them in the `/opt/stronghold/config.hcl` configuration file.

1. Create the /opt/stronghold/config.hcl configuration file for Raft. If the `/etc/stronghold/` directory does not exist, create it:

   ```console
   ui = true
   cluster_addr  = "https://10.20.30.10:8201"
   api_addr      = "https://10.20.30.10:8200"
   disable_mlock = true

   listener "tcp" {
     address            = "0.0.0.0:8200"
     tls_cert_file      = "/opt/stronghold/tls/node-1-cert.pem"
     tls_key_file       = "/opt/stronghold/tls/node-1-key.pem"
   }

   storage "raft" {
     path    = "/opt/stronghold/data"
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

1. Start the Stronghold service:

   ```shell
   systemctl start stronghold
   ```

1. Initialize the cluster:

   ```shell
   stronghold operator init -ca-cert /opt/stronghold/tls/stronghold-ca.pem
   ```

   If necessary, you can specify the following parameters:

   - `-key-shares` — number of key shares (default: 5);
   - `-key-threshold` — minimum number of shares required to unseal the storage (default: 3).

     > After initialization, all key shares and the root token will be displayed in the terminal.
     > Be sure to save them in a secure place.
     > Without the required number of key shares, access to Stronghold data will be impossible.

1. Unseal the cluster. Run the command multiple times, entering the unseal keys:

   ```shell
   stronghold operator unseal -ca-cert /opt/stronghold/tls/stronghold-ca.pem
   ```

   > If the `-key-threshold` parameter was not changed, you need to enter 3 key shares.

1. Configure the remaining nodes:

   - Set the appropriate cluster_addr and api_addr values in `/opt/stronghold/config.hcl`.
   - Skip the initialization step.
   - Immediately proceed to unsealing the cluster (operator unseal).

1. Verify the cluster status:

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
