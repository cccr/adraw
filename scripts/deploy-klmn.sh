#!/bin/sh
# Publishes the engine to klmn.xyz/tools/adraw/.
#
# Only the engine goes across: adraw.wasm, the wasm_exec.js that matches the
# Go that built it, the wasm.js shim, and adraw.json. The page around it —
# layout, adraw.js, adraw.css, tests — lives in the klmn.xyz repo and follows
# that site's design system, so it is edited there, not here. It discovers
# patterns and their controls from the engine, so a new or changed patch
# needs nothing more than this.
#
# Netlify builds klmn.xyz from main, so pushing is deploying.
#
#   make deploy-klmn                  build, test, commit, push
#   NO_PUSH=1 make deploy-klmn        stop after the commit
#   KLMN=~/src/klmn.xyz make deploy-klmn
set -eu

KLMN=${KLMN:-../klmn.xyz}
DEST=static/tools/adraw
FILES="adraw.wasm wasm_exec.js wasm.js adraw.json"

fail() { echo "deploy-klmn: $*" >&2; exit 1; }

[ -d "$KLMN/.git" ] || fail "$KLMN is not the klmn.xyz checkout; set KLMN"
[ -f out/web/adraw.wasm ] || fail "out/web/adraw.wasm missing; run via 'make deploy-klmn'"

cd "$KLMN"
[ "$(git rev-parse --abbrev-ref HEAD)" = main ] || fail "klmn.xyz is not on main"
# Only the engine files are committed, so unrelated work in progress is left
# alone — but uncommitted edits to those same files would be overwritten.
for f in $FILES; do
    git diff --quiet HEAD -- "$DEST/$f" 2>/dev/null ||
        fail "$DEST/$f has uncommitted changes in klmn.xyz"
done
git pull --ff-only --quiet
cd - >/dev/null

mkdir -p "$KLMN/$DEST"
for f in $FILES; do cp "out/web/$f" "$KLMN/$DEST/$f"; done

cd "$KLMN"
if [ -z "$(git status --porcelain -- $(for f in $FILES; do printf '%s ' "$DEST/$f"; done))" ]; then
    echo "deploy-klmn: engine unchanged, nothing to deploy"
    exit 0
fi

# The page's own tests, against the engine just copied in. The full suite is
# klmn.xyz's to run; this is the part an engine change can break.
if [ -d test/node_modules ]; then
    (cd test && node --test --test-concurrency=1 adraw.test.js layout.test.js)
else
    fail "klmn.xyz test dependencies missing; run 'npm install' in $KLMN/test"
fi

for f in $FILES; do git add "$DEST/$f"; done
size=$(du -k "$DEST/adraw.wasm" | cut -f1)
git commit --quiet -m "update the Abstract Drawing engine" \
    -m "Built from adraw on $(date +%Y-%m-%d); adraw.wasm is ${size} KB."
echo "deploy-klmn: committed $(git rev-parse --short HEAD)"

if [ "${NO_PUSH:-}" = 1 ]; then
    echo "deploy-klmn: NO_PUSH set, not pushing"
else
    git push --quiet origin main
    echo "deploy-klmn: pushed; Netlify will publish it"
fi
