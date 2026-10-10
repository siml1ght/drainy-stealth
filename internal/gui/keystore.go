package gui

import (
	"strings"

	"fyne.io/fyne/v2"
)

// Thin layer over the platform keystore. The rest of the app calls these helpers
// and never touches the preferences file for the key.

// storedAPIKey returns the saved API key, or "" if none.
// It also migrates a key left in the plain-text preferences by older versions:
// the key moves into the keystore and is removed from preferences.json.
func storedAPIKey(prefs fyne.Preferences) string {
	if legacy := strings.TrimSpace(prefs.String(SettingAPIKey)); legacy != "" {
		if err := saveAPIKey(legacy); err == nil {
			prefs.RemoveValue(SettingAPIKey)
		}
		return legacy
	}

	key, err := loadAPIKey()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(key)
}

// persistAPIKey saves the key to the keystore (empty clears it). The preferences
// file never receives the key.
func persistAPIKey(prefs fyne.Preferences, key string) error {
	prefs.RemoveValue(SettingAPIKey)
	return saveAPIKey(strings.TrimSpace(key))
}
