// Package renderers imports every renderer for its side effect of registering
// itself. Binaries blank-import this one package instead of tracking the list.
package renderers

import (
	_ "github.com/cccr/adraw/renderers/gcode"
	_ "github.com/cccr/adraw/renderers/svg"
)
