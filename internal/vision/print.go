package vision

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/AristarhUcolov/wheel-alignment/internal/i18n"
)

// Printing the targets.
//
// A target is only as good as its print: the pose solve takes the square size
// on trust, so a board printed 3 % small reads every distance 3 % wrong, and a
// board with a warped or rescaled edge bends the corners the detector finds.
// So the program draws the boards itself, as vector PDF at exact physical
// size, with a 100 mm ruler on every sheet to check the printer did not scale
// it, and splits a board that does not fit the paper along square edges —
// with crop marks — so the pieces butt together without an overlap to guess.
//
// The PDF is written by hand: a few filled rectangles and lines need no
// library, and the one font used (Helvetica, built into every PDF reader)
// carries only the Latin labels. The instructions in the program's own
// language are on screen, where any language can be shown.

// Paper is a sheet size in millimetres. Zero size means "one sheet exactly the
// size of the board" — for a print shop or a plotter.
type Paper struct {
	Name string
	W, H float64
}

// Papers are the sheet sizes offered.
var Papers = map[string]Paper{
	"a4":     {"A4", 210, 297},
	"a3":     {"A3", 297, 420},
	"letter": {"Letter", 215.9, 279.4},
	"single": {"single", 0, 0},
}

// PrintItem is one board to print.
type PrintItem struct {
	Label  string // e.g. "WHEEL FL" — Latin only, it goes into the PDF
	Target Target
}

const (
	printMargin = 10.0 // mm of paper edge left blank; most printers need 4–6
	printFooter = 16.0 // mm at the bottom for the label and the check ruler
)

// PrintPlan is how a board falls onto sheets.
type PrintPlan struct {
	Pages     int     `json:"pages"`
	Across    int     `json:"across"`
	Down      int     `json:"down"`
	Landscape bool    `json:"landscape"`
	BoardWMM  float64 `json:"board_w_mm"`
	BoardHMM  float64 `json:"board_h_mm"`
	PageWMM   float64 `json:"page_w_mm"`
	PageHMM   float64 `json:"page_h_mm"`
	squaresX  int
	squaresY  int
	perPageX  int
	perPageY  int
}

// ErrSquareTooBig is a square that does not fit on the chosen sheet at all.
var ErrSquareTooBig = i18n.Err("клетка мишени больше листа — выберите лист больше или печать одним листом")

// Plan works out how a board falls onto sheets of the given paper, choosing the
// orientation that needs fewer of them.
func Plan(t Target, paper Paper) (PrintPlan, error) {
	if err := t.Validate(); err != nil {
		return PrintPlan{}, err
	}
	nx, ny := t.Cols+1, t.Rows+1
	s := t.SquareMM
	bw, bh := float64(nx)*s, float64(ny)*s
	if paper.W == 0 {
		return PrintPlan{Pages: 1, Across: 1, Down: 1, BoardWMM: bw, BoardHMM: bh,
			PageWMM: bw + 2*printMargin, PageHMM: bh + 2*printMargin + printFooter,
			squaresX: nx, squaresY: ny, perPageX: nx, perPageY: ny}, nil
	}
	best := PrintPlan{}
	for _, land := range []bool{false, true} {
		pw, ph := paper.W, paper.H
		if land {
			pw, ph = ph, pw
		}
		kx := min(int(math.Floor((pw-2*printMargin)/s+1e-9)), nx)
		ky := min(int(math.Floor((ph-2*printMargin-printFooter)/s+1e-9)), ny)
		if kx < 1 || ky < 1 {
			continue
		}
		across, down := (nx+kx-1)/kx, (ny+ky-1)/ky
		p := PrintPlan{Pages: across * down, Across: across, Down: down, Landscape: land,
			BoardWMM: bw, BoardHMM: bh, PageWMM: pw, PageHMM: ph,
			squaresX: nx, squaresY: ny, perPageX: kx, perPageY: ky}
		if best.Pages == 0 || p.Pages < best.Pages {
			best = p
		}
	}
	if best.Pages == 0 {
		return PrintPlan{}, ErrSquareTooBig
	}
	return best, nil
}

// TargetsPDF draws the boards, each on as many sheets as it needs.
func TargetsPDF(items []PrintItem, paper Paper) ([]byte, error) {
	if len(items) == 0 {
		return nil, errors.New(i18n.T("не выбрано ни одной мишени"))
	}
	var doc pdfDoc
	for _, it := range items {
		plan, err := Plan(it.Target, paper)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", it.Label, err)
		}
		for py := 0; py < plan.Down; py++ {
			for px := 0; px < plan.Across; px++ {
				doc.page(plan.PageWMM, plan.PageHMM, drawSheet(it, plan, px, py))
			}
		}
	}
	return doc.bytes(), nil
}

const ptPerMM = 72 / 25.4

