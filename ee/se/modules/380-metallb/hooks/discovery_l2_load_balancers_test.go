/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license.
See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Metallb :: hooks :: discovery_l2_lb ::", func() {
	f := HookExecutionConfigInit(`
{
  "metallb": {
    "internal": {
      "l2lbservices": [
        {}
      ],
      "migrationOfOldFashionedLBsAdoptionComplete": true
    }
  }
}`, "")
	f.RegisterCRD("network.deckhouse.io", "v1alpha1", "MetalLoadBalancerClass", false)

	Context("Empty Cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(""))
			f.RunHook()
		})
		It("Should run", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())
		})
	})

	Context("There are 3 services, 2 MLBC, 3 nodes in cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: v1
kind: Service
metadata:
  name: serv_status_class
  namespace: nginx1
  annotations:
    network.deckhouse.io/l2-load-balancer-external-ips-count: "2"
spec:
  clusterIP: 1.2.3.4
  ports:
  - port: 7473
    protocol: TCP
    targetPort: 7473
  externalTrafficPolicy: Local
  internalTrafficPolicy: Cluster
  selector:
    app: nginx1
  type: LoadBalancer
status:
  conditions:
  - message: 2 of 3 public IPs were assigned
    reason: NotAllIPsAssigned
    status: "False"
    type: AllPublicIPsAssigned
  - message: status_mlbc
    reason: LoadBalancerClassBound
    status: "True"
    type: network.deckhouse.io/load-balancer-class
---
apiVersion: v1
kind: Service
metadata:
  name: serv_config_class
  namespace: nginx2
  annotations:
    network.deckhouse.io/load-balancer-shared-ip-key: "6.6.6.6"
    network.deckhouse.io/l2-load-balancer-external-ips-count: "2"
spec:
  clusterIP: 2.3.4.5
  ports:
  - port: 7474
    protocol: TCP
    targetPort: 7474
  externalTrafficPolicy: Local
  internalTrafficPolicy: Cluster
  selector:
    app: nginx2
  type: LoadBalancer
  loadBalancerClass: config_mlbc
status:
  conditions:
  - lastTransitionTime: null
    message: 1 of 1 public IPs were assigned
    reason: AllIPsAssigned
    status: "True"
    type: AllPublicIPsAssigned
  loadBalancer:
    ingress:
    - ip: 10.3.29.200
      ipMode: VIP
---
apiVersion: v1
kind: Service
metadata:
  name: serv_no_class
  namespace: nginx3
  annotations:
    network.deckhouse.io/l2-load-balancer-external-ips-count: "2"
spec:
  clusterIP: 4.5.6.7
  ports:
  - port: 7475
    protocol: TCP
    targetPort: 7475
  externalTrafficPolicy: Local
  internalTrafficPolicy: Cluster
  selector:
    app: nginx3
  type: LoadBalancer
status:
  conditions:
  - lastTransitionTime: null
    message: 1 of 1 public IPs were assigned
    reason: AllIPsAssigned
    status: "True"
    type: AllPublicIPsAssigned
  loadBalancer:
    ingress:
    - ip: 10.3.29.200
      ipMode: VIP
---
apiVersion: network.deckhouse.io/v1alpha1
kind: MetalLoadBalancerClass
metadata:
  name: default_mlbc
spec:
  isDefault: true
  type: L2
  l2:
    interfaces:
    - eno1
    - eth0.vlan300
  addressPool:
  - 192.168.2.100-192.168.2.150
  nodeSelector:
    node-role.kubernetes.io/frontend: ""
---
apiVersion: network.deckhouse.io/v1alpha1
kind: MetalLoadBalancerClass
metadata:
  name: config_mlbc
spec:
  type: L2
  l2:
    interfaces:
    - eth1
    - eth2
  addressPool:
  - 192.168.3.100-192.168.3.150
  nodeSelector:
    node-role.kubernetes.io/edge: ""
---
apiVersion: v1
kind: Node
metadata:
  name: frontend-1
  labels:
    node-role.kubernetes.io/frontend: ""
---
apiVersion: v1
kind: Node
metadata:
  name: frontend-2
  labels:
    node-role.kubernetes.io/frontend: ""
    node-role.kubernetes.io/edge: ""
---
apiVersion: v1
kind: Node
metadata:
  name: frontend-3
  labels:
    node-role.kubernetes.io/edge: ""
`))
			f.RunHook()
		})

		It("L2LBServices must be present in internal values, services are patched", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())

			svc := f.KubernetesResource("Service", "nginx2", "serv_config_class")
			svc2 := f.KubernetesResource("Service", "nginx3", "serv_no_class")
			Expect(f.ValuesGet("metallb.internal.l2loadbalancers").String()).To(MatchJSON(`
[
          {
            "name": "config_mlbc",
            "addressPool": [
              "192.168.3.100-192.168.3.150"
            ],
            "interfaces": [
              "eth1",
              "eth2"
            ],
            "nodeSelector": {
              "node-role.kubernetes.io/edge": ""
            },
            "isDefault": false
          },
          {
            "name": "default_mlbc",
            "addressPool": [
              "192.168.2.100-192.168.2.150"
            ],
            "interfaces": [
              "eno1",
              "eth0.vlan300"
            ],
            "nodeSelector": {
              "node-role.kubernetes.io/frontend": ""
            },
            "isDefault": true
          }
]
`))
			Expect(f.ValuesGet("metallb.internal.l2lbservices").String()).To(MatchJSON(`
[
          {
            "publishNotReadyAddresses": false,
            "name": "serv_config_class-config_mlbc-0",
            "namespace": "nginx2",
            "serviceName": "serv_config_class",
            "serviceNamespace": "nginx2",
            "preferredNode": "frontend-2",
            "clusterIP": "2.3.4.5",
            "ports": [
              {
                "protocol": "TCP",
                "port": 7474,
                "targetPort": 7474
              }
            ],
            "externalTrafficPolicy": "Local",
            "internalTrafficPolicy": "Cluster",
            "selector": {
              "app": "nginx2"
            },
            "mlbcName": "config_mlbc",
            "desiredIP": "",
            "lbAllowSharedIP": "6.6.6.6"
          },
          {
            "publishNotReadyAddresses": false,
            "name": "serv_config_class-config_mlbc-1",
            "namespace": "nginx2",
            "serviceName": "serv_config_class",
            "serviceNamespace": "nginx2",
            "preferredNode": "frontend-3",
            "clusterIP": "2.3.4.5",
            "ports": [
              {
                "protocol": "TCP",
                "port": 7474,
                "targetPort": 7474
              }
            ],
            "externalTrafficPolicy": "Local",
            "internalTrafficPolicy": "Cluster",
            "selector": {
              "app": "nginx2"
            },
            "mlbcName": "config_mlbc",
            "desiredIP": "",
            "lbAllowSharedIP": "6.6.6.6"
          },
          {
            "publishNotReadyAddresses": false,
            "name": "serv_no_class-default_mlbc-0",
            "namespace": "nginx3",
            "serviceName": "serv_no_class",
            "serviceNamespace": "nginx3",
            "preferredNode": "frontend-1",
            "clusterIP": "4.5.6.7",
            "ports": [
              {
                "protocol": "TCP",
                "port": 7475,
                "targetPort": 7475
              }
            ],
            "externalTrafficPolicy": "Local",
            "internalTrafficPolicy": "Cluster",
            "selector": {
              "app": "nginx3"
            },
            "mlbcName": "default_mlbc",
            "desiredIP": "",
            "lbAllowSharedIP": ""
          },
          {
            "publishNotReadyAddresses": false,
            "name": "serv_no_class-default_mlbc-1",
            "namespace": "nginx3",
            "serviceName": "serv_no_class",
            "serviceNamespace": "nginx3",
            "preferredNode": "frontend-2",
            "clusterIP": "4.5.6.7",
            "ports": [
              {
                "protocol": "TCP",
                "port": 7475,
                "targetPort": 7475
              }
            ],
            "externalTrafficPolicy": "Local",
            "internalTrafficPolicy": "Cluster",
            "selector": {
              "app": "nginx3"
            },
            "mlbcName": "default_mlbc",
            "desiredIP": "",
            "lbAllowSharedIP": ""
          }
]
`))
			Expect(svc.Field("status").String()).To(MatchJSON(`{
"conditions": [
	{
		"message": "1 of 1 public IPs were assigned",
		"reason": "AllIPsAssigned",
		"status": "True",
		"type": "AllPublicIPsAssigned"
	},
	{
		"message": "config_mlbc",
		"reason": "LoadBalancerClassBound",
		"status": "True",
		"type": "network.deckhouse.io/load-balancer-class"
	}
],
"loadBalancer": {
	"ingress": [
		{
			"ip": "10.3.29.200",
			"ipMode": "VIP"
		}
	]
}
}`))
			Expect(svc2.Field("status").String()).To(MatchJSON(`{
"conditions": [
	{
		"message": "1 of 1 public IPs were assigned",
		"reason": "AllIPsAssigned",
		"status": "True",
		"type": "AllPublicIPsAssigned"
	},
	{
		"message": "default_mlbc",
		"reason": "LoadBalancerClassBound",
		"status": "True",
		"type": "network.deckhouse.io/load-balancer-class"
	}
],
"loadBalancer": {
	"ingress": [
		{
			"ip": "10.3.29.200",
			"ipMode": "VIP"
		}
	]
}
}`))
		})
	})

	Context("There is a service with a minimum number of fields", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: v1
kind: Service
metadata:
  name: serv-config-class
  namespace: nginx1
spec:
  ports:
  - port: 7474
  externalTrafficPolicy: Local
  internalTrafficPolicy: Cluster
  type: LoadBalancer
---
apiVersion: network.deckhouse.io/v1alpha1
kind: MetalLoadBalancerClass
metadata:
  name: config-mlbc
spec:
  isDefault: true
  type: L2
  l2:
    interfaces:
    - eth1
    - eth2
  addressPool:
  - 192.168.3.100-192.168.3.150
  nodeSelector:
    node-role.kubernetes.io/frontend: ""
---
apiVersion: v1
kind: Node
metadata:
  name: frontend-1
  labels:
    node-role.kubernetes.io/frontend: ""
---
apiVersion: v1
kind: Node
metadata:
  name: frontend-2
  labels:
    node-role.kubernetes.io/frontend: ""
`))
			f.RunHook()
		})

		It("Values are generated and written without errors", func() { //
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())
			Expect(f.ValuesGet("metallb.internal.l2loadbalancers").String()).To(MatchJSON(`
[
        {
            "name": "config-mlbc",
            "addressPool": [
              "192.168.3.100-192.168.3.150"
            ],
            "interfaces": [
              "eth1",
              "eth2"
            ],
            "nodeSelector": {
              "node-role.kubernetes.io/frontend": ""
            },
            "isDefault": true
        }
]
`))
			Expect(f.ValuesGet("metallb.internal.l2lbservices").String()).To(MatchJSON(`
[
        {
            "publishNotReadyAddresses": false,
            "name": "serv-config-class-config-mlbc-0",
            "namespace": "nginx1",
            "serviceName": "serv-config-class",
            "serviceNamespace": "nginx1",
            "preferredNode": "frontend-2",
            "clusterIP": "",
            "ports": [
              {
                "port": 7474,
                "targetPort": 0
              }
            ],
            "externalTrafficPolicy": "Local",
            "internalTrafficPolicy": "Cluster",
            "selector": null,
            "mlbcName": "config-mlbc",
            "desiredIP": "",
            "lbAllowSharedIP": ""
          }
]
`))
		})
	})

	Context("Migrate Services: transfer IPs to L2LBServices", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: v1
kind: Service
metadata:
  name: serv_to_migrate1
  namespace: nginx1
  annotations:
    network.deckhouse.io/load-balancer-ips: "3.3.3.3"
    network.deckhouse.io/l2-load-balancer-external-ips-count: "2"
spec:
  clusterIP: 2.3.4.5
  ports:
  - port: 7474
    protocol: TCP
    targetPort: 7474
  externalTrafficPolicy: Local
  internalTrafficPolicy: Cluster
  selector:
    app: nginx1
  type: LoadBalancer
---
apiVersion: v1
kind: Service
metadata:
  name: serv_to_migrate2
  namespace: nginx2
  annotations:
    network.deckhouse.io/load-balancer-ips: "1.1.1.1,2.2.2.2"
    network.deckhouse.io/l2-load-balancer-external-ips-count: "2"
spec:
  clusterIP: 4.5.6.7
  ports:
  - port: 7475
    protocol: TCP
    targetPort: 7475
  externalTrafficPolicy: Local
  internalTrafficPolicy: Cluster
  selector:
    app: nginx2
  type: LoadBalancer
---
apiVersion: v1
kind: Service
metadata:
  name: serv_to_migrate3
  namespace: nginx3
  annotations:
    network.deckhouse.io/metal-load-balancer-class: "migrate_mlbc"
spec:
  clusterIP: 5.6.7.8/32
  ports:
  - port: 7476
    protocol: TCP
    targetPort: 7476
  externalTrafficPolicy: Local
  internalTrafficPolicy: Cluster
  selector:
    app: nginx3
  type: LoadBalancer
---
apiVersion: network.deckhouse.io/v1alpha1
kind: MetalLoadBalancerClass
metadata:
  name: migrate_mlbc
spec:
  isDefault: false
  type: L2
  l2:
    interfaces:
    - eno1
    - eth0.vlan300
  addressPool:
  - 7.7.7.7/32
  nodeSelector:
    node-role.kubernetes.io/frontend: ""
---
apiVersion: network.deckhouse.io/v1alpha1
kind: MetalLoadBalancerClass
metadata:
  name: default
spec:
  isDefault: true
  type: L2
  l2:
    interfaces:
    - eno1
    - eth0.vlan300
  addressPool:
  - 1.1.1.1/32
  - 2.2.2.2/32
  - 3.3.3.3/32
  nodeSelector:
    node-role.kubernetes.io/frontend: ""
---
apiVersion: v1
kind: Node
metadata:
  name: frontend-1
  labels:
    node-role.kubernetes.io/frontend: ""
spec:
---
apiVersion: v1
kind: Node
metadata:
  name: frontend-2
  labels:
    node-role.kubernetes.io/frontend: ""
    node-role.kubernetes.io/edge: ""
spec:
---
apiVersion: v1
kind: Node
metadata:
  name: frontend-3
  labels:
    node-role.kubernetes.io/edge: ""
spec:
`))
			f.RunHook()
		})

		It("L2LBServices must be present in internal values", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())

			Expect(f.ValuesGet("metallb.internal.l2loadbalancers").String()).To(MatchJSON(`
[
          {
            "name": "default",
            "addressPool": [
              "1.1.1.1/32",
              "2.2.2.2/32",
              "3.3.3.3/32"
            ],
            "interfaces": [
              "eno1",
              "eth0.vlan300"
            ],
            "nodeSelector": {
              "node-role.kubernetes.io/frontend": ""
            },
            "isDefault": true
          },
		  {
            "name": "migrate_mlbc",
            "addressPool": [
              "7.7.7.7/32"
            ],
            "interfaces": [
              "eno1",
              "eth0.vlan300"
            ],
            "nodeSelector": {
              "node-role.kubernetes.io/frontend": ""
            },
            "isDefault": false
          }
]
`))
			Expect(f.ValuesGet("metallb.internal.l2lbservices").String()).To(MatchJSON(`
[
          {
            "publishNotReadyAddresses": false,
            "name": "serv_to_migrate1-default-0",
            "namespace": "nginx1",
            "serviceName": "serv_to_migrate1",
            "serviceNamespace": "nginx1",
            "preferredNode": "frontend-2",
            "clusterIP": "2.3.4.5",
            "ports": [
              {
                "protocol": "TCP",
                "port": 7474,
                "targetPort": 7474
              }
            ],
            "externalTrafficPolicy": "Local",
            "internalTrafficPolicy": "Cluster",
            "selector": {
              "app": "nginx1"
            },
            "mlbcName": "default",
            "desiredIP": "3.3.3.3",
            "lbAllowSharedIP": ""
          },
          {
            "publishNotReadyAddresses": false,
            "name": "serv_to_migrate1-default-1",
            "namespace": "nginx1",
            "serviceName": "serv_to_migrate1",
            "serviceNamespace": "nginx1",
            "preferredNode": "frontend-1",
            "clusterIP": "2.3.4.5",
            "ports": [
              {
                "protocol": "TCP",
                "port": 7474,
                "targetPort": 7474
              }
            ],
            "externalTrafficPolicy": "Local",
            "internalTrafficPolicy": "Cluster",
            "selector": {
              "app": "nginx1"
            },
            "mlbcName": "default",
            "desiredIP": "",
            "lbAllowSharedIP": ""
          },
          {
            "publishNotReadyAddresses": false,
            "name": "serv_to_migrate2-default-0",
            "namespace": "nginx2",
            "serviceName": "serv_to_migrate2",
            "serviceNamespace": "nginx2",
            "preferredNode": "frontend-2",
            "clusterIP": "4.5.6.7",
            "ports": [
              {
                "protocol": "TCP",
                "port": 7475,
                "targetPort": 7475
              }
            ],
            "externalTrafficPolicy": "Local",
            "internalTrafficPolicy": "Cluster",
            "selector": {
              "app": "nginx2"
            },
            "mlbcName": "default",
            "desiredIP": "1.1.1.1",
            "lbAllowSharedIP": ""
          },
          {
            "publishNotReadyAddresses": false,
            "name": "serv_to_migrate2-default-1",
            "namespace": "nginx2",
            "serviceName": "serv_to_migrate2",
            "serviceNamespace": "nginx2",
            "preferredNode": "frontend-1",
            "clusterIP": "4.5.6.7",
            "ports": [
              {
                "protocol": "TCP",
                "port": 7475,
                "targetPort": 7475
              }
            ],
            "externalTrafficPolicy": "Local",
            "internalTrafficPolicy": "Cluster",
            "selector": {
              "app": "nginx2"
            },
            "mlbcName": "default",
            "desiredIP": "2.2.2.2",
            "lbAllowSharedIP": ""
          },
		  {
            "publishNotReadyAddresses": false,
            "name": "serv_to_migrate3-migrate_mlbc-0",
            "namespace": "nginx3",
            "serviceName": "serv_to_migrate3",
            "serviceNamespace": "nginx3",
            "preferredNode": "frontend-2",
            "clusterIP": "5.6.7.8/32",
            "ports": [
              {
                "protocol": "TCP",
                "port": 7476,
                "targetPort": 7476
              }
            ],
            "externalTrafficPolicy": "Local",
            "internalTrafficPolicy": "Cluster",
            "selector": {
              "app": "nginx3"
            },
            "mlbcName": "migrate_mlbc",
            "desiredIP": "",
            "lbAllowSharedIP": ""
          }
]
`))
		})
	})
})
