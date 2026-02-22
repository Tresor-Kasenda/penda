# Roadmap complete - Micro-framework Go (inspire de Flask)

## 1) Vision du projet

Construire un micro-framework web en Go, pedagogique mais utilisable en production pour des APIs et des apps web simples.

Objectif final:
- API ergonomique type Flask (`app.Get`, `app.Post`, middleware, templates, config, CLI).
- Architecture modulaire avec packages clairs.
- Qualite logicielle: tests, benchmarks, docs, versioning.

Contraintes:
- Standard library d'abord.
- Dependances externes minimales et justifiees.
- Compatibilite Go LTS recente.

---

## 2) Scope fonctionnel (parite "Flask-like")

Features minimum a atteindre:
- `App` + serveur HTTP.
- Routing avec params de chemin.
- Middleware global et par route.
- Context request/response.
- Parsing request (query, JSON, form, multipart).
- Helpers de response (JSON, text, html, redirect, file).
- Gestion d'erreurs personnalisee.
- Templates HTML (layout + partials).
- Fichiers statiques.
- Config via env/fichier.
- Blueprints/modules.
- CLI developer.
- Test client.

Features "plus" (apres MVP):
- Sessions/cookies signees.
- Validation avancee.
- Observabilite (logs, metrics, tracing).
- Securite (headers, CSRF, rate limit).
- Generation OpenAPI.

---

## 3) Architecture cible

Structure conseillee:

```text
.
├── cmd/
│   └── penda/
│       └── main.go
├── examples/
│   ├── hello/
│   ├── rest-api/
│   └── web-app/
├── framework/
│   ├── app/
│   ├── router/
│   ├── context/
│   ├── request/
│   ├── response/
│   ├── middleware/
│   ├── render/
│   ├── errors/
│   ├── config/
│   ├── blueprint/
│   ├── testing/
│   └── observability/
├── internal/
│   └── cli/
├── docs/
├── go.mod
├── Makefile
└── README.md
```

Design principles:
- Separation nette entre API publique (`framework/...`) et implementation interne.
- API stable et simple.
- "Composition over magic": explicite > implicite.

---

## 4) Roadmap detaillee par phases

## Phase 0 - Foundation & project scaffolding

Objectif:
- Poser une base solide avant la logique framework.

A implementer:
- Init module Go.
- Arborescence du repo.
- Outils dev (`make test`, `make lint`, `make bench`).
- CI basique (tests + lint).
- Convention erreurs/logs/tests.

Definition of Done:
- Le projet compile.
- CI verte.
- README setup local.

---

## Phase 1 - HTTP kernel minimal (MVP-1)

Objectif:
- Faire tourner une app HTTP minimale.

A implementer:
- Type `App`:
  - `New()`
  - `Run(addr string) error`
  - `ServeHTTP(w http.ResponseWriter, r *http.Request)`
- Enregistrement routes:
  - `Handle(method, path string, h Handler)`
  - shorthands `Get`, `Post`, `Put`, `Delete`, `Patch`
- Type `Handler`:
  - Option A: `func(*Context) error`
  - Option B: `func(*Context)`
- Type `Context`:
  - `Request`, `Writer`, `Params`, `Locals`

Definition of Done:
- Endpoint `/health` fonctionnel.
- 404 par defaut.
- Tests unitaires de base `App + Context`.

---

## Phase 2 - Router robuste (MVP-2)

Objectif:
- Supporter le routing type micro-framework moderne.

A implementer:
- Matching par methode HTTP.
- Params chemin: `/users/:id`.
- Wildcard: `/files/*path`.
- Route groups:
  - `Group("/api")`
  - middleware par groupe.
- Gestion 405 Method Not Allowed.

Definition of Done:
- Tests table-driven pour tous les cas de matching.
- Benchmarks de routage.
- Couverture tests router >= 90%.

---

## Phase 3 - Pipeline middleware

Objectif:
- Ajouter cross-cutting concerns sans polluer les handlers.

A implementer:
- Middleware global, groupe, route.
- Chaine d'execution deterministe.
- Arret court-circuit (abort).
- Middleware built-in:
  - Recovery (panic -> 500)
  - Logger
  - Request ID
  - Timeout
  - CORS basique

Definition of Done:
- Ordre d'execution documente et teste.
- Panics captures proprement.
- Middleware examples dans `examples/`.

---

## Phase 4 - Request parsing & binding

Objectif:
- Rendre la lecture des entrees simple et sure.

A implementer:
- `Query(key)`, `Param(key)`, `Header(key)`.
- Parsing body JSON.
- Parsing form `application/x-www-form-urlencoded`.
- Parsing `multipart/form-data` + upload fichiers.
- Binding struct (`BindJSON(&dst)`).
- Validation basique (champs requis).

Definition of Done:
- Erreurs de parsing coherentes (400).
- Limite taille body configurable.
- Tests unitaires + tests d'integration parsing.

---

## Phase 5 - Response helpers

Objectif:
- Standardiser la sortie HTTP.

A implementer:
- `Status(code)`.
- `JSON(code, data)`.
- `Text(code, body)`.
- `HTML(code, html)`.
- `Redirect(code, url)`.
- `File(path)` / `Download(path, filename)`.
- Gestion headers et cookies.

