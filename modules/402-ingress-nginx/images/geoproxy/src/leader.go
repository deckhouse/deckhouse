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

package geodownloader

import (
	"context"
	"fmt"
	"os"
	"time"

	"k8s.io/client-go/tools/leaderelection"
	rl "k8s.io/client-go/tools/leaderelection/resourcelock"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/deckhouse/deckhouse/pkg/log"
)

type LeaderElection struct {
	leaseLockName      string
	leaseLockNamespace string
	podName            string
	le                 *leaderelection.LeaderElector
}

func NewLeaderElection(leaseLockName, leaseLockNamespace string) *LeaderElection {
	le := &LeaderElection{
		leaseLockName:      leaseLockName,
		leaseLockNamespace: leaseLockNamespace,
		podName:            os.Getenv("POD_NAME"),
	}

	return le
}

func (l *LeaderElection) AcquireLeaderElection(ctx context.Context) error {
	// Get the active kubernetes context
	cfg, err := ctrl.GetConfig()
	if err != nil {
		log.Fatal(fmt.Sprintf("Failed get config for leader election: %v", err))
	}

	// Create a new lock. This will be used to create a Lease resource in the cluster.
	leader, err := rl.NewFromKubeconfig(
		rl.LeasesResourceLock,
		l.leaseLockNamespace,
		l.leaseLockName,
		rl.ResourceLockConfig{
			Identity: l.podName,
		},
		cfg,
		time.Second*10,
	)

	if err != nil {
		log.Fatal(fmt.Sprintf("Failed get kubeConfig for leader election: %v", err))
	}

	// Create a new leader election configuration with a 15 second lease duration.
	// Visit https://pkg.go.dev/k8s.io/client-go/tools/leaderelection#LeaderElectionConfig
	// for more information on the LeaderElectionConfig struct fields
	lec := leaderelection.LeaderElectionConfig{
		Lock:          leader,
		LeaseDuration: time.Second * 15,
		RenewDeadline: time.Second * 10,
		RetryPeriod:   time.Second * 2,
		Name:          l.leaseLockName,
		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: func(_ context.Context) { println("I am the leader!") },
			OnStoppedLeading: func() { println("I am not the leader anymore!") },
			OnNewLeader:      func(identity string) { fmt.Printf("the leader is %s\n", identity) },
		},
	}

	le, err := leaderelection.NewLeaderElector(lec)
	if err != nil {
		log.Fatal(fmt.Sprintf("Failed get NewLeaderElector for leader election: %v", err))
	}

	l.le = le
	le.Run(ctx)

	<-ctx.Done()
	return nil
}

func (l *LeaderElection) LeaderIsElected() bool {
	return len(l.le.GetLeader()) > 0
}
