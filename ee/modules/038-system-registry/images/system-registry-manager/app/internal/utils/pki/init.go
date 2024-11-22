/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package pki

import (
	"github.com/cloudflare/cfssl/log"
)

// set cfssl global log level to fatal
func init() {
	log.Level = log.LevelFatal
}
