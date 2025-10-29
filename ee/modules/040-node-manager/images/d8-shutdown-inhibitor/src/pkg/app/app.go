/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package app

import (
	"context"
	"fmt"
	"time"

	"log/slog"

	"d8_shutdown_inhibitor/pkg/app/tasks"
	"d8_shutdown_inhibitor/pkg/kubernetes"
	"d8_shutdown_inhibitor/pkg/systemd"
	"d8_shutdown_inhibitor/pkg/taskstarter"

	dlog "github.com/deckhouse/deckhouse/pkg/log"
)

type App struct {
	config      AppConfig
	taskStarter *taskstarter.Starter
	klient      *kubernetes.Klient
}

func NewApp(config AppConfig, klient *kubernetes.Klient) *App {
	return &App{
		config: config,
		klient: klient,
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
	dlog.Info("stop app")
	a.taskStarter.Stop()
}

func (a *App) Done() <-chan struct{} {
	return a.taskStarter.Done()
}

func (a *App) Err() error {
	return a.taskStarter.Err()
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

	if currentInhibitDelay >= a.config.InhibitDelayMax {
		dlog.Info(
			"skip inhibit delay override: current is greater or equal to requested",
			slog.String("current", currentInhibitDelay.Truncate(time.Second).String()),
			slog.String("requested", a.config.InhibitDelayMax.Truncate(time.Second).String()),
		)
		return nil
	}

	dlog.Info(
		"override inhibit delay",
		slog.String("current", currentInhibitDelay.Truncate(time.Second).String()),
		slog.String("requested", a.config.InhibitDelayMax.Truncate(time.Second).String()),
	)

	err = dbusCon.OverrideInhibitDelay(a.config.InhibitDelayMax)
	if err != nil {
		return fmt.Errorf("overrideInhibitDelayMax: unable to override: %v", err)
	}

	err = dbusCon.ReloadLogindConf()
	if err != nil {
		return fmt.Errorf("overrideInhibitDelayMax: unable to reload systemd conf: %v", err)
	}

	currentInhibitDelay, err = dbusCon.CurrentInhibitDelay()
	if err != nil {
		return fmt.Errorf("get current inhibit delay after override: %v", err)
	}

	if currentInhibitDelay < a.config.InhibitDelayMax {
		return fmt.Errorf("overrideInhibitDelayMax: unable to override inhibit delay to %s, current value of InhibitDelayMaxSec (%v) is less than requested", a.config.InhibitDelayMax.Truncate(time.Second).String(), currentInhibitDelay.Truncate(time.Second).String())
	}

	dlog.Info(
		"inhibit delay overridden",
		slog.String("current", currentInhibitDelay.Truncate(time.Second).String()),
	)
	return nil
}

func (a *App) wireAppTasks() []taskstarter.Task {
	// Create channels for events.
	// Event on receiving ShutdownPrepareSignal.
	shutdownSignalCh := make(chan struct{})
	// Event to unlock all inhibitors when shutdown requirements are met.
	unlockInhibitorsCh := make(chan struct{})
	startCordonCh := make(chan struct{})

	return []taskstarter.Task{
		&tasks.ShutdownInhibitor{
			ShutdownSignalCh:   shutdownSignalCh,
			UnlockInhibitorsCh: unlockInhibitorsCh,
		},
		&tasks.PowerKeyInhibitor{
			UnlockInhibitorsCh: unlockInhibitorsCh,
		},
		&tasks.PowerKeyEvent{
			UnlockInhibitorsCh: unlockInhibitorsCh,
		},
		&tasks.PodObserver{
			NodeName:              a.config.NodeName,
			PodsCheckingInterval:  a.config.PodsCheckingInterval,
			WallBroadcastInterval: a.config.WallBroadcastInterval,
			ShutdownSignalCh:      shutdownSignalCh,
			StartCordonCh:         startCordonCh,
			StopInhibitorsCh:      unlockInhibitorsCh,
			PodMatchers: []kubernetes.PodMatcher{
				kubernetes.WithLabel(a.config.PodLabel),
				kubernetes.WithRunningPhase(),
			},
			Klient: a.klient,
		},
		&tasks.NodeCordoner{
			NodeName:           a.config.NodeName,
			StartCordonCh:      startCordonCh,
			UnlockInhibitorsCh: unlockInhibitorsCh,
			Klient:             a.klient,
		},
		&tasks.NodeConditionSetter{
			NodeName:           a.config.NodeName,
			UnlockInhibitorsCh: unlockInhibitorsCh,
			Klient:             a.klient,
		},
	}
}
