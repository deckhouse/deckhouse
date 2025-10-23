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

package lease

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"os"
	"os/user"
	"strings"
	"sync"
	"time"

	coordinationv1 "k8s.io/api/coordination/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/pkg/log"

	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/retry"
)

const lockUserInfoAnnotKey = "dhctl.deckhouse.io/lock-user-info"

type LeaseLockConfig struct {
	Name      string
	Namespace string
	Identity  string

	LeaseDurationSeconds int32
	RenewEverySeconds    int64
	RetryWaitDuration    time.Duration

	OnRenewError func(err error)

	AdditionalUserInfo string
}

// RenewRetries returns a number of possible retries between renew seconds and lease lifetime seconds.
func (c *LeaseLockConfig) RenewRetries() int {
	d := int64(c.LeaseDurationSeconds) - c.RenewEverySeconds
	retries := math.Ceil(float64(d) / c.RetryWaitDuration.Seconds())
	return int(retries)
}

type LockUserInfo struct {
	Name       string `json:"name,omitempty"`
	Host       string `json:"host,omitempty"`
	Additional string `json:"additional,omitempty"`
}

func NewLockUserInfo(additional string) *LockUserInfo {
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	}

	userName := "unknown"
	userOs, err := user.Current()
	if err == nil {
		userName = userOs.Username
	}

	return &LockUserInfo{
		Name:       userName,
		Host:       hostname,
		Additional: additional,
	}
}

type LeaseLock struct {
	getter kubernetes.KubeClientProvider
	config LeaseLockConfig

	lockLease   sync.Mutex
	exitRenewCh chan struct{}
	lease       *coordinationv1.Lease
}

func NewLeaseLock(getter kubernetes.KubeClientProvider, config LeaseLockConfig) *LeaseLock {
	return &LeaseLock{
		getter:      getter,
		config:      config,
		exitRenewCh: make(chan struct{}),
	}
}

func (l *LeaseLock) Lock(ctx context.Context, force bool) error {
	l.lockLease.Lock()
	defer l.lockLease.Unlock()

	lease, err := l.tryAcquire(ctx, force)
	if err != nil {
		return err
	}

	l.lease = lease

	go l.startAutoRenew(ctx)

	return nil
}

func (l *LeaseLock) Unlock(ctx context.Context) {
	l.lockLease.Lock()
	defer l.lockLease.Unlock()

	if l.lease == nil {
		return
	}

	close(l.exitRenewCh)

	deleteRetries := l.config.RenewRetries()
	err := retry.NewSilentLoop("unlock lease", deleteRetries, l.config.RetryWaitDuration).RunContext(ctx, func() error {
		err := l.getter.KubeClient().CoordinationV1().Leases(l.config.Namespace).Delete(ctx, l.lease.Name, metav1.DeleteOptions{})
		if errors.IsNotFound(err) {
			return nil
		}
		return err
	})
	if err != nil {
		log.Errorf("Error while Unlock lease %v", err)
	}

	l.lease = nil
}

func (l *LeaseLock) StopAutoRenew() {
	close(l.exitRenewCh)
}

func (l *LeaseLock) startAutoRenew(ctx context.Context) {
	defer log.Debug("lease autorenew stopped")
	log.Debug("lease autorenew started")

	t := time.NewTicker(time.Duration(l.config.RenewEverySeconds) * time.Second)
	defer t.Stop()

	for {
		select {
		case <-l.exitRenewCh:
			return
		case <-t.C:
			lease, err := l.tryRenew(ctx, l.lease, true)
			if err == nil {
				l.lease = lease
				continue
			}

			if l.config.OnRenewError != nil {
				l.config.OnRenewError(err)
			}
			return
		}
	}
}

func (l *LeaseLock) tryAcquire(ctx context.Context, force bool) (*coordinationv1.Lease, error) {
	var lease *coordinationv1.Lease

	prefix := "Can't acquire lease lock."
	cannotRenew := func(err error) bool {
		return strings.HasPrefix(err.Error(), prefix)
	}

	acquireRetries := l.config.RenewRetries()
	err := retry.NewSilentLoop("acquire lease", acquireRetries, l.config.RetryWaitDuration).BreakIf(cannotRenew).RunContext(ctx, func() error {
		var err error
		lease, err = l.createLease(ctx)
		if err == nil {
			return nil
		}

		if errors.IsAlreadyExists(err) {
			lease, err = l.getter.KubeClient().CoordinationV1().Leases(l.config.Namespace).Get(ctx, l.config.Name, metav1.GetOptions{})
			if err != nil {
				return fmt.Errorf("%s Can't get current lease %v", prefix, err)
			}

			lease, err = l.tryRenew(ctx, lease, force)
			if err != nil {
				return fmt.Errorf("%s \n%v", prefix, err)
			}
		}

		return nil
	})

	return lease, err
}

