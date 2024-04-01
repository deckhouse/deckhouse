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

package internal

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/gob"
	"encoding/json"
	"io"
	"sync"
	"time"

	"github.com/golang/protobuf/proto" // nolint: staticcheck
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/go_lib/dependency/k8s"
	releasescache "github.com/deckhouse/deckhouse/modules/340-monitoring-kubernetes/hooks/internal/releases-cache"
)

const (
	// objectBatchSize - how many secrets to list from k8s at once
	objectBatchSize = int64(10)
	// fetchSecretsInterval pause between fetching the helm secrets from apiserver
	// need for avoiding apiserver overload
	fetchSecretsInterval = 3 * time.Second
)

type Interval int64

const (
	IntervalImmediately Interval = 0
	IntervalMinutes15   Interval = 15
	IntervalMinutes30   Interval = 30
	IntervalHours1      Interval = 60
)

type Release struct {
	Name      string `json:"name,omitempty" protobuf:"bytes,1,opt,name=name,proto3"`
	Namespace string `json:"namespace,omitempty" protobuf:"bytes,8,opt,name=namespace,proto3"`
	Manifest  string `json:"manifest,omitempty" protobuf:"bytes,5,opt,name=manifest,proto3"`

	// set helm version manually
	HelmVersion string `json:"-"`
}

func GetHelmReleases(ctx context.Context, client k8s.Client, releasesC chan<- *Release, interval Interval) (helm3Releases, helm2Releases uint32, err error) {
	var (
		wg                 sync.WaitGroup
		helm3ReleasesCount uint32
		helm2ReleasesCount uint32

		releasesBuffChan = make(chan *Release, objectBatchSize)
	)

	if helm3Releases, helm2Releases, b, err := releasescache.GetInstance().Get(time.Duration(interval) * time.Minute); err != nil {
		releases := []Release{}
		dec := gob.NewDecoder(bytes.NewBuffer(b))
		_ = dec.Decode(&releases)

		for _, release := range releases {
			releasesC <- &release
		}
		close(releasesC)

		return helm3Releases, helm2Releases, nil
	}

	wg.Add(2)
	go func() {
		defer wg.Done()
		var err error
		helm3ReleasesCount, err = getHelm3Releases(ctx, client, releasesBuffChan)
		if err != nil {
			// input.LogEntry.Error(err) // TODO
			return
		}
	}()

	go func() {
		defer wg.Done()
		var err error
		helm2ReleasesCount, err = getHelm2Releases(ctx, client, releasesBuffChan)
		if err != nil {
			// input.LogEntry.Error(err) // TODO
			return
		}
	}()

	go func() {
		// snap to cache
		var releases []Release
		for release := range releasesBuffChan {
			releases = append(releases, *release)
			releasesC <- release
		}

		var buff bytes.Buffer
		enc := gob.NewEncoder(&buff)
		_ = enc.Encode(releases)
		releasescache.GetInstance().Set(helm3ReleasesCount, helm2ReleasesCount, buff.Bytes())

		close(releasesC)
	}()

	wg.Wait()
	close(releasesBuffChan)

	return helm3ReleasesCount, helm2ReleasesCount, nil
}

func getHelm3Releases(ctx context.Context, client k8s.Client, releasesC chan<- *Release) (uint32, error) {
	var totalReleases uint32
	var next string

	for {
		secretsList, err := client.CoreV1().Secrets("").List(ctx, metav1.ListOptions{
			LabelSelector: "owner=helm,status=deployed",
			Limit:         objectBatchSize,
			Continue:      next,
			// https://kubernetes.io/docs/reference/using-api/api-concepts/#semantics-for-get-and-list
			// set explicit behavior:
			//   Return data at any resource version. The newest available resource version is preferred, but strong consistency is not required; data at any resource version may be served.
			ResourceVersion:      "0",
			ResourceVersionMatch: metav1.ResourceVersionMatchNotOlderThan,
		})
		if err != nil {
			return 0, err
		}

		for _, secret := range secretsList.Items {
			releaseData := secret.Data["release"]
			if len(releaseData) == 0 {
				continue
			}

			release, err := helm3DecodeRelease(string(releaseData))
			if err != nil {
				return 0, err
			}
			// release can contain wrong namespace (set by helm and werf) and confuse user with a wrong metric
			// fetch namespace from secret is more reliable
			release.Namespace = secret.Namespace
			release.HelmVersion = "3"

			releasesC <- release
			totalReleases++
		}

		if secretsList.Continue == "" {
			break
		}

		next = secretsList.Continue
		time.Sleep(fetchSecretsInterval)
	}

	return totalReleases, nil
}

