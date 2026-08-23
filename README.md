# Интеграция kagent с K8s Agents Platform

Этот публичный репозиторий содержит слой интеграции и проверки совместимости
между [`pilprod/kagent`](https://github.com/pilprod/kagent) и K8s Agents
Platform.

Текущий статус: **оценка совместимости плюс изолированный M0 testbed**.
Зафиксированная версия пока не удовлетворяет управляемому профилю: отсутствуют
доверенная авторизация, продолжение HITL через публичный шлюз и обязательный
маршрут MCP через Tool Gateway. M0 использует неизменённые официальные charts,
не публикует собственные образы и остаётся непригодным для production.

## Граница ответственности

| Репозиторий | Отвечает за |
|---|---|
| `pilprod/kagent` | изменения исходного кода, сначала предлагаемые в upstream, и регрессионные тесты |
| `pilprod/yourown-chat-kagent` | точный pin исходников, интеграционный контракт и conformance-проверки |
| `pilprod/yourown-chat-agents` | адаптер между Temporal и публичными AgentInstance/A2A API |
| `pilprod/yourown-chat` | общие платформенные профили и механизмы доставки без параметров, специфичных для kagent |

Теги fork являются только маркерами происхождения исходников и не запускают
релиз. В управляемом профиле среда исполнения агента не подключается напрямую
к произвольным MCP-адресам: доступ к инструментам проходит через Tool Gateway.

## Что уже зафиксировано

- проверенный commit upstream и fork в
  [`locks/kagent-source.lock.json`](locks/kagent-source.lock.json);
- архитектурная граница в [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md);
- первые обязательные сценарии в
  [`docs/CONFORMANCE.md`](docs/CONFORMANCE.md);
- точные типы протокола A2A и локально проверяемые правила идентификаторов,
  маршрутизации и MCP-конфигурации в `internal/contract`;
- обоснование зависимостей в [`docs/DEPENDENCIES.md`](docs/DEPENDENCIES.md).
- отдельный stock testbed lock, values и процесс проверки в
  [`docs/TESTBED_RELEASE.md`](docs/TESTBED_RELEASE.md).

## Локальная проверка

```bash
go test ./...
```

Эта команда не обращается к облаку, Kubernetes, registry или секретам. Она не
заменяет проверку совместимости как «чёрного ящика» в общем testbed.
