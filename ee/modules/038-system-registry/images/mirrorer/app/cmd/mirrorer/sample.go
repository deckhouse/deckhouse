/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log/slog"
	"mirrorer/pkg/syncer"
	"net/http"
	"os"
	"time"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

const (
	src = "localhost:8005"
	dst = "localhost:8006"

	//maxRate = 100 * 1024 * 1024 / 8 // MByte/s
	maxRate       = 0
	parallelCount = 20
	sleepTime     = 10 * time.Second
)

var _ = run

func run() {
	logger := slog.New(logHandler)

	log := logger
	ctx := context.Background()

	transport, err := getInsecureTransport()
	if err != nil {
		log.Error("Error getting HTTP transport", "error", err)
		os.Exit(1)
	}

	srcRegistry, err := name.NewRegistry(src)
	if err != nil {
		log.Error("Error parsing reg", "src", src, "error", err)
		os.Exit(1)
	}

	dstRegistry, err := name.NewRegistry(dst)
	if err != nil {
		log.Error("Error parsing dst reg", "dst", dst, "error", err)
		os.Exit(1)
	}

	syncer := &syncer.Syncer{
		Src: srcRegistry,
		Dst: dstRegistry,
		Log: logger.With("module", "regSyncer"),
		SrcOptions: []remote.Option{
			remote.WithContext(ctx),
			remote.WithTransport(transport),
		},
		DstOptions: []remote.Option{
			remote.WithContext(ctx),
			remote.WithTransport(transport),
		},
	}

	for {
		if err := syncer.Sync(ctx); err != nil {
			log.Error(
				"Sync error",
				"src", syncer.Src.String(),
				"dst", syncer.Dst.String(),
				"error", err,
			)
		}

		log.Debug("Sleeping before next loop", "duration", sleepTime)
		time.Sleep(sleepTime)
	}
}

func getInsecureTransport() (http.RoundTripper, error) {
	ret := remote.DefaultTransport.(*http.Transport).Clone()
	ret.TLSClientConfig = &tls.Config{
		InsecureSkipVerify: true, //nolint: gosec
	}

	certPool, err := x509.SystemCertPool()
	if err != nil {
		return nil, fmt.Errorf("cannot get system certificats pool: %w", err)
	}
	ret.TLSClientConfig.RootCAs = certPool

	return ret, nil
}
