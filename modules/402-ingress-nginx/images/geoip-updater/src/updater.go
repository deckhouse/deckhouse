/*
Copyright 2025 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"io"
	"log"
	"os"
	"path"
	"sync"
	"time"
)

var once sync.Once

type Updater struct {
	interval        int
	maxmindClient   *Client
	databasePath    string
	editionsMD5Sums map[string]string
	mu              sync.Mutex
}

func NewUpdater(interval int, licenseKey string, databasePath string, accountID int, editionIDs []string) *Updater {
	client := NewClient(licenseKey)

	editionsMD5Sums := make(map[string]string)
	for _, editionID := range editionIDs {
		editionsMD5Sums[editionID] = ""
	}

	return &Updater{
		interval:        interval,
		databasePath:    databasePath,
		maxmindClient:   client,
		editionsMD5Sums: editionsMD5Sums,
	}
}

func (u *Updater) Run(ctx context.Context, wg *sync.WaitGroup) {
	once.Do(func() {
		u.refreshDatabsesMD5()
	})

	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Duration(u.interval) * time.Minute):
			for editionID, md5 := range u.editionsMD5Sums {
				ctxTimeout, cancelFunc := context.WithTimeout(context.Background(), 30*time.Second)
				u.updateDatabase(ctxTimeout, cancelFunc, wg, editionID, md5)
			}
		}
	}
}

func (u *Updater) updateDatabase(ctx context.Context, cancelFunc context.CancelFunc,
	wg *sync.WaitGroup, editionID, md5 string) {
	defer cancelFunc()
	wg.Add(1)
	defer wg.Done()
	downloadResponse, err := u.maxmindClient.Download(ctx, editionID, md5)
	if err != nil {
		log.Printf("Error downloading database: %v", err)
		return
	}

	if downloadResponse.UpdateAvailable {
		log.Printf("Database update available")
		file, err := os.CreateTemp(u.databasePath, "temp-geoip-database")
		if err != nil {
			log.Printf("Error creating temp file: %v", err)
			return
		}
		defer os.Remove(file.Name())

		if _, err := io.Copy(file, downloadResponse.Reader); err != nil {
			log.Printf("Error copying database to temp file: %v", err)
			return
		}

		filePath := genDatabasePath(u.databasePath, editionID)
		if err := os.Rename(file.Name(), filePath); err != nil {
			log.Printf("Error renaming temp file to database file: %v", err)
			return
		}

		if err := os.Chmod(filePath, 0644); err != nil {
			log.Printf("Error changing file permissions: %v", err)
		}

		u.mu.Lock()
		u.editionsMD5Sums[editionID] = downloadResponse.MD5
		u.mu.Unlock()
		log.Printf("Database updated successfully for %s edition", editionID)
	} else {
		log.Printf("Database %s is up to date", genDatabasePath(u.databasePath, editionID))
	}

}

func (u *Updater) refreshDatabsesMD5() {
	for editionID := range u.editionsMD5Sums {
		filePath := genDatabasePath(u.databasePath, editionID)
		file, err := os.Open(filePath)
		if err != nil {
			log.Printf("Error opening of database file %s: %v", filePath, err)
			continue
		}
		defer file.Close()

		hash := md5.New()
		if _, err := io.Copy(hash, file); err != nil {
			log.Printf("Error calculating md5 checksum of database file %s: %v", filePath, err)
			continue
		}

		u.editionsMD5Sums[editionID] = hex.EncodeToString(hash.Sum(nil))
	}
}

func genDatabasePath(databasePath string, editionID string) string {
	return path.Join(databasePath, editionID) + ".mmdb"
}
