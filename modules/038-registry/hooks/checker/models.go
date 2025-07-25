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
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	validation "github.com/go-ozzo/ozzo-validation/v4"
	gcr_name "github.com/google/go-containerregistry/pkg/name"

	"github.com/deckhouse/deckhouse/modules/038-registry/hooks/helpers"
)

type deckhouseImagesModel struct {
	InitContainers map[string]string
	Containers     map[string]string
}

type queueItem struct {
	Image string `json:"image,omitempty"`
	Info  string `json:"info,omitempty"`
	Error string `json:"error,omitempty"`
}

type Params struct {
	Registries map[string]RegistryParams `json:"registries,omitempty"`
	Version    string                    `json:"version,omitempty"`
}

func (params Params) Validate() error {
	return validation.ValidateStruct(&params,
		validation.Field(&params.Registries),
	)
}

type RegistryParams struct {
	Address  string `json:"address,omitempty"`
	Scheme   string `json:"scheme,omitempty"`
	CA       string `json:"ca,omitempty"`
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
}

func (rp RegistryParams) Validate() error {
	return validation.ValidateStruct(&rp,
		validation.Field(&rp.Address, validation.Required),
		validation.Field(&rp.Scheme, validation.In("HTTP", "HTTPS")),
		validation.Field(&rp.Username, validation.When(rp.Password != "", validation.Required)),
		validation.Field(&rp.Password, validation.When(rp.Username != "", validation.Required)),
	)
}

func (rp *RegistryParams) toGCRepo() (gcr_name.Repository, error) {
	if rp.isHTTPS() {
		return gcr_name.NewRepository(rp.Address)
	}

	return gcr_name.NewRepository(rp.Address, gcr_name.Insecure)
}

func (rp *RegistryParams) isHTTPS() bool {
	return strings.ToUpper(rp.Scheme) == "HTTPS"
}

type Status struct {
	Version string `json:"version,omitempty"`
	Ready   bool   `json:"ready"`
	Message string `json:"message,omitempty"`
}

type stateModel struct {
	Status
	Queues map[string]registryQueue `json:"queues,omitempty"`
}

type stateSecretData struct {
	Params []byte
	State  []byte
}

type registryQueue struct {
	Processed   int         `json:"processed,omitempty"`
	Items       []queueItem `json:"items,omitempty"`
	Retry       []queueItem `json:"retry,omitempty"`
	LastAttempt *time.Time  `json:"last_attempt,omitempty"`
	ParamsHash  string      `json:"params_hash,omitempty"`
}

func (q *registryQueue) any() bool {
	return len(q.Items) > 0 || len(q.Retry) > 0
}

func (q *registryQueue) total() int {
	return q.Processed + len(q.Items) + len(q.Retry)
}

type inputsModel struct {
	Params     Params
	ImagesInfo clusterImagesInfo
}

type clusterImagesInfo struct {
	Repo                 string
	ModulesImagesDigests map[string]string
	DeckhouseImages      deckhouseImagesModel
}

func (state *stateModel) Process(log go_hook.Logger, inputs inputsModel) error {
	var (
		processedItems int64
		startTime      = time.Now()
		err            error
	)

	log = log.With("state.version", state.Version, "execution.start", startTime)
	readyVal := state.Ready
	state.Ready = false

	defer func() {
		if state.Ready != readyVal || processedItems > 0 {
			log.Info("Checker loop done",
				"items.processed", processedItems,
				"state.ready", state.Ready,
				"state.message", state.Message,
				"execution.duration", time.Since(startTime),
			)
		}
	}()

	var isNewConfig bool
	if state.Version != inputs.Params.Version {
		log.Info("Initializing checker with new config",
			"params.version", inputs.Params.Version,
		)

		state.handleNewConfig(inputs)
		isNewConfig = true
	}

	log.Debug("Initializing queues")
	err = state.initQueues(log, inputs)
	if err != nil {
		return fmt.Errorf("cannot init queues: %w", err)
	}

	if len(state.Queues) == 0 {
		state.Message = "Stopped"
		state.Ready = true
		return nil
	}

	if isNewConfig {
		state.Message = state.buildMessage()
		return nil
	}

	log.Debug("Processing queues")
	processedItems, err = state.processQueues(log, inputs)
	if err != nil {
		return fmt.Errorf("error processing queues: %w", err)
	}

	return nil
}

func (state *stateModel) handleNewConfig(inputs inputsModel) {
	state.Queues = nil
	state.Version = inputs.Params.Version
	state.Ready = false
	state.Message = "Initializing"
}

