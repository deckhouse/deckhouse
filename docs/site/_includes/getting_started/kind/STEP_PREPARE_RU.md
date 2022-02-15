[Установите kind](https://kind.sigs.k8s.io/docs/user/quick-start/#installation), если он еще не установлен.

Создайте файл конфигурации кластера kind:
{% snippetcut selector="create-kind-cfg" %}
```shell
cat <<EOF > kind.cfg
apiVersion: kind.x-k8s.io/v1alpha4
kind: Cluster
nodes:
- role: control-plane
  extraPortMappings:
  - containerPort: 80
    hostPort: 80
    listenAddress: "127.0.0.1"
    protocol: TCP
  - containerPort: 443
    hostPort: 443
    listenAddress: "127.0.0.1"
    protocol: TCP
EOF
```
{% endsnippetcut %}

> Перед созданием кластера убедитесь, что у вас не заняты порты 80 и 443.

Создайте кластер kind:
{% snippetcut selector="create-kind-cluster" %}
```shell
kind create cluster --config kind.cfg
```
{% endsnippetcut %}

Пример вывода команды создания кластера:
```
$ kind create cluster --config kind.cfg
Creating cluster "kind" ...
 ✓ Ensuring node image (kindest/node:v1.21.1) 🖼
 ✓ Preparing nodes 📦  
 ✓ Writing configuration 📜 
 ✓ Starting control-plane 🕹️ 
 ✓ Installing CNI 🔌 
 ✓ Installing StorageClass 💾 
Set kubectl context to "kind-kind"
You can now use your cluster with:

kubectl cluster-info --context kind-kind

Have a nice day! 👋
```
