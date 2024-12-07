package main

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"sort"
	"strings"

	"github.com/aquasecurity/trivy/pkg/types"
)

type images map[string]string

type imagesDigests map[string]images

func execCommand(cmdName string, args ...string) ([]byte, error) {
	cmd := exec.Command(cmdName, args...)
	stdoutBuf := &bytes.Buffer{}
	stderrBuf := &bytes.Buffer{}
	cmd.Stdout = stdoutBuf
	cmd.Stderr = stderrBuf

	err := cmd.Run()
	if err != nil {
		code := -1
		if errExit, ok := err.(*exec.ExitError); ok {
			code = errExit.ExitCode()
		}

		err = fmt.Errorf("exitcode %d; %s\n%s", code, err.Error(), stderrBuf.String())
		return nil, err
	}

	return stdoutBuf.Bytes(), nil
}

func getImagesDigests(deckhouseImage string) (imagesDigests, error) {
	stdout, err := execCommand("docker", "run", "--rm", deckhouseImage, "cat", "/deckhouse/modules/images_digests.json")
	if err != nil {
		return nil, err
	}
	digests := imagesDigests{}
	err = json.Unmarshal(stdout, &digests)
	if err != nil {
		return nil, err
	}

	return digests, nil
}

func scanImage(image string) (types.Report, error) {
	stdout, err := execCommand("trivy", "image", "--scanners", "vuln", "--format", "json", image)
	if err != nil {
		return types.Report{}, err
	}

	report := types.Report{}
	err = json.Unmarshal(stdout, &report)
	if err != nil {
		return types.Report{}, err
	}

	return report, nil
}

type image struct {
	name       string
	path       string
	moduleName string
}

type module struct {
	name   string
	images []image
}

func extractModules(digests imagesDigests, repoPath string) ([]module, error) {
	modules := make([]module, 0, len(digests))

	extractImagesForModule := func(moduleName string, Images images) []image {
		res := make([]image, 0, len(digests))
		for img, digest := range Images {
			path := fmt.Sprintf("%s@%s", repoPath, digest)
			res = append(res, image{
				moduleName: moduleName,
				name:       img,
				path:       path,
			})
		}

		sort.Slice(res, func(i, j int) bool {
			return res[i].name < res[j].name
		})

		return res
	}

	extractOneModule := func(name string) error {
		Images, ok := digests[name]
		if !ok {
			return fmt.Errorf("no `%s` found in digests", name)
		}

		modules = append(modules, module{
			name:   name,
			images: extractImagesForModule(name, Images),
		})

		delete(digests, name)
		return nil
	}

	err := extractOneModule("deckhouse")
	if err != nil {
		return nil, err
	}

	err = extractOneModule("terraformManager")
	if err != nil {
		return nil, err
	}

	otherModules := make([]module, 0, len(digests))

	for name, Images := range digests {
		m := module{
			name:   name,
			images: extractImagesForModule(name, Images),
		}

		if strings.HasPrefix(name, "cloudProvider") {
			modules = append(modules, m)
			continue
		}

		otherModules = append(otherModules, m)
	}

	sort.Slice(otherModules, func(i, j int) bool {
		return otherModules[i].name < otherModules[j].name
	})

	modules = append(modules, otherModules...)

	return modules, nil
}

func extractRepoAndTag(mainImage string) (string, string, error) {
	imageParts := strings.Split(mainImage, ":")
	if len(imageParts) != 2 {
		return "", "", fmt.Errorf("invalid image format: %s", mainImage)
	}

	return imageParts[0], imageParts[1], nil
}

func addMainImagesToDeckhouseModule(mainImage string, modules []module) error {
	const moduleName = "deckhouse"
	repo, tag, err := extractRepoAndTag(mainImage)
	if err != nil {
		return err
	}

	installerImage := fmt.Sprintf("%s/install:%s", repo, tag)

	for _, m := range modules {
		if m.name != moduleName {
			continue
		}

		m.images = append(m.images, image{
			moduleName: moduleName,
			path:       mainImage,
			name:       "controller",
		})

		m.images = append(m.images, image{
			moduleName: moduleName,
			path:       installerImage,
			name:       "installer",
		})
	}

	return nil
}