func getHelm2Releases(ctx context.Context, client k8s.Client, releasesC chan<- *Release) (uint32, error) {
	var totalReleases uint32
	var next string

	for {
		cmList, err := client.CoreV1().ConfigMaps("").List(ctx, metav1.ListOptions{
			LabelSelector:        "OWNER=TILLER,STATUS=DEPLOYED",
			Limit:                objectBatchSize,
			Continue:             next,
			ResourceVersion:      "0",
			ResourceVersionMatch: metav1.ResourceVersionMatchNotOlderThan,
		})
		if err != nil {
			return 0, err
		}

		for _, secret := range cmList.Items {
			releaseData := secret.Data["release"]
			if len(releaseData) == 0 {
				continue
			}

			release, err := helm2DecodeRelease(releaseData)
			if err != nil {
				return 0, err
			}
			// release can contain wrong namespace (set by helm and werf) and confuse user with a wrong metric
			// fetch namespace from secret is more reliable
			release.Namespace = secret.Namespace
			release.HelmVersion = "2"

			releasesC <- release
			totalReleases++
		}

		if cmList.Continue == "" {
			break
		}

		next = cmList.Continue
		time.Sleep(fetchSecretsInterval)
	}

	return totalReleases, nil
}

// helm3 decoding

var magicGzip = []byte{0x1f, 0x8b, 0x08}

// Import this from helm3 lib - https://github.com/helm/helm/blob/49819b4ef782e80b0c7f78c30bd76b51ebb56dc8/pkg/storage/driver/util.go#L56
// helm3DecodeRelease decodes the bytes of data into a release
// type. Data must contain a base64 encoded gzipped string of a
// valid release, otherwise an error is returned.
func helm3DecodeRelease(data string) (*Release, error) {
	// base64 decode string
	b, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return nil, err
	}

	// For backwards compatibility with releases that were stored before
	// compression was introduced we skip decompression if the
	// gzip magic header is not found
	if bytes.Equal(b[0:3], magicGzip) {
		r, err := gzip.NewReader(bytes.NewReader(b))
		if err != nil {
			return nil, err
		}
		defer r.Close()
		b2, err := io.ReadAll(r)
		if err != nil {
			return nil, err
		}
		b = b2
	}

	var rls Release
	// unmarshal release object bytes
	if err := json.Unmarshal(b, &rls); err != nil {
		return nil, err
	}
	return &rls, nil
}

// https://github.com/helm/helm/blob/47f0b88409e71fd9ca272abc7cd762a56a1c613e/pkg/storage/driver/util.go#L57
func helm2DecodeRelease(data string) (*Release, error) {
	// base64 decode string
	b, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return nil, err
	}

	// For backwards compatibility with releases that were stored before
	// compression was introduced we skip decompression if the
	// gzip magic header is not found
	if bytes.Equal(b[0:3], magicGzip) {
		r, err := gzip.NewReader(bytes.NewReader(b))
		if err != nil {
			return nil, err
		}
		b2, err := io.ReadAll(r)
		if err != nil {
			return nil, err
		}
		b = b2
	}

	var rls Release
	// unmarshal protobuf bytes
	if err := proto.Unmarshal(b, &rls); err != nil {
		return nil, err
	}
	return &rls, nil
}

// protobuf methods for helm2
func (m *Release) Reset()         { *m = Release{} }
func (m *Release) String() string { return proto.CompactTextString(m) }
func (*Release) ProtoMessage()    {}
