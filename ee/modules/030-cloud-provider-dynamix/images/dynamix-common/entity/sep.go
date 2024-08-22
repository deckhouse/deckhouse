/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package entity

type StorageEndpoint struct {
	ID        uint64
	Name      string
	IsActive  bool
	IsCreated bool
	Pools     []Pool
}

type Pool struct {
	Name string
}
