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

package projectnamespace

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// The controller leaves a namespace of someone else under the resulting name alone, and a
// ProjectNamespace refused that namespace is deleted by removing its finalizer. That update goes
// through, while the name still refuses any other update.
func TestHandle_DeletionIgnoresANamespaceOfSomeoneElse(t *testing.T) {
	someoneElses := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{
		Name:   "team-a-backend",
		Labels: map[string]string{"app": "cache"},
	}}
	v := newValidator(t, projectObj("team-a", false), someoneElses)

	t.Run("the finalizer removal of a ProjectNamespace being deleted is allowed", func(t *testing.T) {
		deleting := pns("backend")
		deleting.DeletionTimestamp = &metav1.Time{Time: time.Now()}

		resp := v.Handle(context.Background(), updateRequest(t, "team-a", deleting))

		assert.True(t, resp.Allowed, resp.Result)
	})

	t.Run("an update of a ProjectNamespace that is not being deleted is refused", func(t *testing.T) {
		resp := v.Handle(context.Background(), updateRequest(t, "team-a", pns("backend")))

		assert.False(t, resp.Allowed)
		require.NotNil(t, resp.Result)
		assert.Contains(t, resp.Result.Message, `namespace "team-a-backend" already exists and is not owned by project "team-a"`)
	})
}
