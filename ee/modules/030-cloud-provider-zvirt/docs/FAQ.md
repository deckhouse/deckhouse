---
title: "Cloud provider — zVirt: FAQ"
---

## How to get vNicProfileId

VNicProfileId can be obtained by querying the zVirt API

```bash
curl -u "<user>@<profile>:<password>" -X GET https://<zVirt API URL>/vnicprofiles
```

Response example

```xml
<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<vnic_profiles>
    <vnic_profile href="/ovirt-engine/api/vnicprofiles/49bb4594-0cd4-4eb7-8288-8594eafd5a86" id="49bb4594-0cd4-4eb7-8288-8594eafd5a86">
        <name>vm-net-01</name>
        <link href="/ovirt-engine/api/vnicprofiles/49bb4594-0cd4-4eb7-8288-8594eafd5a86/permissions" rel="permissions"/>
        <pass_through>
            <mode>disabled</mode>
        </pass_through>
        <port_mirroring>false</port_mirroring>
        <network href="/ovirt-engine/api/networks/74a741c9-0d40-4008-8e58-1c903ee6eba7" id="74a741c9-0d40-4008-8e58-1c903ee6eba7"/>
    </vnic_profile>
    ...
</vnic_profiles>
```
