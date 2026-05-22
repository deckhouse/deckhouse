# CSE edit

## openapi/config-values.yaml

1. remove .allowDeleteLogs

We do not allow logs to be deleted. We chose the OpenAPI option because it cannot be bypassed without rebuilding the controller image.

ValidatingAdmissionPolicy is not suitable because it can be bypassed if someone is determined to do so.

2. set .diskSizeGigabytes.default: 50
3. set .lokiConfig.ingestionBurstSizeMB.default: 100
4. set .lokiConfig.ingestionRateMB.default: 50
5. set .lokiConfig.perStreamRateLimit.default: 30MB
6. set .lokiConfig.perStreamRateLimitBurst.default: 50MB
