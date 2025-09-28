---
title: "Intra-cluster communication"
permalink: en/user/network/intra-cluster.html
---

To organize intra-cluster communication in Deckhouse Kubernetes Platform,
it is recommended to use Services instead of accessing Pods directly.
Services provide load balancing between Pods, stable network connectivity,
and DNS integration for convenient service discovery.
They also support various access scenarios and ensure isolation and security of network traffic.

If necessary, you can use either the standard Service-based load balancer
or the advanced load balancer based on the [`service-with-healthchecks`](/modules/service-with-healthchecks/) module.

## Standard load balancer

In Kubernetes, the Service resource is responsible for both internal and external request load balancing. This resource:

- Distributes requests between the application's working Pods.
- Excludes unhealthy Pods from load balancing.

Readiness probes specified in the specification of the containers belonging to a Pod are used to check
whether the Pod is able to handle incoming requests.

### Limitations of the standard Service load balancer

The standard Service load balancing mechanism is suitable for most cloud application scenarios but has two limitations:

- If at least one container in a Pod fails the readiness probe,
  the entire Pod is considered `NotReady` and is excluded from load balancing for all Services it is associated with.
- Each container can only have one probe configured,
  so you cannot set up separate probes, for example, for read and write availability checks.

Example scenarios where the standard load balancer is insufficient:

- Database:
  - Runs in three Pods: `db-0`, `db-1`, and `db-2`, each containing a single container running the database process.
  - You need to create two Services: `db-write` for writing and `db-read` for reading.
  - Read requests must be balanced across all Pods.
  - Write requests must be balanced only to the Pod designated as the master by the database.
- Virtual machine:
  - A Pod contains a single container running the `qemu` process, acting as a hypervisor for a guest virtual machine.
  - Independent processes, such as a web server and an SMTP server, run on the guest VM.
  - You need to create two Services: `web` and `smtp`, each with its own readiness probe.

### Example Service for a standard load balancer

```yaml
apiVersion: v1
kind: Service
metadata:
  name: productpage
  namespace: bookinfo
spec:
  ports:
  - name: http
    port: 9080
  selector:
    app: productpage
  type: ClusterIP
```

### Example of a standard load balancer for a PostgreSQL cluster

#### Creating a StatefulSet for PostgreSQL

For a StatefulSet to work correctly, you need to create a standard Service to generate DNS names for individual Pods.
This Service will not be used for direct database access.

```yaml
apiVersion: v1
kind: Service
metadata:
  name: postgres
spec:
  selector:
    app: postgres
  ports:
    - protocol: TCP
      port: 5432
      targetPort: 5432
```

Example StatefulSet manifest:

```yaml
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: postgres
  name: my-ns
spec:
  serviceName: postgres
  replicas: 3
  selector:
    matchLabels:
      app: postgres
  template:
    metadata:
      labels:
        app: postgres
    spec:
      containers:
        - name: postgres
          image: postgres:13
          ports:
            - containerPort: 5432
          env:
            - name: POSTGRES_USER
              value: postgres
            - name: POSTGRES_PASSWORD
              value: example
```

## Advanced load balancer

Unlike the standard load balancer where readiness probes are tied to container states,
the ServiceWithHealthcheck–based load balancer allows you to configure active probes for individual TCP ports.
This way, each load balancer serving the same Pod can operate independently from the others.

### Load balancer structure

The load balancer consists of two components:

- Controller — runs on cluster master nodes and manages ServiceWithHealthcheck resources.
- Agents — run on every cluster node and perform probes for Pods running on that node.

The ServiceWithHealthcheck load balancer is designed to be CNI-independent
while using standard Service and EndpointSlice resources:

- When a ServiceWithHealthcheck resource is created,
  the controller automatically creates a Service with the same name in the same namespace, but with an empty `selector` field.
  This prevents the standard controller from creating EndpointSlice objects, which are normally used for load balancing.
- When an agent detects Pods on its node that fall under a ServiceWithHealthcheck,
  it runs the configured probes and creates an EndpointSlice with the list of verified IP addresses and ports.
  This EndpointSlice is linked to the Service created earlier.
- The CNI matches all EndpointSlice objects with the standard Services created earlier
  and performs load balancing across the verified IP addresses and ports on all cluster nodes.

