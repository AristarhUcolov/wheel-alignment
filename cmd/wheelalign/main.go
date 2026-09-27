// Command wheelalign runs the open wheel-alignment stand.
//
// One program, no installer, no internet. On Windows it opens in its own window
// (the WebView2 engine built into Windows 10 and 11); elsewhere, or if that
// engine is missing, it opens the interface in the web browser. The data a
// person enters — their own tolerances, phone calibrations — lives in their
// profile directory and survives updates of the program.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/AristarhUcolov/wheel-alignment/internal/desktop"
	"github.com/AristarhUcolov/wheel-alignment/internal/i18n"
	"github.com/AristarhUcolov/wheel-alignment/internal/phone"
	"github.com/AristarhUcolov/wheel-alignment/internal/server"
	"github.com/AristarhUcolov/wheel-alignment/internal/specs"
)

func usage() string {
	return i18n.T(`Сход-развал — открытый стенд.

  wheelalign                       запустить программу
  wheelalign calibrate <каталог>   откалибровать камеру по снимкам мишени
  wheelalign check-spec <файл>     проверить данные по автомобилю перед отправкой

Ключи запуска:
  -browser  открыть в браузере, а не в отдельном окне
  -addr     адрес интерфейса (по умолчанию 127.0.0.1:8700, при занятости — любой свободный)
  -open     открывать окно или браузер (по умолчанию да; -open=false — только сервер)
  -data     каталог пользовательских данных (по умолчанию — в профиле пользователя)

Ключи калибровки:
  -cols   число внутренних углов мишени по горизонтали (по умолчанию 9)
  -rows   то же по вертикали (по умолчанию 6)
  -square размер клетки в миллиметрах, измеренный штангенциркулем (по умолчанию 30)
  -out    куда записать калибровку (по умолчанию camera.json в каталоге снимков)
`)
}

func main() {
	// Подкоманды и справка печатают сообщения до того, как известен каталог
	// данных из -data, поэтому язык для них — из каталога по умолчанию.
	pickLang(userDataDir(""))
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "calibrate":
			if err := runCalibrate(os.Args[2:]); err != nil {
				fmt.Fprintln(os.Stderr, i18n.T("Ошибка:"), err)
				os.Exit(1)
			}
			return
		case "check-spec":
			if err := runCheckSpec(os.Args[2:]); err != nil {
				fmt.Fprintln(os.Stderr, i18n.T("Ошибка:"), err)
				os.Exit(1)
			}
			return
		}
	}

	addr := flag.String("addr", "127.0.0.1:8700", "адрес интерфейса")
	open := flag.Bool("open", true, "открыть окно или браузер")
	browser := flag.Bool("browser", false, "открыть в браузере, а не в отдельном окне")
	dataDir := flag.String("data", "", "каталог пользовательских данных")
	flag.Usage = func() { fmt.Fprint(os.Stderr, usage()) }
	flag.Parse()

	if err := run(*addr, *open, *browser, *dataDir); err != nil {
		fatal(err)
	}
}

// userDataDir is where the owner's own data lives: %APPDATA%\wheelalign on
// Windows, ~/.config/wheelalign on Linux, ~/Library/Application Support on
// macOS. Falling back to a directory next to the program keeps a copy on a USB
// stick self-contained.
func userDataDir(flagDir string) string {
	if flagDir != "" {
		return flagDir
	}
	if d, err := os.UserConfigDir(); err == nil {
		return filepath.Join(d, "wheelalign")
	}
	if exe, err := os.Executable(); err == nil {
		return filepath.Join(filepath.Dir(exe), "wheelalign-data")
	}
	return "wheelalign-data"
}

// pickLang switches to the person's saved language, or on first start to the
// language of the system, and returns the settings file it was read from.
func pickLang(data string) string {
	settingsPath := filepath.Join(data, "settings.json")
	lang := desktop.LoadSettings(settingsPath).Lang
	if lang == "" {
		lang = desktop.SystemLang()
	}
	i18n.Set(i18n.Lang(lang))
	return settingsPath
}

func run(addr string, open, forceBrowser bool, dataFlag string) error {
	data := userDataDir(dataFlag)
	if err := os.MkdirAll(data, 0o755); err != nil {
		return fmt.Errorf(i18n.T("не удалось создать каталог данных %s: %w"), data, err)
	}
	setupLog(data)

	// The language first, so that every message from here on is in it.
	settingsPath := pickLang(data)

	db, err := specs.LoadWithUser(filepath.Join(data, "vehicles"))
	if err != nil {
		return fmt.Errorf(i18n.T("не удалось загрузить базу автомобилей: %w"), err)
	}
	if msg := server.LoadErrorSummary(db); msg != "" {
		log.Println(msg)
	}

	srv, err := server.New(db)
	if err != nil {
		return err
	}
	defer srv.Close()
	srv.SetSettingsFile(settingsPath)

	// The phone link: a separate HTTPS listener on the local network, off
	// until the person switches it on from the interface.
	link := phone.NewLink(srv.Hub(), data)
	srv.SetPhone(link)
	defer link.Close()

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		// Another copy of the program, or something else, holds the port:
		// any free port will do, the window is told where to look.
		ln, err = net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			return fmt.Errorf(i18n.T("не удалось открыть порт для интерфейса: %w"), err)
		}
	}
	url := "http://" + ln.Addr().String() + "/"

	fmt.Print("\n  ", i18n.T("Сход-развал — открытый стенд"), "\n")
	fmt.Print("  ", i18n.F("Автомобилей в базе: %d", db.Count()), "\n")
	fmt.Print("  ", i18n.F("Интерфейс: %s", url), "\n")
	fmt.Print("  ", i18n.F("Данные пользователя: %s", data), "\n\n")
	log.Printf("старт: %s, данные %s", url, data)

	hs := &http.Server{Handler: server.Guard(srv), ReadHeaderTimeout: 10 * time.Second}
	errc := make(chan error, 1)
	go func() {
		if err := hs.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errc <- err
		}
	}()
	shutdown := func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = hs.Shutdown(ctx)
	}

	if open && !forceBrowser {
		// The window owns the main thread until it is closed; closing it
		// ends the program.
		werr := runWindow(url, data)
		if werr == nil {
			shutdown()
			return nil
		}
		log.Printf("окно недоступно (%v), открываю браузер", werr)
		if !errors.Is(werr, errNoWindow) {
			notify(i18n.F("Окно программы не открылось: %s\n\nИнтерфейс откроется в браузере. "+
				"Если нужно отдельное окно, установите Microsoft Edge WebView2 Runtime с сайта Microsoft.", werr.Error()))
		}
	}
	if open {
		go func() {
			time.Sleep(300 * time.Millisecond)
			_ = desktop.Open(url)
		}()
	}
	fmt.Print("  ", i18n.T("Остановить: Ctrl+C"), "\n\n")

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	select {
	case err := <-errc:
		return err
	case <-stop:
		fmt.Println("\n  " + i18n.T("Останавливаюсь…"))
		shutdown()
		return nil
	}
}

// setupLog writes the log to the data directory as well: a program started
// from a shortcut has no console, and "it did not start" needs a trace.
func setupLog(data string) {
	f, err := os.OpenFile(filepath.Join(data, "wheelalign.log"), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return
	}
	log.SetOutput(io.MultiWriter(f, os.Stderr))
	log.SetFlags(log.LstdFlags)
}

func fatal(err error) {
	log.Println("ошибка:", err)
	fmt.Fprintln(os.Stderr, i18n.T("Ошибка:"), err)
	notify(i18n.F("Ошибка: %s", err.Error()))
	os.Exit(1)
}
