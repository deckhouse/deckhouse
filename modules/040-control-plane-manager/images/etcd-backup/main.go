package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"go.etcd.io/etcd/client/pkg/v3/transport"
	clientv3 "go.etcd.io/etcd/client/v3"
)

func main() {

	cacert := "/etc/kubernetes/pki/etcd/ca.crt"
	cert := "/etc/kubernetes/pki/etcd/ca.crt"
	key := "/etc/kubernetes/pki/etcd/ca.key"
	endpoint := "https://127.0.0.1:2379"

	tlsInfo := transport.TLSInfo{
		CertFile:      cert,
		KeyFile:       key,
		TrustedCAFile: cacert,
	}
	tlsConfig, err := tlsInfo.ClientConfig()
	if err != nil {
		fmt.Printf("Failed to setup TLS: %v\n", err)
		return
	}

	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   []string{endpoint},
		DialTimeout: 5 * time.Second,
		TLS:         tlsConfig,
	})
	if err != nil {
		fmt.Printf("Failed to connect to etcd: %v\n", err)
		return
	}
	defer cli.Close()

	snapshotPath := "/var/lib/etcd/etcd-backup.snapshot"
	snapshotFile, err := os.Create(snapshotPath)
	if err != nil {
		fmt.Printf("Failed to create snapshot file: %v\n", err)
		return
	}
	defer snapshotFile.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	snapshotResp, err := cli.Snapshot(ctx)
	if err != nil {
		fmt.Printf("Failed to initiate snapshot: %v\n", err)
		return
	}
	defer snapshotResp.Close()

	if _, err = io.Copy(snapshotFile, snapshotResp); err != nil {
		fmt.Printf("Failed to save snapshot: %v\n", err)
		return
	}
	fmt.Println("Snapshot saved successfully")

	kubeConfigSrc := "/etc/kubernetes"
	kubeConfigDst := "./kubernetes"
	if err := copyDir(kubeConfigSrc, kubeConfigDst); err != nil {
		fmt.Printf("Failed to copy kubernetes config: %v\n", err)
		return
	}
	defer os.RemoveAll(kubeConfigDst)

	backupArchive := "kube-backup.tar.gz"
	if err := createTarGz(backupArchive, snapshotPath, kubeConfigDst); err != nil {
		fmt.Printf("Failed to create archive: %v\n", err)
		return
	}
	fmt.Println("Backup archive created successfully")

	// Удаление временных файлов
	os.Remove(snapshotPath)
	fmt.Println("Temporary files removed")
}

// Копирование директории
func copyDir(src string, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		destPath := filepath.Join(dst, relPath)

		if info.IsDir() {
			return os.MkdirAll(destPath, info.Mode())
		}

		return copyFile(path, destPath)
	})
}

// Копирование файла
func copyFile(src, dst string) error {
	inFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer inFile.Close()

	outFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer outFile.Close()

	_, err = io.Copy(outFile, inFile)
	return err
}

// Создание tar.gz архива
func createTarGz(output string, files ...string) error {
	cmd := exec.Command("tar", append([]string{"-cvzf", output}, files...)...)
	return cmd.Run()
}
