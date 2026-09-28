package vision

import (
	"bytes"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// pdfRects pulls the filled squares out of each page of a PDF made by
// TargetsPDF, in millimetres, together with the page size.
func pdfRects(t *testing.T, doc []byte) (pages [][][4]float64, sizes [][2]float64) {
	t.Helper()
	box := regexp.MustCompile(`/MediaBox \[0 0 ([\d.]+) ([\d.]+)\]`)
	for _, m := range box.FindAllSubmatch(doc, -1) {
		w, _ := strconv.ParseFloat(string(m[1]), 64)
		h, _ := strconv.ParseFloat(string(m[2]), 64)
		sizes = append(sizes, [2]float64{w / ptPerMM, h / ptPerMM})
	}
	stream := regexp.MustCompile(`(?s)stream\n(.*?)\nendstream`)
	rect := regexp.MustCompile(`([\d.]+) ([\d.]+) ([\d.]+) ([\d.]+) re f`)
	for _, m := range stream.FindAllSubmatch(doc, -1) {
		var rs [][4]float64
		for _, r := range rect.FindAllSubmatch(m[1], -1) {
			var v [4]float64
			for k := 0; k < 4; k++ {
				v[k], _ = strconv.ParseFloat(string(r[k+1]), 64)
				v[k] /= ptPerMM
			}
			rs = append(rs, v)
		}
		pages = append(pages, rs)
	}
	return pages, sizes
}

// TestPrintedTargetIsReadable: the board as drawn in the PDF — rasterised
// straight from its rectangles — is found by the detector with every corner
// in place and the printed square size between them.
func TestPrintedTargetIsReadable(t *testing.T) {
	tg := Target{Cols: 8, Rows: 5, SquareMM: 25}
	doc, err := TargetsPDF([]PrintItem{{Label: "WHEEL FL", Target: tg}}, Papers["a4"])
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(doc, []byte("%PDF-1.4")) || !bytes.HasSuffix(doc, []byte("%%EOF\n")) {
		t.Fatal("not a PDF")
	}
	pages, sizes := pdfRects(t, doc)
	if len(pages) != 1 {
		t.Fatalf("a 225×150 mm board on A4: %d pages", len(pages))
	}
	if want := (tg.Cols + 1) * (tg.Rows + 1) / 2; len(pages[0]) != want && len(pages[0]) != want+1 {
		t.Errorf("%d dark squares drawn, want about %d", len(pages[0]), want)
	}

	// Rasterise at 4 px/mm, as a scanner would see the sheet.
	const px = 4.0
	w, h := int(sizes[0][0]*px), int(sizes[0][1]*px)
	img := NewGray(w, h)
	for i := range img.Pix {
		img.Pix[i] = 0.92
	}
	for _, r := range pages[0] {
		for y := int(r[1] * px); y < int((r[1]+r[3])*px); y++ {
			for x := int(r[0] * px); x < int((r[0]+r[2])*px); x++ {
				img.Set(x, h-1-y, 0.08) // PDF y runs up, image y down
			}
		}
	}
	det, err := DetectCheckerboard(img.Blur(0.8), DetectOptions{Target: tg})
	if err != nil {
		t.Fatalf("the printed board is not found: %v", err)
	}
	// Neighbouring corners one square apart.
	var sum float64
	for i := 0; i+1 < tg.Cols; i++ {
		sum += det.Corners[i].DistTo(det.Corners[i+1])
	}
	if got := sum / float64(tg.Cols-1) / px; math.Abs(got-tg.SquareMM) > 0.2 {
		t.Errorf("corner spacing %.2f mm, the square is %.1f mm", got, tg.SquareMM)
	}
}

// TestBigBoardTiles: a floor board too big for the paper is split along square
// edges, and the sheets together hold every dark square exactly once, in a
// pattern that continues across the joins.
func TestBigBoardTiles(t *testing.T) {
	tg := Target{Cols: 7, Rows: 6, SquareMM: 100}
	plan, err := Plan(tg, Papers["a4"])
	if err != nil {
		t.Fatal(err)
	}
	doc, err := TargetsPDF([]PrintItem{{Label: "FLOOR FRONT", Target: tg}}, Papers["a4"])
	if err != nil {
		t.Fatal(err)
	}
	pages, _ := pdfRects(t, doc)
	if len(pages) != plan.Pages || plan.Pages < 2 {
		t.Fatalf("plan %d sheets, PDF has %d", plan.Pages, len(pages))
	}
	total := 0
	for _, p := range pages {
		for _, r := range p {
			if math.Abs(r[2]-100) > 0.01 || math.Abs(r[3]-100) > 0.01 {
				t.Fatalf("a square drawn %.2f×%.2f mm", r[2], r[3])
			}
		}
		total += len(p)
	}
	if want := (tg.Cols + 1) * (tg.Rows + 1) / 2; total != want {
		t.Errorf("%d dark squares over all sheets, the board has %d", total, want)
	}
	t.Logf("7×6 corners at 100 mm on A4: %d sheets (%d×%d)", plan.Pages, plan.Across, plan.Down)

	// On a single sheet made to measure, it is one page.
	one, err := Plan(tg, Papers["single"])
	if err != nil || one.Pages != 1 {
		t.Errorf("single sheet: %+v %v", one, err)
	}
}

// TestPDFStructure: the cross-reference table points at every object, so any
// reader opens the file without having to repair it.
func TestPDFStructure(t *testing.T) {
	doc, err := TargetsPDF([]PrintItem{
		{Label: "WHEEL FL (x)", Target: Target{Cols: 8, Rows: 5, SquareMM: 25}},
		{Label: "FLOOR", Target: Target{Cols: 6, Rows: 5, SquareMM: 100}},
	}, Papers["a3"])
	if err != nil {
		t.Fatal(err)
	}
	s := string(doc)
	i := strings.LastIndex(s, "startxref\n")
	var xref int
	fmt.Sscanf(s[i+len("startxref\n"):], "%d", &xref)
	if !strings.HasPrefix(s[xref:], "xref\n") {
		t.Fatal("startxref does not point at the xref table")
	}
	lines := strings.Split(s[xref:], "\n")
	var n int
	fmt.Sscanf(lines[1], "0 %d", &n)
	for k := 1; k < n; k++ {
		var off int
		fmt.Sscanf(lines[2+k], "%d", &off)
		if !strings.HasPrefix(s[off:], fmt.Sprintf("%d 0 obj", k)) {
			t.Errorf("object %d is not where the xref says", k)
		}
	}
	for _, m := range regexp.MustCompile(`(?s)/Length (\d+) >>\nstream\n`).FindAllStringSubmatchIndex(s, -1) {
		l, _ := strconv.Atoi(s[m[2]:m[3]])
		if !strings.HasPrefix(s[m[1]+l:], "\nendstream") {
			t.Error("a stream length is wrong")
		}
	}
	if !strings.Contains(s, `(WHEEL FL \(x\)`) {
		t.Error("parentheses in a label are not escaped")
	}
}

// TestSquareTooBigForPaper is refused with a reason.
func TestSquareTooBigForPaper(t *testing.T) {
	if _, err := Plan(Target{Cols: 4, Rows: 3, SquareMM: 300}, Papers["a4"]); err == nil {
		t.Error("a 300 mm square accepted on A4")
	}
}
