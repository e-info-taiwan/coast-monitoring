// Package report fills the supplied Reef Check Excel templates without external services.
package report

import (
	"bytes"
	"encoding/xml"
)

func escape(v string) string {
	var b bytes.Buffer
	_ = xml.EscapeText(&b, []byte(v))
	return b.String()
}
func column(n int) string {
	s := ""
	for n > 0 {
		n--
		s = string(rune('A'+n%26)) + s
		n /= 26
	}
	return s
}
