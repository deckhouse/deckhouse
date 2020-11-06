package unit

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/api/resource"
)

func Convert(mode string, output string) error {
	reader := bufio.NewReader(os.Stdin)

	input, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("failed to read stdin: %s", err)
	}
	input = strings.TrimSuffix(input, "\n")

	switch mode {
	case "duration":
		duration, err := time.ParseDuration(input)
		if err != nil {
			return fmt.Errorf("failed to parse: %s", err)
		}
		switch output {
		case "value":
			fmt.Println(duration.Seconds())
		case "milli":
			fmt.Println(duration.Milliseconds())
		}

	case "kube-resource-unit":
		quantity := resource.MustParse(input)
		switch output {
		case "value":
			fmt.Println(quantity.Value())
		case "milli":
			fmt.Println(quantity.MilliValue())
		}

	default:
		return fmt.Errorf("unknown mode")
	}
	return nil
}
