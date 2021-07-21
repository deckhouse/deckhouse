# How to build base images
1. Create `./vsphere.auto.pkrvars.hcl` file with the following variables:
   ```hcl
   vcenter_server = "<Hostname of vSphere API>"
   vcenter_username = "<Username>"
   vcenter_password = "<Password>"
   vcenter_cluster = "<Cluster name where to run VM>"
   vcenter_datacenter = "<Datacenter where build process will perform>"
   vcenter_resource_pool = "<Resource pool path, where VM will be started>"
   vcenter_datastore = "<Datastore name to request disks in>"
   vcenter_folder = "<Templates folder, e.g. Templates>"
   vm_network = "<VM network name>"
   ```
1. If your machine has no direct access to the vSphere cluster (for instance you are using VPN Tunnel): replace `{{ .HTTPIP }}` with your tunnel interface IP address in `<UbuntuVersion>.pkrvars.hcl` following line: 
   ```hcl
   " url=http://{{ .HTTPIP }}:{{ .HTTPPort }}/preseed.cfg",
   ```
   > If VM wouldn't have access to the Packer HTTP server on your machine installation will stuck on plum colored screen.
1. Build images for the first time:
   ```shell
   ## For Ubuntu 20.04
   packer build --var-file=20.04.pkrvars.hcl .
   ## For Ubuntu 18.04
   packer build --var-file=18.04.pkrvars.hcl .
   ```
1. Subsequent builds of images:
   > ⚠️ Rebuilding exist images with the `-force` flag either deleting old ones and renaming new ones to old names is strictly not recommended, because it will trigger terraform to re-create VM's by template UID change.
   ```shell
   # For Ubuntu 20.04
   packer build --var-file=20.04.pkrvars.hcl -var "image_name_suffix=-new" .
   # For Ubuntu 18.04
   packer build --var-file=18.04.pkrvars.hcl -var "image_name_suffix=-new" .
   ```
1. Clone resulting templates to all other datacenters in vSphere, keeping the same name:
   1. vSphere Client &rarr; VMs and Templates `(ctrl + alt/option + 3)`.
   1. Right-click on particular template &rarr; Clone to Template.
   1. Fill `VM template name:` field with the same template name.
   1. Select a location for the template &mdash; targeting datacenter's Template folder.
   1. Select a compute resource &mdash; targeting cluster in datacenter.
   1. Select storage &mdash; select target storage.
   1. Wait for a while until the cloning process finished.
