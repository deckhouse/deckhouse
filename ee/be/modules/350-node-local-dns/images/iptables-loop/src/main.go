/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package main

import (
	"fmt"
	"math"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/coreos/go-iptables/iptables"
	"github.com/deckhouse/deckhouse/pkg/log"
	"github.com/fsnotify/fsnotify"
	"k8s.io/utils/ptr"
)

const (
	iptablesLoopBinary = "/iptables-loop"

	readinessFilePath = "/tmp/coredns-readiness"
	readyState        = "ready"
	notReadyState     = "not-ready"

	iptableName      = "raw"
	iptableChainName = "PREROUTING"
)

type readinessMgr struct {
	ready        *bool
	fileExists   bool
	path         string
	signalCh     chan os.Signal
	logger       *log.Logger
	iptablesMgr  *iptables.IPTables
	iptablesRule []string
}

func newReadinessMgr(path string, iptablesMgr *iptables.IPTables, iptablesRule []string, signalCh chan os.Signal, logger *log.Logger) *readinessMgr {
	return &readinessMgr{
		path:         path,
		logger:       logger,
		signalCh:     signalCh,
		iptablesMgr:  iptablesMgr,
		iptablesRule: iptablesRule,
	}
}

func (rf *readinessMgr) checkFileExists() error {
	if !rf.fileExists {
		info, err := os.Stat(rf.path)
		if err != nil {
			return err
		}

		if !info.Mode().IsRegular() {
			return fmt.Errorf("'%s' is not a regular file", rf.path)
		}

		rf.fileExists = true
	}

	return nil
}

func (rf *readinessMgr) checkReadinessFile() error {
	status, err := os.ReadFile(rf.path)
	if err != nil {
		return fmt.Errorf("failed to read file '%s': %v", rf.path, err)
	}

	if string(status) != readyState {
		return fmt.Errorf("'coredns' container is not ready yet, reporting '%s' status", string(status))
	}

	return nil
}

func (rf *readinessMgr) checkReadinessStatus() error {
	var err error
	defer func() {
		if err != nil {
			rf.logger.Warnf("failed to check readiness status: %v", err)
		}
	}()

	if err = rf.checkFileExists(); err != nil {
		return err
	}

	if err = rf.checkReadinessFile(); err != nil {
		return err
	}

	return nil
}

func (rf *readinessMgr) handleStatus() error {
	if err := rf.checkReadinessStatus(); err != nil {
		if rf.ready == nil || *rf.ready {
			if err := rf.deleteIPtablesRule(); err != nil {
				return fmt.Errorf("failed to delete the rule: %w", err)
			}

			rf.ready = ptr.To(false)
		}

		return nil
	}

	if rf.ready == nil || !*rf.ready {
		if err := rf.appendIPtablesRule(); err != nil {
			return fmt.Errorf("failed to append the rule: %w", err)
		}

		rf.ready = ptr.To(true)
	}

	return nil
}

func (rf *readinessMgr) deleteIPtablesRule() error {
	ok, err := rf.iptablesMgr.Exists(iptableName, iptableChainName, rf.iptablesRule...)
	if err != nil {
		return err
	}

	if ok {
		if err := rf.iptablesMgr.Delete(iptableName, iptableChainName, rf.iptablesRule...); err != nil {
			return err
		}
	}

	rf.logger.Info("deleted the rule")

	return nil
}

func (rf *readinessMgr) appendIPtablesRule() error {
	ok, err := rf.iptablesMgr.Exists(iptableName, iptableChainName, rf.iptablesRule...)
	if err != nil {
		return err
	}

	if !ok {
		if err := rf.iptablesMgr.Append(iptableName, iptableChainName, rf.iptablesRule...); err != nil {
			return err
		}
	}
	rf.logger.Info("appended the rule")

	return nil
}

func (rf *readinessMgr) startWatcher() error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("failed to create watcher: %w", err)
	}
	defer watcher.Close()

	err = watcher.Add(rf.path)
	if err != nil {
		return fmt.Errorf("failed to watch '%s': %w", rf.path, err)
	}

	rf.logger.Info("starting inotify watcher")

	if err := rf.handleStatus(); err != nil {
		return fmt.Errorf("failed to detect initial readiness status: %w", err)
	}

	waitFor := 100 * time.Millisecond
	t := time.AfterFunc(math.MaxInt64, func() {
		if err := rf.handleStatus(); err != nil {
			rf.logger.Errorf("failed to handle status: %v", err)
		}
	})
	t.Stop()

loop:
	for {
		select {
		case event := <-watcher.Events:
			if event.Op == fsnotify.Write {
				t.Reset(waitFor)
			} else {
				rf.logger.Warnf("unsupported inotify event received: %s, restarting watcher...", event.Op)
				if err := syscall.Exec(iptablesLoopBinary, []string{iptablesLoopBinary}, os.Environ()); err != nil {
					return fmt.Errorf("failed to restart watcher: %v", err)
				}
			}

		case s := <-rf.signalCh:
			rf.logger.Infof("signal %s received, exiting...", s)
			// best-effort delete to make sure the socket rule is deleted on exit
			_ = rf.deleteIPtablesRule()
			break loop
		}
	}

	return nil
}

func main() {
	logger := log.NewLogger()
	log.SetDefault(logger)

	kubeDnsSvc := os.Getenv("KUBE_DNS_SVC_IP")
	if len(kubeDnsSvc) == 0 {
		logger.Fatal("failed to resolve KUBR_DNS_SVC_IP env")
	}

	iptablesRule := strings.Fields(fmt.Sprintf("-d %s/32 -m socket --nowildcard -j NOTRACK", kubeDnsSvc))

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	iptablesMgr, err := iptables.New(iptables.IPFamily(iptables.ProtocolIPv4), iptables.Timeout(60))
	if err != nil {
		logger.Fatalf("failed to init iptables manager: %v", err)
	}

	rf := newReadinessMgr(readinessFilePath, iptablesMgr, iptablesRule, sigs, logger)

	if err := rf.checkReadinessStatus(); err != nil {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()

	loop:
		for {
			select {
			case <-ticker.C:
				if err := rf.checkReadinessStatus(); err != nil {
					continue
				}

				break loop

			case s := <-rf.signalCh:
				rf.logger.Infof("signal %s received, exiting...", s)
				return
			}
		}
	}

	if err := rf.startWatcher(); err != nil {
		log.Errorf("watcher failed: %v", err)
	}
}
