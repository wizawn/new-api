#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
WEB_DIR="$ROOT_DIR/web"
OUT_DIR="${TMPDIR:-/tmp}/newapi-team-automation-smoke"

printf '[1/3] Go service/controller tests...\n'
(
  cd "$ROOT_DIR"
  go test ./service ./controller -run TeamAutomation -count=1
)

printf '[2/3] Frontend bundle smoke check (esbuild)...\n'
rm -rf "$OUT_DIR"
mkdir -p "$OUT_DIR"
(
  cd "$WEB_DIR"
  npx esbuild \
    src/pages/TeamAutomation/index.jsx \
    src/components/settings/TeamAutomationSetting.jsx \
    src/helpers/teamAutomation.js \
    src/helpers/render.jsx \
    src/helpers/lobeIconsRegistry.js \
    src/pages/Home/index.jsx \
    --bundle \
    --format=esm \
    --external:* \
    --outdir="$OUT_DIR"
)

printf '[3/3] Optional reminder...\n'
printf 'Smoke checks passed. Full npm build and remote deployment validation are still separate tasks.\n'
