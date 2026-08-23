# Stock kagent M0 testbed

M0 exists to prove the cluster, controller, UI and A2A baseline before the
fork runtime seam, Agent Substrate and Temporal are introduced.

The release input is the official kagent `v0.9.12` OCI chart pair recorded in
`locks/kagent-testbed.lock.json`. The profile is intentionally single-user and
testbed-only:

- one controller, one UI replica and bundled development PostgreSQL;
- UI exposed only as `ClusterIP` and reached through Cloudflare Access plus the
  existing outbound Tunnel;
- controller limited to `kagent-system` and `kagent-testbed`;
- KMCP, Agent Substrate, the WorkerPool, tools and default agents disabled;
- deterministic Ollama fixture configuration, with no model-provider secret;
- no fork image build, External Agent Host or Temporal dependency.

This profile is never promoted. A later preview candidate replaces it with the
reviewed fork artifacts, an external database, trusted authorization and the
qualified runtime backend.

## Verification

Static product-lock tests:

```bash
go test ./...
```

Exact chart download, checksum and render:

```bash
KAGENT_VERIFY_REMOTE=1 scripts/verify-testbed-charts.sh
```

The candidate tag recorded by the lock is only a reservation. Creating it and
applying the Terraform plans are separate serialized actions and require
action-specific authorization.
