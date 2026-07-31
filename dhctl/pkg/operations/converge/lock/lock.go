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
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	dhlog "github.com/deckhouse/lib-dhctl/pkg/logger"

	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/lease"
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
	getter         kubernetes.KubeClientProviderWithCtx
	unlockConverge func(fullUnlock bool)
}

func NewInLockRunner(ctx context.Context, getter kubernetes.KubeClientProviderWithCtx, identity, sshUser string) *InLockRunner {
	lockConfig := GetLockLeaseConfig(ctx, identity, sshUser)
	return &InLockRunner{
		getter:     getter,
		lockConfig: lockConfig,
		forceLock:  false,
		fullUnlock: true,
	}
}

func NewInLockLocalRunner(ctx context.Context, getter kubernetes.KubeClientProviderWithCtx, identity, sshUser string) *InLockRunner {
	localIdentity := getLocalConvergeLockIdentity(ctx, identity)
	return NewInLockRunner(ctx, getter, localIdentity, sshUser)
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
		dhlog.FromContext(ctx).DebugContext(ctx, "Starting converge unlock from Run")
		if r.unlockConverge != nil {
			r.unlockConverge(true)
			return
		}

		dhlog.FromContext(ctx).DebugContext(ctx, "unlockConverge is nil. Skipping")
	}()

	dhlog.FromContext(ctx).DebugContext(ctx, "lock for Run method was set. Start action")

	return action()
}

func (r *InLockRunner) ResetLock(ctx context.Context) error {
	err := r.setLock(ctx)
	if err != nil {
		return err
	}

	dhlog.FromContext(ctx).DebugContext(ctx, "lock was reset")

	return nil
}

func (r *InLockRunner) Stop() {
	r.unlockConverge(true)
}

func LockConverge(ctx context.Context, provider kubernetes.KubeClientProviderWithCtx, identity, sshUser string) (func(bool), error) {
	localIdentity := getLocalConvergeLockIdentity(ctx, identity)
	lockConfig := GetLockLeaseConfig(ctx, localIdentity, sshUser)
	return LockConvergeWithConfig(ctx, provider, lockConfig)
}

func LockConvergeWithConfig(ctx context.Context, getter kubernetes.KubeClientProviderWithCtx, lockConfig *lease.LeaseLockConfig) (func(bool), error) {
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

func IsConvergeLocked(ctx context.Context, getter kubernetes.KubeClientProviderWithCtx, lockConfig *lease.LeaseLockConfig, checkIsStillLocked bool) (bool, error) {
	leaseLock := lease.NewLeaseLock(getter, *lockConfig)
	return leaseLock.IsLocked(ctx, checkIsStillLocked)
}

func GetLockLeaseConfig(ctx context.Context, identity, sshUser string) *lease.LeaseLockConfig {
	additionalInfo := ""
	if sshUser != "" {
		info := struct {
			SSHUser string `json:"ssh_user,omitempty"`
		}{
			SSHUser: sshUser,
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
			dhlog.FromContext(ctx).WarnContext(ctx, fmt.Sprintf("Lease renewal failed. Sending SIGINT and shutting down: %v", renewErr))
			p, err := os.FindProcess(os.Getpid())
			if err != nil {
				dhlog.FromContext(ctx).ErrorContext(ctx, strings.TrimRight(fmt.Sprintf("Cannot find pid: %v", err), "\n"))
				return
			}

			err = p.Signal(os.Interrupt)
			if err != nil {
				dhlog.FromContext(ctx).ErrorContext(ctx, strings.TrimRight(fmt.Sprintf("Cannot send interrupt signal: %v", err), "\n"))
				return
			}
		},
	}
}

func getLocalConvergeLockIdentity(ctx context.Context, pref string) string {
	const cacheKey = "lock-identifier"

	cache := statecache.Global()

	if hasID, err := cache.InCache(ctx, cacheKey); err == nil && hasID {
		id, err := cache.Load(ctx, cacheKey)
		if err == nil && len(id) > 0 {
			return string(id)
		}
	}

	id := fmt.Sprintf("%v-%v", pref, uuid.New().String())
	if err := cache.Save(ctx, cacheKey, []byte(id)); err != nil {
		panic(err)
	}

	return id
}

func lockLease(
	ctx context.Context,
	getter kubernetes.KubeClientProviderWithCtx,
	config *lease.LeaseLockConfig,
	forceLock bool,
) (func(fullUnlock bool), error) {
	dhlog.FromContext(ctx).DebugContext(ctx, "Creating converge lock and mutex")
	mutex := &sync.Mutex{}
	leaseLock := lease.NewLeaseLock(getter, *config)

	dhlog.FromContext(ctx).DebugContext(ctx, "Trying to lock converge")
	err := leaseLock.Lock(ctx, forceLock)
	if err != nil {
		return nil, err
	}

	// TODO remove after tomb shutdown fix
	unlockConverge := func(fullUnlock bool) {
		mutex.Lock()
		defer mutex.Unlock()

		dhlog.FromContext(ctx).DebugContext(ctx, strings.TrimRight(fmt.Sprintf("Trying to release converge lock. Is it %v", leaseLock == nil), "\n"))

		if leaseLock == nil {
			dhlog.FromContext(ctx).DebugContext(ctx, "Lock was already released. Skipping")
			return
		}

		if fullUnlock {
			dhlog.FromContext(ctx).DebugContext(ctx, "Trying to fully release...")
			leaseLock.Unlock(ctx)
		} else {
			dhlog.FromContext(ctx).DebugContext(ctx, "Trying to stop auto-renew only...")
			leaseLock.StopAutoRenew()
		}

		leaseLock = nil
		dhlog.FromContext(ctx).DebugContext(ctx, "Lock was released")
	}

	dhlog.FromContext(ctx).DebugContext(ctx, "Converge locked successfully")
	return unlockConverge, nil
}
