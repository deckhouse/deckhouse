# Provide credentials in `vsphere.auto.pkrvars.hcl`.
vcenter_server = "<Hostname of vSphere API>"
vcenter_username = "<Username>"
vcenter_password = "<Password>"
vcenter_cluster = "<Cluster name where to run VM>"
vcenter_datacenter = "<Datacenter where build process will perform>"
vcenter_resource_pool = "<Resource pool path, where VM will be started>"
vcenter_datastore = "<Datastore name to request disks in>"
vcenter_folder = "<Templates folder, e.g. Templates>"
vm_network = "<VM network name>"

# Provide your machine IP address in `<UbuntuVersion>.pkrvars.hcl`, for instance if you are using VPN Tunnel.
# If VM wouldn't have access to Packer HTTP server on your machine installation will stuck on plum colored screen.
{{ .HTTPIP }} => <Your machine IP, accessible from VM>

# Run build.
## Ubuntu 20.04
packer build -force --var-file=20.04.pkrvars.hcl .
## Ubuntu 18.04
packer build -force --var-file=18.04.pkrvars.hcl .
