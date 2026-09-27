package phone

import (
	"fmt"
	"strings"

	"rsc.io/qr"
)

// qrSVG renders text as a QR code in SVG: one path, black modules on white, with
// the four-module quiet zone scanners need. A phone camera opens the link
// straight from the screen — no typing an IP address with a thumb.
func qrSVG(text string) (string, error) {
	c, err := qr.Encode(text, qr.M)
	if err != nil {
		return "", err
	}
	const quiet = 4
	n := c.Size + 2*quiet
	var b strings.Builder
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %d %d" shape-rendering="crispEdges" width="100%%" height="100%%">`, n, n)
	fmt.Fprintf(&b, `<rect width="%d" height="%d" fill="#fff"/><path fill="#000" d="`, n, n)
	for y := 0; y < c.Size; y++ {
		for x := 0; x < c.Size; x++ {
			if !c.Black(x, y) {
				continue
			}
			// Merge runs along the row into one rectangle.
			run := 1
			for x+run < c.Size && c.Black(x+run, y) {
				run++
			}
			fmt.Fprintf(&b, "M%d %dh%dv1h-%dz", x+quiet, y+quiet, run, run)
			x += run - 1
		}
	}
	b.WriteString(`"/></svg>`)
	return b.String(), nil
}
