---
title: "Proxy usage"
lang: en
---

# Setting up a proxy for repositories
* Prepare the VM for setting up the proxy.
* The machine must be accessible to the nodes that will use it as a proxy and be connected to the Internet.
* Install squid on the machine (in our example, the Ubuntu machine is used):
```shell
apt-get install squid
```
* Create a config file:
```shell
cat <<EOF > /etc/squid/squid.conf
auth_param basic program /usr/lib/squid3/basic_ncsa_auth /etc/squid/passwords
auth_param basic realm proxy
acl authenticated proxy_auth REQUIRED
http_access allow authenticated

# Choose the port you want. Below we set it to default 3128.
http_port 3128
```
* Create a user (test/test):
```shell
echo "test:$(openssl passwd -crypt test)" >> /etc/squid/passwords
```
* Start squid and enable the system to start it up automatically:
```shell
systemctl restart squid
systemctl enable squid
```
# Configuring proxy usage in Deckhouse
Insert the appropriate configuration into the [ClusterConfiguration](../../candi/openapi/cluster_configuration.yaml) file.

An example:
```yaml
apiVersion: deckhouse.io/v1
kind: ClusterConfiguration
clusterType: Cloud
cloud:
  provider: OpenStack
  prefix: main
podSubnetCIDR: 10.111.0.0/16
serviceSubnetCIDR: 10.222.0.0/16
kubernetesVersion: "1.21"
cri: "Containerd"
clusterDomain: "cluster.local"
packagesProxy:
  uri: "http://192.168.199.25:3128"
  username: "test"
  password: "test"
```