func (l *LeaseLock) createLease(ctx context.Context) (lease *coordinationv1.Lease, err error) {
	userInfo := NewLockUserInfo(l.config.AdditionalUserInfo)
	userInfoStr, err := json.Marshal(userInfo)
	if err != nil {
		userInfoStr = nil
	}

	lease = &coordinationv1.Lease{
		ObjectMeta: metav1.ObjectMeta{
			Name: l.config.Name,
			Annotations: map[string]string{
				lockUserInfoAnnotKey: string(userInfoStr),
			},
		},
		Spec: coordinationv1.LeaseSpec{
			HolderIdentity:       &l.config.Identity,
			AcquireTime:          now(),
			RenewTime:            now(),
			LeaseDurationSeconds: &l.config.LeaseDurationSeconds,
		},
	}

	return l.getter.KubeClient().CoordinationV1().Leases(l.config.Namespace).Create(ctx, lease, metav1.CreateOptions{})
}

func (l *LeaseLock) tryRenew(ctx context.Context, lease *coordinationv1.Lease, force bool) (*coordinationv1.Lease, error) {
	if lease == nil {
		return nil, fmt.Errorf("Lease is nil")
	}

	if *lease.Spec.HolderIdentity != l.config.Identity {
		return nil, getCurrentLockerError(lease)
	}

	if !force {
		if l.isStillLocked(lease) {
			return nil, getCurrentLockerError(lease)
		}

		log.Warn("Lease finished, try to renew lease", slog.String("identity", l.config.Identity), slog.String("renew_time", lease.Spec.RenewTime.Time.String()))
	}

	var newLease *coordinationv1.Lease

	renewRetries := l.config.RenewRetries()
	err := retry.NewSilentLoop("try to renew", renewRetries, l.config.RetryWaitDuration).RunContext(ctx, func() error {
		var err error
		lease.Spec.RenewTime = now()
		newLease, err = l.getter.KubeClient().CoordinationV1().Leases(l.config.Namespace).Update(ctx, lease, metav1.UpdateOptions{})
		if err != nil && strings.Contains(err.Error(), "the object has been modified; please apply your changes to the latest version and try again") {
			leaseTemp, err := l.getter.KubeClient().CoordinationV1().Leases(l.config.Namespace).Get(ctx, l.config.Name, metav1.GetOptions{})
			if err != nil {
				return err
			}
			leaseTemp.Spec.RenewTime = now()
			newLease, err = l.getter.KubeClient().CoordinationV1().Leases(l.config.Namespace).Update(ctx, leaseTemp, metav1.UpdateOptions{})
			return err
		}
		return err
	})

	return newLease, err
}

func (l *LeaseLock) isStillLocked(lease *coordinationv1.Lease) bool {
	if lease == nil || lease.Spec.HolderIdentity == nil {
		return false
	}

	renewTimeMicro := lease.Spec.RenewTime
	if renewTimeMicro == nil {
		return true
	}

	leaseDuration := time.Duration(l.config.LeaseDurationSeconds) * time.Second
	renewTime := renewTimeMicro.Time
	endLeaseTime := renewTime.Add(leaseDuration)

	return time.Now().Before(endLeaseTime)
}

func now() *metav1.MicroTime {
	return &metav1.MicroTime{Time: time.Now()}
}

func getCurrentLockerError(lease *coordinationv1.Lease) error {
	info, _ := LockInfo(lease)
	return fmt.Errorf("%s", info)
}

func LockInfo(lease *coordinationv1.Lease) (string, *LockUserInfo) {
	holder := "unknown"
	acquireTime := time.Time{}
	lastRenew := time.Time{}
	leaseDurationSec := int32(-1)
	zeroUserInfo := LockUserInfo{
		Name:       "unknown",
		Host:       "unknown",
		Additional: "Info is not set",
	}
	userInfo := zeroUserInfo
	if lease != nil {
		holder = *lease.Spec.HolderIdentity
		acquireTime = lease.Spec.AcquireTime.Time
		lastRenew = lease.Spec.RenewTime.Time
		leaseDurationSec = *lease.Spec.LeaseDurationSeconds

		infoJSON, ok := lease.Annotations[lockUserInfoAnnotKey]
		if ok && infoJSON != "" {
			err := json.Unmarshal([]byte(infoJSON), &userInfo)
			if err != nil {
				userInfo = zeroUserInfo
			}
		}
	}

	format := `Locker ID: %v
  acquireTime: %v
  lastRenewTime: %v
  leaseDuration: %vs
  user: %s@%s
  additionalInfo:
    %s
If you sure that lock acquired not by auto-converger, for release lock use:
  dhctl lock release
`
	return fmt.Sprintf(format,
		holder,
		acquireTime,
		lastRenew,
		leaseDurationSec,
		userInfo.Name,
		userInfo.Host,
		userInfo.Additional,
	), &userInfo
}

func RemoveLease(ctx context.Context, kubeCl *client.KubernetesClient, config *LeaseLockConfig, confirm func(lease *coordinationv1.Lease) error) error {
	leasesCl := kubeCl.CoordinationV1().Leases(config.Namespace)
	lease, err := leasesCl.Get(ctx, config.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	if err := confirm(lease); err != nil {
		return err
	}

	log.Infof("Starting remove lease lock")

	err = retry.NewSilentLoop("release lease", 5, config.RetryWaitDuration).RunContext(ctx, func() error {
		err := leasesCl.Delete(ctx, lease.Name, metav1.DeleteOptions{})
		if errors.IsNotFound(err) {
			return nil
		}
		return err
	})

	return err
}
