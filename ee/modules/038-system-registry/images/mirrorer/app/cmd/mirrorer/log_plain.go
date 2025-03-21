/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

//go:build: log_plain
package main

import "log/slog"

func init() {
	logHandler = slog.Default().Handler()
}
