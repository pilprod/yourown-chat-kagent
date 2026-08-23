package contract

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

type testbedLock struct {
	SchemaVersion      string `json:"schemaVersion"`
	Channel            string `json:"channel"`
	CandidateTag       string `json:"candidateTag"`
	ProductionEligible bool   `json:"productionEligible"`
	Source             struct {
		Repository string `json:"repository"`
		Tag        string `json:"tag"`
		Commit     string `json:"commit"`
	} `json:"source"`
	Charts struct {
		Repository  string `json:"repository"`
		Version     string `json:"version"`
		Application struct {
			OCIDigest     string `json:"ociDigest"`
			ArchiveSHA256 string `json:"archiveSHA256"`
		} `json:"application"`
		CRDs struct {
			OCIDigest     string `json:"ociDigest"`
			ArchiveSHA256 string `json:"archiveSHA256"`
		} `json:"crds"`
	} `json:"charts"`
	Images map[string]struct {
		Repository string `json:"repository"`
		Tag        string `json:"tag"`
		OCIDigest  string `json:"ociDigest"`
	} `json:"images"`
	Profile struct {
		ApplicationValuesSHA256 string `json:"applicationValuesSHA256"`
		CRDValuesSHA256         string `json:"crdValuesSHA256"`
		DatabaseMode            string `json:"databaseMode"`
		AgentRuntime            string `json:"agentRuntime"`
		Namespace               string `json:"namespace"`
		WorkloadNamespace       string `json:"workloadNamespace"`
		UIHostname              string `json:"uiHostname"`
		UIService               string `json:"uiService"`
	} `json:"profile"`
	Features struct {
		UI                bool `json:"ui"`
		KMCP              bool `json:"kmcp"`
		Substrate         bool `json:"substrate"`
		DefaultAgents     bool `json:"defaultAgents"`
		RealModelProvider bool `json:"realModelProvider"`
		ExternalAgentHost bool `json:"externalAgentHost"`
		Temporal          bool `json:"temporal"`
	} `json:"features"`
}

func TestStockTestbedReleaseLock(t *testing.T) {
	root := filepath.Join("..", "..")
	contents, err := os.ReadFile(filepath.Join(root, "locks", "kagent-testbed.lock.json"))
	if err != nil {
		t.Fatal(err)
	}
	var lock testbedLock
	if err := json.Unmarshal(contents, &lock); err != nil {
		t.Fatal(err)
	}

	if lock.SchemaVersion != "yourown-chat/kagent-testbed-lock/v1alpha2" || lock.Channel != "testbed" {
		t.Fatalf("unexpected testbed lock identity: %q %q", lock.SchemaVersion, lock.Channel)
	}
	if lock.ProductionEligible {
		t.Fatal("stock M0 must never be production eligible")
	}
	if !regexp.MustCompile(`^testbed-[0-9]{8}-[1-9][0-9]*$`).MatchString(lock.CandidateTag) {
		t.Fatalf("invalid reserved candidate tag %q", lock.CandidateTag)
	}
	if lock.Source.Repository != "https://github.com/kagent-dev/kagent" ||
		lock.Source.Tag != "v0.9.12" ||
		lock.Source.Commit != "b45990582595acea5f6e765b86a10b251c50d5c9" {
		t.Fatal("stock source identity drifted")
	}
	if lock.Charts.Repository != "oci://ghcr.io/kagent-dev/kagent/helm" || lock.Charts.Version != "0.9.12" {
		t.Fatal("stock chart identity drifted")
	}

	digest := regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
	plainSHA := regexp.MustCompile(`^[0-9a-f]{64}$`)
	for _, value := range []string{lock.Charts.Application.OCIDigest, lock.Charts.CRDs.OCIDigest} {
		if !digest.MatchString(value) {
			t.Fatalf("invalid OCI digest %q", value)
		}
	}
	for _, value := range []string{lock.Charts.Application.ArchiveSHA256, lock.Charts.CRDs.ArchiveSHA256} {
		if !plainSHA.MatchString(value) {
			t.Fatalf("invalid archive digest %q", value)
		}
	}
	for _, name := range []string{"controller", "ui", "agent", "skillsInit"} {
		image, ok := lock.Images[name]
		if !ok || image.Repository == "" || image.Tag == "" || !digest.MatchString(image.OCIDigest) {
			t.Fatalf("invalid immutable image lock %q", name)
		}
	}

	assertFileSHA256(t, filepath.Join(root, "deploy", "testbed", "kagent-values.yaml"), lock.Profile.ApplicationValuesSHA256)
	assertFileSHA256(t, filepath.Join(root, "deploy", "testbed", "kagent-crds-values.yaml"), lock.Profile.CRDValuesSHA256)

	if lock.Profile.Namespace != "kagent-system" ||
		lock.Profile.WorkloadNamespace != "kagent-testbed" ||
		lock.Profile.DatabaseMode != "external-cloudsql" ||
		lock.Profile.AgentRuntime != "python-only" ||
		lock.Profile.UIHostname != "kagent.yourown.chat" ||
		lock.Profile.UIService != "http://kagent-ui.kagent-system.svc.cluster.local:8080" {
		t.Fatal("testbed route or namespace identity drifted")
	}
	if !lock.Features.UI || lock.Features.KMCP || lock.Features.Substrate ||
		lock.Features.DefaultAgents || lock.Features.RealModelProvider ||
		lock.Features.ExternalAgentHost || lock.Features.Temporal {
		t.Fatal("M0 feature boundary drifted")
	}
}

func TestStockTestbedValuesStayClosed(t *testing.T) {
	path := filepath.Join("..", "..", "deploy", "testbed", "kagent-values.yaml")
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	values := string(contents)
	for _, required := range []string{
		"tag: \"\"",
		"0.9.12@sha256:d1ea7b70bb8d97de9f0774d44b598971c944b3ab4e88294b0bb78e59d1a63c15",
		"0.9.12@sha256:1d5ada8d7f65a6b9ad28232463f9fd670c4c20875baa1c8008aaa1f1f988382e",
		"mode: unsecure",
		"urlFile: /var/run/secrets/kagent-database/database-url",
		"bundled:\n      enabled: false",
		"driver: secrets-store-gke.csi.k8s.io",
		"default: ollama",
		"host: http://model-fixture.kagent-testbed.svc.cluster.local:11434",
		"type: ClusterIP",
		"startupProbe:\n    httpGet:\n      path: /health\n      port: http\n    initialDelaySeconds: 10\n    periodSeconds: 5\n    failureThreshold: 20",
		"substrateWorkerPool:\n  create: false",
		"oauth2-proxy:\n",
	} {
		if !strings.Contains(values, required) {
			t.Fatalf("required closed-profile value missing: %q", required)
		}
	}
	if strings.Contains(values, "bundled:\n      enabled: true") {
		t.Fatal("testbed profile must not run PostgreSQL inside Kubernetes")
	}
	if regexp.MustCompile(`(?m)^\s*apiKey:\s*\S+`).MatchString(values) {
		t.Fatal("model-provider API key must not be present in the testbed profile")
	}
	if strings.Contains(values, "LoadBalancer") || strings.Contains(values, "NodePort") {
		t.Fatal("testbed values must not request a public Kubernetes Service")
	}
}

func assertFileSHA256(t *testing.T, path, expected string) {
	t.Helper()
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	actual := sha256.Sum256(contents)
	if hex.EncodeToString(actual[:]) != expected {
		t.Fatalf("%s digest does not match release lock", path)
	}
}
