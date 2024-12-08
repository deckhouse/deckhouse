<script type="text/javascript" src='{{ assets["getting-started.js"].digest_path }}'></script>
<script type="text/javascript" src='{{ assets["getting-started-access.js"].digest_path }}'></script>
<script type="text/javascript" src='{{ assets["bcrypt.js"].digest_path }}'></script>

Create a project and a project administrator (in the example, the project `test-project` and the user `test-user@deckhouse.io` are used, change them if necessary):

{% snippetcut selector="project-rbac-yml" %}
{% include_file "_includes/getting_started/dvp/{{ page.platform_code }}/partials/project-rbac.yml.inc" syntax="yaml" %}
{% endsnippetcut %}

Open the web interface for generating the kubeconfig file for remote access to the API server. The address of the web interface is formed according to the DNS name template specified in the global parameter [publicDomainTemplate](/products/virtualization-platform/reference/mc.html#parameters-modules-publicdomaintemplate). For example, if `publicDomainTemplate: %s.kube.my`, then the web interface will be available at the address `kubeconfig.kube.my`.
 
Enter the login (in the example — `test-user@deckhouse.io`) and the password of the created user to obtain the configuration file for access to the cluster:

On a computer with network access to the deployed cluster, create a file `~/.kube/config` (for Linux/MacOS) or `%USERPROFILE%\.kube\config` (for Windows) and paste the kubectl configuration provided in the *Raw Config* tab.

You have configured kubectl on this computer to manage the cluster. Execute the further commands on this computer.

Create a virtual machine:

{% snippetcut %}
```yaml
kubectl create -f - <<EOF
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
{% endsnippetcut %}

Display the list of virtual machines to get their status:

{% snippetcut %}
```shell
kubectl get vm -o wide
```
{% endsnippetcut %}

After a successful start, the virtual machine should change to the `Running` status.

Example of the output:

```console
# kubectl get vm -o wide
NAME   PHASE     CORES   COREFRACTION   MEMORY   NEED RESTART   AGENT   MIGRATABLE   NODE           IPADDRESS     AGE
vm     Running   1       100%           1Gi      False          False   True         virtlab-pt-1   10.66.10.19   6m18s
```

Connect to the virtual machine, enter the login (in the example — `test-user@deckhouse.io`) and the password:

{% snippetcut %}
```shell
d8 v console -n test-project vm
```
{% endsnippetcut %}

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
