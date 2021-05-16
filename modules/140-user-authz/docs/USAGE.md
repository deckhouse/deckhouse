---
title: "The user-authz module: usage"
---

## An example of `ClusterAuthorizationRule`

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ClusterAuthorizationRule
metadata:
  name: test
spec:
  subjects:
  - kind: User
    name: some@example.com
  - kind: ServiceAccount
    name: gitlab-runner-deploy
    namespace: d8-service-accounts
  - kind: Group
    name: some-group-name
  accessLevel: PrivilegedUser
  portForwarding: true
  allowAccessToSystemNamespaces: false     # This option is only available it the enableMultiTenancy parameter is set
  limitNamespaces:                         # This option is only available it the enableMultiTenancy parameter is set
  - review-.*
  - stage
```


## Creating a user

There are two types of users in Kubernetes:
* Service accounts managed by Kubernetes via the API;
* Regular users managed by some external tool that the cluster administrator configures. There are many authentication mechanisms and, accordingly, many ways to create users. Currently, two authentication methods are supported:
    * Via the [user-authn](../../modules/150-user-authn/) module.
    * Via the certificates.

When issuing the authentication certificate, you need to specify the name (`CN=<name>`), the required number of groups (`O=<group>`), and sign it using the root CA of the cluster. It is this mechanism that authenticates you in the cluster when, for example, you use kubectl on a bastion node.

### Creating a ServiceAccount and granting it access
* Create a `ServiceAccount` in the `d8-service-accounts` namespace

	An example of creating `gitlab-runner-deploy` `ServiceAccount`:
	```bash
	kubectl -n d8-service-accounts create serviceaccount gitlab-runner-deploy
	```

* Grant the necessary rights to the `ServiceAccount` (using the [ClusterAuthorizationRule](cr.html#clusterauthorizationrule) CR)

	An example:
	```bash
	kubectl create -f - <<EOF
	apiVersion: deckhouse.io/v1alpha1
	kind: ClusterAuthorizationRule
	metadata:
	 name: gitlab-runner-deploy
	spec:
	 subjects:
	 - kind: ServiceAccount
		 name: gitlab-runner-deploy
		 namespace: d8-service-accounts
	 accessLevel: SuperAdmin
	 allowAccessToSystemNamespaces: true
	EOF
	```

	If the multitenancy mode is enabled in the Deckhouse configuration, you need to specify the `allowAccessToSystemNamespaces: true` parameter to give the ServiceAccount access to the system namespaces. 

* Generate a `kube-config` (don't forget to substitute your values).

	```bash
	cluster_name=my-cluster
	user_name=gitlab-runner-deploy.my-cluster
	context_name=${cluster_name}-${user_name}
	file_name=kube.config
	```

  * The `cluster` section:
      
      * If there is direct access to the API server, then use its IP address:
          
          Get the CA of our Kubernetes cluster:
          ```bash
          cat /etc/kubernetes/kubelet.conf \
            | grep certificate-authority-data | awk '{ print $2 }' \
            | base64 -d > /tmp/ca.crt
          ```
          
          Generate a section using the API server's IP:
          ```bash
          kubectl config set-cluster $cluster_name --embed-certs=true \
            --server=https://<API_SERVER_IP>:6443 \
            --certificate-authority=/tmp/ca.crt \
            --kubeconfig=$file_name
          ```

      *  If there is no direct access to the API server, [enable](../../modules/150-user-authn/configuration.html#parameters) the `publishAPI` parameter containing the `whitelistSourceRanges` array. Or you can do that via a separate Ingress-controller using the `ingressClass` option with the finite `SourceRange`. That is, specify the requests' source addresses in the `acceptRequestsFrom` controller parameter.

          Get the CA from the secret containing the `api.%s` domain's certificate:
          ```bash
          kubectl -n d8-user-authn get secrets kubernetes-tls -o json \
            | jq -rc '.data."ca.crt" // .data."tls.crt"' \
            | base64 -d > /tmp/ca.crt
          ```

          Generate a section with the external domain:
          ```
          kubectl config set-cluster $cluster_name --embed-certs=true \
            --server=https://$(kubectl -n d8-user-authn get ing kubernetes-api -ojson | jq '.spec.rules[].host' -r) \
            --certificate-authority=/tmp/ca.crt \
            --kubeconfig=$file_name
          ```

  * Generate the `user` section using the token from the `ServiceAccount` secret:
      ```bash
      kubectl config set-credentials $user_name \
        --token=$(kubectl get secret $(kubectl get sa gitlab-runner-deploy -n d8-service-accounts  -o json | jq -r .secrets[].name) -n d8-service-accounts -o json |jq -r '.data["token"]' | base64 -d) \
        --kubeconfig=$file_name
      ```

  * Generate the `context` to bind it all together:
      ```bash
      kubectl config set-context $context_name \
        --cluster=$cluster_name --user=$user_name \
        --kubeconfig=$file_name
      ```

### How to create a user using a client certificate
#### Creating a user

* Get the cluster's root certificate (ca.crt and ca.key).
* Generate the user key:

	```shell
	openssl genrsa -out myuser.key 2048
	```

* Create a CSR file and specify in it the username (`myuser`) and groups to which this user belongs (`mygroup1` & `mygroup2`):

	```shell
	openssl req -new -key myuser.key -out myuser.csr -subj "/CN=myuser/O=mygroup1/O=mygroup2"
	```

* Sign the CSR using the cluster root certificate:

	```shell
	openssl x509 -req -in myuser.csr -CA ca.crt -CAkey ca.key -CAcreateserial -out myuser.crt -days 10000
	```

* Now you can use the certificate issued in the config file:

	```shell
	cat << EOF
	apiVersion: v1
	clusters:
	- cluster:
			certificate-authority-data: $(cat ca.crt | base64 -w0)
			server: https://<хост кластера>:6443
		name: kubernetes
	contexts:
	- context:
			cluster: kubernetes
			user: myuser
		name: myuser@kubernetes
	current-context: myuser@kubernetes
	kind: Config
	preferences: {}
	users:
	- name: myuser
		user:
			client-certificate-data: $(cat myuser.crt | base64 -w0)
			client-key-data: $(cat myuser.key | base64 -w0)
	EOF
	```

#### Granting access to the created user

Создадим `ClusterAuthorizationRule`:
```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ClusterAuthorizationRule
metadata:
  name: myuser
