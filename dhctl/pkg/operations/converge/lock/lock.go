// Copyright 2021 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package lock

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/lease"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	statecache "github.com/deckhouse/deckhouse/dhctl/pkg/state/cache"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/tomb"
)

const (
	AutoConvergerIdentity = "terraform-auto-converger"
	tombIdenty            = "unlock converge"
)

type InLockRunner struct {
	lockConfig     *lease.LeaseLockConfig
	forceLock      bool
	fullUnlock     bool
	getter         kubernetes.KubeClientProvider
	unlockConverge func(fullUnlock bool)
	unlockMutex    sync.Mutex
}

func NewInLockRunner(getter kubernetes.KubeClientProvider, identity string) *InLockRunner {
	lockConfig := GetLockLeaseConfig(identity)
	return &InLockRunner{
		getter:     getter,
		lockConfig: lockConfig,
		forceLock:  false,
		fullUnlock: true,
	}
}

func NewInLockLocalRunner(getter kubernetes.KubeClientProvider, identity string) *InLockRunner {
	localIdentity := getLocalConvergeLockIdentity(identity)
	return NewInLockRunner(getter, localIdentity)
}

func (r *InLockRunner) WithForceLock(f bool) *InLockRunner {
	r.forceLock = f
	return r
}

func (r *InLockRunner) WithFullUnlock(f bool) *InLockRunner {
	r.fullUnlock = f
	return r
}

func (r *InLockRunner) setLock(ctx context.Context) error {
	unlockConverge, err := lockLease(ctx, r.getter, r.lockConfig, r.forceLock)
	if err != nil {
		return err
	}
	tomb.ReplaceOnShutdown(tombIdenty, func() {
		unlockConverge(true)
	})

	r.unlockConverge = unlockConverge

	return nil
}

func (r *InLockRunner) Run(ctx context.Context, action func() error) error {
	err := r.setLock(ctx)
	if err != nil {
		return err
	}

	defer func() {
		log.DebugLn("Start unlock converge from Run")
		if r.unlockConverge != nil {
			r.unlockConverge(true)
			return
		}

		log.DebugLn("unlockConverge is nil. Skip")
	}()

	log.DebugLn("lock for Run method was set. Start action")

	return action()
}

func (r *InLockRunner) ResetLock(ctx context.Context) error {
	err := r.setLock(ctx)
	if err != nil {
		return err
	}

	log.DebugLn("lock was reset")

	return nil
}

func (r *InLockRunner) Stop() {
	r.unlockConverge(true)
}

func LockConverge(ctx context.Context, getter kubernetes.KubeClientProvider, identity string) (func(bool), error) {
	localIdentity := getLocalConvergeLockIdentity(identity)
	lockConfig := GetLockLeaseConfig(localIdentity)
	unlockConverge, err := lockLease(ctx, getter, lockConfig, false)
	if err != nil {
		return nil, err
	}

	tomb.ReplaceOnShutdown(tombIdenty, func() {
		// always full unlock on shutdown
		unlockConverge(true)
	})

	return unlockConverge, nil
}

func GetLockLeaseConfig(identity string) *lease.LeaseLockConfig {
	additionalInfo := ""
	if app.SSHUser != "" {
		info := struct {
			SSHUser string `json:"ssh_user,omitempty"`
		}{
			SSHUser: app.SSHUser,
		}

		infoStr, err := json.Marshal(info)
		if err == nil {
			additionalInfo = string(infoStr)
		}
	}

	return &lease.LeaseLockConfig{
		Name:                 "d8-converge-lock",
		Identity:             identity,
		Namespace:            "d8-system",
		LeaseDurationSeconds: 300,
		RenewEverySeconds:    30,
		RetryWaitDuration:    3 * time.Second,
		AdditionalUserInfo:   additionalInfo,
		OnRenewError: func(renewErr error) {
			log.WarnF("Lease renew was failed. Send SIGINT and shutdown: %v\n", renewErr)
			p, err := os.FindProcess(os.Getpid())
			if err != nil {
				log.ErrorF("Cannot find pid: %v", err)
				return
			}

			err = p.Signal(os.Interrupt)
			if err != nil {
				log.ErrorF("Cannot send interrupt signal: %v", err)
				return
			}
		},
	}
}

func getLocalConvergeLockIdentity(pref string) string {
	const cacheKey = "lock-identifier"

	cache := statecache.Global()

	if hasID, err := cache.InCache(cacheKey); err == nil && hasID {
		id, err := cache.Load(cacheKey)
		if err == nil && len(id) > 0 {
			return string(id)
		}
	}

	id := fmt.Sprintf("%v-%v", pref, uuid.New().String())
	if err := cache.Save(cacheKey, []byte(id)); err != nil {
		panic(err)
	}

	return id
}

func lockLease(
	ctx context.Context,
	getter kubernetes.KubeClientProvider,
	config *lease.LeaseLockConfig,
	forceLock bool,
) (toDefer func(fullUnlock bool), err error) {
	log.DebugLn("Create converge lock and mutex")
	mutex := &sync.Mutex{}
	leaseLock := lease.NewLeaseLock(getter, *config)

	log.DebugLn("Try to lock converge")
	err = leaseLock.Lock(ctx, forceLock)
	if err != nil {
		return nil, err
	}

	// TODO remove after tomb shutdown fix
	unlockConverge := func(fullUnlock bool) {
		mutex.Lock()
		defer mutex.Unlock()

		log.DebugLn("Try to release converge lock. Is it %v", leaseLock == nil)

		if leaseLock == nil {
			log.DebugLn("Lock was released. Skip")
			return
		}

		if fullUnlock {
			log.DebugLn("Try to full release...")
			leaseLock.Unlock(ctx)
		} else {
			log.DebugLn("Try to stop autorenew only...")
			leaseLock.StopAutoRenew()
		}

		leaseLock = nil
		log.DebugLn("Lock was released")
	}

	log.DebugLn("Lock converge successful")
	return unlockConverge, nil
}
