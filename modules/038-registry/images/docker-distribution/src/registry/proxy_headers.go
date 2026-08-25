package registry

import (
	"context"
	"crypto/x509"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"

	"github.com/docker/distribution/configuration"
	dcontext "github.com/docker/distribution/context"
	"github.com/gorilla/handlers"
)

func proxyHeadersHandler(ctx context.Context, config *configuration.Configuration, h http.Handler) http.Handler {
	l := dcontext.GetLogger(ctx)
	cfg := config.HTTP.RealIP

	if !cfg.Enabled {
		l.Info("Reverse proxy real IP headers support disabled")
		return h
	}

	var filters []func(r *http.Request) bool
	var opts []string

	if cfg.ClientCert.CA != "" {
		certPool := x509.NewCertPool()

		pem, err := os.ReadFile(cfg.ClientCert.CA)
		if err != nil {
			l.
				WithError(err).
				Fatalf(
					"Cannot load reverse proxy real IP headers support client cert validation CA file %v",
					cfg.ClientCert.CA,
				)
			return h
		}

		certPool.AppendCertsFromPEM(pem)

		filters = append(filters, func(r *http.Request) bool {
			if r.TLS == nil {
				return false
			}

			for _, cert := range r.TLS.PeerCertificates {
				if cert == nil {
					continue
				}

				if _, err := cert.Verify(x509.VerifyOptions{
					Roots: certPool,
					KeyUsages: []x509.ExtKeyUsage{
						x509.ExtKeyUsageClientAuth,
					},
				}); err != nil {
					continue
				}

				if cfg.ClientCert.CN != "" && cert.Subject.CommonName != cfg.ClientCert.CN {
					continue
				}

				return true
			}

			return false
		})

		opts = append(opts, fmt.Sprintf("clientcert.ca: \"%v\"", cfg.ClientCert.CA))
		if cfg.ClientCert.CN != "" {
			opts = append(opts, fmt.Sprintf("clientcert.cn: \"%v\"", cfg.ClientCert.CN))
		}
	}

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// add port if not set
		if _, _, err := net.SplitHostPort(r.RemoteAddr); err != nil {
			r.RemoteAddr = net.JoinHostPort(r.RemoteAddr, "0")
		}

		h.ServeHTTP(w, r)
	})

	ret := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if len(filters) > 0 {
			for _, filter := range filters {
				if filter(r) {
					// Use proxy headers
					handlers.ProxyHeaders(next).ServeHTTP(w, r)
					return
				}
			}
		} else {
			// Use proxy headers
			handlers.ProxyHeaders(next).ServeHTTP(w, r)
			return
		}

		// Call the next handler in the chain.
		next.ServeHTTP(w, r)
	})

	opts = append(opts, fmt.Sprintf("filters: %v", len(filters)))
	l.Infof("Reverse proxy real IP headers support enabled (%v)", strings.Join(opts, ", "))

	return ret
}
