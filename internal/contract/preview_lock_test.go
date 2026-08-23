package contract

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

type previewLock struct {
	SchemaVersion       string `json:"schemaVersion"`
	Classification      string `json:"classification"`
	QualificationStatus string `json:"qualificationStatus"`
	Source              struct {
		ForkRepository       string `json:"forkRepository"`
		UpstreamRepository   string `json:"upstreamRepository"`
		Commit               string `json:"commit"`
		ComparisonBaseCommit string `json:"comparisonBaseCommit"`
		ApplicationChartTree string `json:"applicationChartTree"`
		CRDChartTree         string `json:"crdChartTree"`
	} `json:"source"`
	Build struct {
		GoVersion       string `json:"goVersion"`
		Platform        string `json:"platform"`
		Package         string `json:"package"`
		Dockerfile      string `json:"dockerfile"`
		ImageRepository string `json:"imageRepository"`
		ImageTag        string `json:"imageTag"`
		UI              struct {
			SourcePath      string `json:"sourcePath"`
			Dockerfile      string `json:"dockerfile"`
			ImageRepository string `json:"imageRepository"`
			ImageTag        string `json:"imageTag"`
		} `json:"ui"`
	} `json:"build"`
	Tooling struct {
		GoBuilderImage string `json:"goBuilderImage"`
		GoToolchain    string `json:"goToolchain"`
		HelmImage      string `json:"helmImage"`
		TrivyImage     string `json:"trivyImage"`
		BuildkitImage  string `json:"buildkitImage"`
	} `json:"tooling"`
	Deployment struct {
		SkaffoldFile          string `json:"skaffoldFile"`
		SkaffoldProfile       string `json:"skaffoldProfile"`
		ChartPath             string `json:"chartPath"`
		ValuesPath            string `json:"valuesPath"`
		ValuesSHA256          string `json:"valuesSHA256"`
		Namespace             string `json:"namespace"`
		CloudDeployPipeline   string `json:"cloudDeployPipeline"`
		CloudDeployTarget     string `json:"cloudDeployTarget"`
		ControllerServiceType string `json:"controllerServiceType"`
		UIReplicas            int    `json:"uiReplicas"`
		UIServiceType         string `json:"uiServiceType"`
		UIOrigin              string `json:"uiOrigin"`
		BootstrapCRDs         struct {
			Mode           string `json:"mode"`
			ArtifactPath   string `json:"artifactPath"`
			AutomaticApply bool   `json:"automaticApply"`
			BundleSHA256   string `json:"bundleSHA256"`
		} `json:"bootstrapCRDs"`
		ExcludedTemplates []string `json:"excludedTemplates"`
		VerificationImage string   `json:"verificationImage"`
		Database          struct {
			Mode  string `json:"mode"`
			Image string `json:"image"`
		} `json:"database"`
		Substrate struct {
			Mode       string `json:"mode"`
			Version    string `json:"version"`
			Namespace  string `json:"namespace"`
			WorkerPool string `json:"workerPool"`
		} `json:"substrate"`
	} `json:"deployment"`
	Evidence struct {
		Bucket string `json:"bucket"`
		Prefix string `json:"prefix"`
	} `json:"evidence"`
	Release struct {
		Owner               string `json:"owner"`
		TriggerRepository   string `json:"triggerRepository"`
		TriggerTagPattern   string `json:"triggerTagPattern"`
		ReleaseNameTemplate string `json:"releaseNameTemplate"`
		ForkTagsRelease     bool   `json:"forkTagsRelease"`
		ProductionEligible  bool   `json:"productionEligible"`
	} `json:"release"`
}

func repositoryRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func readPreviewLock(t *testing.T, path string) previewLock {
	t.Helper()
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var lock previewLock
	if err := json.Unmarshal(contents, &lock); err != nil {
		t.Fatal(err)
	}
	return lock
}

