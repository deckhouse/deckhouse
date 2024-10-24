---
title: "The static-routing-manager module: examples"
---

## Создание многоконтейнерного пода с единой проверкой работоспособности

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: multi-container-pod
  namespace: test
  labels:
    app: my-application
spec:
  containers:
    - name: postgres
      image: postgres:13
      env:
        - name: POSTGRES_USER
          value: postgres
        - name: POSTGRES_PASSWORD
          value: example
      ports:
        - containerPort: 5432
          name: postgres

    - name: nginx
      image: nginx:latest
      ports:
        - containerPort: 80
          name: nginx

    - name: node-app
      image: node:14
      command: ["node", "/app/server.js", "-port=8030"]
      ports:
        - containerPort: 8030
          name: app
      volumeMounts:
        - name: app-code
          mountPath: /app

  volumes:
    - name: app-code
      configMap:
        name: node-app-config
```

### создание Secret с данными для доступа к PostgreSQL

```shell
kubectl -n test create secret generic cred-secret --from-literal=user=postgres --from-literal=password=example cred-secret
```

### развертывание в кластер ServiceWithHealthchecks

```yaml
apiVersion: network.deckhouse.io/v1alpha1
kind: ServiceWithHealthchecks
metadata:
  name: nodejs-app
spec:
  internalTrafficPolicy: Cluster
  ipFamilies:
  - IPv4
  ipFamilyPolicy: SingleStack
  ports:
  - port: 8030
    protocol: TCP
    targetPort: 8030
  selector:
    app: my-application
  healthcheck:
    initialDelaySeconds: 3
    periodSeconds: 5
    probes:
    - mode: HTTP
      timeoutSeconds: 1
      successThreshold: 1
      failureThreshold: 3
      http:
        targetPort: 8030
        method: GET
    - mode: PostgreSQL
      timeoutSeconds: 1
      successThreshold: 1
      failureThreshold: 3
      postgreSQL:
        targetPort: 5432
        dbName: postgres
        authSecretName: cred-secret
```

Согласно этому ресурсу, будут запущены проверки, и если результат будет успешным, трафик будет направлен на Pod.

```shell
$ kubectl -n test get servicewithhealthchecks.network.deckhouse.io nodejs-app -o jsonpath={.status.conditions[0]}
...
{
  "lastTransitionTime": "2024-10-24T12:56:20Z",
  "message": "All endpoints are ready",
  "reason": "AllEndpointsAreReady",
  "status": "True",
  "type": "Ready"
}
...
```
