package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/AristarhUcolov/wheel-alignment/internal/vision"
)

// runCalibrate measures a camera from a folder of photographs of the
// calibration board.
//
// It is a separate command rather than part of the web interface because
// calibration is done once per camera and never again, and because the photos
// are already on disk: making someone upload twenty images through a browser to
// produce a file they then have to save somewhere would be worse in every way.
func runCalibrate(args []string) error {
	fs := flag.NewFlagSet("calibrate", flag.ContinueOnError)
	cols := fs.Int("cols", 9, "внутренних углов мишени по горизонтали")
	rows := fs.Int("rows", 6, "внутренних углов мишени по вертикали")
	square := fs.Float64("square", 30, "размер клетки мишени в миллиметрах")
	out := fs.String("out", "", "файл для записи калибровки")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return errors.New("укажите каталог со снимками мишени: wheelalign calibrate <каталог>")
	}
	dir := fs.Arg(0)

	target := vision.Target{Cols: *cols, Rows: *rows, SquareMM: *square}
	if err := target.Validate(); err != nil {
		return err
	}

	paths, err := imageFiles(dir)
	if err != nil {
		return err
	}
	if len(paths) < 3 {
		return fmt.Errorf("в каталоге %s найдено %d снимков — нужно минимум 3, а лучше 10–20", dir, len(paths))
	}

	fmt.Printf("\n  Калибровка камеры\n")
	fmt.Printf("  Мишень: %dx%d внутренних углов, клетка %.1f мм\n", *cols, *rows, *square)
	fmt.Printf("  Снимков: %d\n\n", len(paths))

	var imgs []*vision.Gray
	var labels []string
	for _, p := range paths {
		img, err := vision.LoadImage(p)
		if err != nil {
			fmt.Printf("  ✗ %-28s %v\n", filepath.Base(p), err)
			continue
		}
		imgs = append(imgs, img)
		labels = append(labels, filepath.Base(p))
	}

	res, err := vision.CalibrateFromImages(target, imgs, labels, vision.CalibrateOptions{})
	if err != nil {
		return err
	}

	fmt.Printf("  Ошибка обратного проецирования: %.3f пикс\n", res.RMSPx)
	fmt.Printf("  fx %.1f   fy %.1f   cx %.1f   cy %.1f\n",
		res.Camera.Fx, res.Camera.Fy, res.Camera.Cx, res.Camera.Cy)
	fmt.Printf("  дисторсия: k1 %.4f  k2 %.4f  p1 %.5f  p2 %.5f  k3 %.4f\n",
		res.Camera.K1, res.Camera.K2, res.Camera.P1, res.Camera.P2, res.Camera.K3)
	fmt.Printf("  покрытие кадра %.0f%%, разброс наклона мишени %.0f°\n\n",
		res.CoverageFraction*100, res.TiltSpreadDeg)

	if len(res.Warnings) > 0 {
		fmt.Println("  На что обратить внимание:")
		for _, w := range res.Warnings {
			fmt.Printf("   ! %s\n", wrap(w, 74, "     "))
		}
		fmt.Println()
	}

	dest := *out
	if dest == "" {
		dest = filepath.Join(dir, "camera.json")
	}
	if err := res.Camera.Save(dest); err != nil {
		return err
	}
	fmt.Printf("  Калибровка записана: %s\n", dest)
	fmt.Printf("  Она понадобится при каждом оптическом замере — не потеряйте файл.\n\n")

	// A calibration good enough to publish a number from is worth saying so
	// about; a poor one deserves to be called poor even though it "worked".
	switch {
	case res.RMSPx < 0.3 && res.TiltSpreadDeg >= 25 && res.CoverageFraction >= 0.5:
		fmt.Println("  Качество: хорошее. Можно измерять.")
	case res.RMSPx < 1.0:
		fmt.Println("  Качество: приемлемое, но см. замечания выше — лучше переснять серию.")
	default:
		fmt.Println("  Качество: плохое. Пользоваться такой калибровкой не стоит.")
	}
	return nil
}

func imageFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("не удалось прочитать каталог: %w", err)
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		switch strings.ToLower(filepath.Ext(e.Name())) {
		case ".png", ".jpg", ".jpeg":
			out = append(out, filepath.Join(dir, e.Name()))
		}
	}
	sort.Strings(out)
	return out, nil
}

// wrap breaks a long warning across lines so it stays readable in a terminal.
func wrap(s string, width int, indent string) string {
	words := strings.Fields(s)
	if len(words) == 0 {
		return s
	}
	var b strings.Builder
	line := 0
	for i, w := range words {
		runes := len([]rune(w))
		if i > 0 && line+1+runes > width {
			b.WriteString("\n" + indent)
			line = 0
		} else if i > 0 {
			b.WriteString(" ")
			line++
		}
		b.WriteString(w)
		line += runes
	}
	return b.String()
}
