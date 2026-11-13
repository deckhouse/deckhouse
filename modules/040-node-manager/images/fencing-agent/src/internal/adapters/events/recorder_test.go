/*
Copyright 2026 Flant JSC

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

package events

import (
	"context"
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/deckhouse/deckhouse/pkg/log"
)

func TestEventsStartWithThePrefix(t *testing.T) {
	client := fake.NewClientset()
	recorder := New(client, "w1", log.NewNop())

	recorder.Normal("Armed", "watchdog is armed")
	recorder.Warning("KernelPanicMismatch", "kernel.panic is not 0")
	defer recorder.Shutdown()

	var list *corev1.EventList

	// The broadcaster delivers the events to the sink in the background.
	for range 200 {
		var err error

		list, err = client.CoreV1().Events(metav1.NamespaceAll).List(context.Background(), metav1.ListOptions{})
		if err != nil {
			t.Fatalf("list events: %v", err)
		}

		if len(list.Items) == 2 {
			break
		}

		time.Sleep(10 * time.Millisecond)
	}

	if len(list.Items) != 2 {
		t.Fatalf("got %d events, want 2", len(list.Items))
	}

	for _, event := range list.Items {
		if !strings.HasPrefix(event.Message, "[fencing] ") {
			t.Errorf("event %s message %q has no prefix", event.Reason, event.Message)
		}

		if event.InvolvedObject.Kind != "Node" || event.InvolvedObject.Name != "w1" {
			t.Errorf("event %s is about %+v, want Node w1", event.Reason, event.InvolvedObject)
		}
	}

	types := map[string]string{}
	for _, event := range list.Items {
		types[event.Reason] = event.Type
	}

	if types["Armed"] != corev1.EventTypeNormal || types["KernelPanicMismatch"] != corev1.EventTypeWarning {
		t.Errorf("event types are %v", types)
	}
}
