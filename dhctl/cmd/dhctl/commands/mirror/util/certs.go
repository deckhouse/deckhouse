package util

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func CreateCertsDir(caFile string) (string, error) {
	if caFile == "" {
		return "", nil
	}
	tempCertsDir, err := os.MkdirTemp("/tmp", "deckhouse_certs_*")
	if err != nil {
		return "", err
	}
	tempCAFile := filepath.Join(tempCertsDir, "ca.crt")
	return tempCertsDir, CopyFile(caFile, tempCAFile)
}

func CopyFile(src, dst string) (err error) {
	sfi, err := os.Stat(src)
	if err != nil {
		return
	}
	if !sfi.Mode().IsRegular() {
		// cannot copy non-regular files (e.g., directories,
		// symlinks, devices, etc.)
		return fmt.Errorf("CopyFile: non-regular source file %s (%q)", sfi.Name(), sfi.Mode().String())
	}
	dfi, err := os.Stat(dst)
	switch {
	case err != nil && !os.IsNotExist(err):
		return
	case err != nil:
	default:
		if !(dfi.Mode().IsRegular()) {
			return fmt.Errorf("CopyFile: non-regular destination file %s (%q)", dfi.Name(), dfi.Mode().String())
		}
		if os.SameFile(sfi, dfi) {
			return
		}
	}

	if err = os.Link(src, dst); err == nil {
		return
	}
	err = copyFileContents(src, dst)
	return
}

func copyFileContents(src, dst string) (err error) {
	in, err := os.Open(src)
	if err != nil {
		return
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return
	}
	defer func() {
		cerr := out.Close()
		if err == nil {
			err = cerr
		}
	}()
	if _, err = io.Copy(out, in); err != nil {
		return
	}
	err = out.Sync()
	return
}
