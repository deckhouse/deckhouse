# Steps

## Install OpenLDAP

* Apply [**ldap**](./ldap.yaml) manifest to the cluster.

## Add DexProvider

* Apply [**dex-provider**](./dex-provider.yaml) manifest to the cluster.
* Login to Kubeconfig (username: `janedoe@example.com`, password: `foo` or username: `johndoe@example.com`, password: `bar`).

## Generate kubeconfig

* Copy kubeconfig to your PC.
* Show how kubeconfig works.

## Deploy simple echo server

* Apply [**echo-service**](./echo-service.yaml) manifest to the cluster.
  > **Note!** Do not forget to use your cluster public domain instead of `{{ __cluster__domain__ }}` in the manifest.
* Show how you can access this service without authorization.

## Deploy DexAuthenticator

* Apply [**dex-authenticator**](./dex-authenticator.yaml) manifest to the cluster.
  > **NOTE**: Do not forget to use your cluster public domain instead of `{{ __cluster__domain__ }}` in the manifest.
* Add annotations to the ingress resource.
  ```shell
  kubectl -n openldap-demo annotate ingress echoserver 'nginx.ingress.kubernetes.io/auth-signin=https://$host/dex-authenticator/sign_in'
  kubectl -n openldap-demo annotate ingress echoserver 'nginx.ingress.kubernetes.io/auth-url=https://echoserver-dex-authenticator.openldap-demo.svc.cluster.local/dex-authenticator/auth'
  ```
* Show that access is protected (`janedoe@example.com` can access echo server, `johndoe@example.com` cannot).

## Create a user

* Apply [**dex-user**](./dex-user.yaml) manifest to the cluster.
* Show that you can log in with credentials from the custom resource.
* Add `groups: ["developers"]` to the User spec to show that this user now has access to the echo server. 
  ```shell
  kubectl patch user openldap-demo --type='merge' -- patch '{"spec": {"groups": ["developers"]}}'
  ```
# Cleaning

Execute [**clean_up.sh**](./clean_up.sh)
