#!/bin/bash

# Copyright 2025 Flant JSC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -Eeo pipefail
AWS_CLUSTER_CONFIGURATION=$(cat <<EOF
apiVersion: deckhouse.io/v1
kind: AWSClusterConfiguration
layout: WithoutNAT
provider:
  providerAccessKeyId: test_key
  providerSecretAccessKey: test_key
  region: eu-central-1
masterNodeGroup:
  replicas: 1
  instanceClass:
    diskSizeGb: 30
    diskType: gp3
    instanceType: c5.xlarge
    ami: ami-0caef02b518350c8b
vpcNetworkCIDR: "10.241.0.0/16"
nodeNetworkCIDR: "10.241.32.0/20"
sshPublicKey: test_ssh
EOF
)
YANDEX_CLUSTER_CONFIGURATION=$(cat <<EOF
apiVersion: deckhouse.io/v1
kind: YandexClusterConfiguration
layout: Standard
provider:
  cloudID: test_id
  folderID: test_id
  serviceAccountJSON: "{'test_json': 'test_json'}"
masterNodeGroup:
  replicas: 1
  instanceClass:
    cores: 4
    memory: 8192
    imageID: fd85m9q2qspfnsv055rh
    externalIPAddresses:
    - "Auto"
nodeNetworkCIDR: "10.241.32.0/20"
sshPublicKey: test_ssh
EOF
)
GCP_CLUSTER_CONFIGURATION=$(cat <<EOF
apiVersion: deckhouse.io/v1
kind: GCPClusterConfiguration
layout: WithoutNAT
provider:
  serviceAccountJSON: "{'test_json': 'test_json'}"
  region: europe-west3
labels:
  kube: d8-demo
masterNodeGroup:
  replicas: 1
  instanceClass:
    machineType: n1-standard-4
    image: projects/ubuntu-os-cloud/global/images/ubuntu-2404-noble-amd64-v20250313
    disableExternalIP: false
subnetworkCIDR: 10.0.0.0/24
sshKey: test_ssh
EOF
)
AZURE_CLUSTER_CONFIGURATION=$(cat <<EOF
apiVersion: deckhouse.io/v1
kind: AzureClusterConfiguration
layout: Standard
sshPublicKey: test_ssh
vNetCIDR: 10.241.0.0/16
subnetCIDR: 10.241.0.0/24
masterNodeGroup:
  replicas: 1
  instanceClass:
    machineSize: Standard_D4ds_v4
    urn: Canonical:0001-com-ubuntu-server-jammy:22_04-lts:22.04.202212140
    enableExternalIP: true
provider:
  subscriptionId: test_id
  clientId: test_id
  clientSecret: test_id
  tenantId: test_id
  location: westeurope
EOF
)
case $PROVIDER in
    AWS)
      PROVIDER_CLUSTER_CONFIGURATION=$AWS_CLUSTER_CONFIGURATION
      ;;
    Yandex)
      PROVIDER_CLUSTER_CONFIGURATION=$YANDEX_CLUSTER_CONFIGURATION
      ;;
    GCP)
      PROVIDER_CLUSTER_CONFIGURATION=$GCP_CLUSTER_CONFIGURATION
      ;;
    Azure)
      PROVIDER_CLUSTER_CONFIGURATION=$AZURE_CLUSTER_CONFIGURATION
      ;;
    *)
      echo "PROVIDER not defined"
      exit 1
      ;;
esac
CONFIG_YAML=$(cat <<EOF
apiVersion: deckhouse.io/v1
kind: BashibleTemplateData
bundle: ubuntu-lts
provider: ${PROVIDER}
runType: ClusterBootstrap
registry:
  host: registry.deckhouse.io
  auth: "test:test"
clusterBootstrap:
  clusterDNSAddress: 10.222.0.10
  clusterDomain: cluster.local
  nodeIP: 192.168.199.23
kubernetesVersion: "1.30"
cri: "Containerd"
nodeGroup:
  cloudInstances:
    classReference:
      kind: ${PROVIDER}InstanceClass
      name: master
  instanceClass:
    flavorName: m1.large
    imageName: ubuntu-18-04-cloud-amd64
    mainNetwork: shared
    rootDiskSizeInGb: 20
  maxPerZone: 3
  minPerZone: 1
  name: master
  nodeType: CloudEphemeral
  zones:
  - nova
k8s:
  '1.23':
    patch: 10
    bashible:
      ubuntu:
        '18.04':
          containerd:
            desiredVersion: "containerd.io=1.4.6-1"
            allowedPattern: "containerd.io=1.[4]"
---
apiVersion: deckhouse.io/v1
kind: ClusterConfiguration
clusterType: Cloud
cloud:
  provider: ${PROVIDER}
  prefix: cloud-demo
podSubnetCIDR: 10.111.0.0/16
serviceSubnetCIDR: 10.222.0.0/16
kubernetesVersion: "Automatic"
clusterDomain: "cluster.local"
---
$PROVIDER_CLUSTER_CONFIGURATION
EOF
)
config=config.yaml
volumesRoot=$(pwd)
printf "%s\n" "$CONFIG_YAML" > "$volumesRoot/$config"
dockerExit=0
docker pull ${REGISTRY}/deckhouse/ee/install:stable
cat <<'SCRIPT_END' | docker run -i --rm \
  -v ${volumesRoot}/candi/bashible:/deckhouse/candi/bashible \
  -v ${volumesRoot}/candi/cloud-providers:/deckhouse/candi/cloud-providers \
  -v ${volumesRoot}/${config}:/${config} \
  -e config=config.yaml \
  --entrypoint=bash \
  ${REGISTRY}/deckhouse/ee/install:stable - || dockerExit=1
dhctl config render bashible-bundle --config $config
SCRIPT_END
exit $dockerExit