Migrating from a Service to a ServiceWithHealthchecks resource,
for example in a CI/CD pipeline, should not cause difficulties.
The ServiceWithHealthchecks specification mostly repeats the standard Service specification,
but includes an additional `healthcheck` section.
During the resource lifecycle, a Service with the same name is created in the same namespace
to route traffic to workloads in the cluster in the usual way (via `kube-proxy` or CNI).

### Configuring the load balancer

You can configure this type of load balancing
using the [ServiceWithHealthchecks](/modules/service-with-healthchecks/cr.html#servicewithhealthchecks) resource:

- Its specification is identical to a standard Service with the addition of a `healthcheck` section that contains a set of checks.
- Currently, three types of probes are supported:
  - `TCP`: A basic check using TCP connection establishment.
  - `HTTP`: Sends an HTTP request and expects a specific response code.
  - `PostgreSQL`: Sends an SQL query and expects it to complete successfully.

### Example configuration of advanced ServiceWithHealthchecks load balancers

Create a Secret to store the credentials required for database probe access:

```shell
d8 k -n my-ns create secret generic cred-secret --from-literal=user=postgres --from-literal=password=example cred-secret
```

Example load balancer manifest for reading:

```yaml
apiVersion: network.deckhouse.io/v1alpha1
kind: ServiceWithHealthchecks
metadata:
  name: postgres-read
spec:
  ports:
  - port: 5432
    protocol: TCP
    targetPort: 5432
  selector:
    app: postgres
  healthcheck:
    probes:
    - mode: PostgreSQL
      postgreSQL:
        targetPort: 5432
        dbName: postgres
        authSecretName: cred-secret
        query: "SELECT 1"
```

Example load balancer manifest for writing:

```yaml
apiVersion: network.deckhouse.io/v1alpha1
kind: ServiceWithHealthchecks
metadata:
  name: postgres-write
spec:
  ports:
  - port: 5432
    protocol: TCP
    targetPort: 5432
  selector:
    app: postgres
  healthcheck:
    probes:
    - mode: PostgreSQL
      postgreSQL:
        targetPort: 5432
        dbName: postgres
        authSecretName: cred-secret
        query: "SELECT NOT pg_is_in_recovery()"
```

## Hosting two independent load balancers on a single virtual machine

A Linux-based virtual machine runs two applications: an HTTP server (TCP 8080) and an SMTP server (TCP 2525).
You need to configure two separate load balancers for these services — a web load balancer and an SMTP load balancer.

### Creating the virtual machine

Create a `my-vm` virtual machine based on the examples in the [DVP documentation](/products/virtualization-platform/documentation/user/resource-management/virtual-machines.html).

In the following manifest example, the label `vm: my-vm` is added for further identification in the load balancers.

```yaml
apiVersion: virtualization.deckhouse.io/v1alpha2
kind: VirtualMachine
metadata:
  name: my-vm
  namespace: my-ns
  labels:
    vm: my-vm
spec:
  virtualMachineClassName: host
  cpu:
    cores: 1
  memory:
    size: 1Gi
  provisioning:
    type: UserData
    userData: |
      #cloud-config
      ssh_pwauth: True
      users:
      - name: cloud
        passwd: '$6$rounds=4096$saltsalt$fPmUsbjAuA7mnQNTajQM6ClhesyG0.yyQhvahas02ejfMAq1ykBo1RquzS0R6GgdIDlvS.kbUwDablGZKZcTP/'
        shell: /bin/bash
        sudo: ALL=(ALL) NOPASSWD:ALL
        lock_passwd: False      
  blockDeviceRefs:
    - kind: VirtualDisk
      name: linux-disk
```

### Manifests for advanced load balancers for the web and SMTP services

Example web load balancer manifest:

```yaml
apiVersion: network.deckhouse.io/v1alpha1
kind: ServiceWithHealthchecks
metadata:
  name: web
  namespace: my-ns
spec:
  ports:
  - port: 80
    protocol: TCP
    targetPort: 8080
  selector:
    vm: my-vm
  healthcheck:
    probes:
    - mode: HTTP
      http:
        targetPort: 8080
        method: GET
        path: /healthz
```

Example SMTP load balancer manifest:

```yaml
apiVersion: network.deckhouse.io/v1alpha1
kind: ServiceWithHealthchecks
metadata:
  name: smtp
  namespace: my-ns
spec:
  ports:
  - port: 25
    protocol: TCP
    targetPort: 2525
  selector:
    vm: my-vm
  healthcheck:
    probes:
    - mode: TCP
      tcp:
        targetPort: 2525
```
