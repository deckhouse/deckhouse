/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package api

import (
	"net/http"
)

func SingleRequestMiddlewares(next http.Handler, cfg *SingleRequestConfig) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f := func() error {
			next.ServeHTTP(w, r)
			return nil
		}
		isRun, _ := cfg.tryRunFunc(f)

		if !isRun {
			http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
			return
		}
	})
}