Definition of Done:
- Helpers couvrent 80% des cas applicatifs.
- Tests sur content-type, status, body.

---

## Phase 6 - Error handling centralise

Objectif:
- Uniformiser et personnaliser les erreurs.

A implementer:
- Type d'erreur framework (`HTTPError`).
- Mapping erreur -> status code.
- Handlers personnalises:
  - `OnError(func(*Context, error))`
  - handlers par status (404, 500, ...).
- Reponse JSON ou HTML selon contexte.

Definition of Done:
- Aucune panic non capturee.
- Messages d'erreurs predictibles.
- Documentation "error model".

---

## Phase 7 - Templates et static files

Objectif:
- Support web pages (pas uniquement API).

A implementer:
- Moteur `html/template`.
- Layouts, partials, fonctions templates.
- Auto-reload templates en mode dev (optionnel puis stable).
- Static files:
  - `Static("/assets", "./public")`
  - Cache headers (ETag / Last-Modified).

Definition of Done:
- Exemple web-app complet.
- Tests rendering templates.
- Performance acceptable static serving.

---

## Phase 8 - Config & environment

Objectif:
- Rendre l'app configurable de facon propre.

A implementer:
- Chargement config:
  - defaults
  - env vars
  - fichier (yaml/json/toml)
- Profils: `dev`, `test`, `prod`.
- API: `app.Config()`.

Definition of Done:
- Priorite config claire et documentee.
- Tests precedence (default < file < env).

---

## Phase 9 - Blueprints / modules

Objectif:
- Decouper une app en modules reutilisables.

A implementer:
- Type `Blueprint`:
  - routes
  - middleware
  - static/templates locaux
- `app.Register(bp)`.
- Prefix et namespace.

Definition of Done:
- Exemple multi-modules (`users`, `billing`, `admin`).
- Isolation correcte des routes/assets.

---

## Phase 10 - CLI developer experience

Objectif:
- Accelerer le workflow dev.

A implementer:
- Commandes:
  - `penda new <name>`
  - `penda run`
  - `penda routes`
  - `penda doctor`
- Option hot-reload (integration `air` ou equivalent).
- Templates de projet.

Definition of Done:
- Nouveau projet genere et executable en 1 commande.
- CLI documentee avec `--help`.

---

## Phase 11 - Testing toolkit

Objectif:
- Rendre les apps construites avec le framework facilement testables.

A implementer:
- Test client:
  - `client.Get("/path")`
  - `client.PostJSON("/path", payload)`
- Helpers assertions reponse.
- Fixtures app de test.
- Mocks services externes (patterns et exemples).

Definition of Done:
- Examples de tests API + pages HTML.
- Temps d'execution test raisonnable.

---

## Phase 12 - Observabilite et securite (pre-1.0)

Objectif:
- Ajouter les fondamentaux prod.

A implementer:
- Logging structure.
- Metrics Prometheus.
- Tracing OpenTelemetry (optionnel dans un premier temps).
- Graceful shutdown + health/readiness.
- Middleware securite:
  - security headers
  - rate limiting basique
  - CSRF (si templates/forms)

Definition of Done:
- Guide de hardening.
- Endpoint `/metrics` et `/health`.
- Scenarios de charge basiques passes.

---

## Phase 13 - Documentation et release 1.0

Objectif:
- Stabiliser l'API et publier.

A implementer:
- Docs utilisateur:
  - getting started
  - routing
  - middleware
  - templates
  - testing
  - deploy
- Docs contributeur:
  - architecture
  - conventions
  - process release
- Semantic versioning.
- CHANGELOG.

Definition of Done:
- Tag `v1.0.0`.
- Migration guide (si breaking changes pre-1.0).
- 3 exemples officiels maintenus.

---

## 5) Backlog apres v1.0

Extensions possibles:
- Generation OpenAPI depuis routes + schemas.
- WebSocket/SSE helpers.
- Session store pluggable (memory/redis).
- Cache abstractions.
- Auth helpers (JWT, OAuth2 hooks).
- Plugin ecosystem.

---

## 6) Plan d'apprentissage recommande (12 semaines)

Semaine 1-2:
- Phase 0-1.

Semaine 3-4:
- Phase 2-3.

Semaine 5-6:
- Phase 4-5.

Semaine 7-8:
- Phase 6-7.

Semaine 9:
- Phase 8-9.

Semaine 10:
- Phase 10.

Semaine 11:
- Phase 11.

Semaine 12:
- Phase 12-13 (prep release).

---

## 7) Checklists transverses (a chaque phase)

Qualite:
- Tests unitaires + integration.
- Benchmarks si composant critique (router, middleware).
- Lint sans warning.

API design:
- Noms coherents.
- Zero surprise principle.
- Messages d'erreur utiles.

Documentation:
- README mis a jour.
- Exemple executable correspondant.

---

## 8) Criteres de reussite finaux

Le framework est reussi si:
- Une API REST complete se construit rapidement avec peu de boilerplate.
- Une petite app web serveur-side se construit avec templates + static.
- Les tests sont simples a ecrire.
- L'API est stable, documentee, et performante pour le scope micro-framework.

