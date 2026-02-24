package dryrun

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	errorsutil "k8s.io/apimachinery/pkg/util/errors"
)

type FileToPrint struct {
	RealPath  string
	PrintPath string
}

// PrintDryRunFiles prints the contents of the FileToPrints given to it to the writer w
func PrintDryRunFiles(files []FileToPrint, w io.Writer) error {
	errs := []error{}
	for _, file := range files {
		if len(file.RealPath) == 0 {
			continue
		}

		fileBytes, err := os.ReadFile(file.RealPath)
		if err != nil {
			errs = append(errs, err)
			continue
		}

		// Make it possible to fake the path of the file; i.e. you may want to tell the user
		// "Here is what would be written to /etc/kubernetes/admin.conf", although you wrote it to /tmp/kubeadm-dryrun/admin.conf and are loading it from there
		// Fall back to the "real" path if PrintPath is not set
		outputFilePath := file.PrintPath
		if len(outputFilePath) == 0 {
			outputFilePath = file.RealPath
		}
		outputFilePath = filepath.ToSlash(outputFilePath)

		fmt.Fprintf(w, "[dryrun] Would write file %q with content:\n", outputFilePath)
		fmt.Fprintf(w, "%s", fileBytes)
	}
	return errorsutil.NewAggregate(errs)
}

// NewFileToPrint makes a new instance of FileToPrint with the specified arguments
func NewFileToPrint(realPath, printPath string) FileToPrint {
	return FileToPrint{
		RealPath:  realPath,
		PrintPath: printPath,
	}
}

// PrintDryRunFile is a helper method around PrintDryRunFiles
func PrintDryRunFile(fileName, realDir, printDir string, w io.Writer) error {
	return PrintDryRunFiles([]FileToPrint{
		NewFileToPrint(filepath.Join(realDir, fileName), filepath.Join(printDir, fileName)),
	}, w)
}
