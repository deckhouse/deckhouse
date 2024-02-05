package updater

import "time"

type WebhookDataGetter[R Release] interface {
	GetMessage(release R, time time.Time) string
}
