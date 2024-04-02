---
title: "The metallb module: examples"
---

Metallb can be used in Static (Bare Metal) clusters when there is no option to use cloud load balancers. Metallb can work in L2 or BGP modes.

## Example of Metallb usage in L2 mode

{% raw %}
Below is a small step-by-step guide on how to enable the metallb module, create a LoadBalancer and provide access to an application (Nginx web server).

1. Specify node groups ([_NodeGroup_](../040-node-manager/cr.html#nodegroup)) to run the applications to provide access to.

   For example, Ingress controllers are run on frontend nodes while the application is run on a worker node. Frontend nodes have a label `node-role/metallb=""`.

   ```yaml
   apiVersion: deckhouse.io/v1
   kind: NodeGroup
   metadata:
     name: frontend
   spec:
     disruptions:
       approvalMode: Manual
     nodeTemplate:
       labels:
         node-role.deckhouse.io/metallb: ""
       taints:
       - effect: NoExecute
         key: dedicated.deckhouse.io
         value: metallb
     nodeType: Static
   ---
   apiVersion: deckhouse.io/v1
   kind: NodeGroup
   metadata:
     name: worker
   spec:
     disruptions:
       approvalMode: Manual
     nodeType: Static
   ```

1. Make sure the nodes are labeled correctly:

   ```bash
   kubectl get nodes -l node-role.deckhouse.io/metallb=
   ```

   Your output should look something like this:

   ```bash
   $ kubectl get nodes -l node-role/metallb
   NAME              STATUS   ROLES      AGE   VERSION
   demo-frontend-0   Ready    frontend   61d   v1.21.14
   demo-frontend-1   Ready    frontend   61d   v1.21.14
   ```

1. Enable the metallb module and set the `nodeSelector` and `tolerations` parameters for the MetalLB speakers.

   Below is an example of the module configuration:
  
   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
     name: metallb
   spec:
     version: 1
     enabled: true
     settings:
       addressPools:
       - addresses:
         - 192.168.199.100-192.168.199.102
         name: frontend-pool
         protocol: layer2
       speaker:
         nodeSelector:
           node-role.deckhouse.io/metallb: ""
         tolerations:
         - effect: NoExecute
           key: dedicated.deckhouse.io
           operator: Equal
           value: metallb
   ```

1. Deploy the application (nginx) and Service on port `8080`:

   ```shell
   kubectl create deploy nginx --image=nginx
   kubectl create svc loadbalancer nginx --tcp=8080:80
   ```

1. Verify that the service has been created:

   ```shell
   kubectl get svc nginx
   ```

   Your output should look something like this:

   ```shell
   $ kubectl get svc nginx
   NAME    TYPE           CLUSTER-IP     EXTERNAL-IP       PORT(S)          AGE
   nginx   LoadBalancer   10.222.9.190   192.168.199.101   8080:31689/TCP   3m11s
   ```

1. Check if the application is accessible.

   Example:

   ```console
   $ curl -s -o /dev/null -w "%{http_code}" 192.168.199.101:8080
   200
   ```

{% endraw %}
