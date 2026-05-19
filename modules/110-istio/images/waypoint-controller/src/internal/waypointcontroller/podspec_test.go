//go:build !integration

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

package waypointcontroller

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
)

func TestWaypointPodSpec_Basics(t *testing.T) {
	cfg := newWaypointPodSpecConfig()
	spec, err := waypointPodSpec(cfg)
	if err != nil {
		t.Fatalf("waypointPodSpec returned error: %v", err)
	}

	if spec.ServiceAccountName != cfg.ServiceAccount {
		t.Errorf("ServiceAccountName = %q, want %q", spec.ServiceAccountName, cfg.ServiceAccount)
	}
	if len(spec.ImagePullSecrets) != 1 || spec.ImagePullSecrets[0].Name != "d8-istio-sidecar-registry" {
		t.Errorf("ImagePullSecrets = %v, want [d8-istio-sidecar-registry]", spec.ImagePullSecrets)
	}
	if spec.TerminationGracePeriodSeconds == nil || *spec.TerminationGracePeriodSeconds != 2 {
		t.Errorf("TerminationGracePeriodSeconds = %v, want 2", spec.TerminationGracePeriodSeconds)
	}
	if spec.InitContainers != nil {
		t.Errorf("InitContainers = %v, want nil", spec.InitContainers)
	}
	if len(spec.Containers) != 1 {
		t.Fatalf("expected 1 container, got %d", len(spec.Containers))
	}
	if len(spec.Volumes) == 0 {
		t.Error("Volumes should not be empty")
	}
}

func TestWaypointPodSpec_AntiAffinity(t *testing.T) {
	t.Run("present_when_minReplicas_at_least_2", func(t *testing.T) {
		cfg := newWaypointPodSpecConfig()
		cfg.EnablePodAntiAffinity = true
		spec, err := waypointPodSpec(cfg)
		if err != nil {
			t.Fatalf("waypointPodSpec returned error: %v", err)
		}

		if spec.Affinity == nil || spec.Affinity.PodAntiAffinity == nil {
			t.Fatal("expected pod anti-affinity for minReplicas >= 2")
		}
		pa := spec.Affinity.PodAntiAffinity
		if len(pa.PreferredDuringSchedulingIgnoredDuringExecution) != 1 {
			t.Fatalf("expected 1 preferred term, got %d", len(pa.PreferredDuringSchedulingIgnoredDuringExecution))
		}
		term := pa.PreferredDuringSchedulingIgnoredDuringExecution[0]
		if term.Weight != 100 {
			t.Errorf("anti-affinity weight = %d, want 100", term.Weight)
		}
		if term.PodAffinityTerm.TopologyKey != "kubernetes.io/hostname" {
			t.Errorf("topologyKey = %q, want kubernetes.io/hostname", term.PodAffinityTerm.TopologyKey)
		}
		match := term.PodAffinityTerm.LabelSelector.MatchLabels
		if match == nil {
			t.Fatal("expected MatchLabels")
		}
		if v := match[AppLabelKey]; v != AppLabelValue {
			t.Errorf("MatchLabels[%q] = %q, want %q", AppLabelKey, v, AppLabelValue)
		}
		if v := match["gateway.networking.k8s.io/gateway-name"]; v != "d8-waypoint-main" {
			t.Errorf("MatchLabels[gateway-name] = %q, want d8-waypoint-main", v)
		}
	})

	t.Run("absent_when_disabled", func(t *testing.T) {
		cfg := newWaypointPodSpecConfig()
		cfg.EnablePodAntiAffinity = false
		spec, err := waypointPodSpec(cfg)
		if err != nil {
			t.Fatalf("waypointPodSpec returned error: %v", err)
		}

		if spec.Affinity != nil {
			t.Error("expected nil Affinity")
		}
	})
}

