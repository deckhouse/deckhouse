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

package app

import (
	"context"
	"fmt"
	"sort"
	"time"

	"graceful_shutdown/pkg/app/containerd"
	"graceful_shutdown/pkg/app/tasks"
	"graceful_shutdown/pkg/systemd"
	"graceful_shutdown/pkg/taskstarter"
)

type App struct {
	InhibitDelayMax time.Duration
	PodLabel        string
	NodeName        string
	taskStarter     *taskstarter.Starter
}

func NewApp(maxDelay time.Duration, podLabel string, nodeName string) *App {
	return &App{
		InhibitDelayMax: maxDelay,
		PodLabel:        podLabel,
	}
}

func (a *App) Start() error {
	err := a.overrideInhibitDelayMax()
	if err != nil {
		return err
	}

	tasks := a.wireAppTasks()
	a.taskStarter = taskstarter.NewStarter(tasks...)

	go a.taskStarter.Start(context.Background())

	go func() {
		<-a.taskStarter.Done()
		a.Stop()
	}()

	return nil
}

func (a *App) Stop() {
	fmt.Printf("Stop app...\n")
	a.taskStarter.Stop()
}

func (a *App) Done() <-chan struct{} {
	return a.taskStarter.Done()
}

func (a *App) Err() error {
	return a.taskStarter.Err()
}

// CheckPods list all pods for testing purposes.
func (a *App) CheckPods() {
	//podMatcher := containerd.ByLabel(a.PodLabel)
	podList, err := containerd.ListPods(context.Background())
	if err != nil {
		fmt.Printf("List pods error: %v\n", err)
		return
	}

	sort.SliceStable(podList.Items, func(i, j int) bool {
		return podList.Items[i].Metadata.Name < podList.Items[j].Metadata.Name
	})

	fmt.Printf("Pods with label %s:\n", a.PodLabel)
	matched := containerd.FilterPods(podList.Items, containerd.WithLabel(a.PodLabel), containerd.WithReadyState())
	for _, pod := range matched {
		fmt.Printf("  %s\n", pod.Metadata.Name)
	}

	//fmt.Printf("Other pods:\n")
	//for _, pod := range podList.Items {
	//	if !podMatcher(pod) {
	//		fmt.Printf("  %s\n", pod.Metadata.Name)
	//	}
	//}
}

func (a *App) overrideInhibitDelayMax() error {
	dbusCon, err := systemd.NewDBusCon()
	if err != nil {
		return fmt.Errorf("initiate DBus connection: %v", err)
	}

	currentInhibitDelay, err := dbusCon.CurrentInhibitDelay()
	if err != nil {
		return fmt.Errorf("get current inihibit delay: %v", err)
	}

	/**

	currentInhibitDelay, err := m.dbusCon.CurrentInhibitDelay()
	if err != nil {
		return nil, err
	}

	// If the logind's InhibitDelayMaxUSec as configured in (logind.conf) is less than periodRequested, attempt to update the value to periodRequested.
	if periodRequested := m.periodRequested(); periodRequested > currentInhibitDelay {
		err := m.dbusCon.OverrideInhibitDelay(periodRequested)
		if err != nil {
			return nil, fmt.Errorf("unable to override inhibit delay by shutdown manager: %v", err)
		}

		err = m.dbusCon.ReloadLogindConf()
		if err != nil {
			return nil, err
		}

		// Read the current inhibitDelay again, if the override was successful, currentInhibitDelay will be equal to shutdownGracePeriodRequested.
		updatedInhibitDelay, err := m.dbusCon.CurrentInhibitDelay()
		if err != nil {
			return nil, err
		}

		if periodRequested > updatedInhibitDelay {
			return nil, fmt.Errorf("node shutdown manager was unable to update logind InhibitDelayMaxSec to %v (ShutdownGracePeriod), current value of InhibitDelayMaxSec (%v) is less than requested ShutdownGracePeriod", periodRequested, updatedInhibitDelay)
		}
	}

	*/

	if currentInhibitDelay >= a.InhibitDelayMax {
		fmt.Printf("overrideInhibitDelayMax: current inhibit delay is already greater or equal to requested: %s >= %s\n", currentInhibitDelay.Truncate(time.Second).String(), a.InhibitDelayMax.Truncate(time.Second).String())
		return nil
	}

	fmt.Printf("overrideInhibitDelayMax: current inhibit delay: %s, override to %s\n", currentInhibitDelay.Truncate(time.Second).String(), a.InhibitDelayMax.Truncate(time.Second).String())

	err = dbusCon.OverrideInhibitDelay(a.InhibitDelayMax)
	if err != nil {
		return fmt.Errorf("overrideInhibitDelayMax: unable to override: %v", err)
	}

	err = dbusCon.ReloadLogindConf()
	if err != nil {
		return fmt.Errorf("overrideInhibitDelayMax: unable to reload systemd conf: %v", err)
	}

	// Getting current delay without waiting for reload is not reliable and gives old value.
	// Comment it for now until a better solution.
	//
	currentInhibitDelay, err = dbusCon.CurrentInhibitDelay()
	if err != nil {
		return fmt.Errorf("get current inhibit delay after override: %v", err)
	}

	if currentInhibitDelay < a.InhibitDelayMax {
		return fmt.Errorf("overrideInhibitDelayMax: unable to override inhibit delay to %s, current value of InhibitDelayMaxSec (%v) is less than requested", a.InhibitDelayMax.Truncate(time.Second).String(), currentInhibitDelay.Truncate(time.Second).String())
	}

	fmt.Printf("overrideInhibitDelayMax: overridden inhibit delay: %s\n", currentInhibitDelay.Truncate(time.Second).String())
	return nil
}

