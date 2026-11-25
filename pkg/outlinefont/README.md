# Bundled typefaces

Regular (outline) fonts for typecloud's `style=1`. All are SIL Open Font
License 1.1, taken from github.com/google/fonts; each licence file sits in
`fonts/` beside its font.

| name | face | licence file |
|---|---|---|
| sans | Lato Regular | fonts/OFL-lato.txt |
| serif | Crimson Text Regular | fonts/OFL-crimsontext.txt |
| slab | Arvo Regular | fonts/OFL-arvo.txt |
| fat | Abril Fatface | fonts/OFL-abrilfatface.txt |
| script | Pacifico | fonts/OFL-pacifico.txt |
| condensed | Bebas Neue | fonts/OFL-bebasneue.txt |
| typewriter | Courier Prime | fonts/OFL-courierprime.txt |
| lobster | Lobster | fonts/OFL-lobster.txt |
| blackletter | UnifrakturCook Bold | fonts/OFL-unifrakturcook.txt |
| comic | Bangers | fonts/OFL-bangers.txt |
| deco | Righteous | fonts/OFL-righteous.txt |

To add a face: put the `.ttf` and its licence in `fonts/`, add a row to
`faces` in `outlinefont.go` and a row here. Characters a face lacks are skipped.
