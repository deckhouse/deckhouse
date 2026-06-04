//go:build !integration

/*
Copyright 2026 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license.
See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package waypointcontroller

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
)

func TestWaypointVolumes(t *testing.T) {
	volumes := WaypointVolumes()

	wantVolumes := map[string]struct {
		sourceType string // emptyDir, downwardAPI, projected, configMap
		details    func(vol *corev1.Volume)
	}{
		"workload-socket": {
			sourceType: "emptyDir",
			details: func(vol *corev1.Volume) {
				ed := vol.VolumeSource.EmptyDir
				if ed == nil {
					t.Error("expected EmptyDir source")
				}
				if ed.Medium != "" {
					t.Errorf("workload-socket medium = %q, want empty", ed.Medium)
				}
			},
		},
		"istio-envoy": {
			sourceType: "emptyDir",
			details: func(vol *corev1.Volume) {
				ed := vol.VolumeSource.EmptyDir
				if ed == nil {
					t.Fatal("expected EmptyDir source")
				}
				if ed.Medium != corev1.StorageMediumMemory {
					t.Errorf("istio-envoy medium = %q, want Memory", ed.Medium)
				}
			},
		},
		"istio-data": {
			sourceType: "emptyDir",
		},
		"istio-podinfo": {
			sourceType: "downwardAPI",
			details: func(vol *corev1.Volume) {
				da := vol.VolumeSource.DownwardAPI
				if da == nil {
					t.Fatal("expected DownwardAPI source")
				}
				foundLabels, foundAnn := false, false
				for _, item := range da.Items {
					switch item.Path {
					case "labels":
						foundLabels = true
						if item.FieldRef == nil || item.FieldRef.FieldPath != "metadata.labels" {
							t.Errorf("labels item FieldPath = %v", item.FieldRef)
						}
					case "annotations":
						foundAnn = true
						if item.FieldRef == nil || item.FieldRef.FieldPath != "metadata.annotations" {
							t.Errorf("annotations item FieldPath = %v", item.FieldRef)
						}
					}
				}
				if !foundLabels {
					t.Error("missing labels in istio-podinfo downwardAPI")
				}
				if !foundAnn {
					t.Error("missing annotations in istio-podinfo downwardAPI")
				}
			},
		},
		"istio-token": {
			sourceType: "projected",
			details: func(vol *corev1.Volume) {
				proj := vol.VolumeSource.Projected
				if proj == nil {
					t.Fatal("expected Projected source")
				}
				if len(proj.Sources) != 1 {
					t.Fatalf("expected 1 projected source, got %d", len(proj.Sources))
				}
				sat := proj.Sources[0].ServiceAccountToken
				if sat == nil {
					t.Fatal("expected ServiceAccountToken projection")
				}
				if sat.Audience != "istio-ca" {
					t.Errorf("audience = %q, want istio-ca", sat.Audience)
				}
				if sat.ExpirationSeconds == nil || *sat.ExpirationSeconds != 43200 {
					t.Errorf("expiration = %v, want 43200", sat.ExpirationSeconds)
				}
				if sat.Path != "istio-token" {
					t.Errorf("path = %q, want istio-token", sat.Path)
				}
			},
		},
		"istiod-ca-cert": {
			sourceType: "configMap",
			details: func(vol *corev1.Volume) {
				cm := vol.VolumeSource.ConfigMap
				if cm == nil {
					t.Fatal("expected ConfigMap source")
				}
				if cm.Name != "istio-ca-root-cert" {
					t.Errorf("ConfigMap name = %q, want istio-ca-root-cert", cm.Name)
				}
			},
		},
	}

	if len(volumes) != len(wantVolumes) {
		t.Errorf("got %d volumes, want %d", len(volumes), len(wantVolumes))
	}

	seen := map[string]bool{}
	for i := range volumes {
		vol := &volumes[i]
		seen[vol.Name] = true
		wantVol, ok := wantVolumes[vol.Name]
		if !ok {
			t.Errorf("unexpected volume %q", vol.Name)
			continue
		}
		switch wantVol.sourceType {
		case "emptyDir":
			if vol.VolumeSource.EmptyDir == nil {
				t.Errorf("volume %q: expected EmptyDir source", vol.Name)
			}
		case "downwardAPI":
			if vol.VolumeSource.DownwardAPI == nil {
				t.Errorf("volume %q: expected DownwardAPI source", vol.Name)
			}
		case "projected":
			if vol.VolumeSource.Projected == nil {
				t.Errorf("volume %q: expected Projected source", vol.Name)
			}
		case "configMap":
			if vol.VolumeSource.ConfigMap == nil {
				t.Errorf("volume %q: expected ConfigMap source", vol.Name)
			}
		}
		if wantVol.details != nil {
			wantVol.details(vol)
		}
	}

	for name := range wantVolumes {
		if !seen[name] {
			t.Errorf("missing volume %q", name)
		}
	}
}

func TestWaypointVolumeMounts(t *testing.T) {
	mounts := WaypointVolumeMounts()

	wantMounts := map[string]string{
		"workload-socket": "/var/run/secrets/workload-spiffe-uds",
		"istiod-ca-cert":  "/var/run/secrets/istio",
		"istio-data":      "/var/lib/istio/data",
		"istio-envoy":     "/etc/istio/proxy",
		"istio-token":     "/var/run/secrets/tokens",
		"istio-podinfo":   "/etc/istio/pod",
	}

	if len(mounts) != len(wantMounts) {
		t.Errorf("got %d volume mounts, want %d", len(mounts), len(wantMounts))
	}

	seen := map[string]bool{}
	for _, m := range mounts {
		seen[m.Name] = true
		wantPath, ok := wantMounts[m.Name]
		if !ok {
			t.Errorf("unexpected volume mount %q -> %q", m.Name, m.MountPath)
			continue
		}
		if m.MountPath != wantPath {
			t.Errorf("volume mount %q: MountPath = %q, want %q", m.Name, m.MountPath, wantPath)
		}
	}

	for name := range wantMounts {
		if !seen[name] {
			t.Errorf("missing volume mount %q", name)
		}
	}
}

func TestCAVolumeNameIsIstiodCACert(t *testing.T) {
	// The SPEC says the ConfigMap is mounted under volume name "istiod-ca-cert" so
	// that /var/run/secrets/istio/root-cert.pem resolves at Istio's expected path.
	volumes := WaypointVolumes()
	for i := range volumes {
		if volumes[i].Name == "istiod-ca-cert" && volumes[i].VolumeSource.ConfigMap != nil {
			return
		}
	}
	t.Error("istiod-ca-cert ConfigMap volume not found")
}
