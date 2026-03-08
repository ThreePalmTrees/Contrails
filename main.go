package main

import (
	"embed"
	"log"
	"net/http"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/menu"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
)

//go:embed all:frontend/dist
// Style: Prefix Unexported Globals with _ (go-style-guide.md)
var _assets embed.FS

// Version is set at build time via -ldflags "-X main.Version=x.y.z".
// Defaults to "dev" for local development builds.
var Version = "dev"

// PostHog API key, injected at build time via -ldflags "-X main.PostHogAPIKey=phc_xxxx".
// Empty = analytics disabled (dev builds).
var PostHogAPIKey = ""

func main() {
	app := NewApp()

	// Style: Reduce Scope of Variables (go-style-guide.md)
	if err := wails.Run(&options.App{
		Title:    "Contrails",
		Width:    960,
		Height:   640,
		MinWidth: 680,
		MinHeight: 480,
		AssetServer: &assetserver.Options{
			Assets: _assets,
			Middleware: func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					// Quick debug log to trace what preceding requests cause random Vite/HTTP connection crashes.
					// e.g., "Unsolicited response received on idle HTTP channel"
					log.Printf("[Wails-Proxy] %s %s", r.Method, r.URL.Path)
					next.ServeHTTP(w, r)
				})
			},
		},
		Menu:             menu.NewMenuFromItems(menu.AppMenu(), menu.WindowMenu()),
		BackgroundColour: &options.RGBA{R: 15, G: 15, B: 20, A: 1},
		OnStartup:        app.startup,
		OnShutdown:       app.shutdown,
		Mac: &mac.Options{
			TitleBar: &mac.TitleBar{
				TitlebarAppearsTransparent: true,
				HideTitle:                 true,
				HideTitleBar:              false,
				FullSizeContent:           true,
				UseToolbar:                false,
				HideToolbarSeparator:      true,
			},
			WebviewIsTransparent: true,
			WindowIsTranslucent:  true,
			About: &mac.AboutInfo{
				Title:   "Contrails",
				Message: "Chat history preserver for coding agents",
			},
		},
		Bind: []interface{}{
			app,
		},
	}); err != nil {
		// Style: Exit in Main (go-style-guide.md)
		log.Fatal(err)
	}
}
