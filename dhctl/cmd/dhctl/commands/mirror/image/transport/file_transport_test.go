// Copyright 2023 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package transport

import (
	"archive/tar"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/containers/image/v5/types"
	"github.com/deckhouse/deckhouse/dhctl/cmd/dhctl/commands/mirror/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	tmpDir := os.TempDir()
	explicitTmpDir, err := filepath.EvalSymlinks(tmpDir)
	if err == nil {
		os.Setenv("TMPDIR", explicitTmpDir)
	}
}

func TestTransportName(t *testing.T) {
	assert.Equal(t, "file", Transport.Name())
}

func TestTransportParseReference(t *testing.T) {
	tmpDir := t.TempDir()

	for _, path := range []string{
		"/",
		"/etc",
		tmpDir,
		"relativepath",
		tmpDir + "/thisdoesnotexist",
		"relativepath.tar.gz",
		tmpDir + "/thisdoesnotexist.tar.gz",
	} {
		ref, err := Transport.ParseReference(path)
		require.NoError(t, err, path)
		fileRef, ok := ref.(fileReference)
		require.True(t, ok)
		assert.Equal(t, util.AddTarGzExt(path), fileRef.archivePath, path)
		assert.Equal(t, util.AddTarGzExt(path), fileRef.StringWithinTransport(), path)
	}

	_, err := Transport.ParseReference(tmpDir + "/thisparentdoesnotexist/something")
	assert.Error(t, err)
}

func TestTransportValidatePolicyConfigurationScope(t *testing.T) {
	for _, scope := range []string{
		"/etc",
		"/this/does/not/exist",
	} {
		err := Transport.ValidatePolicyConfigurationScope(scope)
		assert.NoError(t, err, scope)
	}

	for _, scope := range []string{
		"relative/path",
		"/double//slashes",
		"/has/./dot",
		"/has/dot/../dot",
		"/trailing/slash/",
		"/",
	} {
		err := Transport.ValidatePolicyConfigurationScope(scope)
		assert.Error(t, err, scope)
	}
}

// refToTempFile creates a temporary tar.gz acrive and returns a reference to it.
func refToTempFile(t *testing.T) (types.ImageReference, string) {
	tmpDirectory := t.TempDir()
	tmpFile := filepath.Join(tmpDirectory, "image.tar.gz")
	err := util.NewTarGzWriter(tmpFile, func(w *tar.Writer) error { return nil })
	require.NoError(t, err)

	ref, err := Transport.ParseReference(tmpFile)
	require.NoError(t, err)
	return ref, tmpFile
}

func TestReferenceTransport(t *testing.T) {
	ref, _ := refToTempFile(t)
	assert.Equal(t, Transport, ref.Transport())
}

func TestReferenceStringWithinTransport(t *testing.T) {
	ref, tmpFile := refToTempFile(t)
	assert.Equal(t, tmpFile, ref.StringWithinTransport())
}

func TestReferenceDockerReference(t *testing.T) {
	ref, _ := refToTempFile(t)
	assert.Nil(t, ref.DockerReference())
}

func TestReferencePolicyConfigurationIdentity(t *testing.T) {
	ref, tmpFile := refToTempFile(t)

	assert.Equal(t, tmpFile, ref.PolicyConfigurationIdentity())
	// A non-canonical path.  Test just one, the various other cases are
	// tested in explicitfilepath.ResolvePathToFullyExplicit.
	ref, err := Transport.ParseReference(tmpFile)
	require.NoError(t, err)
	assert.Equal(t, tmpFile, ref.PolicyConfigurationIdentity())

	// "/" as a corner case.
	ref, err = Transport.ParseReference("/t.tar.gz")
	require.NoError(t, err)
	assert.Equal(t, "/t.tar.gz", ref.PolicyConfigurationIdentity())
}

func TestReferencePolicyConfigurationNamespaces(t *testing.T) {
	ref, tmpFile := refToTempFile(t)
	// We don't really know enough to make a full equality test here.
	ns := ref.PolicyConfigurationNamespaces()
	require.NotNil(t, ns)
	assert.NotEmpty(t, ns)
	assert.Equal(t, filepath.Dir(tmpFile), ns[0])

	// Test with a known path which should exist. Test just one non-canonical
	// path, the various other cases are tested in explicitfilepath.ResolvePathToFullyExplicit.
	//
	// It would be nice to test a deeper hierarchy, but it is not obvious what
	// deeper path is always available in the various distros, AND is not likely
	// to contains a symbolic link.
	for _, path := range []string{"/usr/share", "/usr/share/./."} {
		_, err := os.Lstat(path)
		require.NoError(t, err)
		ref, err := Transport.ParseReference(path)
		require.NoError(t, err)
		ns := ref.PolicyConfigurationNamespaces()
		require.NotNil(t, ns)
		assert.Equal(t, []string{"/usr"}, ns)
	}

	// "/" as a corner case.
	ref, err := Transport.ParseReference("/")
	require.NoError(t, err)
	assert.Equal(t, []string{}, ref.PolicyConfigurationNamespaces())
}

