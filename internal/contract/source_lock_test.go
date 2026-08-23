package contract

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

type sourceLock struct {
	SchemaVersion  string `json:"schemaVersion"`
	Classification string `json:"classification"`
	Source         struct {
		ForkRepository     string `json:"forkRepository"`
		UpstreamRepository string `json:"upstreamRepository"`
		ForkCommit         string `json:"forkCommit"`
		UpstreamCommit     string `json:"upstreamCommit"`
	} `json:"source"`
	Provenance struct {
		UpstreamCommitURL    string `json:"upstreamCommitURL"`
		Tree                 string `json:"tree"`
		Parent               string `json:"parent"`
		EmbeddedSignature    string `json:"embeddedSignature"`
		VerificationProvider string `json:"verificationProvider"`
	} `json:"provenance"`
	Artifacts struct {
		ControllerImage  *string `json:"controllerImage"`
		RuntimeImage     *string `json:"runtimeImage"`
		ApplicationChart *string `json:"applicationChart"`
		CRDChart         *string `json:"crdChart"`
	} `json:"artifacts"`
	Release struct {
		Owner              *string `json:"owner"`
		ForkTagsRelease    bool    `json:"forkTagsRelease"`
		ProductionEligible bool    `json:"productionEligible"`
	} `json:"release"`
}

func TestSourceLockIsEvaluationOnlyAndImmutable(t *testing.T) {
	path := filepath.Join("..", "..", "locks", "kagent-source.lock.json")
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var lock sourceLock
	if err := json.Unmarshal(contents, &lock); err != nil {
		t.Fatal(err)
	}
	if lock.SchemaVersion != "k8s-agents-platform/kagent-source-lock/v1alpha1" {
		t.Fatalf("unexpected schema version %q", lock.SchemaVersion)
	}
	if lock.Classification != "evaluation-only" {
		t.Fatalf("unexpected classification %q", lock.Classification)
	}
	const (
		forkRepository     = "https://github.com/pilprod/kagent"
		upstreamRepository = "https://github.com/kagent-dev/kagent"
		commit             = "5229184e280a6f3bf205b4e64405a92d1fbc259f"
		tree               = "8feb3dd0296ce5f96cf5b899c8cc92ea4057d5b5"
		parent             = "cb869a0e3532550c5fe5f7bbaf00a6408cb9ab3b"
	)
	if lock.Source.ForkRepository != forkRepository || lock.Source.UpstreamRepository != upstreamRepository {
		t.Fatal("source lock must use the canonical fork and upstream repositories")
	}
	sha := regexp.MustCompile(`^[0-9a-f]{40}$`)
	if !sha.MatchString(lock.Source.ForkCommit) || !sha.MatchString(lock.Source.UpstreamCommit) {
		t.Fatal("source commits must be full immutable Git SHAs")
	}
	if lock.Source.ForkCommit != lock.Source.UpstreamCommit {
		t.Fatal("the initial baseline must contain no untracked fork delta")
	}
	if lock.Source.ForkCommit != commit {
		t.Fatalf("unexpected source commit %q", lock.Source.ForkCommit)
	}
	if lock.Provenance.UpstreamCommitURL != upstreamRepository+"/commit/"+commit ||
		lock.Provenance.Tree != tree || lock.Provenance.Parent != parent ||
		lock.Provenance.EmbeddedSignature != "pgp" || lock.Provenance.VerificationProvider != "github" {
		t.Fatal("source lock is missing the reviewed Git object provenance")
	}
	if lock.Artifacts.ControllerImage != nil || lock.Artifacts.RuntimeImage != nil ||
		lock.Artifacts.ApplicationChart != nil || lock.Artifacts.CRDChart != nil {
		t.Fatal("evaluation lock must not pin or imply release artifacts")
	}
	if lock.Release.Owner != nil || lock.Release.ForkTagsRelease || lock.Release.ProductionEligible {
		t.Fatal("evaluation lock must not imply release ownership or production eligibility")
	}
}
