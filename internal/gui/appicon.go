package gui

import (
	_ "embed"

	"fyne.io/fyne/v2"
)

// appIconPNG is the application icon (assets/go-pd-gui-app-icon.png), embedded
// at build time. Fyne applies it to every window, so the taskbar uses it too.
//
//go:embed appicon/app-icon.png
var appIconPNG []byte

// appIconResource is the icon passed to fyne.App.SetIcon.
var appIconResource = fyne.NewStaticResource("app-icon.png", appIconPNG)