func TestReferenceNewImage(t *testing.T) {
	ref, _ := refToTempFile(t)

	dest, err := ref.NewImageDestination(context.Background(), nil)
	require.NoError(t, err)
	defer dest.Close()
	err = dest.PutManifest(context.Background(), v2s1Manifest, nil)
	assert.NoError(t, err)
	err = dest.Commit(context.Background(), nil) // nil unparsedToplevel is invalid, we don’t currently use the value
	assert.NoError(t, err)

	img, err := ref.NewImage(context.Background(), nil)
	assert.NoError(t, err)
	defer img.Close()
}

func TestReferenceNewImageNoValidManifest(t *testing.T) {
	ref, _ := refToTempFile(t)

	dest, err := ref.NewImageDestination(context.Background(), nil)
	require.NoError(t, err)
	defer dest.Close()
	err = dest.PutManifest(context.Background(), []byte(`{"schemaVersion":1}`), nil)
	assert.NoError(t, err)
	err = dest.Commit(context.Background(), nil) // nil unparsedToplevel is invalid, we don’t currently use the value
	assert.NoError(t, err)

	_, err = ref.NewImage(context.Background(), nil)
	assert.Error(t, err)
}

func TestReferenceNewImageSource(t *testing.T) {
	ref, _ := refToTempFile(t)
	src, err := ref.NewImageSource(context.Background(), nil)
	assert.NoError(t, err)
	defer src.Close()
}

func TestReferenceNewImageDestination(t *testing.T) {
	ref, _ := refToTempFile(t)
	dest, err := ref.NewImageDestination(context.Background(), nil)
	assert.NoError(t, err)
	defer dest.Close()
}

func TestReferenceDeleteImage(t *testing.T) {
	ref, _ := refToTempFile(t)
	err := ref.DeleteImage(context.Background(), nil)
	assert.Error(t, err)
}

