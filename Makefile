SHELL := /bin/bash
.DEFAULT_GOAL := help

CERT_DIR ?= certs
CERT_FILE ?= $(if $(MOBILITHEK_CERT_FILE),$(MOBILITHEK_CERT_FILE),$(CERT_DIR)/client.crt)
KEY_FILE ?= $(if $(MOBILITHEK_KEY_FILE),$(MOBILITHEK_KEY_FILE),$(CERT_DIR)/client.key)
CA_FILE ?= $(MOBILITHEK_CA_FILE)
SUBSCRIPTION_ID ?= $(MOBILITHEK_SUBSCRIPTION_ID)
ENDPOINT ?= auto
OUT_DIR ?= out
SUBSCRIPTION_XML ?= $(OUT_DIR)/subscription.xml
EVENTS_GEOJSON ?= $(OUT_DIR)/events.geojson
SAMPLE_XML ?= examples/data/sample-subscription.xml
PORT ?= 8787

.PHONY: help cert fetch convert sample web all fmt test vet check clean doctor

help:
	@printf '\n'
	@printf 'mobilithek - Go client and demos for Mobilithek subscription data\n'
	@printf '================================================================\n\n'
	@printf 'Typical real-data flow:\n'
	@printf '  make cert\n'
	@printf '  MOBILITHEK_SUBSCRIPTION_ID=123456789012345678 make fetch\n'
	@printf '  make web\n\n'
	@printf 'Fast offline demo without credentials:\n'
	@printf '  make sample web\n\n'
	@printf 'Targets:\n'
	@printf '  make cert      Find a .p12/.pfx below this directory and create %s + %s\n' '$(CERT_FILE)' '$(KEY_FILE)'
	@printf '  make fetch     Fetch subscription XML and write converted GeoJSON to %s\n' '$(EVENTS_GEOJSON)'
	@printf '  make convert   Convert %s to %s\n' '$(SUBSCRIPTION_XML)' '$(EVENTS_GEOJSON)'
	@printf '  make sample    Convert the included synthetic XML fixture to %s\n' '$(EVENTS_GEOJSON)'
	@printf '  make web       Start the MapLibre demo server on http://127.0.0.1:%s/\n' '$(PORT)'
	@printf '  make all       Run cert, fetch, then web\n'
	@printf '  make fmt       Format Go sources\n'
	@printf '  make test      Run Go tests\n'
	@printf '  make vet       Run go vet\n'
	@printf '  make check     Run tests, JS syntax check, and public-folder safety checks\n'
	@printf '  make doctor    Check required local tools\n'
	@printf '  make clean     Remove generated output in %s, but keep certificates\n\n' '$(OUT_DIR)'
	@printf 'Useful variables:\n'
	@printf '  SUBSCRIPTION_ID=...  or MOBILITHEK_SUBSCRIPTION_ID=...\n'
	@printf '  CERT_P12=/path/to/client.p12\n'
	@printf '  FORCE_CERT=1       regenerate PEM files even if they already exist\n'
	@printf '  CERT_FILE=%s KEY_FILE=%s\n' '$(CERT_FILE)' '$(KEY_FILE)'
	@printf '  PORT=%s OUT_DIR=%s ENDPOINT=%s\n\n' '$(PORT)' '$(OUT_DIR)' '$(ENDPOINT)'

doctor:
	@set -euo pipefail; \
	for tool in go openssl find; do \
		if ! command -v "$$tool" >/dev/null 2>&1; then \
			echo "Missing required tool: $$tool"; \
			exit 1; \
		fi; \
	done; \
	if ! command -v node >/dev/null 2>&1; then \
		echo "Optional tool missing: node (needed only for make check JS syntax validation)"; \
	fi; \
	echo "Required tools available."

cert:
	@set -euo pipefail; \
	mkdir -p "$(CERT_DIR)"; \
	if [[ "$${FORCE_CERT:-}" != "1" && -f "$(CERT_FILE)" && -f "$(KEY_FILE)" ]]; then \
		echo "Using existing $(CERT_FILE) and $(KEY_FILE). Set FORCE_CERT=1 to regenerate."; \
		exit 0; \
	fi; \
	p12="$${CERT_P12:-}"; \
	if [[ -z "$$p12" ]]; then \
		p12="$$(find . -type f \( -iname '*.p12' -o -iname '*.pfx' \) -not -path './.git/*' -not -path './$(CERT_DIR)/*' | sort | head -n 1)"; \
	fi; \
	if [[ -z "$$p12" ]]; then \
		echo "No .p12/.pfx file found below this directory."; \
		echo "Get it from Mobilithek after login: Meine Organisation -> Maschinenkonten -> Zertifikat beantragen."; \
		echo "Then place it somewhere below this repository or run: make cert CERT_P12=/path/to/client.p12"; \
		printf "Path to .p12/.pfx file (empty = abort): "; \
		IFS= read -r p12; \
		if [[ -z "$$p12" ]]; then \
			echo "Aborted."; \
			exit 1; \
		fi; \
	fi; \
	if [[ ! -f "$$p12" ]]; then \
		echo "Certificate bundle not found: $$p12"; \
		exit 1; \
	fi; \
	echo "Using PKCS#12 bundle: $$p12"; \
	passin=(); \
	if [[ -n "$${P12_PASSWORD:-}" ]]; then \
		passin=(-passin "pass:$${P12_PASSWORD}"); \
	else \
		printf "PKCS#12 password (hidden, empty = let OpenSSL ask): "; \
		stty -echo; IFS= read -r p12pass; stty echo; printf "\n"; \
		if [[ -n "$$p12pass" ]]; then \
			passin=(-passin "pass:$$p12pass"); \
		fi; \
	fi; \
	openssl pkcs12 -in "$$p12" -clcerts -nokeys -out "$(CERT_FILE)" "$${passin[@]}"; \
	openssl pkcs12 -in "$$p12" -nocerts -nodes -out "$(KEY_FILE)" "$${passin[@]}"; \
	chmod 600 "$(CERT_FILE)" "$(KEY_FILE)"; \
	unset p12pass P12_PASSWORD; \
	echo "Wrote $(CERT_FILE) and $(KEY_FILE)."

