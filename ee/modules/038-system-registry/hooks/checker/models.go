/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
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
	gcr_name "github.com/google/go-containerregistry/pkg/name"
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

type RegistryParams struct {
	Address  string `json:"address,omitempty"`
	Scheme   string `json:"scheme,omitempty"`
	CA       string `json:"ca,omitempty"`
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
}

func (r *RegistryParams) toGCRepo() (gcr_name.Repository, error) {
	var opts []gcr_name.Option

	if strings.ToUpper(r.Scheme) == "HTTP" {
		opts = append(opts, gcr_name.Insecure)
	}

	return gcr_name.NewRepository(r.Address, opts...)
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
		if readyVal != state.Ready || processedItems > 0 {
			log.Info("Checker loop done",
				"processed_items", processedItems,
				"state.ready", state.Ready,
				"state.message", state.Message,
				"execution.duration", time.Since(startTime),
			)
		}
	}()

	if state.Version != inputs.Params.Version {
		log.Info("Initializing checker with new config",
			"params.version", inputs.Params.Version,
		)

		state.handleNewConfig(inputs)
		return nil
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
		if q, ok := state.Queues[name]; ok {
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
			Items: repoImages,
		}

		state.Queues[name] = q
		log.Info("Added queue", "queue.name", name, "queue.items", q.total())
	}

	return nil
}

func (state *stateModel) processQueues(log go_hook.Logger, inputs inputsModel) (int64, error) {
	t := time.Now().UTC()

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
		err := checkRegistry(ctx, &queue, params)

		r := result{
			Name:  name,
			Queue: queue,
			Error: err,
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

	var (
		msg      = new(strings.Builder)
		hasItems bool
		qNames   = make([]string, 0, len(state.Queues))
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
		hasItems = true

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

	state.Message = strings.TrimSpace(msg.String())
	if !hasItems {
		state.Ready = true
	}

	return processedCount, nil
}