func TestPreviewLockPinsControllerOnlyAssembly(t *testing.T) {
	root := repositoryRoot(t)
	lockPath := filepath.Join(root, "locks", "kagent-preview.lock.json")
	lock := readPreviewLock(t, lockPath)
	sha := regexp.MustCompile(`^[0-9a-f]{40}$`)
	sha256Pattern := regexp.MustCompile(`^[0-9a-f]{64}$`)

	if lock.SchemaVersion != "k8s-agents-platform/kagent-preview-lock/v1alpha1" ||
		lock.Classification != "preview-controller-only" ||
		lock.QualificationStatus != "assembly-unqualified" {
		t.Fatal("preview lock classification changed without qualification")
	}
	if lock.Source.ForkRepository != "https://github.com/pilprod/kagent.git" ||
		lock.Source.UpstreamRepository != "https://github.com/kagent-dev/kagent.git" {
		t.Fatal("preview source must be the reviewed fork/upstream pair")
	}
	if !sha.MatchString(lock.Source.Commit) || !sha.MatchString(lock.Source.ComparisonBaseCommit) ||
		!sha.MatchString(lock.Source.ApplicationChartTree) || !sha.MatchString(lock.Source.CRDChartTree) {
		t.Fatal("source and chart inputs must use full immutable Git objects")
	}
	if lock.Build.Platform != "linux/amd64" || lock.Build.Package != "core/cmd/controller-v2/main.go" ||
		lock.Build.Dockerfile != "go/Dockerfile" {
		t.Fatal("preview lane must build only the API v2 controller for linux/amd64")
	}
	if lock.Build.ImageRepository != "europe-west3-docker.pkg.dev/yourown-chat/docker/kagent-controller" ||
		lock.Build.ImageTag != "git-"+lock.Source.Commit {
		t.Fatal("controller tag must be derived from the exact source commit")
	}
	if lock.Build.UI.SourcePath != "ui" || lock.Build.UI.Dockerfile != "ui/Dockerfile" ||
		lock.Build.UI.ImageRepository != "europe-west3-docker.pkg.dev/yourown-chat/docker/kagent-ui" ||
		lock.Build.UI.ImageTag != "git-"+lock.Source.Commit {
		t.Fatal("UI companion image must be built from the exact source commit")
	}
	for name, image := range map[string]string{
		"Go builder": lock.Tooling.GoBuilderImage,
		"Helm":       lock.Tooling.HelmImage,
		"Trivy":      lock.Tooling.TrivyImage,
		"BuildKit":   lock.Tooling.BuildkitImage,
	} {
		if !strings.Contains(image, "@sha256:") {
			t.Fatalf("%s image must be digest pinned", name)
		}
	}
	if lock.Tooling.GoToolchain != lock.Build.GoVersion {
		t.Fatal("downloaded Go toolchain must match the source module version")
	}
	if lock.Deployment.SkaffoldFile != "deploy/skaffold.preview.yaml" ||
		lock.Deployment.SkaffoldProfile != "kagent-testbed" ||
		lock.Deployment.CloudDeployPipeline != "kagent-preview" ||
		lock.Deployment.CloudDeployTarget != "kagent-testbed" {
		t.Fatal("unexpected one-stage preview routing")
	}
	if lock.Deployment.ControllerServiceType != "ClusterIP" || lock.Deployment.UIReplicas != 1 ||
		lock.Deployment.UIServiceType != "ClusterIP" ||
		lock.Deployment.UIOrigin != "http://kagent-preview-ui.kagent-system.svc.cluster.local:8080" ||
		lock.Deployment.Substrate.Mode != "external" || lock.Deployment.Substrate.Version == "" {
		t.Fatal("preview exposure or external Substrate contract changed")
	}
	if lock.Deployment.Database.Mode != "bundled-testbed" ||
		!strings.Contains(lock.Deployment.Database.Image, "@sha256:") {
		t.Fatal("preview database must be disposable and digest pinned")
	}
	if lock.Deployment.BootstrapCRDs.Mode != "one-time-platform-admin" ||
		lock.Deployment.BootstrapCRDs.AutomaticApply ||
		!sha256Pattern.MatchString(lock.Deployment.BootstrapCRDs.BundleSHA256) {
		t.Fatal("CRDs must be a separately pinned, manually applied bootstrap artifact")
	}
	if lock.Deployment.VerificationImage != "docker.io/curlimages/curl:8.10.1@sha256:d9b4541e214bcd85196d6e92e2753ac6d0ea699f0af5741f8c6cccbfcf00ef4b" {
		t.Fatal("verification image must be digest pinned")
	}
	if strings.Join(lock.Deployment.ExcludedTemplates, ",") != "templates/rbac,templates/substrate-ate-api-rbac.yaml" {
		t.Fatal("Terraform-owned RBAC templates must be excluded from the release assembly")
	}
	if lock.Release.ForkTagsRelease || lock.Release.ProductionEligible {
		t.Fatal("preview assembly must not be promoted by fork tags or treated as production")
	}
	if lock.Release.TriggerRepository != "pilprod/yourown-chat-kagent" ||
		lock.Release.TriggerTagPattern != `^preview-[0-9]{8}-[1-9][0-9]*$` ||
		lock.Release.ReleaseNameTemplate != "kagent-{tag}-{sourceShortSHA}" {
		t.Fatal("preview trigger or deterministic release naming changed")
	}
	if !sha256Pattern.MatchString(lock.Deployment.ValuesSHA256) {
		t.Fatal("values file must be digest pinned")
	}
	values, err := os.ReadFile(filepath.Join(root, lock.Deployment.ValuesPath))
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(values)
	if hex.EncodeToString(digest[:]) != lock.Deployment.ValuesSHA256 {
		t.Fatal("preview Helm values drifted from the lock")
	}

	validator := exec.Command("python3", filepath.Join(root, "scripts", "preview-lock-env.py"), lockPath, "--format", "json")
	if output, err := validator.CombinedOutput(); err != nil {
		t.Fatalf("preview lock validator failed: %v\n%s", err, output)
	}
	if _, err := os.ReadFile(filepath.Join(root, "schemas", "kagent-preview-lock.schema.json")); err != nil {
		t.Fatal(err)
	}
}

