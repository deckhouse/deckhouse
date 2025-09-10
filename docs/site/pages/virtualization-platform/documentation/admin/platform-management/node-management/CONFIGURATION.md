---
title: "Node configuration"
permalink: en/virtualization-platform/documentation/admin/platform-management/node-management/configuration.html
lang: en
---

## Custom configuration for nodes

The [NodeGroupConfiguration](../../../../reference/cr/nodegroup.html#nodegroupconfiguration) resource lets you automate actions on group nodes.
It supports running bash scripts on nodes (you can use the [bashbooster](https://github.com/deckhouse/deckhouse/tree/main/candi/bashible/bashbooster) command set) and the [Go Template](https://pkg.go.dev/text/template) templating engine.
It's a great way to automate the following operations:

- Installing and configuring additional OS packages.  

  Example:  

  - [Installing the kubectl plugin](os.html#installing-the-cert-manager-plugin-for-kubectl-on-master-nodes).

- Updating the operating system (OS) kernel to a specific version.
  
  Examples:

  - [Debian kernel update](os.html#for-debian-based-distributions).
  - [CentOS kernel update](os.html#for-centos-based-distributions).

- Modifying OS parameters.

  Examples:

  - [Configuring the sysctl parameter](os.html#modifying-the-sysctl-parameters).
  - [Adding a root certificate](os.html#adding-a-root-certificate).

- Collecting information on a node and carrying out other similar tasks.
- Configuring containerd.

  Examples:

  - [Setting up metrics](containerd.html#enabling-metrics-for-containerd).
  - [Adding a private registry](containerd.html#adding-a-private-registry-with-authentication).

## NodeGroupConfiguration settings

The NodeGroupConfiguration resource lets you assign [priority](../../../../reference/cr/nodegroup.html#nodegroupconfiguration-v1alpha1-spec-weight) to running scripts
or limit them to running on specific [node groups](../../../../reference/cr/nodegroup.html#nodegroupconfiguration-v1alpha1-spec-nodegroups) and [OS types](../../../../reference/cr/nodegroup.html#nodegroupconfiguration-v1alpha1-spec-bundles).

The script code is stored in the [`content`](../../../../reference/cr/nodegroup.html#nodegroupconfiguration-v1alpha1-spec-content) parameter of the resource.
When a script is created on a node, the contents of the `content` parameter are fed into the [Go Template](https://pkg.go.dev/text/template) templating engine.
The latter embeds an extra layer of logic when generating a script.
When parsed by the templating engine, a context with a set of dynamic variables becomes available.

The following variables are supported by the templating engine:

<ul>
<li><code>.cloudProvider</code> (for node groups of nodeType <code>CloudEphemeral</code> or <code>CloudPermanent</code>): Cloud provider dataset.
{% offtopic title="Example of data..." %}
```yaml
cloudProvider:
  instanceClassKind: OpenStackInstanceClass
  machineClassKind: OpenStackMachineClass
  openstack:
    connection:
      authURL: https://cloud.provider.com/v3/
      domainName: Default
      password: p@ssw0rd
      region: region2
      tenantName: mytenantname
      username: mytenantusername
    externalNetworkNames:
    - public
    instances:
      imageName: ubuntu-22-04-cloud-amd64
      mainNetwork: kube
      securityGroups:
      - kube
      sshKeyPairName: kube
    internalNetworkNames:
    - kube
    podNetworkMode: DirectRoutingWithPortSecurityEnabled
  region: region2
  type: openstack
  zones:
  - nova
```
{% endofftopic %}</li>
<li><code>.cri</code>: The CRI in use (starting with Deckhouse 1.49, only <code>containerd</code> is supported).</li>
<li><code>.kubernetesVersion</code>: The Kubernetes version in use.</li>
<li><code>.nodeUsers</code>: The dataset with information about node users added via the <a href="../../../../reference/cr/nodeuser.html">NodeUser</a>.
{% offtopic title="Example of data..." %}
```yaml
nodeUsers:
- name: user1
  spec:
    isSudoer: true
    nodeGroups:
    - '*'
    passwordHash: PASSWORD_HASH
    sshPublicKey: SSH_PUBLIC_KEY
    uid: 1050
```
{% endofftopic %}
</li>
<li><code>.nodeGroup</code>: Node group dataset.
{% offtopic title="Example of data..." %}
```yaml
nodeGroup:
  cri:
    type: Containerd
  disruptions:
    approvalMode: Automatic
  kubelet:
    containerLogMaxFiles: 4
    containerLogMaxSize: 50Mi
    resourceReservation:
      mode: "Off"
  kubernetesVersion: "1.29"
  manualRolloutID: ""
  name: master
  nodeTemplate:
    labels:
      node-role.kubernetes.io/control-plane: ""
      node-role.kubernetes.io/master: ""
    taints:
    - effect: NoSchedule
      key: node-role.kubernetes.io/master
  nodeType: CloudPermanent
  updateEpoch: "1699879470"
```
{% endofftopic %}</li>
</ul>

{% raw %}
Example of using variables in a template:

```shell
{{- range .nodeUsers }}
echo 'Tuning environment for user {{ .name }}'
# Some code for tuning user environment.
{{- end }}
```

Example of using bashbooster commands:

```shell
bb-event-on 'bb-package-installed' 'post-install'
post-install() {
  bb-log-info "Setting reboot flag due to kernel was updated"
  bb-flag-set reboot
}
```

{% endraw %}

To see the progress of running script, refer to the bashible service log on the node using the following command:

```bash
journalctl -u bashible.service
```

The scripts are located in the `/var/lib/bashible/bundle_steps/` directory on the node.

The service decides to re-run the scripts by comparing the single checksum of all files located at `/var/lib/bashible/configuration_checksum` with the checksum located in the `configuration-checksums` secret of the `d8-cloud-instance-manager` namespace in the Kubernetes cluster.

You can see the checksum using the following command:

```bash
d8 k -n d8-cloud-instance-manager get secret configuration-checksums -o yaml
```

The service compares checksums every minute.

The checksum in the cluster changes every 4 hours, thereby re-running the scripts on all nodes.
To force the execution of bashible on a node, delete the file with the script checksum using the following command:

```bash
rm /var/lib/bashible/configuration_checksum
```

### Things to note when writing scripts

When writing your own scripts, it's important to consider the following details of their use in Deckhouse:

1. Scripts in Deckhouse are executed once every 4 hours or based on external triggers.
   Therefore, it's important to write scripts in such a way
   that they check the need for their changes in the system before performing actions,
   thereby not making changes every time they are launched.
1. There are [built-in scripts](https://github.com/deckhouse/deckhouse/tree/main/candi/bashible/common-steps/all) for various actions, including installing and configuring services.
   This is important to consider when choosing the [priority](../../../../reference/cr/nodegroup.html#nodegroupconfiguration-v1alpha1-spec-weight) of custom scripts.
   For example, if a user script intends to restart a service, which is installed by a built-in script with a priority `N`,
   then this user script should have a priority of at least `N+1`.
   Otherwise, the user script will return an error when deploying a new node.