var (
	v2s1Manifest = []byte(`
{
    "schemaVersion": 1,
    "name": "mitr/busybox",
    "tag": "latest",
    "architecture": "amd64",
    "fsLayers": [
        {
            "blobSum": "sha256:5f70bf18a086007016e948b04aed3b82103a36bea41755b6cddfaf10ace3c6ef"
        },
        {
            "blobSum": "sha256:5f70bf18a086007016e948b04aed3b82103a36bea41755b6cddfaf10ace3c6ef"
        },
        {
            "blobSum": "sha256:5f70bf18a086007016e948b04aed3b82103a36bea41755b6cddfaf10ace3c6ef"
        }
    ],
    "history": [
        {
            "v1Compatibility": "{\"id\":\"f1b5eb0a1215f663765d509b6cdf3841bc2bcff0922346abb943d1342d469a97\",\"parent\":\"594075be8d003f784074cc639d970d1fa091a8197850baaae5052c01564ac535\",\"created\":\"2016-03-03T11:29:44.222098366Z\",\"container\":\"c0924f5b281a1992127d0afc065e59548ded8880b08aea4debd56d4497acb17a\",\"container_config\":{\"Hostname\":\"56f0fe1dfc95\",\"Domainname\":\"\",\"User\":\"\",\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"ExposedPorts\":null,\"PublishService\":\"\",\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":null,\"Cmd\":[\"/bin/sh\",\"-c\",\"#(nop) LABEL Checksum=4fef81d30f31f9213c642881357e6662846a0f884c2366c13ebad807b4031368  ./tests/test-images/Dockerfile.2\"],\"Image\":\"594075be8d003f784074cc639d970d1fa091a8197850baaae5052c01564ac535\",\"Volumes\":null,\"VolumeDriver\":\"\",\"WorkingDir\":\"\",\"Entrypoint\":null,\"NetworkDisabled\":false,\"MacAddress\":\"\",\"OnBuild\":null,\"Labels\":{\"Checksum\":\"4fef81d30f31f9213c642881357e6662846a0f884c2366c13ebad807b4031368  ./tests/test-images/Dockerfile.2\",\"Name\":\"atomic-test-2\"}},\"docker_version\":\"1.8.2-fc22\",\"author\":\"\\\"William Temple \\u003cwtemple at redhat dot com\\u003e\\\"\",\"config\":{\"Hostname\":\"56f0fe1dfc95\",\"Domainname\":\"\",\"User\":\"\",\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"ExposedPorts\":null,\"PublishService\":\"\",\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":null,\"Cmd\":null,\"Image\":\"594075be8d003f784074cc639d970d1fa091a8197850baaae5052c01564ac535\",\"Volumes\":null,\"VolumeDriver\":\"\",\"WorkingDir\":\"\",\"Entrypoint\":null,\"NetworkDisabled\":false,\"MacAddress\":\"\",\"OnBuild\":null,\"Labels\":{\"Checksum\":\"4fef81d30f31f9213c642881357e6662846a0f884c2366c13ebad807b4031368  ./tests/test-images/Dockerfile.2\",\"Name\":\"atomic-test-2\"}},\"architecture\":\"amd64\",\"os\":\"linux\",\"Size\":0}\n"
        },
        {
            "v1Compatibility": "{\"id\":\"594075be8d003f784074cc639d970d1fa091a8197850baaae5052c01564ac535\",\"parent\":\"03dfa1cd1abe452bc2b69b8eb2362fa6beebc20893e65437906318954f6276d4\",\"created\":\"2016-03-03T11:29:38.563048924Z\",\"container\":\"fd4cf54dcd239fbae9bdade9db48e41880b436d27cb5313f60952a46ab04deff\",\"container_config\":{\"Hostname\":\"56f0fe1dfc95\",\"Domainname\":\"\",\"User\":\"\",\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"ExposedPorts\":null,\"PublishService\":\"\",\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":null,\"Cmd\":[\"/bin/sh\",\"-c\",\"#(nop) LABEL Name=atomic-test-2\"],\"Image\":\"03dfa1cd1abe452bc2b69b8eb2362fa6beebc20893e65437906318954f6276d4\",\"Volumes\":null,\"VolumeDriver\":\"\",\"WorkingDir\":\"\",\"Entrypoint\":null,\"NetworkDisabled\":false,\"MacAddress\":\"\",\"OnBuild\":null,\"Labels\":{\"Name\":\"atomic-test-2\"}},\"docker_version\":\"1.8.2-fc22\",\"author\":\"\\\"William Temple \\u003cwtemple at redhat dot com\\u003e\\\"\",\"config\":{\"Hostname\":\"56f0fe1dfc95\",\"Domainname\":\"\",\"User\":\"\",\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"ExposedPorts\":null,\"PublishService\":\"\",\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":null,\"Cmd\":null,\"Image\":\"03dfa1cd1abe452bc2b69b8eb2362fa6beebc20893e65437906318954f6276d4\",\"Volumes\":null,\"VolumeDriver\":\"\",\"WorkingDir\":\"\",\"Entrypoint\":null,\"NetworkDisabled\":false,\"MacAddress\":\"\",\"OnBuild\":null,\"Labels\":{\"Name\":\"atomic-test-2\"}},\"architecture\":\"amd64\",\"os\":\"linux\",\"Size\":0}\n"
        },
        {
            "v1Compatibility": "{\"id\":\"03dfa1cd1abe452bc2b69b8eb2362fa6beebc20893e65437906318954f6276d4\",\"created\":\"2016-03-03T11:29:32.948089874Z\",\"container\":\"56f0fe1dfc95755dd6cda10f7215c9937a8d9c6348d079c581a261fd4c2f3a5f\",\"container_config\":{\"Hostname\":\"56f0fe1dfc95\",\"Domainname\":\"\",\"User\":\"\",\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"ExposedPorts\":null,\"PublishService\":\"\",\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":null,\"Cmd\":[\"/bin/sh\",\"-c\",\"#(nop) MAINTAINER \\\"William Temple \\u003cwtemple at redhat dot com\\u003e\\\"\"],\"Image\":\"\",\"Volumes\":null,\"VolumeDriver\":\"\",\"WorkingDir\":\"\",\"Entrypoint\":null,\"NetworkDisabled\":false,\"MacAddress\":\"\",\"OnBuild\":null,\"Labels\":null},\"docker_version\":\"1.8.2-fc22\",\"author\":\"\\\"William Temple \\u003cwtemple at redhat dot com\\u003e\\\"\",\"config\":{\"Hostname\":\"56f0fe1dfc95\",\"Domainname\":\"\",\"User\":\"\",\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"ExposedPorts\":null,\"PublishService\":\"\",\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":null,\"Cmd\":null,\"Image\":\"\",\"Volumes\":null,\"VolumeDriver\":\"\",\"WorkingDir\":\"\",\"Entrypoint\":null,\"NetworkDisabled\":false,\"MacAddress\":\"\",\"OnBuild\":null,\"Labels\":null},\"architecture\":\"amd64\",\"os\":\"linux\",\"Size\":0}\n"
        }
    ],
    "signatures": [
        {
            "header": {
                "jwk": {
                    "crv": "P-256",
                    "kid": "OZ45:U3IG:TDOI:PMBD:NGP2:LDIW:II2U:PSBI:MMCZ:YZUP:TUUO:XPZT",
                    "kty": "EC",
                    "x": "ReC5c0J9tgXSdUL4_xzEt5RsD8kFt2wWSgJcpAcOQx8",
                    "y": "3sBGEqQ3ZMeqPKwQBAadN2toOUEASha18xa0WwsDF-M"
                },
                "alg": "ES256"
            },
            "signature": "dV1paJ3Ck1Ph4FcEhg_frjqxdlGdI6-ywRamk6CvMOcaOEUdCWCpCPQeBQpD2N6tGjkoG1BbstkFNflllfenCw",
            "protected": "eyJmb3JtYXRMZW5ndGgiOjU0NzgsImZvcm1hdFRhaWwiOiJDbjAiLCJ0aW1lIjoiMjAxNi0wNC0xOFQyMDo1NDo0MloifQ"
        }
    ]
}
`)
)
