package contract

import (
	"testing"

	a2a "github.com/a2aproject/a2a-go/v2/a2a"
)

func validIdentity() ExecutionIdentity {
	return ExecutionIdentity{
		TenantID:      "tenant-a",
		RequesterID:   "user-a",
		Namespace:     "agent-testbed",
		RunID:         "run-123",
		ActivityID:    "activity-456",
		CorrelationID: "trace-789",
	}
}

func TestDeterministicBoundaryIdentifiers(t *testing.T) {
	identity := validIdentity()
	requestID, err := AgentInstanceRequestID(identity)
	if err != nil {
		t.Fatal(err)
	}
	messageID, err := A2AMessageID(identity)
	if err != nil {
		t.Fatal(err)
	}

	retryRequestID, _ := AgentInstanceRequestID(identity)
	retryMessageID, _ := A2AMessageID(identity)
	if requestID != retryRequestID || messageID != retryMessageID {
		t.Fatal("a retry must reuse boundary identifiers")
	}

	identity.ActivityID = "activity-457"
	nextRequestID, _ := AgentInstanceRequestID(identity)
	nextMessageID, _ := A2AMessageID(identity)
	if requestID != nextRequestID {
		t.Fatal("activities in one run must reuse the AgentInstance request ID")
	}
	if messageID == nextMessageID {
		t.Fatal("a new activity must receive a distinct A2A message ID")
	}
}

func TestBoundaryIdentifiersAreTenantScoped(t *testing.T) {
	first := validIdentity()
	second := first
	second.TenantID = "tenant-b"

	firstRequestID, _ := AgentInstanceRequestID(first)
	secondRequestID, _ := AgentInstanceRequestID(second)
	firstMessageID, _ := A2AMessageID(first)
	secondMessageID, _ := A2AMessageID(second)
	if firstRequestID == secondRequestID || firstMessageID == secondMessageID {
		t.Fatal("tenant boundaries must change all deterministic identifiers")
	}
}

func TestBoundaryIdentifiersAreRequesterScoped(t *testing.T) {
	first := validIdentity()
	second := first
	second.RequesterID = "user-b"
	firstID, _ := AgentInstanceRequestID(first)
	secondID, _ := AgentInstanceRequestID(second)
	if firstID == secondID {
		t.Fatal("requester boundaries must change the AgentInstance request ID")
	}
}

func TestStableIDUsesUnambiguousTupleEncoding(t *testing.T) {
	first := stableID("test", "a\x00b", "c")
	second := stableID("test", "a", "b\x00c")
	if first == second {
		t.Fatal("distinct tuples must not produce the same stable ID")
	}
}

func TestBoundaryIdentityFailsClosed(t *testing.T) {
	identity := validIdentity()
	identity.RequesterID = ""
	if _, err := AgentInstanceRequestID(identity); err == nil {
		t.Fatal("missing requester identity must be rejected")
	}
}

func TestBoundaryIdentityRejectsControlCharacters(t *testing.T) {
	identity := validIdentity()
	identity.RunID = "run\x00123"
	if _, err := AgentInstanceRequestID(identity); err == nil {
		t.Fatal("control characters must be rejected")
	}
}

func TestTaskStateProjection(t *testing.T) {
	tests := []struct {
		state      a2a.TaskState
		phase      RunPhase
		terminal   bool
		successful bool
	}{
		{a2a.TaskStateSubmitted, PhaseRunning, false, false},
		{a2a.TaskStateWorking, PhaseRunning, false, false},
		{a2a.TaskStateInputRequired, PhaseWaitingInput, false, false},
		{a2a.TaskStateAuthRequired, PhaseWaitingAuth, false, false},
		{a2a.TaskStateCompleted, PhaseCompleted, true, true},
		{a2a.TaskStateFailed, PhaseFailed, true, false},
		{a2a.TaskStateCanceled, PhaseCanceled, true, false},
		{a2a.TaskStateRejected, PhaseRejected, true, false},
	}
	for _, test := range tests {
		projection, err := ProjectTaskState(test.state)
		if err != nil {
			t.Fatalf("ProjectTaskState(%q): %v", test.state, err)
		}
		if projection.Phase != test.phase || projection.Terminal != test.terminal || projection.Successful != test.successful {
			t.Fatalf("ProjectTaskState(%q) = %#v", test.state, projection)
		}
	}
	if _, err := ProjectTaskState(a2a.TaskState("unknown")); err == nil {
		t.Fatal("unknown state must fail closed")
	}
}

func TestManagedMCPRoute(t *testing.T) {
	if err := (ManagedMCPRoute{ToolGatewayResource: "prometheus-query"}).Validate(); err != nil {
		t.Fatalf("valid managed route rejected: %v", err)
	}
	if err := (ManagedMCPRoute{
		ToolGatewayResource: "prometheus-query",
		DirectEndpoint:      "https://external.example/mcp",
	}).Validate(); err == nil {
		t.Fatal("direct MCP endpoint must be rejected")
	}
	if err := (ManagedMCPRoute{
		ToolGatewayResource: "prometheus-query",
		CredentialValue:     "not-allowed",
	}).Validate(); err == nil {
		t.Fatal("credential value must be rejected")
	}
	if err := (ManagedMCPRoute{ToolGatewayResource: "https://external.example/mcp"}).Validate(); err == nil {
		t.Fatal("an endpoint disguised as a logical resource must be rejected")
	}
	if err := (ManagedMCPRoute{ToolGatewayResource: "namespace/prometheus-query"}).Validate(); err == nil {
		t.Fatal("a path disguised as a logical resource must be rejected")
	}
}

func TestAgentInstanceRouteHeaders(t *testing.T) {
	headers, err := (AgentInstanceRoute{
		Namespace:       "agent-testbed",
		AgentInstanceID: "018f8d6a-7b2c-7abc-8def-0123456789ab",
	}).Headers()
	if err != nil {
		t.Fatal(err)
	}
	if headers[AgentInstanceNamespaceHeader] != "agent-testbed" ||
		headers[AgentInstanceIDHeader] != "018f8d6a-7b2c-7abc-8def-0123456789ab" {
		t.Fatalf("unexpected route headers %#v", headers)
	}
	if _, err := (AgentInstanceRoute{Namespace: "agent-testbed", AgentInstanceID: "not-a-uuid"}).Headers(); err == nil {
		t.Fatal("invalid AgentInstance ID must be rejected")
	}
}
