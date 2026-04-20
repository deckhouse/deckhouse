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

package usecase

import (
	"context"

	"fencing-agent/internal/domain"
)

type NodesGetter interface {
	GetNodes(ctx context.Context) (domain.Nodes, error)
}

type GetNodes struct {
	nodesGetter NodesGetter
}

func NewGetNodes(ng NodesGetter) *GetNodes {
	return &GetNodes{nodesGetter: ng}
}

func (gn *GetNodes) GetNodes(ctx context.Context) (domain.Nodes, error) {
	nodes, err := gn.nodesGetter.GetNodes(ctx)
	return nodes, err
}
