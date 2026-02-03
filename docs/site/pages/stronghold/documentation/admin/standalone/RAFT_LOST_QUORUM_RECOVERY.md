---
title: "Recover from lost quorum"
permalink: en/stronghold/documentation/admin/standalone/raft-lost-quorum-recovery.html
---

Quorum is the minimum number of nodes in a cluster required to vote and elect a leader.
A Raft leader is the active cluster node that performs read and write operations and coordinates the other nodes.

With Integrated Storage, maintaining Raft quorum is an important factor when configuring and operating a Stronghold environment with HA enabled.
A Stronghold cluster permanently loses quorum when there is no way to recover enough servers to reach consensus and elect a leader. Without a quorum of cluster servers, Stronghold can no longer perform read and write operations.

The cluster quorum is dynamically updated when new servers join the cluster. Stronghold calculates quorum with the formula `(n+1)/2`, where `n` is the number of servers in the cluster. For example, for a 3-server cluster, you will need at least 2 servers operational for the cluster to function properly, `(3+1)/2 = 2`. Specifically, you will need 2 servers always active to perform read and write operations.

{% alert level="info" %}
There is an exception to this rule if you use the `-non-voter` option while joining the cluster. This feature is available only in Stronghold as a standalone.
{% endalert %}

## Quorum loss scenario

When two of the three servers become unavailable, the cluster loses quorum and becomes inoperable.

Although one of the servers is fully functioning, the cluster won't be able to process read or write requests.

**Example:**

1. Response from console:

    ```text
    $ stronghold operator raft list-peers
    * local node not active but active cluster node not found
    
    $ stronghold kv get kv/apikey
    * local node not active but active cluster node not found
    ```

1. Logs from one of inoperative nodes:

    ```text
    oct 20 10:54:32 standalone-astra stronghold[647]: {"@level":"info","@message":"attempting to join possible raft leader node","@module":"core","@timestamp":"2025-10-20T10:54:02.578963Z","leader_addr":"https://stronghold-0.stronghold.tld:8201"}
    oct 20 10:54:32 standalone-astra stronghold[647]: {"@level":"error","@message":"failed to get raft challenge","@module":"core","@timestamp":"2025-10-20T10:54:32.597558Z","error":"error during raft bootstrap init call: Put \"https://10.0.101.22:8201/v1/sys/storage/raft/bootstrap/challenge\": dial tcp 10.0.101.22:8201: i/o timeout","leader_addr":"https://stronghold-0.stronghold.tld:8201"}
    ```

In this tutorial, you will recover from the permanent loss of two-of-three Stronghold servers by converting it into a single-server cluster.

The remaining server must be fully operational to complete this procedure.

In a 5-server cluster, or when non-voters are present, you must stop all other healthy servers before performing the peers.json recovery.

### Autopilot recovery considerations

Autopilot is a Stronghold mechanism that automatically monitors the state of Raft cluster nodes and manages their participation in quorum.

In some cases, Stronghold may lose quorum due to Autopilot marking servers as unhealthy, while the service is still running.
In such situations, before performing recovery using peers.json, you must stop Stronghold services on the unhealthy servers.

## Recover from lost quorum

### Locate the storage directory

On the healthy Stronghold server, locate the Raft storage directory. To discover the location of the directory, review your Stronghold configuration file. The `storage` stanza will contain the `path` to the directory.

**Example:**

`/opt/stronghold/config.hcl`

```hcl
storage "raft" {
  path    = "/opt/stronghold/data"
  server_id = "stronghold_0"
}

listener "tcp" {
  address     = "0.0.0.0:8200"
  cluster_address     = "0.0.0.0:8201"
  tls_disable = true
}

api_addr = "http://stronghold-0.stronghold.tld:8200"
cluster_addr = "http://stronghold-0.stronghold.tld:8201"
disable_mlock = true
ui=true
```

In this example, the `path` is the file system path where Stronghold stores data, and the `server_id` is the identifier for the server in the Raft cluster. The example `server_id` is `stronghold_0`.

### Create the peers.json file

