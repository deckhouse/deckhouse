package client

import (
	"context"
	"fmt"
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

type LeaseLockConfig struct {
	Name      string
	Namespace string
	Identity  string

	LeaseDurationSeconds         int32
	RenewDurationSeconds         int64
	TolerableExpiredLeaseSeconds int64
	RetryDuration                time.Duration

	OnRenewError func(err error)
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

func (l *LeaseLock) Lock() error {
	l.lockLease.Lock()
	defer l.lockLease.Unlock()

	lease, err := l.tryAcquire()
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

	err := retry.NewSilentLoop("release lease", 5, l.config.RetryDuration).Run(func() error {
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

	t := time.NewTicker(time.Duration(l.config.RenewDurationSeconds) * time.Second)
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

func (l *LeaseLock) tryAcquire() (*coordinationv1.Lease, error) {
	var lease *coordinationv1.Lease

	prefix := "Don't acquire lease lock."
	cannotRenew := func(err error) bool {
		return strings.HasPrefix(err.Error(), prefix)
	}

	err := retry.NewSilentLoop("release lease", 5, l.config.RetryDuration).BreakIf(cannotRenew).Run(func() error {
		var err error
		lease, err = l.createLease()
		if err == nil {
			return nil
		}

		if errors.IsAlreadyExists(err) {
			lease, err = l.leasesCl.Get(context.TODO(), l.config.Name, metav1.GetOptions{})
			if err != nil {
				return fmt.Errorf("%s Don't get current lease %v", prefix, err)
			}

			lease, err = l.tryRenew(lease, false)
			if err != nil {
				return fmt.Errorf("%s %v", prefix, err)
			}
		}

		return nil
	})

	return lease, err
}

func (l *LeaseLock) createLease() (lease *coordinationv1.Lease, err error) {
	lease = &coordinationv1.Lease{
		ObjectMeta: metav1.ObjectMeta{
			Name: l.config.Name,
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

		log.Warnf("Lease for %v finished on %v, Try renew lease\n", l.config.Identity, lease.Spec.RenewTime.Time)
	}

	var newLease *coordinationv1.Lease
	err := retry.NewSilentLoop("try to renew", 5, l.config.RetryDuration).Run(func() error {
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
	tolerable := time.Duration(l.config.TolerableExpiredLeaseSeconds) * time.Second
	renewTime := renewTimeMicro.Time
	endLeaseTime := renewTime.Add(leaseDuration).Add(tolerable)

	return time.Now().Before(endLeaseTime)
}

func now() *metav1.MicroTime {
	return &metav1.MicroTime{Time: time.Now()}
}

func getCurrentLockerError(lease *coordinationv1.Lease) error {
	holder := "unknown"
	acquireTime := time.Time{}
	lastRenew := time.Time{}
	leaseDurationSec := int32(-1)
	if lease != nil {
		holder = *lease.Spec.HolderIdentity
		acquireTime = lease.Spec.AcquireTime.Time
		lastRenew = lease.Spec.RenewTime.Time
		leaseDurationSec = *lease.Spec.LeaseDurationSeconds
	}
	format := "Locked by %v acquireTime %v lastRenewTime %v leaseDuration %vs"
	return fmt.Errorf(format, holder, acquireTime, lastRenew, leaseDurationSec)
}
