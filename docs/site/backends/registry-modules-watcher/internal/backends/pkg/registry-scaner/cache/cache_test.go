package cache

import (
	"fmt"
	"testing"
)

func TestSyncReleaseChannels(t *testing.T) {
	c := New()
	// c := Cache{
	// 	val: make(map[registryName]map[moduleName]module),
	// }
	c.SetTar("TestReg", "testModule", "1.0.0", "alpha", []byte("test"))
	c.SetTar("TestReg", "testModule", "1.0.1", "beta", []byte("test2"))
	c.SetTar("TestReg", "testModule", "1.0.1", "alpha", []byte("test"))

	c.SetReleaseChecksum("TestReg", "testModule", "alpha", "test checksumm")
	fmt.Println(c)
}
