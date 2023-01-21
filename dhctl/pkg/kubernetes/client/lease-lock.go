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

package client

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/user"
	"strings"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
	coordinationv1 "k8s.io/api/coordination/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	coordinationclientv1 "k8s.io/client-go/kubernetes/typed/coordination/v1"

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
	leasesCl coordinationclientv1.LeaseInterface
	config   LeaseLockConfig

	lockLease   sync.Mutex
	exitRenewCh chan struct{}
	lease       *coordinationv1.Lease
}

func NewLeaseLock(kubeCl *KubernetesClient, config LeaseLockConfig) *LeaseLock {
	return &LeaseLock{
		leasesCl:    kubeCl.CoordinationV1().Leases(config.Namespace),
		config:      config,
		exitRenewCh: make(chan struct{}),
	}
}

func (l *LeaseLock) Lock(force bool) error {
	l.lockLease.Lock()
	defer l.lockLease.Unlock()

	lease, err := l.tryAcquire(force)
	if err != nil {
		return err
	}

	l.lease = lease

	go l.startAutoRenew()

	return nil
}

func (l *LeaseLock) Unlock() {
	l.lockLease.Lock()
	defer l.lockLease.Unlock()

	if l.lease == nil {
		return
	}

	close(l.exitRenewCh)

	deleteRetries := l.config.RenewRetries()
	err := retry.NewSilentLoop("unlock lease", deleteRetries, l.config.RetryWaitDuration).Run(func() error {
		err := l.leasesCl.Delete(context.TODO(), l.lease.Name, metav1.DeleteOptions{})
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

func (l *LeaseLock) startAutoRenew() {
	defer log.Debugln("lease autorenew stopped")
	log.Debugln("lease autorenew started")

	t := time.NewTicker(time.Duration(l.config.RenewEverySeconds) * time.Second)
	defer t.Stop()

	for {
		select {
		case <-l.exitRenewCh:
			return
		case <-t.C:
			lease, err := l.tryRenew(l.lease, true)
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

func (l *LeaseLock) tryAcquire(force bool) (*coordinationv1.Lease, error) {
	var lease *coordinationv1.Lease

	prefix := "Can't acquire lease lock."
	cannotRenew := func(err error) bool {
		return strings.HasPrefix(err.Error(), prefix)
	}

	acquireRetries := l.config.RenewRetries()
	err := retry.NewSilentLoop("acquire lease", acquireRetries, l.config.RetryWaitDuration).BreakIf(cannotRenew).Run(func() error {
		var err error
		lease, err = l.createLease()
		if err == nil {
			return nil
		}

		if errors.IsAlreadyExists(err) {
			lease, err = l.leasesCl.Get(context.TODO(), l.config.Name, metav1.GetOptions{})
			if err != nil {
				return fmt.Errorf("%s Can't get current lease %v", prefix, err)
			}

			lease, err = l.tryRenew(lease, force)
			if err != nil {
				return fmt.Errorf("%s \n%v", prefix, err)
			}
		}

		return nil
	})

	return lease, err
}

func (l *LeaseLock) createLease() (lease *coordinationv1.Lease, err error) {
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

	return l.leasesCl.Create(context.TODO(), lease, metav1.CreateOptions{})
}

func (l *LeaseLock) tryRenew(lease *coordinationv1.Lease, force bool) (*coordinationv1.Lease, error) {
	if *lease.Spec.HolderIdentity != l.config.Identity {
		return nil, getCurrentLockerError(lease)
	}

	if !force {
		if l.isStillLocked(lease) {
			return nil, getCurrentLockerError(lease)
		}

		log.Warnf("Lease for %v finished on %v, try to renew lease\n", l.config.Identity, lease.Spec.RenewTime.Time)
	}

	var newLease *coordinationv1.Lease

	renewRetries := l.config.RenewRetries()
	err := retry.NewSilentLoop("try to renew", renewRetries, l.config.RetryWaitDuration).Run(func() error {
		var err error
		lease.Spec.RenewTime = now()
		newLease, err = l.leasesCl.Update(context.TODO(), lease, metav1.UpdateOptions{})
		return err
	})

	return newLease, err
}

func (l *LeaseLock) isStillLocked(lease *coordinationv1.Lease) bool {
	if lease.Spec.HolderIdentity == nil {
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
	return fmt.Errorf(info)
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

func RemoveLease(kubeCl *KubernetesClient, config *LeaseLockConfig, confirm func(lease *coordinationv1.Lease) error) error {
	leasesCl := kubeCl.CoordinationV1().Leases(config.Namespace)
	lease, err := leasesCl.Get(context.TODO(), config.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	if err := confirm(lease); err != nil {
		return err
	}

	log.Infof("Starting remove lease lock")

	err = retry.NewSilentLoop("release lease", 5, config.RetryWaitDuration).Run(func() error {
		err := leasesCl.Delete(context.TODO(), lease.Name, metav1.DeleteOptions{})
		if errors.IsNotFound(err) {
			return nil
		}
		return err
	})

	return err
}
