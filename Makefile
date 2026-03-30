# ─── SGroups DSL — Makefile ─────────────────────────────────────
#
# Этот Makefile — инструмент разработки DSL-генератора.
# generated/ — самодостаточный Go-проект со своим Makefile.
#
# DSL Codegen (sgctl):
#   build            — компилирует sgctl CLI
#   generate         — полная генерация из schema.yaml
#   generate-sql     — только SQL миграции
#   generate-go      — только Go код
#   generate-proto   — только Protobuf
#   generate-agl     — только AGL (kube-apiserver proxy)
#   generate-docker  — только Dockerfiles
#   validate         — валидация DSL без генерации
#   test             — go vet sgctl + validate + regenerate + file count check
#   clean            — удалить generated/ и sgctl
#
# Shortcuts (делегируют в generated/Makefile после generate):
#   docker-build     — generate + docker build всех образов
#   kind-up          — generate + полный цикл в Kind
#   deploy           — generate + kubectl apply
#
# Любой target из generated/Makefile можно вызвать напрямую:
#   make -C generated deploy
#   make -C generated status
#   make -C generated logs-backend
#
# Переключатели:
#   AGL_ENABLED ?= 1  — включить AGL-генерацию (0 = выкл)
#

# ─── Paths ────────────────────────────────────────────────────────

SGCTL         := ./bin/sgctl
SCHEMA        := schema.yaml
TEMPLATES     := ./templates
OUTPUT        := ./generated

# ─── Feature flags ───────────────────────────────────────────────

AGL_ENABLED   ?= 1

.PHONY: build generate generate-sql generate-go generate-proto \
        generate-agl generate-docker validate test vet clean clean-agl \
        docker-build kind-up deploy

# ═══════════════════════════════════════════════════════════════════
#  DSL Codegen
# ═══════════════════════════════════════════════════════════════════

build:
	@mkdir -p bin
	go build -o $(SGCTL) ./cmd/sgctl
	@echo "built $(SGCTL)"

generate: build _generate-agl
	$(SGCTL) generate --schema $(SCHEMA) --output $(OUTPUT) --templates $(TEMPLATES)

_generate-agl: build
ifeq ($(AGL_ENABLED),1)
	$(SGCTL) generate --schema $(SCHEMA) --target agl --output $(OUTPUT) --templates $(TEMPLATES)
endif

generate-agl: build
	$(SGCTL) generate --schema $(SCHEMA) --target agl --output $(OUTPUT) --templates $(TEMPLATES)

generate-sql: build
	$(SGCTL) generate --schema $(SCHEMA) --target sql --output $(OUTPUT) --templates $(TEMPLATES)

generate-go: build
	$(SGCTL) generate --schema $(SCHEMA) --target go --output $(OUTPUT) --templates $(TEMPLATES)

generate-proto: build
	$(SGCTL) generate --schema $(SCHEMA) --target proto --output $(OUTPUT) --templates $(TEMPLATES)

generate-docker: build
	$(SGCTL) generate --schema $(SCHEMA) --target docker --output $(OUTPUT) --templates $(TEMPLATES)

validate: build
	$(SGCTL) validate --schema $(SCHEMA)

clean:
	rm -rf $(OUTPUT)
	rm -f $(SGCTL)
	@echo "cleaned"

clean-agl:
	rm -rf $(OUTPUT)/pkg/apis $(OUTPUT)/pkg/client \
	       $(OUTPUT)/internal/apiserver $(OUTPUT)/internal/registry \
	       $(OUTPUT)/cmd/sgroups-k8s-apiserver $(OUTPUT)/deploy/apiservice_gen.yaml
	@echo "cleaned agl"

# ═══════════════════════════════════════════════════════════════════
#  DSL Tests
# ═══════════════════════════════════════════════════════════════════

vet:
	go vet ./cmd/... ./internal/...

test: vet validate
	@echo "── regenerating ──"
	@$(MAKE) generate
	@echo "── checking generated project ──"
	@test -f $(OUTPUT)/go.mod      || (echo "FAIL: go.mod not found";  exit 1)
	@test -f $(OUTPUT)/Makefile    || (echo "FAIL: Makefile not found"; exit 1)
	@TOTAL=$$(find $(OUTPUT) -type f \
		\( -name '*.go' -o -name '*.sql' -o -name '*.proto' -o -name '*.yaml' -o -name 'Dockerfile' -o -name '*.Dockerfile' \) \
		2>/dev/null | wc -l); \
	 echo "  generated files: $$TOTAL"; \
	 if [ $$TOTAL -lt 50 ]; then echo "FAIL: expected >=50 files, got $$TOTAL"; exit 1; fi
	@echo "all checks passed"

# ═══════════════════════════════════════════════════════════════════
#  Shortcuts  (generate → delegate to generated/Makefile)
# ═══════════════════════════════════════════════════════════════════

docker-build: generate
	$(MAKE) -C $(OUTPUT) docker-build

kind-up: generate
	$(MAKE) -C $(OUTPUT) kind-up

deploy: generate
	$(MAKE) -C $(OUTPUT) deploy
