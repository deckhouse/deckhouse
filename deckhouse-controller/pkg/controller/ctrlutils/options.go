// Copyright 2024 Flant JSC
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

package ctrlutils

import (
	"k8s.io/apimachinery/pkg/util/wait"
)

type UpdateOption func(optionsApplier UpdateOptionApplier)

func (opt UpdateOption) Apply(o UpdateOptionApplier) {
	opt(o)
}

type UpdateOptionApplier interface {
	WithOnErrorBackoff(b *wait.Backoff)
	WithRetryOnConflictBackoff(b *wait.Backoff)

	withStatusUpdate()
}

func WithOnErrorBackoff(b *wait.Backoff) UpdateOption {
	return func(optionsApplier UpdateOptionApplier) {
		optionsApplier.WithOnErrorBackoff(b)
	}
}

func WithRetryOnConflictBackoff(b *wait.Backoff) UpdateOption {
	return func(optionsApplier UpdateOptionApplier) {
		optionsApplier.WithRetryOnConflictBackoff(b)
	}
}

func withStatusUpdate() UpdateOption {
	return func(optionsApplier UpdateOptionApplier) {
		optionsApplier.withStatusUpdate()
	}
}
