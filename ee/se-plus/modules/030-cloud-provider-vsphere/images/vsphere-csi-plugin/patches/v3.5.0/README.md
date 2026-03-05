## 001-go-mod.patch

Bump versions and fix CVE

## 002-replace-gofsutil.patch

This is the gofsutils library patch for our fork, which removed the standard 5% block reservation limit for csi volumes: https://github.com/kubernetes-sigs/vsphere-csi-driver/issues/2713

## 003-fetch-hosts-by-datastore.patch

This patch adds the ability to obtain the topology of attached hosts to the datastore