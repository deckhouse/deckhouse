## 001-go-mod.patch

+ Bump go.mod dependencies to fix known CVEs.
+ This is the gofsutils library patch for our fork, which removed the standard 5% block reservation limit for csi volumes: https://github.com/kubernetes-sigs/vsphere-csi-driver/issues/2713
