/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package start

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	log "github.com/sirupsen/logrus"
	"k8s.io/client-go/tools/leaderelection"
	"system-registry-manager/internal/worker/steps"
	pkg_cfg "system-registry-manager/pkg/cfg"
	kube_actions "system-registry-manager/pkg/kubernetes/actions"
	pkg_logs "system-registry-manager/pkg/logs"
)

const (
	serverAddr         = "127.0.0.1:8097"
	shutdownTimeout    = 5 * time.Second
	leaderCheckTimeout = 10 * time.Second
	workInterval       = 10 * time.Second
	leaderWorkDelay    = 3 * time.Second
	slaveWorkDelay     = 3 * time.Second
)

var (
	server *http.Server
)

func Start() {
	initLogger()

	log.Info("Starting service")
	log.Infof("Config file: %s", pkg_cfg.GetConfigFilePath())

	if err := pkg_cfg.InitConfig(); err != nil {
		log.Fatalf("Error initializing config: %v", err)
	}

	server = &http.Server{
		Addr: serverAddr,
	}

	http.HandleFunc("/healthz", healthzHandler)
	http.HandleFunc("/readyz", readyzHandler)

	go handleShutdown()
	go startHTTPServer()

	leaderCh := make(chan bool)
	leaderCtx, cancelLeader := context.WithCancel(context.Background())
	defer cancelLeader()
	go checkLeader(leaderCtx, leaderCh)
	go runLeader(leaderCtx, leaderCh)

	for {
		if err := startManager(); err != nil {
			log.Errorf("Manager error: %v", err)
		}
		log.Info("Waiting for the next cycle...")
		time.Sleep(workInterval)
	}
}

func initLogger() {
	log.SetLevel(log.DebugLevel)
	log.SetFormatter(&log.JSONFormatter{})
}

func healthzHandler(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func readyzHandler(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func handleShutdown() {
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	log.Info("Shutting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		log.Errorf("Error shutting down server: %v", err)
	}
}

func startHTTPServer() {
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Errorf("Error starting server: %v", err)
	}
}

func startManager() error {
	manifestsSpec := pkg_cfg.NewManifestsSpec()
	context := context.Background()
	context = pkg_logs.SetLoggerToContext(context, "manager")
	log := pkg_logs.GetLoggerFromContext(context)

	if err := steps.PrepareWorkspace(context, manifestsSpec); err != nil {
		return err
	}
	if err := steps.GenerateCerts(context, manifestsSpec); err != nil {
		return err
	}
	if err := steps.CheckDestFiles(context, manifestsSpec); err != nil {
		return err
	}
	if !manifestsSpec.NeedChange() {
		log.Info("No changes")
		return nil
	}

	if err := kube_actions.SetMyStatusAndWaitApprove("update", 0); err != nil {
		return err
	}
	if err := steps.UpdateManifests(context, manifestsSpec); err != nil {
		return err
	}
	if err := kube_actions.SetMyStatusDone(); err != nil {
		return err
	}
	return nil
}

func checkLeader(ctx context.Context, isLeader chan<- bool) {
	leaderCallbacks := leaderelection.LeaderCallbacks{
		OnStartedLeading: func(ctx context.Context) {
			isLeader <- true
		},
		OnStoppedLeading: func() {
			isLeader <- false
		},
	}

	defer func() {
		isLeader <- false
	}()

	recorder := kube_actions.NewLeaderElectionRecorder(log.NewEntry(log.New()))
	err := kube_actions.StartLeaderElection(ctx, recorder, leaderCallbacks)
	if err != nil {
		log.Errorf("Failed to start leader election: %v", err)
	}
}

func runLeader(ctx context.Context, isLeader <-chan bool) {
	leader := false

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				leader = <-isLeader
			}
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return
		default:
			if leader {
				log.Info("Performing master's work...")
				time.Sleep(leaderWorkDelay)
			} else {
				log.Info("Performing slave's work...")
				time.Sleep(slaveWorkDelay)
			}
		}
	}
}
