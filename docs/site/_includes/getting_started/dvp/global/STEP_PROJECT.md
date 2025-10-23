<script type="text/javascript" src='{% javascript_asset_tag getting-started %}[_assets/js/getting-started.js]{% endjavascript_asset_tag %}'></script>
<script type="text/javascript" src='{% javascript_asset_tag getting-started-access %}[_assets/js/getting-started-access.js]{% endjavascript_asset_tag %}'></script>
<script type="text/javascript" src='{% javascript_asset_tag bcrypt %}[_assets/js/bcrypt.js]{% endjavascript_asset_tag %}'></script>

Create a project. To create a project, use the command (the example uses the `test-project` project; change it if necessary):

{% capture includePath %}_includes/getting_started/dvp/{{ page.platform_code }}/partials/project-rbac.yml.inc{% endcapture %}
{% include_file "{{ includePath }}" syntax="yaml" %}

Wait for the namespace to be created. To verify that it has been created, use the command:

```shell
d8 k get ns test-project
```

Create a project administrator and associate them with the `d8:use:role:admin` role in the namespace you created earlier.
To do this, use the command (the example uses the user `test-user@deckhouse.io`; change this if necessary):

{% capture includePath %}_includes/getting_started/dvp/{{ page.platform_code }}/partials/project-rbac-user.yml.inc{% endcapture %}
{% include_file "{{ includePath }}" syntax="yaml" %}

Open the web interface for generating the kubeconfig file for remote access to the API server. The address of the web interface is formed according to the DNS name template specified in the global parameter [publicDomainTemplate](/products/kubernetes-platform/documentation/v1/reference/api/global.html#parameters-modules-publicdomaintemplate). For example, if `publicDomainTemplate: %s.kube.my`, then the web interface will be available at the address `kubeconfig.kube.my`.

Enter the login (in the example — `test-user@deckhouse.io`) and the password of the created user to obtain the configuration file for access to the cluster:

On a computer with network access to the deployed cluster, create a file `~/.kube/config` (for Linux/MacOS) or `%USERPROFILE%\.kube\config` (for Windows) and paste the kubectl configuration provided in the *Raw Config* tab.

You have configured kubectl on this computer to manage the cluster. Execute the further commands on this computer.

Create a password for the user inside the virtual machine and generate its hash:

```bash
mkpasswd --method=SHA-512 --rounds=4096
```

To add a user and ssh key to the virtual machine, create a `cloud-config` file.
In the example, change the fields as desired:

- `name` — contains the username `test-user`, replace it with your own.
- `passwd` — contains the password hash `test-user` in quotation marks, replace it with your own hash.
- `ssh_authorized_keys` — contains the public ssh key, generate your own and replace it.

```bash
#cloud-config
ssh_pwauth: True
users:
- name: test-user
  passwd: '$6$rounds=4096$.ed4Qtpv1WeKmhH6$3ZCZGvv1QIe2bIsEGT549mAPnmCUVLG5TJAVsBr02bhdyKTGPt3HFC9Bc7x/NiGAwAqibIuUpRQk4SltW4Kd//'
  shell: /bin/bash
  sudo: ALL=(ALL) NOPASSWD:ALL
  lock_passwd: False
  ssh_authorized_keys:
    - ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIFxcXHmwaGnJ8scJaEN5RzklBPZpVSic4GdaAsKjQoeA your_email@example.com
packages:
  - qemu-guest-agent
runcmd:
  - systemctl enable qemu-guest-agent --now
  - chown -R cloud:cloud /home/cloud
```

Create a secret file containing `cloud-config` in base64 format.

```yaml
d8 k create -f - <<EOF
---
apiVersion: v1
data:
  userData: |
    `cat cloud-config | base64 -w0`
kind: Secret
metadata:
  name: secret-cloud-init
  namespace: test-project
type: provisioning.virtualization.deckhouse.io/cloud-init
EOF
```

Create a virtual machine:

```yaml
d8 k create -f - <<EOF
---
apiVersion: virtualization.deckhouse.io/v1alpha2
kind: VirtualImage
metadata:
  name: ubuntu-2204
  namespace: test-project
spec:
  storage: ContainerRegistry
  dataSource:
    type: HTTP
    http:
      url: https://cloud-images.ubuntu.com/jammy/current/jammy-server-cloudimg-amd64.img
---
apiVersion: virtualization.deckhouse.io/v1alpha2
kind: VirtualDisk
metadata:
  name: disk
  namespace: test-project
spec:
  dataSource:
    objectRef:
      kind: VirtualImage
      name: ubuntu-2204
    type: ObjectRef
  persistentVolumeClaim:
    size: 4G
---
apiVersion: virtualization.deckhouse.io/v1alpha2
kind: VirtualMachine
metadata:
  name: vm
  namespace: test-project
spec:
  provisioning:
    type: UserDataRef
    userDataRef:
      kind: Secret
      name: secret-cloud-init
  virtualMachineClassName: generic
  runPolicy: AlwaysOn
  blockDeviceRefs:
  - kind: VirtualDisk
    name: disk
  cpu:
    cores: 1
  memory:
    size: 1Gi
EOF
```

Display the list of virtual machines to get their status:

```shell
d8 k get vm -o wide
```

After a successful start, the virtual machine should change to the `Running` status.

Example of the output:

```console
NAME   PHASE     CORES   COREFRACTION   MEMORY   NEED RESTART   AGENT   MIGRATABLE   NODE           IPADDRESS     AGE
vm     Running   1       100%           1Gi      False          False   True         virtlab-pt-1   10.66.10.19   6m18s
```

Connect to the virtual machine, enter the login (in the example — `test-user`) and the password:

```shell
d8 v console -n test-project vm
```

To exit the console, press `Ctrl+]`.

Congratulations! You have created a virtual machine and connected to it.

<script type="text/javascript">
$(document).ready(function () {
    generate_password(true);
    update_parameter('dhctl-user-password-hash', 'password', '<GENERATED_PASSWORD_HASH>', null, null);
    update_parameter('dhctl-user-password-hash', null, '<GENERATED_PASSWORD_HASH>', null, '[project-rbac-yml]');
    update_parameter('dhctl-user-password', null, '<GENERATED_PASSWORD>', null, '[project-rbac-yml]');
    update_parameter('dhctl-user-password', null, '<GENERATED_PASSWORD>', null, 'code span.c1');
    update_domain_parameters();
    config_highlight();
});

</script>
