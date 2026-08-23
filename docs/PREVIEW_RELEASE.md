# Controller-only delta preview с UI companion: выпуск, проверка и откат

## Назначение и текущий статус

Этот процесс нужен для быстрой проверки новых controller-функций kagent в
общем testbed. Он не является stable/production release. Из exact source commit
собираются controller и неизменённый относительно comparison base UI companion;
ADK и runtime images не собираются.

Текущий candidate:

- source: `https://github.com/pilprod/kagent.git`;
- commit: `c4dd89b8035716ed201fc48fdc1492cb098848a7`;
- comparison base: `dc2d113ba5c61ed91bd8bfe6915722477ab99b60`;
- controller: `linux/amd64`,
  `europe-west3-docker.pkg.dev/yourown-chat/docker/kagent-controller:git-<SHA>`;
- UI companion: `linux/amd64`,
  `europe-west3-docker.pkg.dev/yourown-chat/docker/kagent-ui:git-<SHA>`;
- qualification: `assembly-unqualified`;
- фактических build/deploy действий пока не выполнялось.

Канонический состав находится в `locks/kagent-preview.lock.json`. Source fork
владеет кодом. Этот product repo владеет сборкой, evidence и release assembly.
Terraform владеет trigger, bucket, Cloud Deploy pipeline/target, namespace/RBAC
и readiness gates.

## Предварительные условия

### 1. One-time bootstrap текущих kagent CRD

В live cluster нет API v2 CRD, а старый chart `v0.9.12` несовместим с текущими
`AgentHarness`, `AgentTemplate`, `AgentInstance` и связанными типами. Поэтому
application release никогда не применяет CRD автоматически.

Platform-admin получает deterministic kagent-only bundle локально:

```bash
scripts/render-bootstrap-crds.sh \
  --source /path/to/pilprod-kagent-at-c4dd89b8 \
  --output /secure/review/bootstrap-crds.yaml
```

Render byte-sensitive: script требует exact Helm `v3.19.0`, соответствующий
digest-pinned `tooling.helmImage` из lock, и отклоняет Helm 4 или другую версию.

Ожидаемый SHA-256:

```text
b34b1165e642e5c621443550f8b212957f49ed9df77e36b87832ee7df51fe1f7
```

Bundle содержит ровно 10 `*.kagent.dev` CRD и не содержит KMCP или Substrate
dependencies. После review platform-admin выполняет server-side dry-run,
применяет exact bundle отдельной административной процедурой и ждёт
`Established=True` для всех десяти CRD. Удалять или автоматически откатывать
эти CRD нельзя. Terraform gate `_CRDS_READY=true` меняется только вместе с exact
`_CRD_BUNDLE_SHA256`.

### 2. Внешний Substrate

kagent chart не устанавливает Substrate и не создаёт WorkerPool. До release
platform должен подтвердить:

- beta Kubernetes API включён и node rollout завершён;
- Substrate release ровно `0.0.20` здоров;
- `ate-api` и `atenet-router` доступны внутри cluster;
- WorkerPool `ate-system/kagent-default` существует и готов;
- Substrate CRD установлены их собственным bootstrap-процессом.

Только после этого Terraform передаёт `_SUBSTRATE_READY=true` и
`_SUBSTRATE_VERSION=0.0.20`.

### 3. Testbed dependencies

Namespaces `kagent-system` и `kagent-testbed` и узкие permissions Cloud Deploy
создаёт Terraform. Getter/writer и ate-api env-source Roles/RoleBindings также
принадлежат Terraform: product assembly удаляет upstream `templates/rbac` и
`templates/substrate-ate-api-rbac.yaml`, а render gate запрещает любые RBAC
objects. Preview использует disposable bundled PostgreSQL
`18.3-alpine` по OCI digest; migration tree не отличается от comparison base.
Внешний Secret базы данных не требуется.

Terraform-owned NetworkPolicy разрешает Pod с label
`platform.yourown.chat/verify=kagent-preview` обращаться только к controller
`:8083` и UI `:8080`, иначе обязательный Skaffold verify fail-closed при
default-deny.

Default ModelConfig указывает на deterministic Ollama fixture в
`kagent-testbed`; provider credential не является startup prerequisite.

### 4. Preview UI через Cloudflare Access

Application release создаёт одну UI replica и только Service
`kagent-preview-ui` типа `ClusterIP` на порту `8080`. Канонический origin:

```text
http://kagent-preview-ui.kagent-system.svc.cluster.local:8080
```

Chart не создаёт Ingress, Gateway или Route и не публикует controller/Gateway.
Terraform-owned Cloudflare gate независимо включает Tunnel route, DNS, Access
application и узкие NetworkPolicy. До этого UI остаётся доступен только внутри
default-deny cluster network.

## Release trigger и gates

Единственный trigger — immutable tag product repo:

```text
^preview-[0-9]{8}-[1-9][0-9]*$
```

Например, `preview-20260823-1`. `main`, fork tag, пустой `TAG_NAME` и ручной
build без tag отклоняются. Release ID детерминирован:
`kagent-{tag}-{sourceShortSHA}`.

Terraform trigger передаёт только этот внешний contract:

