---
title: "Использование Proxy"
lang: ru
---

# Настройка прокси для репозиториев
* Подготавливаем виртуальную машину для прокси.
* Машина должна быть доступна для нод, которые будут использовать ее как прокси, и иметь доступ в интернет.
* Устанавливаем на машину squid (на примере Ubuntu):
```
apt-get install squid
```
* Создаем конфиг:
```
cat <<EOF > /etc/squid/squid.conf
auth_param basic program /usr/lib/squid3/basic_ncsa_auth /etc/squid/passwords
auth_param basic realm proxy
acl authenticated proxy_auth REQUIRED
http_access allow authenticated

# Choose the port you want. Below we set it to default 3128.
http_port 3128
```
* Создаем пользователя (для примера test/test):
```
echo "test:$(openssl passwd -crypt test)" >> /etc/squid/passwords
```
* Запускаем и разрешаем автозагрузку squid:
```
systemctl restart squid
systemctl enable squid
```
# Настройка использования прокси в Deckhouse
Эта настройка расположена в [ClusterConfiguration](../../candi/openapi/cluster_configuration.yaml).

Пример:
```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ClusterConfiguration
clusterType: Cloud
cloud:
  provider: OpenStack
  prefix: main
podSubnetCIDR: 10.111.0.0/16
serviceSubnetCIDR: 10.222.0.0/16
kubernetesVersion: "1.19"
cri: "Containerd"
clusterDomain: "cluster.local"
packagesProxy:
  uri: "http://192.168.199.25:3128"
  username: "test"
  password: "test"
```
