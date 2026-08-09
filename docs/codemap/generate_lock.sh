#!/bin/bash
set -euo pipefail

# Script to deterministically compute module fingerprints and generate docs/codemap/codemap.lock

LOCK_FILE="docs/codemap/codemap.lock"
mkdir -p docs/codemap

COMMIT=$(git rev-parse HEAD 2>/dev/null || echo "unknown")
BRANCH=$(git rev-parse --abbrev-ref HEAD 2>/dev/null || echo "unknown")

DIRTY="false"
if ! git diff --quiet HEAD || ! git diff --cached --quiet HEAD; then
    DIRTY="true"
fi

DIRTY_PATHS=$(git status -s | awk '{print $2}' | jq -R -s -c 'split("\n")[:-1]')
if [ -z "$DIRTY_PATHS" ] || [ "$DIRTY_PATHS" == "[]" ]; then
    DIRTY_PATHS="[]"
fi

TIMESTAMP=$(date -u +"%Y-%m-%dT%H:%M:%SZ")

compute_fingerprint() {
    local dir_or_file=$1
    if [ -d "$dir_or_file" ]; then
        find "$dir_or_file" -type f ! -path "*/.git/*" ! -path "*/node_modules/*" ! -path "*/bin/*" ! -path "*/docs/codemap/*" ! -path "*/.env" 2>/dev/null | sort | while read -r file; do
             if git check-ignore -q "$file"; then continue; fi
             echo -n "$file"
             cat "$file"
        done | sha256sum | awk '{print $1}'
    elif [ -f "$dir_or_file" ]; then
         (echo -n "$dir_or_file"; cat "$dir_or_file") | sha256sum | awk '{print $1}'
    else
         echo "null"
    fi
}

ROOT_CONFIG_HASH=$(
    (
        for file in README.MD AGENTS.md .gitignore SimpleMQTTMonitoring.code-workspace .env.example; do
            if [ -f "$file" ]; then
                echo -n "$file"
                cat "$file"
            fi
        done
    ) | sha256sum | awk '{print $1}'
)

DOCS_HASH=$(
    find docs -type f -name "*.md" ! -path "*/codemap/*" 2>/dev/null | sort | while read -r file; do
         echo -n "$file"
         cat "$file"
    done | sha256sum | awk '{print $1}'
)

cat << JSON > "$LOCK_FILE"
{
  "commit": "$COMMIT",
  "branch": "$BRANCH",
  "working_tree_dirty": $DIRTY,
  "dirty_paths": $DIRTY_PATHS,
  "generated_at": "$TIMESTAMP",
  "scope": [
    "sensor/",
    "mosquitto/",
    "docker-compose.yml",
    "Makefile",
    "db/",
    "ui/",
    "api/",
    "consumer/",
    "docs/*.md",
    "README.MD",
    "AGENTS.md",
    ".env.example",
    "SimpleMQTTMonitoring.code-workspace"
  ],
  "excluded": [
    ".git/",
    "ui/node_modules/",
    "docs/codemap/ — generated output, excluded from its own fingerprint",
    "sensor/.env — gitignored local secret/config file, excluded from fingerprint",
    ".env — gitignored local secret/config file, excluded from fingerprint",
    "docs/image/ — binary asset, not included in the docs *.md fingerprint",
    "api/api, api/bin/ — build artifacts, gitignored (api/.gitignore)"
  ],
  "fingerprint_algorithm": "sha256 over sorted file list per module; for each file, hash the UTF-8 relative path concatenated with raw file bytes (sha256sum), then sha256 over the concatenation of those per-file digests — computed against current working-tree file contents, not just the last commit",
  "module_fingerprints": {
    "sensor": "$(compute_fingerprint "sensor")",
    "mosquitto": "$(compute_fingerprint "mosquitto")",
    "docker-compose.yml": "$(compute_fingerprint "docker-compose.yml")",
    "Makefile": "$(compute_fingerprint "Makefile")",
    "db": "$(compute_fingerprint "db")",
    "api": "$(compute_fingerprint "api")",
    "consumer": "$(compute_fingerprint "consumer")",
    "ui": "$(compute_fingerprint "ui")",
    "docs (*.md)": "$DOCS_HASH",
    "root-config": "$ROOT_CONFIG_HASH"
  }
}
JSON

echo "codemap.lock updated at $LOCK_FILE"
