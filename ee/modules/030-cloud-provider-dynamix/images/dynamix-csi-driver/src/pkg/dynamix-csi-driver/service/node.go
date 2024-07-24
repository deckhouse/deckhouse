/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package service

type NodeService struct{}

func NewNode() *NodeService {
	return &NodeService{}
}
