/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package kubernetes

import "testing"

func Test_podListJRson(t *testing.T) {
	in := `
{
  "items": [
    {
      "metadata": {
        "name": "upmeter-probe-scheduler-e41a54f7",
        "uid": "35a8a266-f484-4d71-adc3-9850e1254e83",
        "namespace": "d8-upmeter",
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
        }
      },
      "spec": {}
    },
    {
      "metadata": {
        "name": "d8-etcd-backup-d4d6b7e172a7767af18e645d178aadf77-28981440-4k2np",
        "uid": "51faffda-1a83-45e8-997f-968000859af1",
        "namespace": "kube-system",
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
        }
      },
      "spec": {}
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

	if podList.Items[0].Metadata.Labels["heritage"] != "upmeter" {
		t.Fatalf("should have label 'heritage' with value 'upmeter', got %s", podList.Items[0].Metadata.Labels)
	}
}
