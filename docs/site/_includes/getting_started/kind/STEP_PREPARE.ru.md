[Установите kind](https://kind.sigs.k8s.io/docs/user/quick-start/#installation), если он еще не установлен.

> В данном руководстве приводится минимальная конфигурация кластера `kind`, необходимая для установки Deckhouse. Конфигурация предусматривает установку кластера Kubernetes с одним узлом и проброс в кластер двух портов, необходимых для работы Ingress-контроллера. Вы можете использовать свою конфигурацию кластера в kind, [увеличить](https://kind.sigs.k8s.io/docs/user/configuration/#nodes) количество узлов или [настроить](https://kind.sigs.k8s.io/docs/user/local-registry/) локальный container registry.

Создайте файл конфигурации кластера kind:

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

> Перед созданием кластера убедитесь, что у вас не заняты порты 80 и 443.

Создайте кластер kind:

```shell
kind create cluster --image "kindest/node:v1.23.6@sha256:51d988ac40b04965b5379e251a113cdd44150b758ae339b0e941769e584040f5" --config kind.cfg
```

Пример вывода команды создания кластера:

```shell
$ kind create cluster --image "kindest/node:v1.23.6@sha256:51d988ac40b04965b5379e251a113cdd44150b758ae339b0e941769e584040f5" --config kind.cfg
Creating cluster "kind" ...
 ✓ Ensuring node image (kindest/node:v1.23.6) 🖼
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
