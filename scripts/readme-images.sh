#!/bin/sh
# Renders the README's example images into docs/images/.
#
# Every image is real output: the CLI renders each patch to SVG, then headless
# Chrome rasterizes it on white paper. PNG rather than the SVG itself, for two
# reasons — the SVG has no background, so its black lines vanish in GitHub's
# dark mode, and its 0.3 mm pen width becomes a hairline once GitHub shrinks
# the image to fit a column.
#
# Tiles use each patch's gallery preview from adraw.json (seed and coarser
# settings tuned to read at small sizes), so the README shows what the
# gallery shows. Re-run after changing a patch or its preview.
#
#   make readme-images
set -eu

OUT=docs/images
CHROME=${CHROME:-"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"}
[ -x "$CHROME" ] || CHROME=$(command -v google-chrome || command -v chromium || true)
[ -n "$CHROME" ] || { echo "readme-images: no Chrome found; set CHROME" >&2; exit 1; }
command -v jq >/dev/null || { echo "readme-images: needs jq" >&2; exit 1; }

TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT
mkdir -p "$OUT"

# render <patch> <png> <width-px> <stroke-mm> <seed> [key=value ...]
render() {
    patch=$1 png=$2 width=$3 stroke=$4 seed=$5
    shift 5
    sets=""
    for kv in "$@"; do sets="$sets --set $kv"; done
    # shellcheck disable=SC2086 # $sets is deliberately split into flags
    ./adraw render --patch "$patch" --seed "$seed" $sets --out "$TMP/$patch.svg" >/dev/null
    sed -i.bak "s/stroke-width=\"[0-9.]*\"/stroke-width=\"$stroke\"/" "$TMP/$patch.svg"

    # A4 landscape: the height follows from the width.
    height=$(( width * 210 / 297 ))
    cat > "$TMP/$patch.html" <<EOF
<!doctype html><html><body style="margin:0;background:#fff">
<img src="$patch.svg" style="display:block;width:${width}px;height:${height}px">
</body></html>
EOF
    "$CHROME" --headless=new --disable-gpu --hide-scrollbars --force-device-scale-factor=1 \
        --window-size="$width,$height" --screenshot="$PWD/$png" \
        "file://$TMP/$patch.html" >/dev/null 2>&1
    echo "readme-images: $png"
}

# The hero: tangle at its real defaults, large.
render tangle "$OUT/tangle.png" 1200 0.35 3

# One tile per patch, in the order adraw.json lists them, then any patch the
# config does not mention, so a new patch is never silently left out.
listed=$(jq -r '.patches | keys_unsorted[]' adraw.json)
all=$(./adraw list | awk '/^Patches:/{p=1;next} /^$/{p=0} p{print $1}')
for patch in $listed $(printf '%s\n' $all | grep -vxF "$listed" || true); do
    seed=$(jq -r --arg p "$patch" '.patches[$p].preview.seed // 1' adraw.json)
    # shellcheck disable=SC2046 # one key=value word per preview value
    render "$patch" "$OUT/patch-$patch.png" 640 0.45 "$seed" \
        $(jq -r --arg p "$patch" '.patches[$p].preview.values // {} | to_entries[] | "\(.key)=\(.value)"' adraw.json)
done