func (a *App) wireAppTasks() []taskstarter.Task {
	// Create channels for events.
	// Event on receiving ShutdownPrepareSignal.
	shutdownSignalCh := make(chan struct{})
	// Event from PowerKey observer.
	//powerKeyPressCh := make(chan struct{})
	// Event to unlock all inhibitors when shutdown requirements are met.
	unlockInhibitorsCh := make(chan struct{})

	return []taskstarter.Task{
		//&tasks.ShutdownBlockInhibitor{
		//	UnlockInhibitorsCh: unlockInhibitorsCh,
		//},
		&tasks.ShutdownInhibitor{
			ShutdownSignalCh:   shutdownSignalCh,
			UnlockInhibitorsCh: unlockInhibitorsCh,
		},
		&tasks.PowerKeyInhibitor{
			UnlockInhibitorsCh: unlockInhibitorsCh,
		},
		&tasks.PowerKeyEvent{
			//PowerKeyPressedCh:  powerKeyPressCh,
			UnlockInhibitorsCh: unlockInhibitorsCh,
		},
		&tasks.PodObserver{
			CheckInterval:    5 * time.Second,
			ShutdownSignalCh: shutdownSignalCh,
			//PowerKeyPressedCh: powerKeyPressCh,
			StopInhibitorsCh: unlockInhibitorsCh,
			PodMatchers: []containerd.PodMatcher{
				containerd.WithLabel(a.PodLabel),
				containerd.WithReadyState(),
			},
		},
		&tasks.NodeCordoner{
			NodeName:         a.NodeName,
			ShutdownSignalCh: shutdownSignalCh,
		},
		&tasks.StatusReporter{
			UnlockInhibitorsCh: unlockInhibitorsCh,
		},
	}
}

func someTask(ctx context.Context, id string) {
	tt := time.NewTicker(1 * time.Second)
	ttt := 120
	for {
		select {
		case <-tt.C:
			fmt.Printf("ticker %s: %d\n", id, ttt)
			ttt--
			if ttt == 0 {
				fmt.Printf("ticker %s done: %d\n", id, ttt)
				return
			}
		case <-ctx.Done():
			tt.Stop()
			fmt.Printf("Got cancel. Ticker %s stopped. Will exit in 5 sec.\n", id)
			time.Sleep(5 * time.Second)
			return
		}
	}
}

//dbusConn, err := systemd.NewDBusCon()
//if err != nil {
//	fmt.Printf("open Dbus connection: %v\n", err)
//	os.Exit(1)
//}
//inhibitDelay, err := dbusConn.CurrentInhibitDelay()
//if err != nil {
//	fmt.Printf("get inhibit delay: %v\n", err)
//	os.Exit(1)
//}
//
//fmt.Printf("Inhibit delay: %s\n", inhibitDelay.Truncate(time.Second).String())
//
