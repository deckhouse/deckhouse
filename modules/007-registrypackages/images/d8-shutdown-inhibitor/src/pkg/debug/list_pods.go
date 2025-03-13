package debug

import (
	"fmt"
	"os"
	"time"

	"graceful_shutdown/pkg/app"
)

func ListPods(podLabel string) {
	nodeName, err := os.Hostname()
	if err != nil {
		fmt.Printf("START Error: get hostname: %v\n", err)
		os.Exit(1)
	}

	// Create application.
	app := app.NewApp(30*time.Minute, podLabel, nodeName)

	app.ListPods()
}
