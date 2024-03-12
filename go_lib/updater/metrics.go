package updater

type MetricsUpdater interface {
	ReleaseBlocked(name, reason string)
	WaitingManual(name string, totalPendingManualReleases float64)
}
