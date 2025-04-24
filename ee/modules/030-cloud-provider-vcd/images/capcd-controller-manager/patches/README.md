### 001-our-machinery.patch
This patch is for our usage cases of cluster-api cloud provider.

### 002-patch-webhook-server-port.patch
Change webhook server port to 4201
Implement simple wrapper to redirect klog logs to zap
Set zap as klog/v2 logger

### 003-go-mod.patch

Update dependencies
Add klogv2 as direct dependency
