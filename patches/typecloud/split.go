package typecloud

import "strings"

// splitChunks cuts the user's text into chunks at each ";". Inside a chunk,
// "\;" is a literal semicolon and "\\" a literal backslash; a backslash before
// anything else is kept as it is. Chunks are trimmed and empty ones dropped, so
// "a;;b" and a trailing ";" cost nothing.
func splitChunks(s string) []string {
	var chunks []string
	var cur strings.Builder
	flush := func() {
		if t := strings.TrimSpace(cur.String()); t != "" {
			chunks = append(chunks, t)
		}
		cur.Reset()
	}
	rs := []rune(s)
	for i := 0; i < len(rs); i++ {
		switch r := rs[i]; {
		case r == '\\' && i+1 < len(rs) && (rs[i+1] == ';' || rs[i+1] == '\\'):
			cur.WriteRune(rs[i+1])
			i++
		case r == ';':
			flush()
		default:
			cur.WriteRune(r)
		}
	}
	flush()
	return chunks
}