// drawSheet is the content stream of one sheet: its block of squares, crop
// marks on the edges that join another sheet, the label and the ruler.
func drawSheet(it PrintItem, plan PrintPlan, px, py int) string {
	var b strings.Builder
	f := func(v float64) string {
		return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.3f", v*ptPerMM), "0"), ".")
	}
	s := it.Target.SquareMM
	i0, j0 := px*plan.perPageX, py*plan.perPageY
	i1, j1 := min(i0+plan.perPageX, plan.squaresX), min(j0+plan.perPageY, plan.squaresY)
	top := plan.PageHMM - printMargin
	left := printMargin
	if plan.Pages == 1 {
		// A board on one sheet sits in the middle of it.
		left = (plan.PageWMM - float64(i1-i0)*s) / 2
		top = plan.PageHMM - (plan.PageHMM-printFooter-float64(j1-j0)*s)/2
	}

	// Squares: dark where column + row is even, counted over the whole board,
	// so the sheets continue one another's pattern.
	b.WriteString("0 g\n")
	for j := j0; j < j1; j++ {
		for i := i0; i < i1; i++ {
			if (i+j)%2 != 0 {
				continue
			}
			x := left + float64(i-i0)*s
			y := top - float64(j-j0+1)*s
			fmt.Fprintf(&b, "%s %s %s %s re f\n", f(x), f(y), f(s), f(s))
		}
	}

	// Crop marks where this sheet meets another: short lines in the margin,
	// in line with the edge, never over the pattern.
	b.WriteString("0.4 G 0.3 w\n")
	bx0, bx1 := left, left+float64(i1-i0)*s
	by1, by0 := top, top-float64(j1-j0)*s
	mark := func(x0, y0, x1, y1 float64) {
		fmt.Fprintf(&b, "%s %s m %s %s l S\n", f(x0), f(y0), f(x1), f(y1))
	}
	const ml = 6.0
	if i0 > 0 {
		mark(bx0, by1+1, bx0, by1+ml)
		mark(bx0, by0-1, bx0, by0-ml)
	}
	if i1 < plan.squaresX {
		mark(bx1, by1+1, bx1, by1+ml)
		mark(bx1, by0-1, bx1, by0-ml)
	}
	if j0 > 0 {
		mark(bx0-1, by1, bx0-ml, by1)
		mark(bx1+1, by1, bx1+ml, by1)
	}
	if j1 < plan.squaresY {
		mark(bx0-1, by0, bx0-ml, by0)
		mark(bx1+1, by0, bx1+ml, by0)
	}

	// The check ruler: 100 mm with a tick every 10 mm.
	ry := printMargin + 3
	rx := printMargin
	b.WriteString("0 G 0.5 w\n")
	mark(rx, ry, rx+100, ry)
	for k := 0; k <= 10; k++ {
		h := 2.0
		if k%5 == 0 {
			h = 4
		}
		mark(rx+float64(k)*10, ry, rx+float64(k)*10, ry+h)
	}

	t := it.Target
	label := fmt.Sprintf("%s   %dx%d corners (%dx%d squares), square %.1f mm", it.Label, t.Cols, t.Rows, t.Cols+1, t.Rows+1, t.SquareMM)
	if plan.Pages > 1 {
		label += fmt.Sprintf("   sheet row %d of %d, column %d of %d", py+1, plan.Down, px+1, plan.Across)
	}
	fmt.Fprintf(&b, "BT /F1 8 Tf %s %s Td (%s) Tj ET\n", f(rx), f(ry+6), pdfText(label))
	fmt.Fprintf(&b, "BT /F1 7 Tf %s %s Td (%s) Tj ET\n", f(rx+104), f(ry-1),
		pdfText("<- must measure exactly 100 mm. Print at 100% (actual size), no scaling."))
	return b.String()
}

// pdfText escapes a Latin string for a PDF literal.
func pdfText(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r == '(' || r == ')' || r == '\\':
			b.WriteByte('\\')
			b.WriteRune(r)
		case r < 32 || r > 126:
			b.WriteByte('?')
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// pdfDoc is a minimal PDF writer: pages with one content stream each and one
// standard font.
type pdfDoc struct {
	pages []pdfPage
}

type pdfPage struct {
	w, h    float64
	content string
}

func (d *pdfDoc) page(wMM, hMM float64, content string) {
	d.pages = append(d.pages, pdfPage{wMM * ptPerMM, hMM * ptPerMM, content})
}

func (d *pdfDoc) bytes() []byte {
	var buf bytes.Buffer
	var offsets []int
	obj := func(body string) {
		offsets = append(offsets, buf.Len())
		fmt.Fprintf(&buf, "%d 0 obj\n%s\nendobj\n", len(offsets), body)
	}
	buf.WriteString("%PDF-1.4\n%\xe2\xe3\xcf\xd3\n")

	// 1 catalog, 2 pages, 3 font, then two objects per page.
	n := len(d.pages)
	kids := make([]string, n)
	for i := range d.pages {
		kids[i] = fmt.Sprintf("%d 0 R", 4+2*i)
	}
	obj("<< /Type /Catalog /Pages 2 0 R >>")
	obj(fmt.Sprintf("<< /Type /Pages /Count %d /Kids [%s] >>", n, strings.Join(kids, " ")))
	obj("<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>")
	for i, p := range d.pages {
		obj(fmt.Sprintf("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 %.2f %.2f] /Resources << /Font << /F1 3 0 R >> >> /Contents %d 0 R >>",
			p.w, p.h, 5+2*i))
		// The end-of-line before "endstream" is not part of the stream.
		obj(fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(p.content), p.content))
	}
	xref := buf.Len()
	fmt.Fprintf(&buf, "xref\n0 %d\n0000000000 65535 f \n", len(offsets)+1)
	for _, o := range offsets {
		fmt.Fprintf(&buf, "%010d 00000 n \n", o)
	}
	fmt.Fprintf(&buf, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(offsets)+1, xref)
	return buf.Bytes()
}
