package main

import (
	"context"
	"fmt"
	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"time"
)

func newRouter() *mux.Router {
	r := mux.NewRouter()

	staticFileDirectory := http.Dir("./root")

	r.PathPrefix("/status").HandlerFunc(statusHandler)
	r.PathPrefix("/{lang:ru|en}/documentation/{group:v[0-9]+.[0-9]+}-{channel:alpha|beta|ea|stable|rock-solid}").HandlerFunc(groupChannelHandler)
	r.PathPrefix("/{lang:ru|en}/documentation/{group:v[0-9]+}").HandlerFunc(groupHandler)
	r.PathPrefix("/{lang:ru|en}/documentation").HandlerFunc(rootDocHandler)
	r.PathPrefix("/health").HandlerFunc(healthCheckHandler)
	r.Path("/{lang:ru|en}/includes/header.html").HandlerFunc(headerHandler)
	r.Path("/includes/version-menu.html").HandlerFunc(headerHandler)
	r.Path("/includes/group-menu.html").HandlerFunc(groupMenuHandler)
	r.Path("/includes/channel-menu.html").HandlerFunc(channelMenuHandler)
	r.Path("/404.html").HandlerFunc(notFoundHandler)
	// Other (En) static
	r.PathPrefix("/").Handler(serveFilesHandler(staticFileDirectory))

	r.Use(LoggingMiddleware)

	r.NotFoundHandler = r.NewRoute().HandlerFunc(notFoundHandler).GetHandler()

	return r
}

func main() {
	logLevel := strings.ToLower(os.Getenv("LOG_LEVEL"))

	switch logLevel {
	case "debug":
		log.SetLevel(log.DebugLevel)
	case "trace":
		log.SetLevel(log.TraceLevel)
	default:
		log.SetLevel(log.InfoLevel)
	}

	log.SetFormatter(&log.TextFormatter{
		DisableColors: true,
		FullTimestamp: true,
	})

	log.Infoln(fmt.Sprintf("Started with LOG_LEVEL %s", logLevel))
	r := newRouter()

	srv := &http.Server{
		Handler:      r,
		Addr:         "0.0.0.0:8080",
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		err := srv.ListenAndServe()
		if err == http.ErrServerClosed {
			err = nil
		}
		if err != nil {
			log.Errorln(err)
		}
	}()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	<-c
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("shutdown failed:%+s", err)
	}
	log.Infoln("Shutting down...")
}
