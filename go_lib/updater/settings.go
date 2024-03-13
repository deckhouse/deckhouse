package updater

type Settings interface {
	GetDisruptionApprovalMode() (string, bool)
}
