/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package staticpod

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"sync"

	dlog "github.com/deckhouse/deckhouse/pkg/log"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/render"
	slogchi "github.com/samber/slog-chi"
)

const (
	listenAddr = "127.0.0.1:4576"

	authConfigPath         = "/etc/kubernetes/system-registry/auth_config/config.yaml"
	distributionConfigPath = "/etc/kubernetes/system-registry/distribution_config/config.yaml"
	pkiConfigDirectoryPath = "/etc/kubernetes/system-registry/pki"
	mirrorerConfigPath     = "/etc/kubernetes/system-registry/mirrorer/config.yaml"

	registryStaticPodConfigPath = "/etc/kubernetes/manifests/system-registry.yaml"
)

type apiServer struct {
	requestMutex sync.Mutex
	log          *dlog.Logger
	hostIP       string
	nodeName     string
}

func Run(ctx context.Context, hostIP, nodeName string) error {
	log := dlog.Default().
		With("component", "static pod manager")

	log.Info("Starting")
	defer log.Info("Stopped")

	api := &apiServer{
		log:      log,
		hostIP:   hostIP,
		nodeName: nodeName,
	}

	logConfig := slogchi.Config{
		WithSpanID:    true,
		WithTraceID:   true,
		WithRequestID: true,
	}

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(slogchi.NewWithConfig(slog.New(log.Handler()), logConfig))
	r.Use(middleware.Recoverer)
	r.Use(middleware.NoCache)
	r.Use(middleware.AllowContentType("application/json"))
	r.Use(render.SetContentType(render.ContentTypeJSON))

	r.Post("/staticpod", api.handleStaticPodPost)
	r.Delete("/staticpod", api.handleStaticPodDelete)

	httpServer := &http.Server{
		Addr:    listenAddr,
		Handler: r,
	}

	context.AfterFunc(ctx, func() {
		log.Info("Shutting down API server")

		if err := httpServer.Shutdown(ctx); err != nil {
			log.Error("Error shutting down API server", "error", err)
		}
	})

	log.Info("Starting API server", "addr", httpServer.Addr)
	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Error("API server error", err)
		return fmt.Errorf("failed to start API server: %w", err)
	}

	return nil
}

func (s *apiServer) handleStaticPodPost(w http.ResponseWriter, r *http.Request) {
	log := s.log.With(
		"handler", "post",
		"endpoint", r.RequestURI,
		"method", r.Method,
		"remoteAddr", r.RemoteAddr,
	)

	sendInternalError := func(message string, err error) {
		log.Error(message, "error", err)
		render.Render(w, r, ErrInternalErrorText(message))
	}

	var (
		data Config
		err  error
	)

	// Decode request body to struct EmbeddedRegistryConfig and return error if decoding fails
	if err = render.Bind(r, &data); err != nil {
		log.Warn("Validation error", "error", err)
		render.Render(w, r, ErrBadRequest(err))
		return
	}

	model := templateModel{
		Config:   data,
		Address:  s.hostIP,
		NodeName: s.nodeName,
	}

	// Lock the requestMutex to prevent concurrent requests and release it after the request is processed
	s.requestMutex.Lock()
	defer s.requestMutex.Unlock()

	var resp ChangesReponse

	// Sync the PKI files
	if resp.PKI, err = data.PKI.syncPKIFiles(
		pkiConfigDirectoryPath,
		&model.Hashes,
	); err != nil {
		sendInternalError("Error saving PKI files", err)
		return
	}

	// Process the templates with the given data and create the static pod and configuration files
	if resp.Auth, err = model.processTemplate(
		authConfigTemplateName,
		authConfigPath,
		&model.Hashes.AuthTemplate,
	); err != nil {
		sendInternalError("Error processing Auth template", err)
		return
	}

	if resp.Distribution, err = model.processTemplate(
		distributionConfigTemplateName,
		distributionConfigPath,
		&model.Hashes.DistributionTemplate,
	); err != nil {
		sendInternalError("Error processing Distribution template", err)
		return
	}

	if model.Registry.Mode == RegistryModeDetached {
		if resp.Mirrorer, err = model.processTemplate(
			mirrorerConfigTemplateName,
			mirrorerConfigPath,
			&model.Hashes.MirrorerTemplate,
		); err != nil {
			sendInternalError("Error processing Mirrorer template", err)
			return
		}
	} else {
		// Delete the mirrorer config file
		if resp.Mirrorer, err = deleteFile(mirrorerConfigPath); err != nil {
			sendInternalError("Error deleting Mirrorer config file", err)
			return
		}
	}

	if resp.Pod, err = model.processTemplate(
		registryStaticPodTemplateName,
		registryStaticPodConfigPath,
		nil,
	); err != nil {
		sendInternalError("Error processing static pod template", err)
		return
	}

	if resp.HasChanges() {
		log.Info(
			"Static pod and configuration created/updated successfully",
			"changes", resp,
		)
	} else {
		log.Info("No changes in static pod and configuration")
	}

	if err = render.Render(w, r, resp); err != nil {
		sendInternalError("Error encoding response", err)
	}
}

func (s *apiServer) handleStaticPodDelete(w http.ResponseWriter, r *http.Request) {
	log := s.log.With(
		"handler", "delete",
		"endpoint", r.RequestURI,
		"method", r.Method,
		"remoteAddr", r.RemoteAddr,
	)

	sendInternalError := func(message string, err error) {
		log.Error(message, "error", err)
		render.Render(w, r, ErrInternalErrorText(message))
	}

	// Lock the requestMutex to prevent concurrent requests and release it after the request is processed
	s.requestMutex.Lock()
	defer s.requestMutex.Unlock()

	var err error
	var resp ChangesReponse

	// Delete the static pod file
	if resp.Pod, err = deleteFile(registryStaticPodConfigPath); err != nil {
		sendInternalError("Error deleting static pod file", err)
		return
	}

	// Delete the auth config file
	if resp.Auth, err = deleteFile(authConfigPath); err != nil {
		sendInternalError("Error deleting Auth config file", err)
		return
	}

	// Delete the distribution config file
	if resp.Distribution, err = deleteFile(distributionConfigPath); err != nil {
		sendInternalError("Error deleting Distribution config file", err)
		return
	}

	// Delete the mirrorer config file
	if resp.Mirrorer, err = deleteFile(mirrorerConfigPath); err != nil {
		sendInternalError("Error deleting Mirrorer config file", err)
		return
	}

	if resp.PKI, err = deleteDirectory(pkiConfigDirectoryPath); err != nil {
		sendInternalError("Error deleting registry PKI directory", err)
		return
	}

	if resp.HasChanges() {
		log.Info(
			"Static pod and configuration deleted successfully",
			"changes", resp,
		)
	} else {
		log.Info("No static pod and configuration to delete")
	}

	if err = render.Render(w, r, resp); err != nil {
		sendInternalError("Error encoding response", err)
	}
}