func TestCloudBuildPinsAndReleasesBothPreviewImages(t *testing.T) {
	root := repositoryRoot(t)
	lock := readPreviewLock(t, filepath.Join(root, "locks", "kagent-preview.lock.json"))
	contents, err := os.ReadFile(filepath.Join(root, "cloudbuild.preview.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	config := string(contents)
	for name, required := range map[string]string{
		"Go builder pin":           lock.Tooling.GoBuilderImage,
		"Helm pin":                 lock.Tooling.HelmImage,
		"Trivy pin":                lock.Tooling.TrivyImage,
		"BuildKit pin":             lock.Tooling.BuildkitImage,
		"BuildKit driver override": `--driver-opt "image=$${BUILDKIT_IMAGE}"`,
		"controller SBOM":          "controller-sbom.spdx.json",
		"UI SBOM":                  "ui-sbom.spdx.json",
		"controller scan":          "controller-trivy-high-critical.json",
		"UI scan":                  "ui-trivy-high-critical.json",
		"UI render assertion":      `--ui-image "$${UI_IMAGE_REPOSITORY}:$${UI_IMAGE_TAG}"`,
		"two-image release map":    `$${IMAGE_REPOSITORY}=$${IMMUTABLE_CONTROLLER_IMAGE},$${UI_IMAGE_REPOSITORY}=$${IMMUTABLE_UI_IMAGE}`,
	} {
		if !strings.Contains(config, required) {
			t.Fatalf("cloudbuild preview is missing %s", name)
		}
	}
	if strings.Contains(config, "GOTOOLCHAIN=local") {
		t.Fatal("source requires the locked downloaded Go toolchain, not the builder-local version")
	}
}

func TestRenderedPreviewGateRejectsExposureAndUIReplicaDrift(t *testing.T) {
	root := repositoryRoot(t)
	controllerImage := "registry.invalid/controller:locked"
	uiImage := "registry.invalid/ui:locked"
	databaseImage := "registry.invalid/postgres@sha256:" + strings.Repeat("a", 64)
	valid := fmt.Sprintf(`
apiVersion: v1
kind: ServiceAccount
metadata:
  name: kagent-preview-controller
  namespace: kagent-system
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: kagent-preview-postgresql
  namespace: kagent-system
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: kagent-preview-ui
  namespace: kagent-system
---
apiVersion: v1
kind: Secret
metadata:
  name: kagent-preview-postgresql
  namespace: kagent-system
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: kagent-builtin-prompts
  namespace: kagent-system
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: kagent-preview-controller
  namespace: kagent-system
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: kagent-preview-ui-config
  namespace: kagent-system
---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: kagent-preview-postgresql
  namespace: kagent-system
---
apiVersion: v1
kind: Service
metadata:
  name: kagent-preview-controller
  namespace: kagent-system
spec:
  type: ClusterIP
  ports:
    - port: 8083
      targetPort: 8083
---
apiVersion: v1
kind: Service
metadata:
  name: kagent-preview-postgresql
  namespace: kagent-system
spec:
  type: ClusterIP
---
apiVersion: v1
kind: Service
metadata:
  name: kagent-preview-ui
  namespace: kagent-system
spec:
  type: ClusterIP
  ports:
    - port: 8080
      targetPort: 8080
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: kagent-preview-controller
  namespace: kagent-system
spec:
  replicas: 1
  template:
    spec:
      containers:
        - image: %s
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: kagent-preview-postgresql
  namespace: kagent-system
spec:
  replicas: 1
  template:
    spec:
      containers:
        - image: %s
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: kagent-preview-ui
  namespace: kagent-system
spec:
  replicas: 1
  template:
    spec:
      containers:
        - image: %s
---
apiVersion: kagent.dev/v1alpha3
kind: ModelConfig
metadata:
  name: default-model-config
  namespace: kagent-system
`, controllerImage, databaseImage, uiImage)

	tests := []struct {
		name     string
		manifest string
		wantOK   bool
	}{
		{name: "locked internal manifest", manifest: valid, wantOK: true},
		{
			name: "UI scaled above one",
			manifest: strings.Replace(
				valid,
				"name: kagent-preview-ui\n  namespace: kagent-system\nspec:\n  replicas: 1",
				"name: kagent-preview-ui\n  namespace: kagent-system\nspec:\n  replicas: 2",
				1,
			),
		},
		{
			name: "public UI service",
			manifest: strings.Replace(
				valid,
				"name: kagent-preview-ui\n  namespace: kagent-system\nspec:\n  type: ClusterIP",
				"name: kagent-preview-ui\n  namespace: kagent-system\nspec:\n  type: LoadBalancer",
				1,
			),
		},
		{
			name: "release-owned ingress",
			manifest: valid + `---
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: forbidden
  namespace: kagent-system
`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			manifest := filepath.Join(t.TempDir(), "rendered.yaml")
			mustWrite(t, manifest, test.manifest)
			command := exec.Command(
				"python3",
				filepath.Join(root, "scripts", "assert-rendered-preview.py"),
				manifest,
				"--controller-image", controllerImage,
				"--ui-image", uiImage,
				"--database-image", databaseImage,
			)
			output, err := command.CombinedOutput()
			if test.wantOK && err != nil {
				t.Fatalf("render gate rejected valid manifest: %v\n%s", err, output)
			}
			if !test.wantOK && err == nil {
				t.Fatalf("render gate accepted unsafe manifest:\n%s", output)
			}
		})
	}
}

func TestControllerOnlyGuard(t *testing.T) {
	tests := []struct {
		name       string
		changePath string
		updateTree bool
		wantOK     bool
		wantText   string
	}{
		{name: "exact baseline", wantOK: true},
		{name: "controller implementation", changePath: "go/core/v2/feature.go", wantOK: true},
		{name: "public API", changePath: "go/api/v1alpha3/types.go", wantText: "protected API/CRD/migration/runtime paths changed"},
		{name: "migration", changePath: "go/core/pkg/migrations/core/000099_bad.up.sql", wantText: "protected API/CRD/migration/runtime paths changed"},
		{name: "Go ADK runtime", changePath: "go/adk/runtime.go", wantText: "protected API/CRD/migration/runtime paths changed"},
		{name: "controller runtime image", changePath: "go/Dockerfile", wantText: "protected API/CRD/migration/runtime paths changed"},
		{name: "UI companion source", changePath: "ui/package.json", wantText: "protected API/CRD/migration/runtime paths changed"},
		{name: "unreviewed Helm manifest", changePath: "helm/kagent/templates/controller.yaml", wantText: "application chart tree drifted"},
		{name: "reviewed Helm manifest", changePath: "helm/kagent/templates/controller.yaml", updateTree: true, wantOK: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := newGuardFixture(t)
			if test.changePath != "" {
				fixture.commitChange(t, test.changePath)
			}
			fixture.writeLock(t, test.updateTree)
			command := exec.Command(
				fixture.guard,
				"--source", fixture.source,
				"--lock", fixture.lock,
				"--assembly-root", fixture.assembly,
			)
			output, err := command.CombinedOutput()
			if test.wantOK && err != nil {
				t.Fatalf("guard rejected valid fixture: %v\n%s", err, output)
			}
			if !test.wantOK && err == nil {
				t.Fatalf("guard accepted invalid fixture:\n%s", output)
			}
			if test.wantText != "" && !strings.Contains(string(output), test.wantText) {
				t.Fatalf("guard output %q does not contain %q", output, test.wantText)
			}
		})
	}
}

