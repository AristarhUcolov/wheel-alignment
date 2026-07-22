// Command wheelalign runs the open wheel-alignment stand.
//
// One binary, no installer, no internet: it serves its own interface on
// localhost and reads its vehicle database from inside itself. Start it, open
// the address it prints, and work offline in the garage.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/AristarhUcolov/wheel-alignment/internal/server"
	"github.com/AristarhUcolov/wheel-alignment/internal/specs"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:8700", "адрес, на котором слушать")
	open := flag.Bool("open", true, "открыть браузер при запуске")
	flag.Parse()

	if err := run(*addr, *open); err != nil {
		fmt.Fprintln(os.Stderr, "Ошибка:", err)
		os.Exit(1)
	}
}

func run(addr string, open bool) error {
	db, err := specs.Load()
	if err != nil {
		return fmt.Errorf("не удалось загрузить базу автомобилей: %w", err)
	}
	if msg := server.LoadErrorSummary(db); msg != "" {
		fmt.Fprintln(os.Stderr, msg)
	}

	srv, err := server.New(db)
	if err != nil {
		return err
	}

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("не удалось занять адрес %s: %w", addr, err)
	}
	url := "http://" + ln.Addr().String()

	fmt.Printf("\n  Сход-развал — открытый стенд\n")
	fmt.Printf("  Автомобилей в базе: %d\n", db.Count())
	fmt.Printf("  Откройте в браузере: %s\n", url)
	fmt.Printf("  Остановить: Ctrl+C\n\n")

	hs := &http.Server{
		Handler:           srv,
		ReadHeaderTimeout: 10 * time.Second,
	}

	errc := make(chan error, 1)
	go func() {
		if err := hs.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errc <- err
		}
	}()

	if open {
		go openBrowser(url)
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	select {
	case err := <-errc:
		return err
	case <-stop:
		fmt.Println("\n  Останавливаюсь…")
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return hs.Shutdown(ctx)
	}
}

func openBrowser(url string) {
	time.Sleep(300 * time.Millisecond)
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	_ = cmd.Start()
}
