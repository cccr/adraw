package typecloud

import (
	"reflect"
	"testing"
)

func TestSplitChunks(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want []string
	}{
		{"plain", "a;b;c", []string{"a", "b", "c"}},
		{"one chunk", "hello world", []string{"hello world"}},
		{"escaped semicolon", `a\;b`, []string{"a;b"}},
		{"escaped backslash then separator", `a\\;b`, []string{`a\`, "b"}},
		{"only separators", ";;  ;", nil},
		{"empty", "", nil},
		{"whitespace only", " \t ", nil},
		{"trims", " one ; two ", []string{"one", "two"}},
		{"empty chunk dropped", "a;;b", []string{"a", "b"}},
		{"trailing backslash kept", `a\`, []string{`a\`}},
		{"other escape kept verbatim", `a\nb`, []string{`a\nb`}},
		{"non-ascii kept", "héllo;世界", []string{"héllo", "世界"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := splitChunks(c.in)
			if !reflect.DeepEqual(got, c.want) {
				t.Errorf("splitChunks(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}
