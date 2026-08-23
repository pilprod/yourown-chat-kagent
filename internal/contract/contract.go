package contract

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
	"unicode"

	a2a "github.com/a2aproject/a2a-go/v2/a2a"
)

// ExecutionIdentity contains only stable identifiers required at the
// Temporal-to-kagent boundary. Authorization is established independently by
// server-side credentials; these values are correlation data, not authority.
type ExecutionIdentity struct {
	TenantID      string
	RequesterID   string
	Namespace     string
	RunID         string
	ActivityID    string
	CorrelationID string
}

func (identity ExecutionIdentity) Validate() error {
	fields := []struct {
		name  string
		value string
	}{
		{name: "tenant ID", value: identity.TenantID},
		{name: "requester ID", value: identity.RequesterID},
		{name: "namespace", value: identity.Namespace},
		{name: "run ID", value: identity.RunID},
		{name: "activity ID", value: identity.ActivityID},
		{name: "correlation ID", value: identity.CorrelationID},
	}
	for _, field := range fields {
		if field.value == "" || strings.TrimSpace(field.value) != field.value {
			return fmt.Errorf("%s must be non-empty and have no surrounding whitespace", field.name)
		}
		if len(field.value) > 256 {
			return fmt.Errorf("%s exceeds 256 bytes", field.name)
		}
		if strings.IndexFunc(field.value, unicode.IsControl) >= 0 {
			return fmt.Errorf("%s contains a control character", field.name)
		}
	}
	return nil
}

// AgentInstanceRequestID is stable across Temporal Activity retries for the
// same authenticated requester, namespace, tenant, and AgentRun.
func AgentInstanceRequestID(identity ExecutionIdentity) (string, error) {
	if err := identity.Validate(); err != nil {
		return "", err
	}
	return stableID("kap-run", identity.TenantID, identity.RequesterID, identity.Namespace, identity.RunID), nil
}

// A2AMessageID is stable for a Temporal Activity retry, while a new Activity
// produces a distinct message in the same AgentRun.
func A2AMessageID(identity ExecutionIdentity) (string, error) {
	if err := identity.Validate(); err != nil {
		return "", err
	}
	return stableID("kap-msg", identity.TenantID, identity.RequesterID, identity.Namespace, identity.RunID, identity.ActivityID), nil
}

func stableID(prefix string, parts ...string) string {
	hash := sha256.New()
	var length [8]byte
	for _, part := range parts {
		binary.BigEndian.PutUint64(length[:], uint64(len(part)))
		_, _ = hash.Write(length[:])
		_, _ = hash.Write([]byte(part))
	}
	return prefix + "-" + hex.EncodeToString(hash.Sum(nil)[:16])
}

type RunPhase string

const (
	PhaseRunning         RunPhase = "running"
	PhaseWaitingInput    RunPhase = "waiting-input"
	PhaseWaitingApproval RunPhase = "waiting-approval"
	PhaseWaitingAuth     RunPhase = "waiting-authentication"
	PhaseCompleted       RunPhase = "completed"
	PhaseFailed          RunPhase = "failed"
	PhaseCanceled        RunPhase = "canceled"
	PhaseRejected        RunPhase = "rejected"
)

type TaskProjection struct {
	Phase      RunPhase
	Terminal   bool
	Successful bool
}

func ProjectTaskState(state a2a.TaskState) (TaskProjection, error) {
	switch state {
	case a2a.TaskStateSubmitted, a2a.TaskStateWorking:
		return TaskProjection{Phase: PhaseRunning}, nil
	case a2a.TaskStateInputRequired:
		return TaskProjection{Phase: PhaseWaitingInput}, nil
	case a2a.TaskStateAuthRequired:
		return TaskProjection{Phase: PhaseWaitingAuth}, nil
	case a2a.TaskStateCompleted:
		return TaskProjection{Phase: PhaseCompleted, Terminal: true, Successful: true}, nil
	case a2a.TaskStateFailed:
		return TaskProjection{Phase: PhaseFailed, Terminal: true}, nil
	case a2a.TaskStateCanceled:
		return TaskProjection{Phase: PhaseCanceled, Terminal: true}, nil
	case a2a.TaskStateRejected:
		return TaskProjection{Phase: PhaseRejected, Terminal: true}, nil
	default:
		return TaskProjection{}, fmt.Errorf("unsupported A2A task state %q", state)
	}
}

// ManagedMCPRoute deliberately contains no endpoint or credential field used
// by a valid route. Non-empty bypass fields are retained only to make unsafe
// configuration fail closed during validation.
type ManagedMCPRoute struct {
	ToolGatewayResource string
	DirectEndpoint      string
	CredentialValue     string
}

var (
	logicalResourceName = regexp.MustCompile(`^[a-z0-9](?:[-a-z0-9]{0,61}[a-z0-9])?$`)
	uuid                = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[1-8][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
)

func (route ManagedMCPRoute) Validate() error {
	if !logicalResourceName.MatchString(route.ToolGatewayResource) {
		return fmt.Errorf("Tool Gateway resource must be a DNS-1123 logical name")
	}
	if route.DirectEndpoint != "" {
		return fmt.Errorf("direct MCP endpoint is prohibited in the managed profile")
	}
	if route.CredentialValue != "" {
		return fmt.Errorf("credential values must not cross the runtime contract")
	}
	return nil
}

const (
	AgentInstanceNamespaceHeader = "x-kagent-agent-instance-namespace"
	AgentInstanceIDHeader        = "x-kagent-agent-instance-id"
)

// AgentInstanceRoute is required in addition to the A2A context ID. The
// pinned public gateway selects the instance from these gRPC metadata values.
type AgentInstanceRoute struct {
	Namespace       string
	AgentInstanceID string
}

func (route AgentInstanceRoute) Headers() (map[string]string, error) {
	if !logicalResourceName.MatchString(route.Namespace) {
		return nil, fmt.Errorf("AgentInstance namespace must be a DNS-1123 name")
	}
	if !uuid.MatchString(route.AgentInstanceID) {
		return nil, fmt.Errorf("AgentInstance ID must be a lowercase UUID")
	}
	return map[string]string{
		AgentInstanceNamespaceHeader: route.Namespace,
		AgentInstanceIDHeader:        route.AgentInstanceID,
	}, nil
}
