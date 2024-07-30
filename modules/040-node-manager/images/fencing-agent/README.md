# fencing-agent

## How it works

The agent is designed to control Kubernetes API and to shut down a node in case of its unavailability.

The order of operation of the agent is as follows:

1. After startup, the agent checks the availability of the Kubernetes API.
2. The agent analyzes the node annotations to determine if the node is in maintenance mode.
3. If the node is in maintenance mode, the watchdog is terminated using the standard procedure, otherwise the watchdog is activated.
4. If API access is available, normal operation is signaled to the watchdog device.
5. The agent catches the signals and correctly terminates the work with watchdog when they are received.

## How to test

1. Create watchdog test file `touch ./test-watchdog-device`
2. Create `.env` file

```bash
WATCHDOG_DEVICE=./test-watchdog-device
NODE_NAME=<k8s-cluster-node-name-for-testing>
KUBECONFIG=<path to kube config>
LOG_LEVEL=DEBUG
```

3. `go run cmd/main.go`
4. Disable fencing `kubectl annotate node ${NODE_NAME} node-manager.deckhouse.io/fencing-disable=""`
5. Enable fencing `kubectl annotate node ${NODE_NAME} node-manager.deckhouse.io/fencing-disable-`

Example output:

```json
{"level":"debug","timestamp":"2024-02-25T11:22:23.501+0300","msg":"Current config","config":{"WatchdogDevice":"./test-watchdog-device","WatchdogFeedInterval":5000000000,"KubernetesAPICheckInterval":5000000000,"KubernetesAPITimeout":10000000000,"HealthProbeBindAddress":":8081","NodeName":"virtlab-pt-1"}}
{"level":"info","timestamp":"2024-02-25T11:22:23.507+0300","msg":"Starting the healthz server","node":"virtlab-pt-1"}
{"level":"debug","timestamp":"2024-02-25T11:22:28.591+0300","msg":"The API is available","node":"virtlab-pt-1"}
{"level":"info","timestamp":"2024-02-25T11:22:28.591+0300","msg":"Arm the watchdog","node":"virtlab-pt-1"}
{"level":"info","timestamp":"2024-02-25T11:22:28.591+0300","msg":"Set fencing node label","node":"virtlab-pt-1","label":"node-manager.deckhouse.io/fencing-enabled"}
{"level":"debug","timestamp":"2024-02-25T11:22:28.692+0300","msg":"Watchdog status","node":"virtlab-pt-1","is armed":true}
{"level":"debug","timestamp":"2024-02-25T11:22:28.692+0300","msg":"Feeding the watchdog","node":"virtlab-pt-1"}
{"level":"debug","timestamp":"2024-02-25T11:22:33.546+0300","msg":"The API is available","node":"virtlab-pt-1"}
{"level":"debug","timestamp":"2024-02-25T11:22:33.546+0300","msg":"Watchdog status","node":"virtlab-pt-1","is armed":true}
{"level":"debug","timestamp":"2024-02-25T11:22:33.546+0300","msg":"Feeding the watchdog","node":"virtlab-pt-1"}
^C{"level":"info","timestamp":"2024-02-25T11:22:37.368+0300","msg":"Got a signal","signal":"interrupt"}
{"level":"debug","timestamp":"2024-02-25T11:22:37.368+0300","msg":"Finishing the API check","node":"virtlab-pt-1"}
{"level":"info","timestamp":"2024-02-25T11:22:37.368+0300","msg":"Remove fencing node label","node":"virtlab-pt-1","label":"node-manager.deckhouse.io/fencing-enabled"}
{"level":"info","timestamp":"2024-02-25T11:22:37.465+0300","msg":"Disarm the watchdog","node":"virtlab-pt-1"}
```
