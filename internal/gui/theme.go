package gui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

const (
	SettingTheme = "setting.theme"
	ThemeDark    = "dark"
	ThemeLight   = "light"
)

// Palette "graphite minimalism": a restrained slate-grey primary, pure white text,
// and a darker neutral for buttons so disabled elements recede.
var (
	graphitePrimary      = color.NRGBA{R: 0x33, G: 0x39, B: 0x45, A: 0xFF} // #333945
	graphitePrimaryHover = color.NRGBA{R: 0x3F, G: 0x46, B: 0x54, A: 0xFF}
	graphitePrimaryDown  = color.NRGBA{R: 0x28, G: 0x2D, B: 0x37, A: 0xFF}

	buttonNeutral         = color.NRGBA{R: 0x1E, G: 0x22, B: 0x2B, A: 0xFF} // #1E222B
	buttonNeutralHover    = color.NRGBA{R: 0x27, G: 0x2C, B: 0x36, A: 0xFF}
	buttonDisabledNeutral = color.NRGBA{R: 0x1E, G: 0x22, B: 0x2B, A: 0xFF}

	foregroundWhite = color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF}
)

// drainyTheme wraps a base Fyne theme with the amber accent and larger corner radii.
type drainyTheme struct {
	fyne.Theme
	variant fyne.ThemeVariant
}

func newDrainyTheme(variant fyne.ThemeVariant) fyne.Theme {
	base := theme.DarkTheme()
	if variant == theme.VariantLight {
		base = theme.LightTheme()
	}
	return &drainyTheme{Theme: base, variant: variant}
}

func (t *drainyTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	switch name {
	// Primary: High-importance buttons (Upload all, Save) and focus/links.
	case theme.ColorNamePrimary, theme.ColorNameFocus, theme.ColorNameHyperlink:
		return graphitePrimary
	case theme.ColorNameForegroundOnPrimary:
		return foregroundWhite
	case theme.ColorNameForeground:
		// Global text colour: white only in the dark theme, so light theme text stays readable.
		if variant == theme.VariantLight {
			return t.Theme.Color(name, variant)
		}
		return foregroundWhite

	// Regular buttons (Add files, Show History, Copy).
	case theme.ColorNameButton:
		return buttonNeutral
	case theme.ColorNameDisabledButton:
		return buttonDisabledNeutral
	case theme.ColorNameHover:
		if variant == theme.VariantLight {
			return buttonNeutralHover
		}
		return graphitePrimaryHover
	case theme.ColorNamePressed:
		return graphitePrimaryDown

	// Scrollbar: translucent grey so the thumb reads as a subtle overlay.
	case theme.ColorNameScrollBar:
		return color.NRGBA{R: 0x9A, G: 0xA3, B: 0xB2, A: 0x66} // ~40% opacity
	}
	return t.Theme.Color(name, variant)
}

func (t *drainyTheme) Size(name fyne.ThemeSizeName) float32 {
	switch name {
	case theme.SizeNameInputRadius, theme.SizeNameSelectionRadius:
		return 10
	case theme.SizeNameInputBorder:
		return 1

	// Thin scrollbar: the same width when idle and on hover, so it does not
	// swell under the cursor.
	case theme.SizeNameScrollBar:
		return 4
	case theme.SizeNameScrollBarSmall:
		return 3
	case theme.SizeNameScrollBarRadius:
		return 2
	}
	return t.Theme.Size(name)
}

// applyTheme sets the Fyne theme and persists the choice.
// Uses DarkTheme/LightTheme intentionally so the user choice overrides the OS preference.
func applyTheme(name string) {
	if MyApp.App == nil {
		return
	}

	switch name {
	case ThemeLight:
		MyApp.App.Settings().SetTheme(newDrainyTheme(theme.VariantLight))
	default:
		name = ThemeDark
		MyApp.App.Settings().SetTheme(newDrainyTheme(theme.VariantDark))
	}

	MyApp.App.Preferences().SetString(SettingTheme, name)
}

// loadAndApplyTheme applies the stored theme preference (default: dark).
func loadAndApplyTheme() {
	name := MyApp.App.Preferences().StringWithFallback(SettingTheme, ThemeDark)
	applyTheme(name)
}

// currentThemeName returns the stored theme preference.
func currentThemeName() string {
	if MyApp.App == nil {
		return ThemeDark
	}
	return MyApp.App.Preferences().StringWithFallback(SettingTheme, ThemeDark)
}

// themeForeground returns the current theme's foreground color for canvas.Text usage.
func themeForeground() color.Color {
	return theme.ForegroundColor()
}
