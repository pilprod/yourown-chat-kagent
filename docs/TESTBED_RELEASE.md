# Stock kagent M0 testbed

M0 exists to prove the cluster, controller, UI and A2A baseline before the
fork runtime seam, Agent Substrate and Temporal are introduced.

The release input is the official kagent `v0.9.12` OCI chart pair recorded in
`locks/kagent-testbed.lock.json`. The profile is intentionally single-user and
testbed-only:

- one controller and one UI replica;
- a dedicated logical database and role in the platform's existing Cloud SQL
  instance; its URI is mounted from Secret Manager through the GKE CSI driver;
- UI exposed only as `ClusterIP` and reached through Cloudflare Access plus the
  existing outbound Tunnel;
- controller limited to `kagent-system` and `kagent-testbed`;
- KMCP, Agent Substrate, the WorkerPool, tools and default agents disabled;
- only the Python agent runtime is qualified; Go agents remain out of scope
  until kagent publishes a separately lockable Go runtime image;
- deterministic Ollama fixture configuration, with no model-provider secret;
- no fork image build, External Agent Host or Temporal dependency.

This profile is never promoted. A later preview candidate replaces it with the
reviewed fork artifacts, trusted authorization and the qualified runtime
backend. The external Cloud SQL database remains independently backed up and
managed by the platform.

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
