/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package utils

import (
	"bytes"
	"encoding/xml"
	"fmt"
)

func XMLEncode(input []byte) (string, error) {
	buf := &bytes.Buffer{}
	buf.Grow(len(input))
	if err := xml.EscapeText(buf, input); err != nil {
		return "", fmt.Errorf("xml-encode: %w", err)
	}
	return buf.String(), nil
}
