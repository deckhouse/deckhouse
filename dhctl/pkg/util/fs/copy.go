package fs

import (
	"fmt"
	"io"
	"os"
	"time"

	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
)

func CreateFileBackup(fName string) {
	suffix := time.Now().Format("150405-000")

	// Make copies of intermediate states.
	outName := fmt.Sprintf("%s-%s", fName, suffix)
	log.DebugF("save to: %s\n", outName)

	in, err := os.Open(fName)
	if err != nil {
		log.DebugF("open '%s': %v\n", fName, err)
		return
	}
	defer in.Close()

	out, err := os.Create(outName)
	if err != nil {
		log.DebugF("create copy '%s': %v\n", outName, err)
		return
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	if err != nil {
		log.DebugF("save copy: %v\n", err)
		return
	}
	_ = out.Close()
}
