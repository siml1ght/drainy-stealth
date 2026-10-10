# Changelog

All notable changes to **Go-PD-GUI (DRAINY)** are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project aims to follow [Semantic Versioning](https://semver.org/).

## [Unreleased]

### Added

- Automated GitHub Releases on version tags (`v*`) via fyne-cross + GoReleaser.
- **Dark / Light mode switch** in Settings (persisted via app preferences; default remains dark). (#4)
- **Multi-file upload**: stage multiple files (Add files), review/remove them in a queue, then upload sequentially with overall progress. Copy pastes all result URLs (one per line). (#3)
- **Native multi-select** when adding files on Linux (XDG portal / kdialog / zenity) and Windows (common dialog). Android keeps the single-file Fyne picker.
- Linux multi-select uses **one** backend only (KDE → kdialog/Dolphin; elsewhere portal when available) so portal+kdialog no longer open two pickers on Kubuntu.
- Settings: **Filepicker** choice — *Fyne default* (single file) or *OS Native Multipicker* (multi-select on desktop).

### Changed

- Download links in the README now point to GitHub Release assets instead of committed `fyne-cross/` binaries.
- UI labels that used hardcoded white canvas text now use theme-aware widgets so Light mode stays readable.
- Main screen upload actions are split into **Add files** and **Upload** / **Upload all** (Fyne’s file dialog only returns one file per pick).

### Fixed

- **Android uploads** no longer fail with `open /document/primary%3A…: no such file or directory` when picking photos via the Storage Access Framework. Content URIs are streamed into a temp file before `go-pd` upload. (#5)

## [1.4.1] - 2026-10-10

### Added

- Frameless windows (main, Settings, History) with a custom top bar: settings, history, minimize and close buttons, drag by the bar.
- Drag and drop of files onto the window.
- Command-line arguments: `Drainy.exe <file>` queues the file and starts the upload (for the "Send to" menu).
- Windows installer (Inno Setup, per-user install, no UAC) and `build.bat` that builds both `Drainy.exe` and the installer.
- Application icon is embedded in the exe and shown in the taskbar.
- Upload history as a scrollable list with copy-link buttons.

### Changed

- The API key is stored in Windows Credential Manager instead of the plain-text preferences file. An old key in preferences is migrated automatically and removed.
- Main screen redesign: compact 400x580 window, graphite palette, custom scrollbar, larger drop zone.
- Uploads use a streamed multipart body with a 4 MiB copy buffer; the link is copied to the clipboard automatically after upload.
- Upload client has connect and response-header timeouts. Long transfers are not limited.

### Fixed

- Opening Settings crashed the app (theme applied before the native window existed).
- "Add files" did not open on Windows (invalid multi-select filter string).
- History showed a generic error when no API key was set; it now explains that the account is not connected.
- Uninstall left `Drainy.exe` behind when the app was running.

## [1.3.0] - 2026-07-31

### Changed

- Upgrade UI toolkit to **Fyne v2.8** (from v2.4).
- Raise Go toolchain to **1.25**.
- Update dependencies: `go-pd` v1.6.0, `req/v3` v3.60, and transitive modules.
- Enable Fyne `fyneDo` migration (`FyneApp.toml`) for the new threading model.
- UI updates from background work now go through `fyne.Do` (copy feedback, history highlight, progress bar).

### Fixed

- File open dialog no longer panics on resize under Fyne 2.8 (`Show` before `Resize`).
- File picker no longer freezes during upload; transfer runs on a background goroutine.
- Upload path uses a real local filesystem path for `go-pd` / `SetFile` (required for progress size and reliable uploads).
- API key is loaded from preferences at startup so upload auth works without opening Settings first.
- Clearer upload error messages.

### Notes

- `theme.DarkTheme()` remains in use but is deprecated upstream (removal planned for Fyne v3). Prefer system/default theme later if needed.

## [1.2.0] - 2024-05-22

### Added

- Real progress bar (0–100%) during PixelDrain upload.
- Progress bar preview in the README.

### Changed

- Go toolchain to 1.21.
- Dependency updates.
- Rebuilt Windows, Linux, and Android binaries.

## [1.1.0] - 2023-01-25

### Added

- Show last uploaded file on the main screen.
- Package / dependency updates around the go-pd client.

## [1.0.0] - 2022-06-30

### Added

- First public release of DRAINY.
- Cross-platform PixelDrain upload UI (Windows, Linux, Android).
- Optional API key authentication via settings.
- Copy link button and result hyperlink.
- Fyne-based desktop/mobile packaging via fyne-cross.

[1.3.0]: https://github.com/ManuelReschke/go-pd-gui/releases/tag/v1.3.0
[1.2.0]: https://github.com/ManuelReschke/go-pd-gui/releases/tag/v1.2.0
[1.1.0]: https://github.com/ManuelReschke/go-pd-gui/releases/tag/v1.1.0
[1.0.0]: https://github.com/ManuelReschke/go-pd-gui/releases/tag/v1.0.0