func TestWaypointPodSpec_NodeSelectorTolerations(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		cfg := newWaypointPodSpecConfig()
		spec, err := waypointPodSpec(cfg)
		if err != nil {
			t.Fatalf("waypointPodSpec returned error: %v", err)
		}
		if len(spec.NodeSelector) != 0 {
			t.Errorf("NodeSelector should be empty, got %v", spec.NodeSelector)
		}
		if len(spec.Tolerations) != 0 {
			t.Errorf("Tolerations should be empty, got %v", spec.Tolerations)
		}
	})

	t.Run("populated", func(t *testing.T) {
		cfg := newWaypointPodSpecConfig()
		cfg.NodeSelector = map[string]string{"node-role/app": ""}
		cfg.Tolerations = []corev1.Toleration{
			{Key: "node-role/app", Operator: corev1.TolerationOpExists},
		}
		spec, err := waypointPodSpec(cfg)
		if err != nil {
			t.Fatalf("waypointPodSpec returned error: %v", err)
		}
		if v, ok := spec.NodeSelector["node-role/app"]; !ok || v != "" {
			t.Errorf("NodeSelector = %v, want {node-role/app: \"\"}", spec.NodeSelector)
		}
		if len(spec.Tolerations) != 1 || spec.Tolerations[0].Key != "node-role/app" {
			t.Errorf("Tolerations = %v, want [node-role/app exists]", spec.Tolerations)
		}
	})
}

func TestIstioProxyContainer_Basics(t *testing.T) {
	cfg := newWaypointPodSpecConfig()
	container, err := istioProxyContainer(cfg)
	if err != nil {
		t.Fatalf("istioProxyContainer returned error: %v", err)
	}

	if container.Name != "istio-proxy" {
		t.Errorf("Name = %q, want istio-proxy", container.Name)
	}
	if container.Image != cfg.ProxyImage {
		t.Errorf("Image = %q, want %q", container.Image, cfg.ProxyImage)
	}
	if container.ImagePullPolicy != corev1.PullIfNotPresent {
		t.Errorf("ImagePullPolicy = %v, want IfNotPresent", container.ImagePullPolicy)
	}
}

func TestIstioProxyContainer_Args(t *testing.T) {
	cfg := newWaypointPodSpecConfig()
	container, err := istioProxyContainer(cfg)
	if err != nil {
		t.Fatalf("istioProxyContainer returned error: %v", err)
	}

	expect := []string{
		"proxy",
		"waypoint",
		"--domain", "$(POD_NAMESPACE).svc." + cfg.ClusterDomain,
		"--serviceCluster", "d8-waypoint-" + cfg.InstanceName + ".$(POD_NAMESPACE)",
		"--proxyLogLevel", "warning",
		"--proxyComponentLogLevel", "misc:error",
		"--log_output_level", "default:info",
	}

	if len(container.Args) != len(expect) {
		t.Fatalf("args length = %d, want %d; got=%v", len(container.Args), len(expect), container.Args)
	}
	for i, want := range expect {
		if container.Args[i] != want {
			t.Errorf("args[%d] = %q, want %q", i, container.Args[i], want)
		}
	}
}

func TestIstioProxyContainer_Ports(t *testing.T) {
	cfg := newWaypointPodSpecConfig()
	container, err := istioProxyContainer(cfg)
	if err != nil {
		t.Fatalf("istioProxyContainer returned error: %v", err)
	}

	wantPorts := map[string]int32{
		"metrics":         15020,
		"status-port":     15021,
		"http-envoy-prom": 15090,
	}

	gotPorts := map[string]int32{}
	for _, p := range container.Ports {
		gotPorts[p.Name] = p.ContainerPort
	}

	for name, port := range wantPorts {
		got, ok := gotPorts[name]
		if !ok {
			t.Errorf("missing port %q", name)
			continue
		}
		if got != port {
			t.Errorf("port %q = %d, want %d", name, got, port)
		}
	}
}

