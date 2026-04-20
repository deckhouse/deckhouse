---
title: "Module service-with-healthchecks: examples"
description: "Configuring a Load Balancer with the service-with-healthchecks Module in Deckhouse Kubernetes Platform"
---

{% alert level="info" %}

For the ServiceWithHealthchecks load balancers you create to work, the following conditions must be met:

* The network policy of the custom project in which the ServiceWithHealthchecks will be created must include a rule allowing incoming traffic from all pods in the `d8-service-with-healthchecks` namespace:
  
  ```yaml
  ingress:
  - from:
    - namespaceSelector:
        matchLabels:
          kubernetes.io/metadata.name: d8-service-with-healthchecks
  ```

  For more information on network policies, see the [Configuring Network Policies](/products/kubernetes-platform/documentation/v1/admin/configuration/network/policy/configuration.html) section.

* The cluster role used in ClusterRoleBinding and RoleBinding when assigning permissions to users and service accounts for the ServiceWithHealthchecks resource must be extended with the following rules:

  * `get`
  * `list`
  * `watch`
  * `create`
  * `update`
  * `patch`
  * `delete`.

  For more details, see the section [Granting permissions to users and service accounts](/products/kubernetes-platform/documentation/latest/admin/configuration/access/authorization/granting.html).

{% endalert %}

{% alert level="warning" %}
Enabling the module does not automatically replace existing Service resources with ServiceWithHealthcheck resources. To replace existing services with ServiceWithHealthcheck, follow these steps:

* Create ServiceWithHealthcheck resources with the same names and parameters as the existing Service resources you want to replace. When creating a ServiceWithHealthcheck, specify the required [`healthchecks`](cr.html#servicewithhealthchecks-v1alpha1-spec-healthcheck) parameters.
* Delete the Service resources that you want to replace with ServiceWithHealthcheck.
{% endalert %}

## Running two independent balancers on the same virtual machine

Suppose that there are two applications running on a Linux virtual machine — an HTTP server (TCP 8080) and an SMTP server (TCP 2525). You need to set up two separate balancers for these services, a web balancer and an SMTP balancer.

### Creating a virtual machine

Create a `my-vm` virtual machine by following the examples in the [DVP documentation](https://deckhouse.io/products/virtualization-platform/documentation/user/resource-management/virtual-machines.html).

In the manifest example below, the `vm: my-vm` label is included so that the virtual machine can be bound to load balancers.

```yaml
apiVersion: virtualization.deckhouse.io/v1alpha2
kind: VirtualMachine
metadata:
  name: my-vm
  namespace: my-ns
  labels:
    vm: my-vm
spec:
  virtualMachineClassName: generic
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

### Web service and SMTP load balancer manifests

Below is an example of a manifest of a web service load balancer:

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

Below is an example of a manifest of a SMTP load balancer:

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

## Load balancers for working with a PostgreSQL cluster

### Creating a StatefulSet for PostgreSQL

In order for `StatefulSet` to operate properly, you will have to create a regular Service to generate the pod DNS names. This service will not be used for direct access to the database.

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

Below is an example of a `StatefulSet` manifest:

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

### Configuring ServiceWithHealthchecks load balancers

Create a Secret to store credentials so that probes can access the database:

```shell
d8 k -n my-ns create secret generic cred-secret --from-literal=user=postgres --from-literal=password=example cred-secret
```

Below is an example of a load balancer manifest for reading:

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

And here is an example of a load balancer manifest for writing:

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