func (state *stateModel) initQueues(log go_hook.Logger, inputs inputsModel) error {
	if len(inputs.Params.Registries) == 0 {
		state.Queues = nil
		return nil
	}

	if state.Queues == nil {
		state.Queues = make(map[string]registryQueue)
	}

	for name := range state.Queues {
		if _, ok := inputs.Params.Registries[name]; !ok {
			log.Info("Deleting queue", "queue.name", name)
			delete(state.Queues, name)
		}
	}

	t := time.Now().UTC()
	for name, registryParams := range inputs.Params.Registries {
		hash, err := helpers.ComputeHash(registryParams)
		if err != nil {
			return fmt.Errorf("cannot compute registry %q params hash: %w", name, err)
		}

		if q, ok := state.Queues[name]; ok && q.ParamsHash == hash {
			if len(q.Items) == 0 && len(q.Retry) > 0 {
				q.Items = q.Retry
				q.Retry = nil
				q.LastAttempt = &t

				state.Queues[name] = q

				log.Info("Retry items moved", "queue.name", name)
			}

			continue
		}

		repo, err := registryParams.toGCRepo()
		if err != nil {
			return fmt.Errorf("cannot parse registry %q params: %w", name, err)
		}

		repoImages, err := buildRepoQueue(inputs.ImagesInfo, repo)
		if err != nil {
			return fmt.Errorf("cannot build registry %q queue: %w", name, err)
		}

		q := registryQueue{
			Items:      repoImages,
			ParamsHash: hash,
		}

		state.Queues[name] = q
		log.Info("Added queue", "queue.name", name, "queue.items", q.total())
	}

	return nil
}

func (state *stateModel) processQueues(log go_hook.Logger, inputs inputsModel) (int64, error) {
	t := time.Now().UTC()

	// fast path
	if !state.hasItems() {
		state.Message = state.buildMessage()
		state.Ready = true
		return 0, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), processTimeout)
	defer cancel()

	type result struct {
		Name           string
		Queue          registryQueue
		ProcessedCount int
		Error          error
	}

	resultsCh := make(chan result, len(state.Queues))
	worker := func(ctx context.Context, name string, queue registryQueue, params RegistryParams, done func()) {
		defer done()

		log.Debug("Checking registry", "name", name)
		count, err := checkRegistry(ctx, &queue, params)

		r := result{
			Name:           name,
			Queue:          queue,
			Error:          err,
			ProcessedCount: count,
		}

		if err == nil {
			log.Debug("Registry check done", "name", name)
		} else {
			log.Error("Check registry error", "name", name, "error", err)
		}

		resultsCh <- r
	}

	var wg sync.WaitGroup
	for name, q := range state.Queues {
		if !q.any() {
			continue
		}

		if q.LastAttempt != nil {
			rt := q.LastAttempt.Add(retryDelay)
			if t.Before(rt) {
				continue
			}

			q.LastAttempt = nil
		}

		if len(q.Items) == 0 {
			continue
		}

		wg.Add(1)
		params := inputs.Params.Registries[name]
		go worker(ctx, name, q, params, wg.Done)
	}

	log.Debug("Waiting for checkers...")
	wg.Wait()
	close(resultsCh)
	log.Debug("All checkers are done")

	var (
		errs           []error
		processedCount int64
	)
	for r := range resultsCh {
		if r.Error != nil {
			errs = append(errs, fmt.Errorf("check registry %q error: %w", r.Name, r.Error))
			continue
		}

		state.Queues[r.Name] = r.Queue
		processedCount += int64(r.ProcessedCount)
	}

	if len(errs) > 0 {
		return 0, errors.Join(errs...)
	}

	state.Message = state.buildMessage()
	state.Ready = !state.hasItems()

	return processedCount, nil
}

func (state *stateModel) buildMessage() string {
	var (
		msg    = new(strings.Builder)
		qNames = make([]string, 0, len(state.Queues))
	)

	for name := range state.Queues {
		qNames = append(qNames, name)
	}

	sort.Strings(qNames)

	for _, name := range qNames {
		q := state.Queues[name]

		if !q.any() {
			fmt.Fprintf(msg, "%v: all %v items are checked\n", name, q.Processed)
			continue
		}

		errItems := make([]queueItem, 0, len(q.Retry)+len(q.Items))
		errItems = append(errItems, q.Retry...)

		for _, item := range q.Items {
			if item.Error != "" {
				errItems = append(errItems, item)
			}
		}

		if len(errItems) > 0 {
			fmt.Fprintf(msg,
				"%v: %v of %v items processed, %v items with errors:\n",
				name, q.Processed, q.total(), len(errItems),
			)

			for i, item := range errItems {
				if i >= showMaxErrItems {
					fmt.Fprintf(msg, "\n  ...and more\n")
					break
				}

				fmt.Fprintf(msg,
					"- source: %v\n  image: %v\n  error: %v\n",
					item.Info, item.Image, item.Error,
				)
			}

			fmt.Fprintln(msg)
			continue
		}

		fmt.Fprintf(msg,
			"%v: %v of %v items processed\n",
			name, q.Processed, q.total(),
		)
	}

	return strings.TrimSpace(msg.String())
}

func (state *stateModel) hasItems() bool {
	for _, q := range state.Queues {
		if q.any() {
			return true
		}
	}

	return false
}