```text
_PROJECT_ID=yourown-chat
_REGION=europe-west3
_ARTIFACT_REPOSITORY=docker
_DELIVERY_PIPELINE=kagent-preview
_INITIAL_TARGET=kagent-testbed
_PREVIEW_TAG_REGEX=^preview-[0-9]{8}-[1-9][0-9]*$
_PREVIEW_LOCK=locks/kagent-preview.lock.json
_EVIDENCE_BUCKET=yourown-chat-kagent-preview-europe-west3
_CRDS_READY=<false|true>
_CRD_BUNDLE_SHA256=b34b1165e642e5c621443550f8b212957f49ed9df77e36b87832ee7df51fe1f7
_SUBSTRATE_READY=<false|true>
_SUBSTRATE_VERSION=0.0.20
```

Skaffold file/profile: `deploy/skaffold.preview.yaml` / `kagent-testbed`.
Go, Helm, Trivy и BuildKit substitutions имеют digest-pinned defaults внутри
product config и дополнительно сверяются с lock; Terraform не должен их
переопределять. Cloudflare UI readiness не является Cloud Build substitution:
это отдельный platform gate после внутреннего rollout.

`cloudbuild.preview.yaml` последовательно:

1. валидирует tag, exact lock и Terraform substitutions;
2. требует CRD и Substrate readiness до build;
3. получает exact pushed fork commit и запускает controller tests/build;
4. блокирует изменения `go/api`, `helm/kagent-crds`, migrations, `go/adk`,
   controller Dockerfile, Python/docker runtimes и UI source; application
   chart/values сверяет по tree/hash;
5. рендерит application manifest и отдельный CRD evidence bundle;
6. запрещает Role/RoleBinding/ClusterRole/ClusterRoleBinding, CRD, Ingress,
   Gateway/Route, LoadBalancer/NodePort и незакреплённые controller/UI/DB images;
7. собирает controller и exact-source UI companion с BuildKit SBOM/provenance;
8. разрешает оба registry tag в immutable digests и сканирует оба через Trivy;
9. сохраняет lock, rendered manifests, CRD bundle, build metadata, SBOM и scan
   report в dedicated CMEK bucket;
10. блокирует HIGH/CRITICAL;
11. создаёт Cloud Deploy release в pipeline `kagent-preview`, target
    `kagent-testbed`, profile `kagent-testbed`.

Target одноэтапный, `requireApproval=false`: immutable preview tag уже является
явным release intent. Skaffold verify обязателен.

Cloud Build images для Go, Helm, Trivy и BuildKit зафиксированы OCI digests;
Go toolchain также зафиксирован exact version. Остаточный supply-chain blocker:
upstream `go/Dockerfile` и `ui/Dockerfile` используют
`cgr.dev/...:latest`, а `apk add` не привязан к snapshot repository. Поэтому
выпущенные controller/UI digests, provenance и SBOM неизменяемы, но повторная
сборка того же source commit пока не обязана быть bit-for-bit идентичной. Это
не позволяет повысить `assembly-unqualified` до stable qualification; исправление
требует reviewed base digests и package-repository snapshot в source fork.
Google-managed `gcloud`, `git` и `docker` Cloud Build aliases также остаются
platform-managed, а не product-pinned; exact BuildKit driver и его version
preflight записываются в evidence. Stable lane должен заменить эти aliases
reviewed custom-builder digests либо добавить эквивалентную policy attestation.

Evidence сохраняется под:

```text
gs://yourown-chat-kagent-preview-europe-west3/
  evidence/yourown-chat-kagent/preview/<tag>/<build-id>/
```

## Проверка после выпуска

Release считается только развернутым, но ещё не qualified, когда Cloud Deploy
и Skaffold verify завершились успешно. Затем оператор сверяет:

1. release annotations содержат exact product commit, source commit и
   controller digest из evidence;
2. controller Pod `imageID` совпадает с locked Artifact Registry digest;
3. controller Deployment имеет одну ready replica, Service — только
   `ClusterIP`;
4. UI Deployment имеет одну ready replica с locked digest, а Service
   `kagent-preview-ui` остаётся `ClusterIP:8080`; Ingress/Gateway/Route нет;
5. bundled PostgreSQL использует locked digest и готов;
6. в release нет Role/RoleBinding/ClusterRole/ClusterRoleBinding, CRD,
   LoadBalancer или NodePort;
7. external Substrate и WorkerPool по-прежнему готовы;
8. выполняются KAP-C001…KAP-C013 из `docs/CONFORMANCE.md`, включая Temporal →
   public kagent Gateway, reconnect, cancel и negative identity cases.

Одного HTTP `/health` недостаточно для qualification: он является только
Skaffold rollout smoke.

## Откат

Откат использует предыдущий Cloud Deploy Release и immutable controller/UI
image digests;
mutable tag или прямой `helm rollback` не используются.

Перед откатом оператор проверяет, что:

- previous release относится к тому же CRD tree;
- migration tree между candidate и comparison base не менялся (это также
  гарантирует controller-only guard);
- активные/неоднозначные Task остановлены или reconciled, чтобы откат не
  повторил внешний side effect.

После Cloud Deploy rollback повторяются проверки imageID, readiness, Task
history и cancel/reconnect. Bundled PostgreSQL PVC сохраняется. CRD bundle и
Substrate не откатываются этим release: их изменение требует отдельного
совместимого migration plan и нового release lane.

Если rollout или rollback имеет неоднозначный результат, новые AgentRun не
запускаются до сверки Cloud Deploy rollout, controller digest, AgentInstance и
Task state. Ручное удаление controller/DB/CRD не считается откатом.

## Обновление следующего candidate

Один review должен согласованно обновить source/base commit, application и CRD
Git trees, controller tag, `Chart.preview.yaml` appVersion, values hash и CRD
bundle digest. Любое защищённое изменение требует отдельного full release lane,
а не waiver в controller-only guard.
