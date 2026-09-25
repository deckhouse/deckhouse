// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package actions

import (
	"bytes"
	"context"
	"testing"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/stretchr/testify/require"

	dhlog "github.com/deckhouse/lib-dhctl/pkg/logger"
)

func alreadyExists() error {
	return apierrors.NewAlreadyExists(schema.GroupResource{Resource: "secrets"}, "d8-cluster-uuid")
}

// ctxWithStreamLogger renders through the plain sink, the way a terminal sees it.
func ctxWithStreamLogger() (context.Context, *bytes.Buffer) {
	var buf bytes.Buffer
	return dhlog.ToContext(context.Background(), dhlog.NewStreamLogger(&buf)), &buf
}

// TestUpdateFailureSaysWhatFailed: an update that fails used to log the bare word "ERROR!" - the
// tail half of a single-line style ("X already exists. Trying to update ... OK!") that the logger
// now splits into separate records. Error level pins it and carries it into the closing summary,
// so a bootstrap that retried a conflicting update seven times and then succeeded signed off with
// seven contextless ERROR! lines above "Deckhouse cluster created successfully!".
//
// The failure is reported by the error this returns, which the retry loop logs per attempt and
// carries into its exhaustion message. The marker only ever restated it, and without the text.
func TestUpdateFailureSaysWhatFailed(t *testing.T) {
	ctx, buf := ctxWithStreamLogger()

	task := ManifestTask{
		Name:       `Secret "d8-cluster-uuid"`,
		Manifest:   func() any { return &metav1.ObjectMeta{} },
		CreateFunc: func(context.Context, any) error { return alreadyExists() },
		UpdateFunc: func(context.Context, any) error { return apierrors.NewConflict(schema.GroupResource{Resource: "secrets"}, "d8-cluster-uuid", nil) },
	}

	err := task.CreateOrUpdate(ctx)

	require.Error(t, err)
	require.ErrorIs(t, err, ErrManifestTaskTransient, "a conflict is worth retrying")
	require.Contains(t, err.Error(), "update resource", "the error must say what failed")

	out := buf.String()
	require.NotContains(t, out, "ERROR!",
		"a retried attempt must not shout a contextless ERROR into the summary: %q", out)
	require.Contains(t, out, `Secret "d8-cluster-uuid" already exists, updating`,
		"the line that does print must say what is happening: %q", out)
}

// The success half of the same broken line: "OK!" on its own says nothing once it is a record of
// its own rather than the tail of the line above it.
func TestUpdateSuccessSaysWhatSucceeded(t *testing.T) {
	ctx, buf := ctxWithStreamLogger()

	task := ManifestTask{
		Name:       `Secret "d8-cluster-uuid"`,
		Manifest:   func() any { return &metav1.ObjectMeta{} },
		CreateFunc: func(context.Context, any) error { return alreadyExists() },
		UpdateFunc: func(context.Context, any) error { return nil },
	}

	require.NoError(t, task.CreateOrUpdate(ctx))

	out := buf.String()
	require.NotContains(t, out, "OK!", "a bare OK! names nothing: %q", out)
	require.Contains(t, out, `Secret "d8-cluster-uuid" updated`, "say what was updated: %q", out)
}
