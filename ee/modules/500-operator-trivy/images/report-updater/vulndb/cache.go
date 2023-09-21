/*
Copyright 2023 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package vulndb

import (
	"log"
	"sync"
	"time"

	"report-updater/vulndb/parser"
)

const (
	defaultTTL = 24 * time.Hour
	bduPath    = "/tmp/vulndb-cache/export.xml"
)

type Cache interface {
	Get(string) (Vulnerability, bool)
	Check() error
}

type Vulnerability struct {
	OSs []string
	IDs []string
}

type VulnDbCache struct {
	logger *log.Logger

	mu   sync.RWMutex
	data map[string]Vulnerability
}

func NewVulnDbCache(logger *log.Logger) (*VulnDbCache, error) {
	c := &VulnDbCache{
		logger: logger,
		data:   make(map[string]Vulnerability),
	}

	err := c.initCache()
	if err != nil {
		return nil, err
	}

	return c, nil
}

// think about healthz check
func (c *VulnDbCache) Check() error {
	return nil
}

func (c *VulnDbCache) initCache() error {
	c.logger.Println("init cache")
	err := parser.DownloadAndExtractBdu(bduPath)
	if err != nil {
		c.logger.Println("failed to download and extract BDU")
		return err
	}

	c.logger.Println("parse BDU")
	bdu, err := parser.Parse(bduPath)
	if err != nil {
		c.logger.Println("failed to parse BDU")
		return err
	}
	cache := make(map[string]Vulnerability)

	for _, entry := range bdu.Entries {
		for _, id := range entry.CveIDs {
			if obj, ok := cache[id]; ok {
				obj.IDs = append(obj.IDs, entry.BduID)
				cache[id] = obj
			} else {
				cache[id] = Vulnerability{IDs: []string{entry.BduID}}
			}
		}
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.data = cache

	return nil
}

func (c *VulnDbCache) Get(vulnerability string) (Vulnerability, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, ok := c.data[vulnerability]
	return entry, ok
}
