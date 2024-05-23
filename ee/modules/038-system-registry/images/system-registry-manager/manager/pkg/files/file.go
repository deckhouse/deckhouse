/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package pkg

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
)

func CompareChecksum(lFilePath, rFilePath string) (bool, error) {
	lSum, err := GetChecksum(lFilePath)
	if err != nil {
		return false, fmt.Errorf("error calculating checksum for %s: %v", lFilePath, err)
	}
	rSum, err := GetChecksum(rFilePath)
	if err != nil {
		return false, fmt.Errorf("error calculating checksum for %s: %v", rFilePath, err)
	}
	return lSum == rSum, nil
}

func GetChecksum(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	sum := hash.Sum(nil)
	checksum := hex.EncodeToString(sum)
	return checksum, nil
}

func CopyFile(src, dst string) error {
	source, err := os.Open(src)
	if err != nil {
		return err
	}
	defer source.Close()

	if err := MkdirAllForFile(dst, os.ModePerm); err != nil {
		return err
	}

	destination, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destination.Close()

	_, err = io.Copy(destination, source)
	if err != nil {
		return err
	}

	return nil
}

func WriteFile(name string, data []byte, perm os.FileMode) error {
	if err := MkdirAllForFile(name, os.ModePerm); err != nil {
		return err
	}
	return os.WriteFile(name, data, perm)
}

func DeleteFileIfExist(fileName string) error {
	if _, err := os.Stat(fileName); err == nil {
		err := os.Remove(fileName)
		if err != nil {
			return fmt.Errorf("failed to delete file '%s': %w", fileName, err)
		}
	} else if os.IsNotExist(err) {
		return nil
	} else {
		return fmt.Errorf("failed to check if file exists '%s': %w", fileName, err)
	}
	return nil
}
