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
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/render"
	slogchi "github.com/samber/slog-chi"
)

const (
	httpAPIAddr = "127.0.0.1:4576"
)

type apiServer struct {
	log      *slog.Logger
	services *servicesManager
}

func (api *apiServer) Run(ctx context.Context) error {
	log := api.log

	log.Info("Starting")
	defer log.Info("Stopped")

	logConfig := slogchi.Config{
		WithSpanID:    true,
		WithTraceID:   true,
		WithRequestID: true,
	}

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(slogchi.NewWithConfig(log, logConfig))
	r.Use(middleware.Recoverer)
	r.Use(middleware.NoCache)
	r.Use(middleware.AllowContentType("application/json"))
	r.Use(render.SetContentType(render.ContentTypeJSON))

	r.Post("/staticpod", api.handlePost)
	r.Delete("/staticpod", api.handleDelete)

	httpServer := &http.Server{
		Addr:    httpAPIAddr,
		Handler: r,
	}

	context.AfterFunc(ctx, func() {
		log.Info("Shutting down")

		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			log.Error("Shutting down error", "error", err)
		}
	})

	log.Info("Starting HTTP API", "addr", httpServer.Addr)
	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Error("HTTP API server error", "error", err)
		return fmt.Errorf("failed to start HTTP API server: %w", err)
	}

	return nil
}

func (api *apiServer) handlePost(w http.ResponseWriter, r *http.Request) {
	log := api.log.With(
		"handler", "POST",
		"endpoint", r.RequestURI,
		"method", r.Method,
		"remoteAddr", r.RemoteAddr,
	)

	var (
		err error
	)

	var request NodeServicesConfigModel

	// Decode request body to struct EmbeddedRegistryConfig and return error if decoding fails
	if err = render.Bind(r, &request); err != nil {
		log.Warn("Validation error", "error", err)
		render.Render(w, r, ErrBadRequest(err))
		return
	}

	changes, err := api.services.applyConfig(request)
	if err != nil {
		log.Error("Services configuration request error", "error", err)
		render.Render(w, r, ErrInternalError(err))
		return
	}

	if changes.HasChanges() {
		log.Info(
			"Services configuration created/updated successfully",
			"changes", changes,
		)
	} else {
		log.Info("No changes in services configuration")
	}

	if err = render.Render(w, r, changesReponse{ChangesModel: changes}); err != nil {
		log.Error("Cannot render services configuration response", "error", err)
		render.Render(w, r, ErrInternalError(err))
		return
	}
}

func (api *apiServer) handleDelete(w http.ResponseWriter, r *http.Request) {
	log := api.log.With(
		"handler", "DELETE",
		"endpoint", r.RequestURI,
		"method", r.Method,
		"remoteAddr", r.RemoteAddr,
	)

	changes, err := api.services.StopServices()
	if err != nil {
		log.Error("Stop services request error", "error", err)
		render.Render(w, r, ErrInternalError(err))
		return
	}

	if changes.HasChanges() {
		log.Info(
			"All services are stopped successfully",
			"changes", changes,
		)
	} else {
		log.Info("No services need to be stopped")
	}

	if err = render.Render(w, r, changesReponse{ChangesModel: changes}); err != nil {
		log.Error("Cannot render services stop response", "error", err)
		render.Render(w, r, ErrInternalError(err))
		return
	}
}
