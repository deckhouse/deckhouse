package downloader

import "time"

type DownloadStatistic struct {
	Size         int
	PullDuration time.Duration
}
