/*
Copyright 2023 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package validators

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/aquasecurity/trivy/pkg/types"
	"github.com/go-logr/logr"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/open-policy-agent/frameworks/constraint/pkg/externaldata"
)

const insecureRegistryKey = "trivy.insecureRegistry."

type remoteValidator struct {
	remoteURL          string
	logger             logr.Logger
	scanOpts           types.ScanOptions
	insecureRegistries map[string]struct{}
}

func NewRemoteValidator(remoteURL string, logger logr.Logger, envs []string) *remoteValidator {
	insecureRegistries := make(map[string]struct{}, 0)
	for _, keyValue := range envs {
		key, value, found := strings.Cut(keyValue, "=")
		if !found {
			continue
		}

		if strings.HasPrefix(key, insecureRegistryKey) {
			logger.Info("insecure registry added", "registry", value)
			insecureRegistries[value] = struct{}{}
		}
	}

	return &remoteValidator{
		remoteURL: remoteURL,
		logger:    logger,
		scanOpts: types.ScanOptions{
			PkgTypes:            types.PkgTypes,
			Scanners:            types.AllScanners,
			ImageConfigScanners: types.AllImageConfigScanners,
			ScanRemovedPackages: true,
		},
		insecureRegistries: insecureRegistries,
	}
}

func (v *remoteValidator) ScanReport(ctx context.Context, data []byte) externaldata.Response {
	scanResults, err := v.validate(ctx, data)
	if err != nil {
		v.logger.Error(err, "Error validating images")
		return externaldata.Response{SystemError: err.Error()}
	}
	return externaldata.Response{Items: scanResults}
}

func (v *remoteValidator) validate(ctx context.Context, data []byte) ([]externaldata.Item, error) {
	var providerRequest externaldata.ProviderRequest
	if err := json.Unmarshal(data, &providerRequest); err != nil {
		return nil, fmt.Errorf("unable to unmarshal data to externaldata.ProviderRequest: %w", err)
	}

	results := make([]externaldata.Item, 0, len(providerRequest.Request.Keys))
	for _, img := range providerRequest.Request.Keys {
		results = append(results, v.scanImageReport(ctx, img))
	}
	return results, nil
}

func (v *remoteValidator) scanImageReport(ctx context.Context, img string) externaldata.Item {
	v.logger.Info("validate", "image", img, "remote", v.remoteURL)
	insecure := false
	ref, err := name.ParseReference(img, name.StrictValidation)
	if err == nil {
		if _, found := v.insecureRegistries[ref.Context().RegistryStr()]; found {
			v.logger.Info("insecure registry scan", "image", img)
			insecure = true
		}
	}

	scanReport, err := scanArtifact(ctx, img, v.remoteURL, http.Header{}, v.scanOpts, insecure)
	if err != nil {
		return externaldata.Item{
			Key:   img,
			Error: fmt.Errorf("unable to scan image: %w", err).Error(),
		}
	}

	v.logger.Info("validate", "image", img, "vulnerabilities found", scanReport.Results.Failed())
	if scanReport.Results.Failed() {
		vulnDescription := mutateResult(scanReport.Results)

		return externaldata.Item{
			Key:   img,
			Error: vulnDescription,
		}
	}

	return externaldata.Item{
		Key:   img,
		Value: "vulnerabilities not found",
	}
}

func mutateResult(results types.Results) string {
	vulnIDs := make([]string, 0)
	misIDs := make([]string, 0)
	for _, result := range results {
		for _, vuln := range result.Vulnerabilities {
			vulnIDs = append(vulnIDs, vuln.VulnerabilityID)
		}

		for _, mis := range result.Misconfigurations {
			if mis.Status == types.MisconfStatusFailure {
				misIDs = append(misIDs, mis.ID)
			}
		}
	}

	if len(vulnIDs) > 0 {
		return fmt.Sprintf("vulnerabilities: %v", vulnIDs)
	}

	if len(misIDs) > 0 {
		return fmt.Sprintf("misconfigurations: %v", misIDs)
	}

	data, _ := json.Marshal(results)

	return fmt.Sprintf("image contain errors: %s", data)
}
