/*
Copyright 2025 Flant JSC

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

package checker

import (
	"context"
	"crypto/x509"
	"fmt"
	"net/http"
	"sync"

	"github.com/google/go-containerregistry/pkg/authn"
	gcr_name "github.com/google/go-containerregistry/pkg/name"
	gcr_remote "github.com/google/go-containerregistry/pkg/v1/remote"

	"github.com/deckhouse/deckhouse/go_lib/registry/pki"
)

type checkRegistryResult struct {
	Item    queueItem
	Success bool
}

func checkRegistryItem(ctx context.Context, puller *gcr_remote.Puller, isHTTPS bool, refStr string) error {
	var (
		ref gcr_name.Reference
		err error
	)

	if isHTTPS {
		ref, err = gcr_name.ParseReference(refStr)
	} else {
		ref, err = gcr_name.ParseReference(refStr, gcr_name.Insecure)
	}

	if err != nil {
		return fmt.Errorf("parse reference error: %w", err)
	}

	_, err = puller.Head(ctx, ref)
	if err != nil {
		if ctx.Err() == nil {
			return err
		}

		return fmt.Errorf("Request timeout")
	}

	return nil
}

func checkRegistryWorker(
	ctx context.Context,
	puller *gcr_remote.Puller,
	isHTTPS bool,
	queueCh <-chan queueItem,
	resultsCh chan<- checkRegistryResult,
	done func(),
) {
	defer done()

	for {
		select {
		case item, ok := <-queueCh:
			if !ok {
				return
			}

			result := checkRegistryResult{
				Item:    item,
				Success: true,
			}

			err := checkRegistryItem(ctx, puller, isHTTPS, item.Image)
			if err != nil {
				result.Item.Error = err.Error()
				result.Success = false
			}

			resultsCh <- result
		case <-ctx.Done():
			return
		}
	}
}

func buildPullerRoundTripper(ca string) (http.RoundTripper, error) {
	ret := gcr_remote.DefaultTransport.(*http.Transport).Clone()

	certPool, err := x509.SystemCertPool()
	if err != nil {
		return nil, fmt.Errorf("cannot get system CAs pool: %w", err)
	}

	if ca != "" {
		cert, err := pki.DecodeCertificate([]byte(ca))
		if err != nil {
			return nil, fmt.Errorf("cannot decode CA certificate: %w", err)
		}
		certPool.AddCert(cert)
	}

	ret.TLSClientConfig.RootCAs = certPool

	return ret, nil
}

func buildPuller(params RegistryParams) (*gcr_remote.Puller, error) {
	var (
		auth      = authn.Anonymous
		transport = gcr_remote.DefaultTransport
		err       error
	)

	if params.Username != "" {
		auth = &authn.Basic{
			Username: params.Username,
			Password: params.Password,
		}
	}

	if params.isHTTPS() {
		transport, err = buildPullerRoundTripper(params.CA)
		if err != nil {
			return nil, fmt.Errorf("cannot build puller transport: %w", err)
		}
	}

	return gcr_remote.NewPuller(
		gcr_remote.WithTransport(transport),
		gcr_remote.WithAuth(auth),
	)
}

func checkRegistry(ctx context.Context, queue *registryQueue, params RegistryParams) (int, error) {
	puller, err := buildPuller(params)
	if err != nil {
		return 0, fmt.Errorf("cannot create puller: %w", err)
	}

	var (
		wg                       sync.WaitGroup
		queueCh                  = make(chan queueItem, parallelizmPerRegistry*10)
		resultsCh                = make(chan checkRegistryResult, parallelizmPerRegistry*10)
		checkCtx, checkCtxCancel = context.WithCancel(context.Background())
		checkedCount             int
	)
	defer checkCtxCancel()

	// start workers
	wg.Add(parallelizmPerRegistry)
	for range parallelizmPerRegistry {
		go checkRegistryWorker(
			checkCtx,
			puller,
			params.isHTTPS(),
			queueCh,
			resultsCh,
			wg.Done,
		)
	}

	// enqueue and process first item
	for len(queue.Items) > 0 && checkedCount == 0 {
		item := queue.Items[0]

		select {
		case queueCh <- item:
			queue.Items = queue.Items[1:]
		case resultItem := <-resultsCh:
			if resultItem.Success {
				queue.Processed++
			} else {
				queue.Retry = append(queue.Retry, resultItem.Item)
			}
			checkedCount++
		}
	}

	// process other items and handle ctx cancellation
	for len(queue.Items) > 0 && checkCtx.Err() == nil {
		item := queue.Items[0]

		select {
		case queueCh <- item:
			queue.Items = queue.Items[1:]
		case resultItem := <-resultsCh:
			if resultItem.Success {
				queue.Processed++
			} else {
				queue.Retry = append(queue.Retry, resultItem.Item)
			}
			checkedCount++
		case <-ctx.Done():
			checkCtxCancel()
		}
	}
	close(queueCh)

	// wait for workers
	go func() {
		wg.Wait()
		close(resultsCh)
	}()

	for resultItem := range resultsCh {
		if resultItem.Success {
			queue.Processed++
		} else {
			queue.Retry = append(queue.Retry, resultItem.Item)
		}
		checkedCount++
	}

	return checkedCount, nil
}
