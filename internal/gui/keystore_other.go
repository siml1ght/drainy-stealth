//go:build !windows

package gui

import "errors"

// Outside Windows there is no Credential Manager wired up. The key is not persisted
// to the preferences file; the user enters it per session (or via PIXELDRAIN_API_KEY).

var errKeyNotFound = errors.New("api key not found")

func saveAPIKey(string) error { return nil }

func loadAPIKey() (string, error) { return "", errKeyNotFound }

func deleteAPIKey() error { return nil }