spec:
  subjects:
  - kind: User
    name: myuser
  accessLevel: PrivilegedUser
  portForwarding: true
```

## Configuring kube-apiserver

For the `enableMultiTenancy` parameter to work correctly, you need to configure kube-apiserver. A dedicated [control-plane-manager](../../modules/040-control-plane-manager/) module can help you with this.

{% offtopic title="Changes that will be made to the manifest" %}

* The `--authorization-mode` argument will be modified: the Webhook method will be added in front of the RBAC method (e.g., --authorization-mode=Node,Webhook,RBAC);
* The `--authorization-webhook-config-file=/etc/kubernetes/authorization-webhook-config.yaml` will be added;
* The `volumeMounts` parameter will be added:

  ```yaml
  - name: authorization-webhook-config
    mountPath: /etc/kubernetes/authorization-webhook-config.yaml
    readOnly: true
  ```
* The `volumes` parameter will be added:

	```yaml
	- name:authorization-webhook-config
		hostPath:
			path: /etc/kubernetes/authorization-webhook-config.yaml
			type: FileOrCreate
	```
{% endofftopic %}

## How do I check that a user has access?
Execute the command below with the following parameters:
* `resourceAttributes` (the same as in RBAC) - target resources;
* `user` - the name of the user;
* `groups` - user groups;

P.S. You can use Dex logs to find out groups and a username if this module is used together with the `user-authn` module (`kubectl -n d8-user-authn logs -l app=dex`); logs available only if the user is authorized).

```bash
cat  <<EOF | 2>&1 kubectl create -v=8 -f - | tail -2 \
  | grep "Response Body" | awk -F"Response Body:" '{print $2}' \
  | jq -rc .status
apiVersion: authorization.k8s.io/v1
kind: SubjectAccessReview
spec:
  resourceAttributes:
    namespace: d8-monitoring
    verb: get
    group: ""
    resource: "pods"
  user: "user@gmail.com"
  groups:
  - Everyone
  - Admins
EOF
```

You will see if access is allowed and what role is used:

```bash
{
  "allowed": true,
  "reason": "RBAC: allowed by ClusterRoleBinding \"user-authz:myuser:super-admin\" of ClusterRole \"user-authz:super-admin\" to User \"user@gmail.com\""
}
```

## Customizing rights of pre-installed AccessLevels

If you want to grant more privileges to a specific AccessLevel, you only need to create a ClusterRole with the `user-authz.deckhouse.io/access-level: <AccessLevel>` anootation.

An example:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  annotations:
    user-authz.deckhouse.io/access-level: PrivilegedUser
  name: d8-mymodule-ns:privileged-user
rules:
- apiGroups:
  - mymodule.io
  resources:
  - destinationrules
  - virtualservices
  - serviceentries
  verbs:
  - create
  - list
  - get
  - update
  - delete
```

<!--## TODO-->

<!--1. There is a CR `ClusterAuthorizationRule`. Its resources are used to generate `ClusterRoleBindings` for users who mentioned in the field `subjects`. The set of `ClusterRoles` to bind is declared by fields:-->
<!--    1. `accessLevel` — pre-defined `ClusterRole` set.-->
<!--    2. `portForwarding` — pre-defined `ClusterRole` set.-->
<!--    3. `additionalRoles` — user-defined `ClusterRole` set.-->
<!--2. The configuration of fields `allowAccessToSystemNamespaces` and `limitNamespaces` affects the `user-authz-webhook` DaemonSet, which is authorization agent of apiserver,-->
<!--3. When creating `ClusterRole` objects with annotation `user-authz.deckhouse.io/access-level`, the set of `ClusterRoles` for binding to the corresponding subject is extended.-->
