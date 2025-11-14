package debug

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"path"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/deckhouse/deckhouse/pkg/log"
)

type Server struct {
	router chi.Router

	logger *log.Logger
}

func NewServer(logger *log.Logger) *Server {
	router := chi.NewRouter()
	router.Use(middleware.Recoverer)

	return &Server{
		router: router,
		logger: logger,
	}
}

func (s *Server) Start(socketPath string) error {
	if err := os.MkdirAll(path.Dir(socketPath), 0o700); err != nil {
		return fmt.Errorf("create socket dir '%s': %w", path.Dir(socketPath), err)
	}

	if _, err := os.Stat(socketPath); err == nil {
		if err = os.Remove(socketPath); err != nil {
			return fmt.Errorf("remove socket '%s': %w", socketPath, err)
		}
	}

	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		return fmt.Errorf("create listener: %w", err)
	}

	go func() {
		if err = http.Serve(listener, s.router); err != nil {
			s.logger.Error("error starting debug socket server", log.Err(err))
			os.Exit(1)
		}
	}()

	return nil
}

func (s *Server) RegisterGet(url string, handler func(http.ResponseWriter, *http.Request)) {
	s.router.Get(url, func(writer http.ResponseWriter, request *http.Request) {
		handler(writer, request)
	})
}
