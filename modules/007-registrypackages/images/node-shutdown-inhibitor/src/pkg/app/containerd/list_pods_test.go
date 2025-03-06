/*
Copyright 2025 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package containerd

import "testing"

func Test_podListJson(t *testing.T) {
	in := `
{
  "items": [
    {
      "id": "e98c6d0d5c306402e1887c6b1a41d062b08122a18f665a2ac2d12b1408dc3c69",
      "metadata": {
        "name": "upmeter-probe-scheduler-e41a54f7",
        "uid": "35a8a266-f484-4d71-adc3-9850e1254e83",
        "namespace": "d8-upmeter",
        "attempt": 0
      },
      "state": "SANDBOX_READY",
      "createdAt": "1738902802403931439",
      "labels": {
        "heritage": "upmeter",
        "io.kubernetes.pod.name": "upmeter-probe-scheduler-e41a54f7",
        "io.kubernetes.pod.namespace": "d8-upmeter",
        "io.kubernetes.pod.uid": "35a8a266-f484-4d71-adc3-9850e1254e83",
        "upmeter-agent": "e41a54f7",
        "upmeter-group": "control-plane",
        "upmeter-probe": "scheduler"
      },
      "annotations": {
        "kubernetes.io/config.seen": "2025-02-07T07:33:22.057843914+03:00",
        "kubernetes.io/config.source": "api"
      },
      "runtimeHandler": ""
    },
    {
      "id": "68c7b8ee3bae91badbec7fdb6ed8f867f04897213286e63ecaa337bf9db6fa6e",
      "metadata": {
        "name": "d8-etcd-backup-d4d6b7e172a7767af18e645d178aadf77-28981440-4k2np",
        "uid": "51faffda-1a83-45e8-997f-968000859af1",
        "namespace": "kube-system",
        "attempt": 0
      },
      "state": "SANDBOX_NOTREADY",
      "createdAt": "1738886400475228091",
      "labels": {
        "batch.kubernetes.io/controller-uid": "599b9603-7d0e-4b8b-b38c-65ae7a67c161",
        "batch.kubernetes.io/job-name": "d8-etcd-backup-d4d6b7e172a7767af18e645d178aadf77-28981440",
        "controller-uid": "599b9603-7d0e-4b8b-b38c-65ae7a67c161",
        "io.kubernetes.pod.name": "d8-etcd-backup-d4d6b7e172a7767af18e645d178aadf77-28981440-4k2np",
        "io.kubernetes.pod.namespace": "kube-system",
        "io.kubernetes.pod.uid": "51faffda-1a83-45e8-997f-968000859af1",
        "job-name": "d8-etcd-backup-d4d6b7e172a7767af18e645d178aadf77-28981440"
      },
      "annotations": {
        "kubernetes.io/config.seen": "2025-02-07T03:00:00.131235836+03:00",
        "kubernetes.io/config.source": "api"
      },
      "runtimeHandler": ""
    }
  ]
}`
	podList, err := podsListFromJSON([]byte(in))
	if err != nil {
		t.Fatalf("should unmarshal json into PodList: %v", err)
	}

	if len(podList.Items) != 2 {
		t.Fatalf("should have 2 items, got %d", len(podList.Items))
	}

	if podList.Items[0].Labels["heritage"] != "upmeter" {
		t.Fatalf("should have label 'heritage' with value 'upmeter', got %s", podList.Items[0].Labels)
	}
}