type guardFixture struct {
	root       string
	source     string
	assembly   string
	lock       string
	guard      string
	baseCommit string
	valuesHash string
}

func newGuardFixture(t *testing.T) guardFixture {
	t.Helper()
	root := t.TempDir()
	source := filepath.Join(root, "source")
	assembly := filepath.Join(root, "assembly")
	mustMkdir(t, filepath.Join(source, "go", "core", "cmd", "controller-v2"))
	mustMkdir(t, filepath.Join(source, "helm", "kagent", "templates"))
	mustMkdir(t, filepath.Join(source, "helm", "kagent-crds", "templates"))
	mustMkdir(t, filepath.Join(source, "ui"))
	mustMkdir(t, filepath.Join(assembly, "deploy", "helm"))
	mustWrite(t, filepath.Join(source, "go", "go.mod"), "module example.invalid/kagent\n\ngo 1.27.0\n\nreplace github.com/agent-substrate/substrate => github.com/kagent-dev/substrate v0.0.20\n")
	mustWrite(t, filepath.Join(source, "go", "Dockerfile"), "FROM scratch\n")
	mustWrite(t, filepath.Join(source, "ui", "Dockerfile"), "FROM scratch\n")
	mustWrite(t, filepath.Join(source, "go", "core", "cmd", "controller-v2", "main.go"), "package main\nfunc main() {}\n")
	mustWrite(t, filepath.Join(source, "helm", "kagent", "values.yaml"), "controller: {}\n")
	mustWrite(t, filepath.Join(source, "helm", "kagent-crds", "templates", "example.yaml"), "apiVersion: example.invalid/v1\nkind: Example\n")
	values := []byte("controller:\n  service:\n    type: ClusterIP\nui:\n  replicas: 0\n")
	mustWrite(t, filepath.Join(assembly, "deploy", "helm", "values.preview.yaml"), string(values))
	digest := sha256.Sum256(values)

	runGit(t, source, "init")
	runGit(t, source, "config", "user.name", "Test")
	runGit(t, source, "config", "user.email", "test@example.invalid")
	runGit(t, source, "remote", "add", "origin", "https://github.com/pilprod/kagent.git")
	runGit(t, source, "add", ".")
	runGit(t, source, "commit", "-m", "baseline")
	base := strings.TrimSpace(runGit(t, source, "rev-parse", "HEAD"))

	return guardFixture{
		root:       root,
		source:     source,
		assembly:   assembly,
		lock:       filepath.Join(root, "preview.lock.json"),
		guard:      filepath.Join(repositoryRoot(t), "scripts", "assert-controller-only.sh"),
		baseCommit: base,
		valuesHash: hex.EncodeToString(digest[:]),
	}
}