func TestIstioProxyContainer_SecurityContext(t *testing.T) {
	cfg := newWaypointPodSpecConfig()
	container, err := istioProxyContainer(cfg)
	if err != nil {
		t.Fatalf("istioProxyContainer returned error: %v", err)
	}

	sc := container.SecurityContext
	if sc == nil {
		t.Fatal("SecurityContext is nil")
	}
	if sc.AllowPrivilegeEscalation == nil || *sc.AllowPrivilegeEscalation != false {
		t.Errorf("AllowPrivilegeEscalation = %v, want false", sc.AllowPrivilegeEscalation)
	}
	if sc.Privileged == nil || *sc.Privileged != false {
		t.Errorf("Privileged = %v, want false", sc.Privileged)
	}
	if sc.ReadOnlyRootFilesystem == nil || *sc.ReadOnlyRootFilesystem != true {
		t.Errorf("ReadOnlyRootFilesystem = %v, want true", sc.ReadOnlyRootFilesystem)
	}
	if sc.RunAsNonRoot == nil || *sc.RunAsNonRoot != true {
		t.Errorf("RunAsNonRoot = %v, want true", sc.RunAsNonRoot)
	}
	if sc.RunAsUser == nil || *sc.RunAsUser != 1337 {
		t.Errorf("RunAsUser = %v, want 1337", sc.RunAsUser)
	}
	if sc.RunAsGroup == nil || *sc.RunAsGroup != 1337 {
		t.Errorf("RunAsGroup = %v, want 1337", sc.RunAsGroup)
	}
	if sc.Capabilities == nil || len(sc.Capabilities.Drop) != 1 || sc.Capabilities.Drop[0] != "ALL" {
		t.Errorf("Capabilities.Drop = %v, want [ALL]", sc.Capabilities)
	}
}

func TestIstioProxyContainer_ReadinessProbe(t *testing.T) {
	cfg := newWaypointPodSpecConfig()
	container, err := istioProxyContainer(cfg)
	if err != nil {
		t.Fatalf("istioProxyContainer returned error: %v", err)
	}

	rp := container.ReadinessProbe
	if rp == nil {
		t.Fatal("ReadinessProbe is nil")
	}
	if rp.FailureThreshold != 4 {
		t.Errorf("FailureThreshold = %d, want 4", rp.FailureThreshold)
	}
	if rp.PeriodSeconds != 15 {
		t.Errorf("PeriodSeconds = %d, want 15", rp.PeriodSeconds)
	}
	if rp.HTTPGet == nil {
		t.Fatal("HTTPGet is nil")
	}
	if rp.HTTPGet.Path != "/healthz/ready" {
		t.Errorf("path = %q, want /healthz/ready", rp.HTTPGet.Path)
	}
	if rp.HTTPGet.Port.IntValue() != 15021 {
		t.Errorf("port = %d, want 15021", rp.HTTPGet.Port.IntValue())
	}
}

func TestIstioProxyContainer_StartupProbe(t *testing.T) {
	cfg := newWaypointPodSpecConfig()
	container, err := istioProxyContainer(cfg)
	if err != nil {
		t.Fatalf("istioProxyContainer returned error: %v", err)
	}

	sp := container.StartupProbe
	if sp == nil {
		t.Fatal("StartupProbe is nil")
	}
	if sp.FailureThreshold != 30 {
		t.Errorf("FailureThreshold = %d, want 30", sp.FailureThreshold)
	}
	if sp.InitialDelaySeconds != 1 {
		t.Errorf("InitialDelaySeconds = %d, want 1", sp.InitialDelaySeconds)
	}
	if sp.PeriodSeconds != 1 {
		t.Errorf("PeriodSeconds = %d, want 1", sp.PeriodSeconds)
	}
	if sp.HTTPGet == nil {
		t.Fatal("HTTPGet is nil")
	}
	if sp.HTTPGet.Path != "/healthz/ready" {
		t.Errorf("path = %q, want /healthz/ready", sp.HTTPGet.Path)
	}
	if sp.HTTPGet.Port.IntValue() != 15021 {
		t.Errorf("port = %d, want 15021", sp.HTTPGet.Port.IntValue())
	}
}
