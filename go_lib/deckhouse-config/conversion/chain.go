/*
Copyright 2022 Flant JSC

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

package conversion

import (
	"fmt"
	"sync"
)

// Chain is a chain of conversions for module.
type Chain struct {
	m sync.RWMutex

	moduleName string

	// version -> convertor
	conversions map[int]*Conversion

	latestVersion int
}

func NewChain(moduleName string) *Chain {
	return &Chain{
		moduleName:  moduleName,
		conversions: make(map[int]*Conversion),
	}
}

func (c *Chain) Add(conversion *Conversion) {
	c.m.Lock()
	defer c.m.Unlock()

	c.conversions[conversion.Source] = conversion

	// Update latest version.
	if c.latestVersion == 0 || conversion.Target > c.latestVersion {
		c.latestVersion = conversion.Target
	}
}

func (c *Chain) ConvertToLatest(fromVersion int, values map[string]interface{}) (int, map[string]interface{}, error) {
	// Shortcut: return if fromVersion is already the latest.
	if fromVersion == c.latestVersion {
		return fromVersion, values, nil
	}

	c.m.Lock()
	defer c.m.Unlock()

	// Error if version has no registered conversions.
	if len(c.conversions) > 0 {
		if _, has := c.conversions[fromVersion]; !has {
			return 0, nil, fmt.Errorf("version %d is unknown", fromVersion)
		}
	}

	maxTries := len(c.conversions)

	tries := 0
	currentVersion := fromVersion
	currentValues, err := SettingsFromMap(values)
	if err != nil {
		return 0, nil, fmt.Errorf("bad input values: %v", err)
	}

	for {
		conv := c.conversions[currentVersion]
		if conv == nil {
			return 0, nil, fmt.Errorf("convert from %d: conversion chain interrupt: no conversion from %d", fromVersion, currentVersion)
		}
		newVer := conv.Target
		newValues, err := conv.Convert(currentValues)
		if err != nil {
			return 0, nil, fmt.Errorf("convert from %d: conversion chain error for %d: %v", fromVersion, currentVersion, err)
		}

		// Stop after converting to the latest version.
		if newVer == c.latestVersion {
			newMap, err := newValues.Map()
			if err != nil {
				return 0, nil, fmt.Errorf("convert from %d: map error for %d: %v", fromVersion, currentVersion, err)
			}
			return newVer, newMap, nil
		}

		currentVersion = newVer
		currentValues = newValues

		// Prevent looped conversions.
		tries++
		if tries > maxTries {
			return 0, nil, fmt.Errorf("convert from %d: conversion chain too long or looped", fromVersion)
		}
	}
}

func (c *Chain) Conversion(srcVersion int) *Conversion {
	c.m.RLock()
	defer c.m.RUnlock()

	return c.conversions[srcVersion]
}

func (c *Chain) LatestVersion() int {
	return c.latestVersion
}

// Count returns a number of registered conversions for the module.
func (c *Chain) Count() int {
	c.m.RLock()
	defer c.m.RUnlock()

	return len(c.conversions)
}

// IsKnownVersion returns whether version has registered conversion or the latest.
func (c *Chain) IsKnownVersion(version int) bool {
	c.m.RLock()
	defer c.m.RUnlock()

	_, has := c.conversions[version]
	if has {
		return true
	}
	return version == c.latestVersion
}

// VersionList returns all valid versions (all previous and the latest).
func (c *Chain) VersionList() []int {
	c.m.RLock()
	defer c.m.RUnlock()
	versions := make([]int, 0)
	for ver := range c.conversions {
		versions = append(versions, ver)
	}
	versions = append(versions, c.latestVersion)
	return versions
}

// PreviousVersionsList returns supported previous versions.
func (c *Chain) PreviousVersionsList() []int {
	c.m.RLock()
	defer c.m.RUnlock()

	versions := make([]int, 0)
	for ver := range c.conversions {
		versions = append(versions, ver)
	}
	return versions
}

// NewNoConvChain return a chain with the latestVersion equal to 1 for modules without registered conversions.
func NewNoConvChain(moduleName string) *Chain {
	return &Chain{
		moduleName:    moduleName,
		conversions:   make(map[int]*Conversion),
		latestVersion: 1,
	}
}
