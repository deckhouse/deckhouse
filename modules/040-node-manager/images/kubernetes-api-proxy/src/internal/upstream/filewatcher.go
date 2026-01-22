package upstream

import (
	"context"
	"encoding/json"
	"os"
	"time"
)

// fileWatcher watches and writes a file for changes
type fileWatcher struct {
	filePath string
	onChange func([]*Upstream)
	ticker   *time.Ticker
}

func newFileWatcher(filePath string, onChange func([]*Upstream)) (*fileWatcher, error) {
	return &fileWatcher{
		filePath: filePath,
		onChange: onChange,
	}, nil
}

func (fw *fileWatcher) Start(ctx context.Context) {
	fw.ticker = time.NewTicker(5 * time.Second)
	defer fw.ticker.Stop()

	fw.triggerChangedOutside()

	var lastModTime time.Time
	if fileInfo, err := os.Stat(fw.filePath); err == nil {
		lastModTime = fileInfo.ModTime()
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-fw.ticker.C:
			if fileInfo, err := os.Stat(fw.filePath); err == nil {
				if fileInfo.ModTime().After(lastModTime) {
					lastModTime = fileInfo.ModTime()
					fw.triggerChangedOutside()
				}
			}
		}
	}
}

func (fw *fileWatcher) Stop() {
	fw.ticker.Stop()
}

func (fw *fileWatcher) triggerChangedOutside() {
	changedFile, err := os.Open(fw.filePath)
	if err != nil {
		return
	}
	defer changedFile.Close() //nolint:errcheck

	var upstreamRecords []string

	if err := json.NewDecoder(changedFile).Decode(&upstreamRecords); err != nil {
		return
	}

	upstreams := make([]*Upstream, 0, len(upstreamRecords))
	for _, record := range upstreamRecords {
		upstreams = append(upstreams, NewUpstream(record))
	}

	fw.onChange(upstreams)
}

func (fw *fileWatcher) triggerChangedInside(upstreams []*Upstream) {
	changeFile, err := os.OpenFile(fw.filePath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0640)
	if err != nil {
		return
	}
	defer changeFile.Close() //nolint:errcheck

	upstreamRecords := make([]string, 0, len(upstreams))
	for _, record := range upstreams {
		upstreamRecords = append(upstreamRecords, record.Addr)
	}

	if err := json.NewEncoder(changeFile).Encode(&upstreamRecords); err != nil {
		return
	}
}
