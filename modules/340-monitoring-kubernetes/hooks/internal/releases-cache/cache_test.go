/*
Copyright 2024 Flant JSC

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

package releasescache

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

const (
	timeToSleepAfterCashing = 5 // sec
	timeDeepToGetCache      = 3 // sec
)

func Test(t *testing.T) {
	GetInstance().Set(5, 3, []byte("test releases"))
	helm3Releases, helm2Releases, byteReleases, err := GetInstance().Get(timeDeepToGetCache * time.Second)
	assert.Equal(t, uint32(5), helm3Releases, "expected helm3Releases count %v but got %v", 5, helm3Releases)
	assert.Equal(t, uint32(3), helm2Releases, "expected helm2Releases count %v but got %v", 3, helm2Releases)
	assert.Equal(t, []byte("test releases"), byteReleases, "expected byteReleases %v but got %v", []byte("test releases"), byteReleases)
	assert.Nil(t, err, "expected that err must be nil, but got %v", err)

	time.Sleep(timeToSleepAfterCashing * time.Second)
	helm3Releases, helm2Releases, byteReleases, err = GetInstance().Get(timeDeepToGetCache * time.Second)
	assert.Equal(t, uint32(0), helm3Releases, "expected helm3Releases count %v but got %v", 0, helm3Releases)
	assert.Equal(t, uint32(0), helm2Releases, "expected helm2Releases count %v but got %v", 0, helm2Releases)
	assert.Nil(t, byteReleases, "expected byteReleases must be nil but got %v", byteReleases)
	assert.Equal(t, errors.New("time to live expired"), err, "expected err: \"time to live expired\", but got %v", err)
}