Inside the storage directory (`/opt/stronghold/data`), there is a folder named `raft`.

```text
/opt
└ stronghold
  └── data
      ├── raft
      │   ├── raft.db
      │   └── snapshots
      └── vault.db
```

To enable the single, remaining Stronghold server to reach quorum and elect itself as the leader, create a `raft/peers.json` file that holds the server information. The file format is a JSON array containing the server ID, *address:port*, and suffrage information of the healthy Stronghold server.

**Example:**

```bash
$ cat > /stronghold/data/raft/peers.json << EOF
[
  {
    "id": "stronghold_0",
    "address": "stronghold-0.stronghold.tld:8201",
    "non_voter": false
  }
]
EOF
```

- **id** (string: \<required\>) - Specifies the server ID of the server.
- **address** (string: \<required\>) - Specifies the host and port of the server. The port is the server's cluster port.
- **non_voter** (bool: \<false\>) - This controls whether the server is a non-voter.

Make sure `stronghold` user has *read* and *edit* permissions on `peers.json` file.

```bash
chown stronghold:stronghold /opt/stronghold/data/raft/peers.json
chmod 600 /opt/stronghold/data/raft/peers.json
```

### Restart Stronghold

Restart the Stronghold process to enable Stronghold to load the new `peers.json` file.

```bash
sudo systemctl restart stronghold
```

{% alert level="info" %}
f you use Systemd, a `SIGHUP` signal will not work.
{% endalert %}

### Unseal Stronghold

If not configured to use auto-unseal, unseal Stronghold and then check the status.

**Example:**

```bash
$ stronghold operator unseal
Unseal Key (will be hidden):

$ stronghold status
Key                      Value
---                      -----
Recovery Seal Type       shamir
Initialized              true
Sealed                   false
Total Recovery Shares    1
Threshold                1
Version                  1.16.0+hsm
Storage Type             raft
Cluster Name             stronghold-cluster-4a1a40af
Cluster ID               d09df2c7-1d3e-f7d0-a9f7-93fadcc29110
HA Enabled               true
HA Cluster               https://stronghold-0.stronghold.tld:8201
HA Mode                  active
Active Since             2021-07-20T00:07:32.215236307Z
Raft Committed Index     155344
Raft Applied Index       155344
```

### Verify recovery success

The recovery procedure is successful when Stronghold starts up and displays these messages in the system logs.

```text
...snip...
[INFO]  core.cluster-listener: serving cluster requests: cluster_listen_address=[::]:8201
[INFO]  storage.raft: raft recovery initiated: recovery_file=peers.json
[INFO]  storage.raft: raft recovery found new config: config="{[{Voter stronghold_0 https://stronghold-0.stronghold.tld:8201}]}"
[INFO]  storage.raft: raft recovery deleted peers.json
...snip...
```

### View the peer list

You now have a cluster with one server that can reach the quorum. Verify that there is just one server in the cluster with `stronghold operator raft list-peers` command.

```bash
$ stronghold operator raft list-peers
Node            Address                                     State     Voter
----            -------                                     -----     -----
stronghold_0    https://stronghold-0.stronghold.tld:8201    leader    true
```

### Next steps

In this tutorial, you recovered the loss of quorum by converting a 3-server cluster into a single-server cluster using the `peers.json`. The `peers.json` file enabled you to manually overwrite the Raft peer list to the one remaining server, which allowed that server to reach quorum and complete a leader election.

If the failed servers are **recoverable**, the best option is to bring them back online and have them reconnect to the cluster using the same host addresses. This will return the cluster to a fully healthy state. In such an event, the `raft/peers.json` should contain the server ID, *address:port*, and suffrage information of each Stronghold server you wish to be in the cluster.

```json
[
  {
    "id": "stronghold_0",
    "address": "stronghold-0.stronghold.tld:8201",
    "non_voter": false
  },
  {
    "id": "stronghold_1",
    "address": "stronghold-1.stronghold.tld:8201",
    "non_voter": false
  },
  {
    "id": "stronghold_2",
    "address": "stronghold-2.stronghold.tld:8201",
    "non_voter": false
  }
]
```
