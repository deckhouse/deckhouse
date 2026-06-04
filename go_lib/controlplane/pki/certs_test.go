/*
Copyright 2026 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package pki

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateRootCertIfNotExists_CreatesNew(t *testing.T) {
	dir := t.TempDir()
	cfg := makeTestConfig(t, dir)
	spec := getRootCertSpec(CACertName)

	var rep PKIApplyReport
	cert, key, err := createRootCertIfNotExists(cfg, spec, &rep)
	require.NoError(t, err)
	require.Len(t, rep.Entries, 1)
	assert.Equal(t, PKIActionWrittenCreated, rep.Entries[0].Action)
	assert.NotNil(t, cert)
	assert.NotNil(t, key)
	assert.True(t, cert.IsCA)
	assert.Equal(t, "kubernetes", cert.Subject.CommonName)

	// The certificate must be persisted to disk.
	onDisk, _, err := readCertAndKey(dir, "ca")
	require.NoError(t, err)
	assert.Equal(t, cert.SerialNumber, onDisk.SerialNumber)
}

func TestCreateRootCertIfNotExists_ReusesExisting(t *testing.T) {
	dir := t.TempDir()
	cfg := makeTestConfig(t, dir)
	spec := getRootCertSpec(CACertName)

	var rep PKIApplyReport
	cert1, _, err := createRootCertIfNotExists(cfg, spec, &rep)
	require.NoError(t, err)
	require.Len(t, rep.Entries, 1)
	assert.Equal(t, PKIActionWrittenCreated, rep.Entries[0].Action)

	// Second call must return the same certificate without regenerating.
	rep = PKIApplyReport{}
	cert2, _, err := createRootCertIfNotExists(cfg, spec, &rep)
	require.NoError(t, err)
	require.Len(t, rep.Entries, 1)
	assert.Equal(t, PKIActionUnchanged, rep.Entries[0].Action)
	assert.Equal(t, cert1.SerialNumber, cert2.SerialNumber)
}

func TestCreateRootCertIfNotExists_FailsOnInvalidCA(t *testing.T) {
	dir := t.TempDir()
	cfg := makeTestConfig(t, dir)
	spec := getRootCertSpec(CACertName)

	// Write a soon-to-expire CA so that validateCert fails.
	expiredCert, expiredKey := makeExpiringSoonCACert(t, "kubernetes")
	err := writeCertAndKey(dir, "ca", expiredCert, expiredKey)
	require.NoError(t, err)

	var rep PKIApplyReport
	_, _, err = createRootCertIfNotExists(cfg, spec, &rep)

	var certErr *CertValidationError
	require.ErrorAs(t, err, &certErr)
	assert.Equal(t, "ca", certErr.BaseName)
}

func TestCreateLeafCertIfNotExists_CreatesNew(t *testing.T) {
	dir := t.TempDir()
	cfg := makeTestConfig(t, dir)

	caCert, caKey := makeTestCACert(t, "kubernetes")
	spec := getLeafCertSpec(ApiserverCertName)

	var rep PKIApplyReport
	err := createLeafCertIfNotExists(cfg, spec, caCert, caKey, &rep)
	require.NoError(t, err)
	require.Len(t, rep.Entries, 1)
	assert.Equal(t, PKIActionWrittenCreated, rep.Entries[0].Action)

	cert, _, err := readCertAndKey(dir, "apiserver")
	require.NoError(t, err)
	assert.Equal(t, "kube-apiserver", cert.Subject.CommonName)
}

func TestCreateLeafCertIfNotExists_SkipsValid(t *testing.T) {
	dir := t.TempDir()
	cfg := makeTestConfig(t, dir)

	caCert, caKey := makeTestCACert(t, "kubernetes")
	spec := getLeafCertSpec(ApiserverCertName)

	var rep PKIApplyReport
	err := createLeafCertIfNotExists(cfg, spec, caCert, caKey, &rep)
	require.NoError(t, err)
	require.Len(t, rep.Entries, 1)
	assert.Equal(t, PKIActionWrittenCreated, rep.Entries[0].Action)

	cert1, _, err := readCertAndKey(dir, "apiserver")
	require.NoError(t, err)

	// Second call must not regenerate the certificate.
	rep = PKIApplyReport{}
	err = createLeafCertIfNotExists(cfg, spec, caCert, caKey, &rep)
	require.NoError(t, err)
	require.Len(t, rep.Entries, 1)
	assert.Equal(t, PKIActionUnchanged, rep.Entries[0].Action)

	cert2, _, err := readCertAndKey(dir, "apiserver")
	require.NoError(t, err)
	assert.Equal(t, cert1.SerialNumber, cert2.SerialNumber)
}

func TestCreateLeafCertIfNotExists_RegeneratesInvalid(t *testing.T) {
	dir := t.TempDir()
	cfg := makeTestConfig(t, dir)

	caCert, caKey := makeTestCACert(t, "kubernetes")
	spec := getLeafCertSpec(ApiserverCertName)

	// Put a soon-to-expire cert on disk so validation fails.
	stale, staleKey := makeExpiringSoonLeafCert(t, "kube-apiserver", caCert, caKey)
	err := writeCertAndKey(dir, "apiserver", stale, staleKey)
	require.NoError(t, err)

	// Must regenerate without error.
	var rep PKIApplyReport
	err = createLeafCertIfNotExists(cfg, spec, caCert, caKey, &rep)
	require.NoError(t, err)
	require.Len(t, rep.Entries, 1)
	assert.Equal(t, PKIActionWrittenRegenerated, rep.Entries[0].Action)

	newCert, _, err := readCertAndKey(dir, "apiserver")
	require.NoError(t, err)
	assert.NotEqual(t, stale.SerialNumber, newCert.SerialNumber, "serial number should change after regeneration")
}
