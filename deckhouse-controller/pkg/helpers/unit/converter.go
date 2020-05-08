package unit

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/api/resource"
)

func Convert(mode string) error {
	reader := bufio.NewReader(os.Stdin)

	input, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("failed to read stdin: %s", err)
	}
	input = strings.TrimSuffix(input, "\n")

	switch mode {
	case "duration":
		bro, err := time.ParseDuration(input)
		if err != nil {
			return fmt.Errorf("failed to parse: %s", err)
		}
		fmt.Println(bro.Seconds())

	case "kube-resource-unit":
		quantity := resource.MustParse(input)
		fmt.Println(quantity.Value())

	default:
		return fmt.Errorf("unknown mode")
	}
	return nil
}
