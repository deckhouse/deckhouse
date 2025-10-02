<script type="text/javascript" src='{% javascript_asset_tag getting-started %}[_assets/js/getting-started.js]{% endjavascript_asset_tag %}'></script>
<script type="text/javascript" src='{% javascript_asset_tag getting-started-access %}[_assets/js/getting-started-access.js]{% endjavascript_asset_tag %}'></script>
<script type="text/javascript" src='{% javascript_asset_tag bcrypt %}[_assets/js/bcrypt.js]{% endjavascript_asset_tag %}'></script>

At this point, you have created a cluster that consists of a **single** master node. Only a limited set of system components run on the master node by default. You have to add at least one worker node to the cluster for the cluster to work properly.

Add a new node to the cluster (for more information about adding a static node to a cluster, read [the documentation](/products/virtualization-platform/documentation/admin/platform-management/platform-scaling/node/bare-metal-node.html#adding-nodes-to-a-bare-metal-cluster)):

- Prepare a server that will be a worker node of the cluster.

- Create a [NodeGroup](/modules/node-manager/cr.html#nodegroup) `worker`. To do this, run the following command on the **master node**:


  ```shell
  sudo d8 k create -f - << EOF
  apiVersion: deckhouse.io/v1
  kind: NodeGroup
  metadata:
    name: worker
  spec:
    nodeType: Static
    staticInstances:
      count: 1
      labelSelector:
        matchLabels:
          role: worker
EOF
  ```
  
- Generate an SSH key with an empty passphrase. To do this, run the following command on the **master node**:

  ```
  ssh-keygen -t rsa -f /dev/shm/caps-id -C "" -N ""
  ```

- Create an [SSHCredentials](/modules/node-manager/cr.html#sshcredentials) resource in the cluster. To do this, run the following command on the **master node**:

  ```shell
  sudo -i d8 k create -f - <<EOF
  apiVersion: deckhouse.io/v1alpha1
  kind: SSHCredentials
  metadata:
    name: caps
  spec:
    user: caps
    privateSSHKey: "`cat /dev/shm/caps-id | base64 -w0`"
  EOF
  ```

- Print the public part of the previously generated SSH key (you will need it in the next step). To do so, run the following command on the **master node**:

  ```
  cat /dev/shm/caps-id.pub
  ```

- Create the `caps` user on the **virtual machine you have started**. To do so, run the following command, specifying the public part of the SSH key obtained in the previous step:

  ```shell
  export KEY='<SSH-PUBLIC-KEY>' # Specify the public part of the user's SSH key.
  useradd -m -s /bin/bash caps
  usermod -aG sudo caps
  echo 'caps ALL=(ALL) NOPASSWD: ALL' | sudo EDITOR='tee -a' visudo
  mkdir /home/caps/.ssh
  echo $KEY >> /home/caps/.ssh/authorized_keys
  chown -R caps:caps /home/caps
  chmod 700 /home/caps/.ssh
  chmod 600 /home/caps/.ssh/authorized_keys
  ```

- Create a [StaticInstance](/modules/node-manager/cr.html#staticinstance) for the node to be added. To do so, run the following command on the **master node** (specify IP address of the node):

  ```shell
  export NODE=<NODE-IP-ADDRESS> # Specify the IP address of the node you want to connect to the cluster.
  sudo -i d8 k create -f - <<EOF
  apiVersion: deckhouse.io/v1alpha1
  kind: StaticInstance
  metadata:
    name: dvp-worker
    labels:
      role: worker
  spec:
    address: "$NODE"
    credentialsRef:
      kind: SSHCredentials
      name: caps
  EOF
  ```

- Ensure that all nodes in the cluster are `Ready`.
  On the **master node**, run the following command to get nodes list:

  ```shell
  sudo kubectl get no
  ```
