// Package patches imports every patch for its side effect of registering
// itself. Adding a patch means creating its directory and adding one line
// here — no switch statement, no CLI change, no UI change.
package patches

import (
	_ "github.com/cccr/adraw/patches/cells"
	_ "github.com/cccr/adraw/patches/flow"
	_ "github.com/cccr/adraw/patches/tangle"
	_ "github.com/cccr/adraw/patches/topo"
)
