package server

import (
	"context"
	"io"
	"os"

	"github.com/fsnotify/fsnotify"
	"github.com/prometheus/common/log"
	"gopkg.in/yaml.v3"
)

const (
	telemetryConfigFile = "/var/files/telemetry_config.yml"
)

type telemetryMessageProcessor struct {
	discardProcessor *discardProcessor
}

func newTelemetryMessageProcessor() *telemetryMessageProcessor {
	return &telemetryMessageProcessor{discardProcessor: newDiscardProcessor(nil)}
}

func (tmp *telemetryMessageProcessor) LoadConfig(ctx context.Context) error {
	err := tmp.parseConfig()
	if err != nil {
		return err
	}

	go tmp.runConfigWatcher(ctx)

	return nil
}

func (tmp *telemetryMessageProcessor) parseConfig() error {
	log.Info("Loading telemetry config")
	f, err := os.Open(telemetryConfigFile) // reader
	if err != nil {
		return err
	}
	defer f.Close()

	var config telemetryConfig

	err = yaml.NewDecoder(f).Decode(&config)
	if err != nil && err != io.EOF {
		return err
	}

	if config.Discard != nil {
		dp := newDiscardProcessor(config.Discard)
		tmp.discardProcessor = dp
	}

	return nil
}

func (tmp *telemetryMessageProcessor) runConfigWatcher(ctx context.Context) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatalf("start file watcher failed: %s", err)
	}
	defer watcher.Close()

	err = watcher.Add(telemetryConfigFile)
	if err != nil {
		log.Fatalf("add watcher for file failed: %s", err)
	}

	for {
		select {
		case event := <-watcher.Events:
			if event.Op == fsnotify.Remove {
				// k8s configmaps use symlinks,
				// old file is deleted and a new link with the same name is created
				_ = watcher.Remove(event.Name)
				err = watcher.Add(event.Name)
				if err != nil {
					log.Fatal(err)
				}
				switch event.Name {
				case telemetryConfigFile:
					err := tmp.parseConfig()
					if err != nil {
						log.Fatalf("Config reload failed: %s", err)
					}
				}
			}

		case err := <-watcher.Errors:
			log.Errorf("watch files error: %s", err)

		case <-ctx.Done():
			return
		}
	}
}

type telemetryConfig struct {
	Discard *discardConfig `yaml:"discard,omitempty"`
}