fetch:
	@set -euo pipefail; \
	subscription_id="$(SUBSCRIPTION_ID)"; \
	if [[ -z "$$subscription_id" ]]; then \
		echo "Missing subscription ID."; \
		echo "Set it with: MOBILITHEK_SUBSCRIPTION_ID=123456789012345678 make fetch"; \
		echo "The subscription ID is assigned after subscribing to an offer in Mobilithek. It is not the offer ID."; \
		exit 1; \
	fi; \
	mkdir -p "$(OUT_DIR)"; \
	cmd=(go run ./cmd/mobilithek-fetch -subscription-id "$$subscription_id" -endpoint "$(ENDPOINT)" -out "$(SUBSCRIPTION_XML)" -geojson-out "$(EVENTS_GEOJSON)"); \
	if [[ -f "$(CERT_FILE)" && -f "$(KEY_FILE)" ]]; then \
		cmd+=(-cert "$(CERT_FILE)" -key "$(KEY_FILE)"); \
	else \
		echo "No client certificate found at $(CERT_FILE) and $(KEY_FILE)."; \
		echo "Trying without mTLS. If Mobilithek returns 401/403, run: make cert"; \
	fi; \
	if [[ -n "$(CA_FILE)" ]]; then \
		cmd+=(-ca "$(CA_FILE)"); \
	fi; \
	"$${cmd[@]}"; \
	echo "Done. Start the demo with: make web"; \
	echo "Open converted data: http://127.0.0.1:$(PORT)/?data=/converted/events.geojson"

convert:
	@set -euo pipefail; \
	if [[ ! -f "$(SUBSCRIPTION_XML)" ]]; then \
		echo "Missing $(SUBSCRIPTION_XML). Run make fetch first, or set SUBSCRIPTION_XML=/path/to/subscription.xml."; \
		exit 1; \
	fi; \
	mkdir -p "$(OUT_DIR)"; \
	go run ./cmd/mobilithek-geojson -in "$(SUBSCRIPTION_XML)" -out "$(EVENTS_GEOJSON)"; \
	echo "Converted data ready: $(EVENTS_GEOJSON)"

sample:
	@set -euo pipefail; \
	mkdir -p "$(OUT_DIR)"; \
	go run ./cmd/mobilithek-geojson -in "$(SAMPLE_XML)" -out "$(EVENTS_GEOJSON)"; \
	echo "Sample GeoJSON ready: $(EVENTS_GEOJSON)"; \
	echo "Start the demo with: make web"

web:
	@set -euo pipefail; \
	if [[ -f "$(EVENTS_GEOJSON)" ]]; then \
		echo "Converted data URL: http://127.0.0.1:$(PORT)/?data=/converted/events.geojson"; \
	else \
		echo "No $(EVENTS_GEOJSON) found. Starting with built-in sample data."; \
		echo "Create converted data with: make sample   or   make fetch"; \
		echo "Sample URL: http://127.0.0.1:$(PORT)/"; \
	fi; \
	echo "Stop server with Ctrl-C."; \
	PORT="$(PORT)" go run ./examples/maplibre

all: cert fetch web

fmt:
	@find . -name '*.go' -not -path './out/*' -exec gofmt -w {} +

test:
	@go test ./...

vet:
	@go vet ./...

check: test
	@set -euo pipefail; \
	node --check examples/maplibre/app.js; \
	if find . -type f \( -name '*.p12' -o -name '*.pfx' -o -name '*.key' -o -name '*.pem' -o -name '*.crt' -o -name '.env' -o -name '.env.*' -o -name '.DS_Store' \) -print -quit | grep -q .; then \
		echo "Public-folder safety check failed: credential or metadata file found."; \
		find . -type f \( -name '*.p12' -o -name '*.pfx' -o -name '*.key' -o -name '*.pem' -o -name '*.crt' -o -name '.env' -o -name '.env.*' -o -name '.DS_Store' \) -print; \
		exit 1; \
	fi; \
	if rg -n "BEGIN (RSA |EC |OPENSSH |)PRIVATE KEY" .; then \
		echo "Public-folder safety check failed: private key material found."; \
		exit 1; \
	fi; \
	echo "Checks passed."

clean:
	@rm -rf "$(OUT_DIR)"
	@echo "Removed $(OUT_DIR). Certificates in $(CERT_DIR) were not touched."
