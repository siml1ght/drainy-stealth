//go:build !windows

package gui

import "fyne.io/fyne/v2"

// MakeWindowFrameless is a no-op outside Windows: the system frame stays.
func MakeWindowFrameless(_ fyne.Window) {}

// StartNativeDrag is a no-op outside Windows.
func StartNativeDrag(_ fyne.Window) {}

// MinimizeNativeWindow is a no-op outside Windows; use the Fyne window API instead.
func MinimizeNativeWindow(_ fyne.Window) {}
