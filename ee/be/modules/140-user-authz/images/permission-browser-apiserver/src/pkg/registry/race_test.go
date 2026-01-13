/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package registry

import (
	"context"
	"sync"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/authorization/authorizer"
	"k8s.io/apiserver/pkg/endpoints/request"

	"permission-browser-apiserver/pkg/apis/authorization/v1alpha1"
)

// mockAuthorizerForRace implements authorizer.Authorizer for race testing
type mockAuthorizerForRace struct {
	decision authorizer.Decision
}

func (m *mockAuthorizerForRace) Authorize(ctx context.Context, attrs authorizer.Attributes) (authorizer.Decision, string, error) {
	return m.decision, "mock", nil
}

// TestBulkSARStorage_ConcurrentCreate tests that concurrent Create calls don't race
func TestBulkSARStorage_ConcurrentCreate(t *testing.T) {
	auth := &mockAuthorizerForRace{decision: authorizer.DecisionAllow}
	storage := NewBulkSARStorage(auth)

	const goroutines = 100
	const iterations = 50

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func(id int) {
			defer wg.Done()

			for j := 0; j < iterations; j++ {
				// Create a new BulkSubjectAccessReview for each request
				bsar := &v1alpha1.BulkSubjectAccessReview{
					Spec: v1alpha1.BulkSubjectAccessReviewSpec{
						Requests: []v1alpha1.SubjectAccessReviewRequest{
							{
								ResourceAttributes: &v1alpha1.ResourceAttributes{
									Namespace: "default",
									Verb:      "get",
									Resource:  "pods",
								},
							},
							{
								ResourceAttributes: &v1alpha1.ResourceAttributes{
									Namespace: "kube-system",
									Verb:      "list",
									Resource:  "secrets",
								},
							},
							{
								NonResourceAttributes: &v1alpha1.NonResourceAttributes{
									Path: "/healthz",
									Verb: "get",
								},
							},
						},
					},
				}

				// Create context with user info
				userInfo := &user.DefaultInfo{
					Name:   "test-user",
					Groups: []string{"system:authenticated"},
				}
				ctx := request.WithUser(context.Background(), userInfo)

				result, err := storage.Create(ctx, bsar, nil, &metav1.CreateOptions{})
				if err != nil {
					t.Errorf("goroutine %d iteration %d: unexpected error: %v", id, j, err)
					return
				}

				resultBsar := result.(*v1alpha1.BulkSubjectAccessReview)
				if len(resultBsar.Status.Results) != 3 {
					t.Errorf("goroutine %d iteration %d: expected 3 results, got %d", id, j, len(resultBsar.Status.Results))
				}
			}
		}(i)
	}

	wg.Wait()
}

// TestBulkSARStorage_ConcurrentCreateWithDifferentUsers tests concurrent access with different users
func TestBulkSARStorage_ConcurrentCreateWithDifferentUsers(t *testing.T) {
	auth := &mockAuthorizerForRace{decision: authorizer.DecisionAllow}
	storage := NewBulkSARStorage(auth)

	const goroutines = 50

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func(id int) {
			defer wg.Done()

			// Self mode (no user specified in spec)
			bsarSelf := &v1alpha1.BulkSubjectAccessReview{
				Spec: v1alpha1.BulkSubjectAccessReviewSpec{
					Requests: []v1alpha1.SubjectAccessReviewRequest{
						{
							ResourceAttributes: &v1alpha1.ResourceAttributes{
								Verb:     "get",
								Resource: "pods",
							},
						},
					},
				},
			}

			userInfo := &user.DefaultInfo{
				Name:   "user-" + string(rune('A'+id%26)),
				Groups: []string{"group-" + string(rune('A'+id%10))},
			}
			ctx := request.WithUser(context.Background(), userInfo)

			_, err := storage.Create(ctx, bsarSelf, nil, &metav1.CreateOptions{})
			if err != nil {
				t.Errorf("goroutine %d self mode: unexpected error: %v", id, err)
				return
			}

			// Non-self mode (user specified in spec)
			bsarNonSelf := &v1alpha1.BulkSubjectAccessReview{
				Spec: v1alpha1.BulkSubjectAccessReviewSpec{
					User:   "other-user-" + string(rune('A'+id%26)),
					Groups: []string{"other-group"},
					Requests: []v1alpha1.SubjectAccessReviewRequest{
						{
							ResourceAttributes: &v1alpha1.ResourceAttributes{
								Verb:     "list",
								Resource: "secrets",
							},
						},
					},
				},
			}

			_, err = storage.Create(ctx, bsarNonSelf, nil, &metav1.CreateOptions{})
			if err != nil {
				t.Errorf("goroutine %d non-self mode: unexpected error: %v", id, err)
				return
			}
		}(i)
	}

	wg.Wait()
}
