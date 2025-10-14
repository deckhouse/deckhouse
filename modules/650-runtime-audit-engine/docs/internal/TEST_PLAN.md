# Testing plan for the new version of Falco

If we have updated the Falco version, it is necessary to test its performance and correctness.

## Preliminary preparation

- Create a secret:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: audit-policy
  namespace: kube-system
data:
  audit-policy.yaml: YXBpVmVyc2lvbjogYXVkaXQuazhzLmlvL3YxCmtpbmQ6IFBvbGljeQpydWxlczoKLSBsZXZlbDogTWV0YWRhdGEKICBvbWl0U3RhZ2VzOgogIC0gUmVxdWVzdFJlY2VpdmVkCgo=
```

- Create a ModuleConfig:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: control-plane-manager
spec:
  version: 2
  settings:
    apiserver:
      auditPolicyEnabled: true
```

As a result of applying these two resources, AuditPolicy will be configured for all resources for all namespaces:

```yaml
apiVersion: audit.k8s.io/v1
kind: Policy
rules:
- level: Metadata
  omitStages:
  - RequestReceived
```

## Test 1

We determine the accesses to the node via ssh.

- Log in to the master node via ssh.
- Look the logs:

```shell
kubectl -n d8-runtime-audit-engine logs daemonsets/runtime-audit-engine -f
```

- The logs should contain an entry like:

```json
{
  "hostname":"romanenko-master-0",
  "output":"11:18:41.382570987: Notice Inbound SSH Connection (command=sshd pid=1298 connection=185.125.115.231:63352->10.10.0.10:22 user=root user_loginuid=-1 type=accept)",
  "output_fields":{"evt.time":1749208721382570987,"evt.type":"accept","fd.name":"185.125.115.231:63352->10.10.0.10:22","proc.cmdline":"sshd","proc.pid":1298,"user.loginuid":-1,"user.name":"root"},
  "priority":"Notice",
  "rule":"Inbound SSH Connection",
  "source":"syscall",
  "tags":["auth_attempts","fstec"],
  "time":"2025-06-06T11:18:41.382570987Z"
}
```

## Test 2

We determine the fact of exec into the pod.

- Deploy a FalcoAuditRule:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: FalcoAuditRules
metadata:
  name: run-shell-in-container
spec:
  rules:
  - macro:
      name: container
      condition: container.id != host
  - macro:
      name: spawned_process
      condition: evt.type = execve and evt.dir=<
  - rule:
      name: run_shell_in_container
      desc: a shell was spawned by a non-shell program in a container. Container entrypoints are excluded.
      condition: container and proc.name = bash and spawned_process and proc.pname exists and not proc.pname in (bash, docker)
      output: "Shell spawned in a container other than entrypoint (user=%user.name container_id=%container.id container_name=%container.name shell=%proc.name parent=%proc.pname cmdline=%proc.cmdline)"
      priority: Warning      
```

- Deploy a pod:

```shell
kubectl run --image nginx nginx
```

- Exec to the pod:

```shell
kubectl exec -ti nginx -- bash
```

- Look the logs:

```shell
node=$(kubectl get pods nginx -o json | jq -r .spec.nodeName)
pod=$(kubectl -n d8-runtime-audit-engine get pods -o wide | grep $node | awk '{print $1}')
kubectl -n d8-runtime-audit-engine logs $pod -f
```

- The logs should contain an entry like

```json
{"hostname":"romanenko-worker-f05368e7-4m2dt-wkcj4",
  "output":"11:23:11.855188321: Warning Shell spawned in a container other than entrypoint (user=root container_id=998306071edc container_name=nginx shell=bash parent=runc cmdline=bash)",
  "output_fields":{"container.id":"998306071edc","container.name":"nginx","evt.time":1749208991855188321,"proc.cmdline":"bash","proc.name":"bash","proc.pname":"runc","user.name":"root"},
  "priority":"Warning",
  "rule":"run_shell_in_container",
  "source":"syscall",
  "tags":[],
  "time":"2025-06-06T11:23:11.855188321Z"
}
```
