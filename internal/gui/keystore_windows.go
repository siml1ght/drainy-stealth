//go:build windows

package gui

import (
	"errors"
	"unsafe"

	"golang.org/x/sys/windows"
)

// The API key is stored in Windows Credential Manager (DPAPI-protected per user),
// never in the plain-text Fyne preferences file.

const (
	credTypeGeneric = 1 // CRED_TYPE_GENERIC
	// CRED_PERSIST_LOCAL_MACHINE: stored by Windows and encrypted with the user's
	// DPAPI key, so only this Windows account can read it.
	credPersistLocal = 2
	errNotFound      = 1168 // ERROR_NOT_FOUND
	credTargetPrefix = "Drainy/"
)

// credential mirrors the Win32 CREDENTIALW struct.
type credential struct {
	Flags              uint32
	Type               uint32
	TargetName         *uint16
	Comment            *uint16
	LastWritten        windows.Filetime
	CredentialBlobSize uint32
	CredentialBlob     *byte
	Persist            uint32
	AttributeCount     uint32
	Attributes         uintptr
	TargetAlias        *uint16
	UserName           *uint16
}

var (
	advapi32        = windows.NewLazySystemDLL("advapi32.dll")
	procCredWriteW  = advapi32.NewProc("CredWriteW")
	procCredReadW   = advapi32.NewProc("CredReadW")
	procCredDeleteW = advapi32.NewProc("CredDeleteW")
	procCredFree    = advapi32.NewProc("CredFree")
)

var errKeyNotFound = errors.New("api key not found")

func credTarget() (*uint16, error) {
	return windows.UTF16PtrFromString(credTargetPrefix + AppID)
}

// saveAPIKey writes the key to Credential Manager. An empty key deletes the entry.
func saveAPIKey(key string) error {
	if key == "" {
		return deleteAPIKey()
	}
	target, err := credTarget()
	if err != nil {
		return err
	}
	blob := []byte(key)
	cred := credential{
		Type:               credTypeGeneric,
		TargetName:         target,
		CredentialBlobSize: uint32(len(blob)),
		CredentialBlob:     &blob[0],
		Persist:            credPersistLocal,
	}
	r, _, callErr := procCredWriteW.Call(uintptr(unsafe.Pointer(&cred)), 0)
	if r == 0 {
		return callErr
	}
	return nil
}

// loadAPIKey reads the key from Credential Manager. Returns errKeyNotFound if none is stored.
func loadAPIKey() (string, error) {
	target, err := credTarget()
	if err != nil {
		return "", err
	}
	var pcred *credential
	r, _, callErr := procCredReadW.Call(
		uintptr(unsafe.Pointer(target)),
		credTypeGeneric,
		0,
		uintptr(unsafe.Pointer(&pcred)),
	)
	if r == 0 {
		if errno, ok := callErr.(windows.Errno); ok && errno == errNotFound {
			return "", errKeyNotFound
		}
		return "", callErr
	}
	defer procCredFree.Call(uintptr(unsafe.Pointer(pcred)))

	blob := unsafe.Slice(pcred.CredentialBlob, pcred.CredentialBlobSize)
	return string(blob), nil
}

// deleteAPIKey removes the stored key. A missing entry is not an error.
func deleteAPIKey() error {
	target, err := credTarget()
	if err != nil {
		return err
	}
	r, _, callErr := procCredDeleteW.Call(
		uintptr(unsafe.Pointer(target)),
		credTypeGeneric,
		0,
	)
	if r == 0 {
		if errno, ok := callErr.(windows.Errno); ok && errno == errNotFound {
			return nil
		}
		return callErr
	}
	return nil
}
