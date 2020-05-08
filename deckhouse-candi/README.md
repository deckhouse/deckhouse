Deckhouse CandI (Cluster and Infrastructure)
============================================
Приложение, призванное навести порядок в мире развертывания новых кластеров. 
Оно сеет ужас в сердцах конкурентов. 
О нем слагают баллады. 
На день рождения каждый ребенок хочет получить в подарок именно Deckhouse CandI. 

Трепещите. Он уже здесь!   

### Разработка cloud provider'ов

Пример конфигурации для OpenStack:
```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ClusterConfiguration
clusterType: Cloud
cloud:
  provider: OpenStack
  prefix: pivot
podSubnetCIDR: 10.111.0.0/16
serviceSubnetCIDR: 10.222.0.0/16
kubernetesVersion: "1.16"
clusterDomain: "cluster.local"
---
apiVersion: deckhouse.io/v1alpha1
kind: InitConfiguration
sshPublicKeys:
- ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQDTXjTmx3hq2EPDQHWSJN7By1VNFZ8colI5tEeZDBVYAe9Oxq4FZsKCb1aGIskDaiAHTxrbd2efoJTcPQLBSBM79dcELtqfKj9dtjy4S1W0mydvWb2oWLnvOaZX/H6pqjz8jrJAKXwXj2pWCOzXerwk9oSI4fCE7VbqsfT4bBfv27FN4/Vqa6iWiCc71oJopL9DldtuIYDVUgOZOa+t2J4hPCCSqEJK/r+ToHQbOWxbC5/OAufXDw2W1vkVeaZUur5xwwAxIb3wM3WoS3BbwNlDYg9UB2D8+EZgNz1CCCpSy1ELIn7q8RnrTp0+H8V9LoWHSgh3VCWeW8C/MnTW90IR stas@stas-ThinkPad
masterNodeGroup:
  zones:
  - nova
  minPerZone: 1
  maxPerZone: 3
deckhouse:
  imagesRepo: registry.flant.com/sys/antiopa
  registryDockerCfg: ***REMOVED***==
  devBranch: deckhouse-candi
  configOverrides:
    global:
      clusterName: main
      project: pivot
---
apiVersion: deckhouse.io/v1alpha1
kind: OpenStackClusterConfiguration
layout: Standard
standard:
  internalNetworkCIDR: 192.168.199.0/24
  internalNetworkDNSServers:
  - 8.8.8.8
  - 4.2.2.2
  internalNetworkSecurity: true
  externalNetworkName: public
provider:
  authURL: https://cloud.flant.com/v3/
  domainName: Default
  tenantName: xxx
  username: xxx
  password: xxx
  region: HetznerFinland
---
apiVersion: deckhouse.io/v1alpha1
kind: OpenStackInitConfiguration
masterInstanceClass:
  flavorName: m1.large
  imageName: ubuntu-18-04-cloud-amd64
  rootDiskSizeInGb: 20
```
Запуск werf:
```yaml
werf build --stages-storage :local install
werf run install \
  --docker-options="-it -v $(pwd)/config.yaml:/config.yaml -v $HOME/.ssh/:/root/.ssh/" \
  --stages-storage :local -- bash
```

Вместо сборки можно взять готовый образ `registry.flant.com/sys/antiopa/install` тегом `имя_вашей_ветки`.


Установка кластера:
```yaml
deckhouse-candi bootstrap \
  --ssh-user=ubuntu \
  --ssh-agent-private-keys=~/.ssh/tfadm-id-rsa \
  --ssh-bastion-user=y.gagarin \
  --ssh-bastion-host=tf.hf-bastion \
  --config=/config.yaml 
```
Удаление кластер:
```bash
deckhouse-candi helper run-terraform-destroy-all --config=/config.yaml
```
