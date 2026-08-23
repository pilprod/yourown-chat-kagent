# Интеграция kagent с K8s Agents Platform

Этот публичный репозиторий содержит слой интеграции и проверки совместимости
между [`pilprod/kagent`](https://github.com/pilprod/kagent) и K8s Agents
Platform.

Текущий статус: **controller-only delta preview assembly с UI companion готов,
но ещё не запущен**.
Preview lock фиксирует pushed commit fork
`c4dd89b8035716ed201fc48fdc1492cb098848a7`. Ни образ, ни Cloud Deploy Release,
ни изменение кластера этим PR не создаются. Кандидат остаётся
`assembly-unqualified`: доверенная авторизация, HITL continuation и обязательный
маршрут MCP через Tool Gateway ещё не доказаны.

## Граница ответственности

| Репозиторий | Отвечает за |
|---|---|
| `pilprod/kagent` | изменения исходного кода, сначала предлагаемые в upstream, и регрессионные тесты |
| `pilprod/yourown-chat-kagent` | точный pin исходников, controller/UI companion builds, SBOM/provenance, security gate, preview assembly и conformance-проверки |
| `pilprod/yourown-chat-agents` | адаптер между Temporal и публичными AgentInstance/A2A API |
| `pilprod/yourown-chat` | Terraform-owned Cloud Build/Deploy rails, namespaces/RBAC, one-time CRD bootstrap approval и готовность внешнего Substrate |

Теги fork являются только маркерами происхождения исходников и не запускают
релиз. Единственный release intent — immutable tag этого репозитория вида
`preview-YYYYMMDD-N`. Ручной build без такого tag завершается fail-closed.
Preview pipeline создаёт одноэтапный release `kagent-preview` →
`kagent-testbed` только после CRD/Substrate readiness gates и блокировки всех
HIGH/CRITICAL находок controller и UI images. UI остаётся внутренним
`ClusterIP`; Cloudflare Access/Tunnel включается отдельным Terraform gate.

## Что уже зафиксировано

- проверенный commit upstream и fork в
  [`locks/kagent-source.lock.json`](locks/kagent-source.lock.json);
- controller-only candidate, chart/CRD trees, runtime dependencies и assembly
  hashes в [`locks/kagent-preview.lock.json`](locks/kagent-preview.lock.json);
- tag-only Cloud Build в [`cloudbuild.preview.yaml`](cloudbuild.preview.yaml) и
  Skaffold profile `kagent-testbed` в
  [`deploy/skaffold.preview.yaml`](deploy/skaffold.preview.yaml);
- отдельный kagent-only CRD bootstrap, который Cloud Deploy не применяет, и
  verify/rollback runbook в
  [`docs/PREVIEW_RELEASE.md`](docs/PREVIEW_RELEASE.md);
- архитектурная граница в [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md);
- первые обязательные сценарии в
  [`docs/CONFORMANCE.md`](docs/CONFORMANCE.md);
- точные типы протокола A2A и локально проверяемые правила идентификаторов,
  маршрутизации и MCP-конфигурации в `internal/contract`;
- обоснование зависимостей в [`docs/DEPENDENCIES.md`](docs/DEPENDENCIES.md).

## Локальная проверка

```bash
go test ./...
scripts/assert-controller-only.sh \
  --source /path/to/pilprod-kagent-at-the-locked-commit
```

Эта команда не обращается к облаку, Kubernetes, registry или секретам. Она не
заменяет проверку совместимости как «чёрного ящика» в общем testbed.