type vuln struct {
	id           string
	binary       string
	pkg          string
	installedVer string
	fixedVer     string
	severity     string
	url          string
	Image        image
}

func scanImageWithLogs(Image image) (types.Report, error) {
	slog.Debug(fmt.Sprintf("Start scan image %s/%s: %s", Image.moduleName, Image.name, Image.path))

	report, err := scanImage(Image.path)
	if err != nil {
		slog.Debug(fmt.Sprintf("Scan image %s/%s: %s finnished with error", Image.moduleName, Image.name, Image.path))
		return types.Report{}, err
	}

	slog.Debug(fmt.Sprintf("Scan image %s/%s: %s finnished", Image.moduleName, Image.name, Image.path))

	return report, nil
}

func extractVulnFromReport(report types.Report, Image image) []vuln {
	res := make([]vuln, 0)
	for _, r := range report.Results {
		for _, v := range r.Vulnerabilities {
			res = append(res, vuln{
				id:           v.VulnerabilityID,
				binary:       r.Target,
				pkg:          v.PkgName,
				installedVer: v.InstalledVersion,
				fixedVer:     v.FixedVersion,
				severity:     v.Severity,
				url:          v.PrimaryURL,
				Image:        Image,
			})
		}
	}

	return res
}

type vulnImage struct {
	Image image
	vulns []vuln
}

type moduleReport struct {
	module module
	images []vulnImage
}

func scanModule(m module) (moduleReport, error) {
	res := moduleReport{
		module: m,
	}
	for _, img := range m.images {
		report, err := scanImageWithLogs(img)
		if err != nil {
			return moduleReport{}, nil
		}
		vunls := extractVulnFromReport(report, img)
		vunlImg := vulnImage{
			Image: img,
			vulns: vunls,
		}
		res.images = append(res.images, vunlImg)
	}

	return res, nil
}

func scanModules(modules []module) ([]moduleReport, error) {
	res := make([]moduleReport, len(modules))

	for _, m := range modules {
		report, err := scanModule(m)
		if err != nil {
			return nil, err
		}
		res = append(res, report)
	}

	return res, nil
}

func buildCSVReport(reports []moduleReport, w io.Writer) error {
	vulnToStringSlice := func(v vuln) []string {
		return []string{
			v.binary,
			v.pkg,
			v.installedVer,
			v.fixedVer,
			v.severity,
			v.id,
			v.url,
			v.Image.name,
			v.Image.path,
		}
	}

	csvWriter := csv.NewWriter(w)

	for _, report := range reports {
		err := csvWriter.Write([]string{
			report.module.name,
		})
		if err != nil {
			return err
		}

		for _, img := range report.images {
			for _, v := range img.vulns {
				err := csvWriter.Write(vulnToStringSlice(v))
				if err != nil {
					return err
				}
			}

			err := csvWriter.Write([]string{})
			if err != nil {
				return err
			}
		}
	}

	csvWriter.Flush()

	return nil
}

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	slog.SetDefault(logger)

	if len(os.Args) < 2 {
		slog.Error("Deckhouse image was not passed as argument.")
		os.Exit(1)
	}

	deckhouseImage := os.Args[1]
	digests, err := getImagesDigests(deckhouseImage)
	if err != nil {
		slog.Error("Failed to get images digests: ", err)
		os.Exit(2)
	}

	repo, _, err := extractRepoAndTag(deckhouseImage)
	if err != nil {
		slog.Error("Failed to extract repo and tag: ", err)
		os.Exit(3)
	}

	modules, err := extractModules(digests, repo)
	if err != nil {
		slog.Error("Failed to extract modules: ", err)
		os.Exit(4)
	}

	err = addMainImagesToDeckhouseModule(deckhouseImage, modules)
	if err != nil {
		slog.Error("Failed to add main images to deckhouse: ", err)
		os.Exit(5)
	}

	reports, err := scanModules(modules)
	if err != nil {
		slog.Error("Failed to scan modules: ", err)
		os.Exit(6)
	}

	err = buildCSVReport(reports, os.Stdout)
	if err != nil {
		slog.Error("Failed to build reports: ", err)
		os.Exit(7)
	}
}
