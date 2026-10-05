// Invoices is a desktop app for generating and printing hourly invoices.
//
// The UI is a server-rendered Gin app. Wails hosts it in a native window and
// routes the webview's requests straight to Gin through its custom URL scheme
// (wails:// on macOS and Linux, http://wails.localhost on Windows), so nothing
// listens on a network port.
package main

import (
	"context"
	_ "embed"
	"log"
	"net/http"
	"os/exec"
	"path/filepath"
	goruntime "runtime"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/menu"
	"github.com/wailsapp/wails/v2/pkg/menu/keys"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
	"github.com/wailsapp/wails/v2/pkg/runtime"

	"invoices/internal/desktop"
	"invoices/internal/store"
	"invoices/internal/web"
	"invoices/public"
	"invoices/views"
)

const appName = "Invoices"

//go:embed build/appicon.png
var appIcon []byte

type app struct {
	ctx    context.Context
	store  *store.Store
	dbPath string
}

func main() {
	// Must happen before Wails creates the webview.
	desktop.InstallWebViewHooks()

	a := &app{}
	handler := a.open()

	err := wails.Run(&options.App{
		Title:     appName,
		Width:     1180,
		Height:    860,
		MinWidth:  760,
		MinHeight: 520,
		AssetServer: &assetserver.Options{
			Handler: desktop.FollowRedirects(handler),
		},
		Menu:                     a.menu(),
		OnStartup:                a.startup,
		OnShutdown:               a.shutdown,
		EnableDefaultContextMenu: true,
		// One writer at a time: a second launch focuses the running window
		// instead of opening the database again.
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId:               "invoices.desktop",
			OnSecondInstanceLaunch: a.focus,
		},
		Mac: &mac.Options{
			About: &mac.AboutInfo{
				Title:   appName,
				Message: "Generate and print hourly invoices.",
				Icon:    appIcon,
			},
		},
	})
	if err != nil {
		log.Fatal(err)
	}
}

// open opens the database in the app's data directory and builds the Gin
// app. If that fails, the window shows the error instead.
func (a *app) open() http.Handler {
	dir, err := desktop.DataDir(appName)
	if err != nil {
		log.Printf("Startup error: %v", err)
		return web.Unavailable(err)
	}

	a.dbPath = filepath.Join(dir, "invoices.db")
	a.store, err = store.Open(a.dbPath)
	if err != nil {
		log.Printf("Startup error: %v", err)
		return web.Unavailable(err)
	}
	log.Printf("Database  →  %s", a.dbPath)

	engine, err := web.New(a.store, views.FS, public.FS)
	if err != nil {
		log.Printf("Startup error: %v", err)
		return web.Unavailable(err)
	}
	return engine
}

func (a *app) startup(ctx context.Context) {
	a.ctx = ctx
}

func (a *app) shutdown(context.Context) {
	if a.store != nil {
		a.store.Close()
	}
}

func (a *app) focus(options.SecondInstanceData) {
	if a.ctx == nil {
		return
	}
	runtime.WindowUnminimise(a.ctx)
	runtime.Show(a.ctx)
}

// menu builds the macOS menu bar. On Windows and Linux the webview already
// handles printing and navigation shortcuts, and a menu bar would change the
// window's layout, so there is none.
func (a *app) menu() *menu.Menu {
	if goruntime.GOOS != "darwin" {
		return nil
	}

	m := menu.NewMenu()
	m.Append(menu.AppMenu())

	file := m.AddSubmenu("File")
	file.AddText("Print…", keys.CmdOrCtrl("p"), func(*menu.CallbackData) {
		runtime.WindowExecJS(a.ctx, "window.print()")
	})
	file.AddSeparator()
	file.AddText("Show Database in Finder", nil, func(*menu.CallbackData) {
		if a.dbPath != "" {
			_ = exec.Command("open", "-R", a.dbPath).Start()
		}
	})

	m.Append(menu.EditMenu())

	view := m.AddSubmenu("View")
	view.AddText("Back", keys.CmdOrCtrl("["), func(*menu.CallbackData) {
		runtime.WindowExecJS(a.ctx, "history.back()")
	})
	view.AddText("Forward", keys.CmdOrCtrl("]"), func(*menu.CallbackData) {
		runtime.WindowExecJS(a.ctx, "history.forward()")
	})
	view.AddSeparator()
	view.AddText("Reload", keys.CmdOrCtrl("r"), func(*menu.CallbackData) {
		runtime.WindowReload(a.ctx)
	})

	m.Append(menu.WindowMenu())
	return m
}