func (fixture guardFixture) commitChange(t *testing.T, path string) {
	t.Helper()
	mustMkdir(t, filepath.Dir(filepath.Join(fixture.source, path)))
	mustWrite(t, filepath.Join(fixture.source, path), "fixture change\n")
	runGit(t, fixture.source, "add", ".")
	runGit(t, fixture.source, "commit", "-m", "change")
}

func (fixture guardFixture) writeLock(t *testing.T, updateChartTree bool) {
	t.Helper()
	commit := strings.TrimSpace(runGit(t, fixture.source, "rev-parse", "HEAD"))
	chartCommit := fixture.baseCommit
	if updateChartTree {
		chartCommit = commit
	}
	chartTree := strings.TrimSpace(runGit(t, fixture.source, "rev-parse", chartCommit+":helm/kagent"))
	crdTree := strings.TrimSpace(runGit(t, fixture.source, "rev-parse", fixture.baseCommit+":helm/kagent-crds"))
	lock := fmt.Sprintf(`{
  "schemaVersion": "k8s-agents-platform/kagent-preview-lock/v1alpha1",
  "classification": "preview-controller-only",
  "qualificationStatus": "assembly-unqualified",
  "source": {
    "forkRepository": "https://github.com/pilprod/kagent.git",
    "upstreamRepository": "https://github.com/kagent-dev/kagent.git",
    "commit": %q,
    "comparisonBaseCommit": %q,
    "applicationChartTree": %q
    ,"crdChartTree": %q
  },
  "build": {
    "goVersion": "1.27.0",
    "platform": "linux/amd64",
    "package": "core/cmd/controller-v2/main.go",
    "dockerfile": "go/Dockerfile",
    "imageRepository": "europe-west3-docker.pkg.dev/yourown-chat/docker/kagent-controller",
    "imageTag": "git-%s",
    "ui": {"sourcePath": "ui", "dockerfile": "ui/Dockerfile", "imageRepository": "europe-west3-docker.pkg.dev/yourown-chat/docker/kagent-ui", "imageTag": "git-%s"}
  },
  "tooling": {
    "goBuilderImage": "docker.io/library/golang:1.26.5-bookworm@sha256:53eeac89074db483fdf0ab3be1df32bf6e47562263d2d0d6baa7f26acb4957dd",
    "goToolchain": "1.27.0",
    "helmImage": "docker.io/alpine/helm:3.19.0@sha256:aef9b56f64e866207d9591d0abd8f6d767b36aadd12edf68f8a719716d9d29c9",
    "trivyImage": "docker.io/aquasec/trivy:0.67.2@sha256:e2b22eac59c02003d8749f5b8d9bd073b62e30fefaef5b7c8371204e0a4b0c08",
    "buildkitImage": "docker.io/moby/buildkit:v0.23.0@sha256:a38cf64aa6415899097fac5bfcf6c07c95d5a68c67a21e3d254ba398a3c9187f"
  },
  "deployment": {
    "skaffoldFile": "deploy/skaffold.preview.yaml",
    "skaffoldProfile": "kagent-testbed",
    "chartPath": "../source/kagent/helm/kagent",
    "valuesPath": "deploy/helm/values.preview.yaml",
    "valuesSHA256": %q,
    "namespace": "kagent-system",
    "cloudDeployPipeline": "kagent-preview",
    "cloudDeployTarget": "kagent-testbed",
    "controllerServiceType": "ClusterIP",
    "uiReplicas": 1,
    "uiServiceType": "ClusterIP",
    "uiOrigin": "http://kagent-preview-ui.kagent-system.svc.cluster.local:8080",
    "bootstrapCRDs": {"mode": "one-time-platform-admin", "artifactPath": "evidence/bootstrap-crds.yaml", "automaticApply": false, "bundleSHA256": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
    "excludedTemplates": ["templates/rbac", "templates/substrate-ate-api-rbac.yaml"],
    "verificationImage": "docker.io/curlimages/curl:8.10.1@sha256:d9b4541e214bcd85196d6e92e2753ac6d0ea699f0af5741f8c6cccbfcf00ef4b",
    "database": {"mode": "bundled-testbed", "image": "docker.io/library/postgres:18.3-alpine@sha256:54451ecb8ab38c24c3ec123f2fd501303a3a1856a5c66e98cecf2460d5e1e9d7"},
    "substrate": {"mode": "external", "version": "0.0.20", "namespace": "ate-system", "workerPool": "kagent-default"}
  },
  "evidence": {"bucket": "yourown-chat-kagent-preview-europe-west3", "prefix": "evidence/yourown-chat-kagent/preview"},
  "release": {"owner": "pilprod/yourown-chat-kagent", "triggerRepository": "pilprod/yourown-chat-kagent", "triggerTagPattern": "^preview-[0-9]{8}-[1-9][0-9]*$", "releaseNameTemplate": "kagent-{tag}-{sourceShortSHA}", "forkTagsRelease": false, "productionEligible": false}
}
`, commit, fixture.baseCommit, chartTree, crdTree, commit, commit, fixture.valuesHash)
	mustWrite(t, fixture.lock, lock)
}

func mustMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
}

func mustWrite(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}

func runGit(t *testing.T, directory string, arguments ...string) string {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", directory}, arguments...)...)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s failed: %v\n%s", strings.Join(arguments, " "), err, output)
	}
	return string(output)
}
