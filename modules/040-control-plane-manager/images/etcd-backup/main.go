package main

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"time"

	"github.com/robfig/cron/v3"
	"go.etcd.io/etcd/client/pkg/v3/transport"
	clientv3 "go.etcd.io/etcd/client/v3"
)

type Config struct {
	EtcdCACert   string
	EtcdCert     string
	EtcdCaKey    string
	EtcdEndpoint string
	BackupDir    string
	MaxSnapshots int
	CronSchedule string
}

var (
	buf    bytes.Buffer
	logger = log.New(&buf, "logger: ", log.Lshortfile)
)

func loadConfig() Config {
	maxSnapshots, _ := strconv.Atoi(getEnv("MAX_SNAPSHOTS", "5"))
	return Config{
		EtcdCACert:   getEnv("ETCD_CACERT", "/etc/kubernetes/pki/etcd/ca.crt"),
		EtcdCert:     getEnv("ETCD_CERT", "/etc/kubernetes/pki/etcd/ca.crt"),
		EtcdCaKey:    getEnv("ETCD_KEY", "/etc/kubernetes/pki/etcd/ca.key"),
		EtcdEndpoint: getEnv("ETCD_ENDPOINT", "https://127.0.0.1:2371"),
		BackupDir:    getEnv("BACKUP_DIR", "/opt/deckhouse/backup/"),
		MaxSnapshots: maxSnapshots,
		CronSchedule: getEnv("CRON_SCHEDULE", "* * * * *"),
	}
}

func main() {

	config := loadConfig()
	c := cron.New()
	_, err := c.AddFunc(config.CronSchedule, func() { runBackup(config) })
	if err != nil {
		logger.Printf("Error adding cron job: %v\n", err)
		return
	}

	http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		logger.Println("healthz: ok")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	http.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		if err := checkReadiness(config); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("not ready"))
			logger.Println("readyz: failed")
		} else {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("ok"))
			logger.Println("readyz: ok")
		}
	})

	go http.ListenAndServe(":8096", nil)

	c.Start()
	select {}
}

func checkReadiness(config Config) error {
	// Check etcd connection
	tlsInfo := transport.TLSInfo{CertFile: config.EtcdCert, KeyFile: config.EtcdCaKey, TrustedCAFile: config.EtcdCACert}
	tlsConfig, err := tlsInfo.ClientConfig()
	if err != nil {
		return err
	}
	cli, err := clientv3.New(clientv3.Config{Endpoints: []string{config.EtcdEndpoint}, DialTimeout: 5 * time.Second, TLS: tlsConfig})
	if err != nil {
		return err
	}

	cli.Close()
	return nil
}

func runBackup(config Config) {
	timestamp := time.Now().Format("20060102-150405")
	snapshotPath := filepath.Join(config.BackupDir, fmt.Sprintf("etcd-backup-%s.snapshot", timestamp))
	backupArchive := filepath.Join(config.BackupDir, fmt.Sprintf("kube-backup-%s.tar", timestamp))
	archiveRoot := fmt.Sprintf("kube-backup-%s", timestamp)

	if err := backupEtcd(config, snapshotPath); err != nil {
		logger.Printf("Failed to backup etcd: %v\n", err)
		return
	}

	if err := createTarArchive(backupArchive, archiveRoot, snapshotPath, "/etc/kubernetes"); err != nil {
		logger.Printf("Failed to create archive: %v\n", err)
		return
	}

	logger.Println("Backup archive created successfully")
	os.Remove(snapshotPath)
	logger.Println("Temporary files removed")

	if err := manageSnapshots(config.BackupDir, config.MaxSnapshots); err != nil {
		logger.Printf("Failed to manage snapshots: %v\n", err)
	}
}

func backupEtcd(config Config, snapshotPath string) error {
	tlsInfo := transport.TLSInfo{CertFile: config.EtcdCert, KeyFile: config.EtcdCaKey, TrustedCAFile: config.EtcdCACert}
	tlsConfig, err := tlsInfo.ClientConfig()
	if err != nil {
		return fmt.Errorf("failed to setup TLS: %v", err)
	}

	cli, err := clientv3.New(clientv3.Config{Endpoints: []string{config.EtcdEndpoint}, DialTimeout: 5 * time.Second, TLS: tlsConfig})
	if err != nil {
		return fmt.Errorf("failed to connect to etcd: %v", err)
	}
	defer cli.Close()

	snapshotFile, err := os.Create(snapshotPath)
	if err != nil {
		return fmt.Errorf("failed to create snapshot file: %v", err)
	}
	defer snapshotFile.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	snapshotResp, err := cli.Snapshot(ctx)
	if err != nil {
		return fmt.Errorf("failed to initiate snapshot: %v", err)
	}
	defer snapshotResp.Close()

	if _, err = io.Copy(snapshotFile, snapshotResp); err != nil {
		return fmt.Errorf("failed to save snapshot: %v", err)
	}
	logger.Println("Snapshot saved successfully")
	return nil
}

func createTarArchive(output, rootDir string, files ...string) error {
	outFile, err := os.Create(output)
	if err != nil {
		return err
	}
	defer outFile.Close()

	tarWriter := tar.NewWriter(outFile)
	defer tarWriter.Close()

	for _, file := range files {
		if err := addFileOrDirToTar(tarWriter, rootDir, file); err != nil {
			return err
		}
	}
	return nil
}

func addFileOrDirToTar(tarWriter *tar.Writer, rootDir, path string) error {
	return filepath.Walk(path, func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		header, err := tar.FileInfoHeader(info, filePath)
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(filepath.Dir(path), filePath)
		if err != nil {
			return err
		}
		header.Name = filepath.Join(rootDir, relPath)

		if err := tarWriter.WriteHeader(header); err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		file, err := os.Open(filePath)
		if err != nil {
			return err
		}
		defer file.Close()

		_, err = io.Copy(tarWriter, file)
		return err
	})
}

func manageSnapshots(backupDir string, maxSnapshots int) error {
	files, err := os.ReadDir(backupDir)
	if err != nil {
		return err
	}

	var snapshots []os.FileInfo
	for _, file := range files {
		if !file.IsDir() && filepath.Ext(file.Name()) == ".tar" {
			info, err := file.Info()
			if err != nil {
				return err
			}
			snapshots = append(snapshots, info)
		}
	}

	if len(snapshots) <= maxSnapshots {
		return nil
	}

	sort.Slice(snapshots, func(i, j int) bool {
		return snapshots[i].ModTime().Before(snapshots[j].ModTime())
	})

	for i := 0; i < len(snapshots)-maxSnapshots; i++ {
		filePath := filepath.Join(backupDir, snapshots[i].Name())
		if err := os.Remove(filePath); err != nil {
			return fmt.Errorf("failed to remove old snapshot %s: %v", filePath, err)
		}
		logger.Printf("Removed old snapshot: %s\n", filePath)
	}
	return nil
}

func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}
