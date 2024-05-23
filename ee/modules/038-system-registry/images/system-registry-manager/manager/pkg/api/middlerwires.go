/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package api

import (
	"net/http"
)

type SingleRequestConfig struct {
	requestCh chan struct{}
}

func (cfg *SingleRequestConfig) IsBusy() bool {
	isBusy := true
	select {
	case <-cfg.requestCh:
		defer func() {
			cfg.requestCh <- struct{}{}
		}()

		isBusy = false
	default:
		isBusy = true
	}
	return isBusy
}

func CreateSingleRequestConfig() *SingleRequestConfig {
	cfg := &SingleRequestConfig{
		requestCh: make(chan struct{}, 1),
	}
	cfg.requestCh <- struct{}{}
	return cfg
}

func SingleRequestMiddlewares(next http.Handler, cfg *SingleRequestConfig) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-cfg.requestCh:
			defer func() {
				cfg.requestCh <- struct{}{}
			}()
			next.ServeHTTP(w, r)
		default:
			http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
			return
		}
	})
}
