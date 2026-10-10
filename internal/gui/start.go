package gui

import (
	"errors"
	"fmt"
	"image/color"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/ManuelReschke/go-pd-gui/internal/filepick"
	"github.com/ManuelReschke/go-pd-gui/internal/fileprep"
	"github.com/ManuelReschke/go-pd-gui/internal/uploadutil"
)

const (
	Version      = "v1.4.1"
	AppID        = "com.gopdgui.app"
	WindowTitle  = "Go-PD-GUI - DRAINY - PixelDrain Upload Tool"
	WindowWidth  = 400
	WindowHeight = 580

	ButtonCopy      = "Copy"
	ButtonUpload    = "Upload"
	ButtonAddFiles  = "Add files"
	ButtonUploadAll = "Upload all"
	EmptyString     = ""
	FooterText      = "This tool was made by Manuel Reschke under MIT Licence. "
)

// pendingFile is a local file ready for PixelDrain upload (path may be a temp copy).
type pendingFile struct {
	localPath string
	fileName  string
	cleanup   func()
}

type Settings struct {
	APIKey       binding.String
	Username     binding.String
	Subscription binding.String
}

type UploadHistory struct {
	UploadDate time.Time
	FileName   string
	URL        string
}

type Storage struct {
	Username      string
	LastElement   UploadHistory
	UploadHistory []UploadHistory
}

type AppData struct {
	App        fyne.App
	Window     fyne.Window
	Containers map[string]*fyne.Container
	Settings   Settings
	Storage    Storage
}

var MyApp AppData

func BuildStart() {
	MyApp.App = app.NewWithID(AppID)
	// App icon: Fyne applies it to every window, so the taskbar shows the same
	// artwork as the .exe (the exe icon comes from rsrc.syso / main.rc).
	MyApp.App.SetIcon(appIconResource)
	loadAndApplyTheme()

	MyApp.Window = MyApp.App.NewWindow(WindowTitle)
	MyApp.Window.Resize(fyne.NewSize(WindowWidth, WindowHeight))

	// BINDINGS
	MyApp.Settings.APIKey = binding.NewString()
	MyApp.Settings.Username = binding.NewString()
	MyApp.Settings.Subscription = binding.NewString()
	// Seed from preferences so upload works without opening Settings first.
	_ = MyApp.Settings.APIKey.Set(storedAPIKey(MyApp.App.Preferences()))

	// binding result link
	linkBound := binding.NewString()

	copySuccessWidget := widget.NewIcon(theme.ConfirmIcon())
	containerCopySuccess := container.New(layout.NewCenterLayout(), copySuccessWidget)
	containerCopySuccess.Hide()

	// RESULT CARD: link, copy and open buttons, and a status line that confirms
	// the link is already in the clipboard after an upload.
	var copyLinkButton *widget.Button
	copyClickID := 0 // защита от наложения таймеров при быстрых нажатиях
	var currentResultURL string

	resultTitle := widget.NewLabelWithStyle("Ссылка готова", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	resultStatus := widget.NewLabel("")
	resultStatus.Importance = widget.SuccessImportance
	resultWidget := widget.NewHyperlink(EmptyString, nil)
	resultWidget.Wrapping = fyne.TextWrapWord

	showCopyFeedback := func(text string) {
		copyClickID++
		myID := copyClickID
		resultStatus.SetText(text)
		copyLinkButton.SetText("Скопировано")
		go func() {
			time.Sleep(2 * time.Second)
			fyne.Do(func() {
				if myID != copyClickID {
					return
				}
				copyLinkButton.SetText(ButtonCopy)
				resultStatus.SetText("Ссылка в буфере обмена")
			})
		}()
	}

	copyLinkButton = widget.NewButtonWithIcon(ButtonCopy, theme.ContentCopyIcon(), func() {
		content, err := linkBound.Get()
		if err != nil || content == "" {
			return
		}
		MyApp.Window.Clipboard().SetContent(content)
		showCopyFeedback("Скопировано в буфер обмена")
	})

	openLinkButton := widget.NewButtonWithIcon("Открыть", theme.ComputerIcon(), func() {
		if currentResultURL == "" {
			return
		}
		if u, err := url.Parse(currentResultURL); err == nil {
			_ = MyApp.App.OpenURL(u)
		}
	})

	resultButtons := container.NewGridWithColumns(2, copyLinkButton, openLinkButton)
	resultCard := widget.NewCard(
		"",
		"",
		container.NewVBox(resultTitle, resultWidget, resultStatus, resultButtons),
	)
	resultContainer := container.NewVBox(resultCard)
	resultContainer.Hide()

	// PROGRESSBAR
	progressBar := widget.NewProgressBar()
	containerProgressBar := container.New(layout.NewVBoxLayout(), layout.NewSpacer(), progressBar, layout.NewSpacer())
	containerProgressBar.Hide() // hide per default

	// LAST ELEMENT
	lastElementContainer := container.New(layout.NewHBoxLayout())
	lastElementContainer.Hide()

	// MULTI-FILE QUEUE
	// Fyne's FileOpen dialog only returns one file per pick, so we accumulate
	// selections in a queue and upload them sequentially on "Upload all".
	var pending []pendingFile
	queueStatus := widget.NewLabel("")
	queueStatus.Hide()
	queueBox := container.NewVBox()
	queueScroll := container.NewVScroll(queueBox)
	queueScroll.SetMinSize(fyne.NewSize(0, 100))
	queueContainer := container.NewVBox(queueStatus, queueScroll)
	queueContainer.Hide()

	// Empty-state hint: shown while the queue is empty and nothing is uploading.
	emptyHint := widget.NewLabelWithStyle(
		"Перетащите файлы сюда\nили нажмите Add files",
		fyne.TextAlignCenter,
		fyne.TextStyle{Italic: true},
	)
	emptyHint.Importance = widget.LowImportance
	emptyHintBox := container.NewCenter(emptyHint)
	emptyHintBox.Show()

	// Upload status (e.g. "Uploading 2/5: photo.png")
	uploadStatus := widget.NewLabel("")
	uploadStatus.Alignment = fyne.TextAlignCenter
	uploadStatus.Hide()

	var addFilesButton, uploadAllButton *widget.Button

	// Declare first so the remove-button callback can call refreshQueue recursively
	// (:= would leave the name undefined inside the function body).
	var refreshQueue func()
	refreshQueue = func() {
		queueBox.Objects = nil
		for i := range pending {
			idx := i
			f := pending[idx]
			nameLabel := widget.NewLabel(f.fileName)
			nameLabel.Truncation = fyne.TextTruncateEllipsis
			removeBtn := widget.NewButtonWithIcon("", theme.DeleteIcon(), func() {
				if idx < 0 || idx >= len(pending) {
					return
				}
				if pending[idx].cleanup != nil {
					pending[idx].cleanup()
				}
				pending = append(pending[:idx], pending[idx+1:]...)
				refreshQueue()
			})
			row := container.NewBorder(nil, nil, nil, removeBtn, nameLabel)
			queueBox.Add(row)
		}
		queueBox.Refresh()

		n := len(pending)
		if n == 0 {
			queueContainer.Hide()
			queueStatus.Hide()
			emptyHintBox.Show()
			uploadAllButton.Disable()
			uploadAllButton.SetText(ButtonUploadAll)
			return
		}
		emptyHintBox.Hide()
		queueStatus.SetText(fmt.Sprintf("%d file(s) selected", n))
		queueStatus.Show()
		queueContainer.Show()
		uploadAllButton.Enable()
		if n == 1 {
			uploadAllButton.SetText(ButtonUpload)
		} else {
			uploadAllButton.SetText(fmt.Sprintf("%s (%d)", ButtonUploadAll, n))
		}
	}

	appendPending := func(filePath, fileName string, cleanup func()) {
		if cleanup == nil {
			cleanup = func() {}
		}
		pending = append(pending, pendingFile{
			localPath: filePath,
			fileName:  fileName,
			cleanup:   cleanup,
		})
		refreshQueue()
	}

	// Fyne single-file picker (Android and any platform without a native multi-picker).
	openFyneSingleFilePicker := func() {
		fileOpen := dialog.NewFileOpen(func(reader fyne.URIReadCloser, err error) {
			if err != nil {
				dialog.ShowError(err, MyApp.Window)
				return
			}
			if reader == nil {
				return // user cancelled
			}

			// Materialize off the UI thread (Android SAF may copy into a temp file).
			go func() {
				filePath, fileName, cleanup, matErr := materializeForUpload(reader)
				if matErr != nil {
					fyne.Do(func() {
						dialog.ShowError(fmt.Errorf("cannot prepare file for upload: %w", matErr), MyApp.Window)
					})
					return
				}

				fyne.Do(func() {
					appendPending(filePath, fileName, cleanup)
				})
			}()
		}, MyApp.Window)
		// In Fyne v2.8+, Resize before Show panics: MinSize() needs a built dialog.
		// Show creates the dialog; Resize afterward applies the preferred size.
		fileOpen.Show()
		fileOpen.Resize(fyne.NewSize(600, 600))
	}

	// Open according to Settings → Filepicker (Fyne default | OS Native Multipicker).
	// Native falls back to Fyne when unsupported (e.g. Android).
	openFilePicker := func() {
		if currentFilePickerMode() == FilePickerFyne {
			openFyneSingleFilePicker()
			return
		}

		go func() {
			paths, err := filepick.OpenMultiple("Select files to upload")
			if errors.Is(err, filepick.ErrUnsupported) {
				fyne.Do(openFyneSingleFilePicker)
				return
			}
			if err != nil {
				fyne.Do(func() {
					dialog.ShowError(fmt.Errorf("file picker failed: %w", err), MyApp.Window)
				})
				return
			}
			if len(paths) == 0 {
				return // cancelled
			}

			// Validate paths and stage them on the UI thread.
			type staged struct {
				path, name string
			}
			var okFiles []staged
			var failMsgs []string
			for _, p := range paths {
				local, readable := fileprep.ReadableLocalFile(p)
				if !readable {
					failMsgs = append(failMsgs, fmt.Sprintf("%s: not a readable file", filepath.Base(p)))
					continue
				}
				okFiles = append(okFiles, staged{
					path: local,
					name: fileprep.SanitizeFileName(filepath.Base(local)),
				})
			}

			fyne.Do(func() {
				for _, f := range okFiles {
					appendPending(f.path, f.name, nil)
				}
				if len(failMsgs) > 0 {
					dialog.ShowError(
						fmt.Errorf("%d of %d path(s) could not be added:\n%s",
							len(failMsgs), len(paths), strings.Join(failMsgs, "\n")),
						MyApp.Window,
					)
				}
			})
		}()
	}

	addFilesButton = widget.NewButtonWithIcon(ButtonAddFiles, theme.ContentAddIcon(), func() {
		openFilePicker()
	})

	// Upload must run off the UI thread so the UI stays responsive and the
	// progress bar can update via fyne.Do.
	uploadAllButton = widget.NewButtonWithIcon(ButtonUploadAll, theme.UploadIcon(), func() {
		if len(pending) == 0 {
			return
		}

		key, _ := MyApp.Settings.APIKey.Get()
		if key == "" {
			// Binding may be empty until Settings is opened; fall back to the keystore.
			key = storedAPIKey(MyApp.App.Preferences())
		}
		if key == "" {
			// Last resort: environment variable (never hardcode keys in source).
			key = strings.TrimSpace(os.Getenv(EnvAPIKey))
		}

		// Take ownership of the queue so the user can stage new files afterwards.
		batch := pending
		pending = nil
		refreshQueue()

		progressBar.SetValue(0)
		containerProgressBar.Show()
		uploadStatus.SetText(fmt.Sprintf("Uploading 0/%d…", len(batch)))
		uploadStatus.Show()
		addFilesButton.Disable()
		uploadAllButton.Disable()

		go func() {
			defer fyne.Do(func() {
				addFilesButton.Enable()
				containerProgressBar.Hide()
				uploadStatus.Hide()
				// Re-apply queue button state (pending may still be empty).
				refreshQueue()
			})

			var (
				successes []UploadHistory
				failMsgs  []string
			)
			total := len(batch)

			for i, file := range batch {
				idx := i
				fyne.Do(func() {
					uploadStatus.SetText(fmt.Sprintf("Uploading %d/%d: %s", idx+1, total, file.fileName))
					// Per-file progress mapped into the overall bar.
					progressBar.SetValue(float64(idx) / float64(total))
				})

				// Wrap the shared progress bar so each file contributes 1/total.
				pixelDrainURL, err := upload(file.localPath, file.fileName, key, progressBar, idx, total)
				if file.cleanup != nil {
					file.cleanup()
				}
				if err != nil {
					failMsgs = append(failMsgs, fmt.Sprintf("%s: %v", file.fileName, err))
					continue
				}

				item := UploadHistory{
					UploadDate: time.Now(),
					FileName:   file.fileName,
					URL:        pixelDrainURL,
				}
				successes = append(successes, item)
			}

			fyne.Do(func() {
				progressBar.SetValue(1)

				if len(successes) > 0 {
					last := successes[len(successes)-1]
					MyApp.Storage.LastElement = last
					MyApp.Storage.UploadHistory = append(MyApp.Storage.UploadHistory, successes...)

					// Result link: last URL. Copy uses all URLs when multi (one per line).
					urls := make([]string, len(successes))
					for i, s := range successes {
						urls[i] = s.URL
					}
					joined := uploadutil.JoinURLs(urls)
					_ = linkBound.Set(joined)
					// Auto-copy the result link(s) to the clipboard once the upload finishes.
					MyApp.Window.Clipboard().SetContent(joined)
					parsedURL, _ := url.Parse(last.URL)
					currentResultURL = last.URL
					if len(successes) == 1 {
						resultWidget.SetText(last.URL)
						resultTitle.SetText("Ссылка готова")
					} else {
						resultWidget.SetText(last.URL)
						resultTitle.SetText(fmt.Sprintf("Загружено файлов: %d (показана последняя)", len(successes)))
					}
					resultWidget.SetURL(parsedURL)
					resultStatus.SetText("Ссылка уже скопирована в буфер обмена")
					copyLinkButton.SetText(ButtonCopy)
					resultContainer.Show()

					lastElementContainer.Objects = nil
					lastElementURLParsed, _ := url.Parse(last.URL)
					lastElementText := widget.NewLabel("Last upload:")
					lastElementLink := widget.NewHyperlink(last.FileName, lastElementURLParsed)
					lastElementContainer.Add(lastElementText)
					lastElementContainer.Add(lastElementLink)
					lastElementContainer.Show()
					lastElementContainer.Refresh()
				}

				if len(failMsgs) > 0 {
					msg := fmt.Sprintf("%d of %d upload(s) failed:\n%s", len(failMsgs), total, strings.Join(failMsgs, "\n"))
					dialog.ShowError(errors.New(msg), MyApp.Window)
				}
			})
		}()
	})
	uploadAllButton.Importance = widget.HighImportance
	uploadAllButton.Disable()

	containerButton := container.New(layout.NewGridLayoutWithColumns(2), addFilesButton, uploadAllButton)

	// Bottom account line (small, muted); updated via the username binding.
	accountLine := buildAccountLine()

	// Drop zone: rounded card with an upload icon. Files can also be dropped
	// anywhere on the window; the card is the visual target.
	queueArea := container.NewStack(
		buildDropZone(emptyHintBox),
		queueContainer,
	)

	// Custom top bar (replaces the system title bar in frameless mode).
	// Left side: drag area. Right side: settings, minimize, close.
	historyButton := topBarButton(theme.HistoryIcon(), func() {
		BuildHistoryWindow()
	})
	gearButton := topBarButton(theme.SettingsIcon(), func() {
		BuildSettingsWindow()
	})
	minimizeButton := topBarButton(theme.ContentRemoveIcon(), func() {
		MinimizeNativeWindow(MyApp.Window)
	})
	closeButton := topBarButton(theme.WindowCloseIcon(), func() {
		MyApp.Window.Close()
	})
	controls := container.NewHBox(historyButton, gearButton, minimizeButton, closeButton)

	// Title on the left (muted), empty drag area in the middle, controls on the right.
	titleLabel := widget.NewLabel("DRAINY")
	titleLabel.TextStyle = fyne.TextStyle{Bold: true}
	titleLabel.Importance = widget.LowImportance
	dragArea := newDragArea(MyApp.Window)
	topBar := container.NewBorder(nil, nil, titleLabel, controls, dragArea)

	// Bottom line: account name, small and muted.
	copyrightLine := buildCopyright()

	body := container.NewVBox(
		topBar,
		queueArea,
		containerButton,
		uploadStatus,
		containerProgressBar,
		resultContainer,
		container.New(layout.NewHBoxLayout(), layout.NewSpacer(), lastElementContainer, layout.NewSpacer()),
	)

	container00 := container.NewBorder(
		nil,
		container.NewVBox(widget.NewSeparator(), accountLine, copyrightLine),
		nil, nil,
		container.NewPadded(body),
	)

	MyApp.Window.SetContent(container00)

	// Drag & drop: files dropped anywhere on the window go into the same queue
	// as "Add files". Directories are skipped.
	MyApp.Window.SetOnDropped(func(_ fyne.Position, items []fyne.URI) {
		var paths []string
		for _, u := range items {
			if u == nil {
				continue
			}
			paths = append(paths, u.Path())
		}
		stageDroppedPaths(paths, func(filePath, fileName string, cleanup func()) {
			fyne.Do(func() { appendPending(filePath, fileName, cleanup) })
		})
	})

	// Command-line / "Send to" integration: files passed as arguments are queued
	// and uploaded right away, skipping the manual picker.
	if argPaths := filesFromArgs(os.Args[1:]); len(argPaths) > 0 {
		for _, p := range argPaths {
			local, readable := fileprep.ReadableLocalFile(p)
			if !readable {
				continue
			}
			appendPending(local, fileprep.SanitizeFileName(filepath.Base(local)), nil)
		}
		if len(pending) > 0 {
			// Start the upload once the window is up.
			go func() {
				time.Sleep(300 * time.Millisecond)
				fyne.Do(func() { uploadAllButton.OnTapped() })
			}()
		}
	}

	MyApp.Window.Show()
	// Remove the system frame once the native window exists. fyne.Do runs the
	// call on the main loop after the first frame.
	fyne.Do(func() { MakeWindowFrameless(MyApp.Window) })
	MyApp.App.Run()
}

// topBarButtonSize is the fixed size of every top-bar control (settings, history,
// minimize, close) so the buttons match in all windows.
var topBarButtonSize = fyne.NewSize(32, 32)

// topBarButton builds a borderless icon button of the shared top-bar size.
// The fixed size wrapper stops icon-dependent sizing from making buttons uneven.
func topBarButton(icon fyne.Resource, onTapped func()) fyne.CanvasObject {
	btn := widget.NewButtonWithIcon("", icon, onTapped)
	btn.Importance = widget.LowImportance
	return container.NewGridWrap(topBarButtonSize, btn)
}

// dragArea is an invisible strip that starts a native window move on left-button
// press. It fills the free space of the top bar.
type dragArea struct {
	widget.BaseWidget
	win fyne.Window
}

func newDragArea(win fyne.Window) *dragArea {
	d := &dragArea{win: win}
	d.ExtendBaseWidget(d)
	return d
}

func (d *dragArea) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(canvas.NewRectangle(color.Transparent))
}

// MouseDown starts the native drag on the primary (left) button.
func (d *dragArea) MouseDown(ev *desktop.MouseEvent) {
	if ev.Button == desktop.MouseButtonPrimary {
		StartNativeDrag(d.win)
	}
}

// MouseUp is required by desktop.Mouseable; nothing to do on release.
func (d *dragArea) MouseUp(*desktop.MouseEvent) {}

// stageDroppedPaths validates paths (files only) and queues them via add.
// Unreadable entries are reported in one error dialog, like the file picker does.
func stageDroppedPaths(paths []string, add func(filePath, fileName string, cleanup func())) {
	var failMsgs []string
	for _, p := range paths {
		local, readable := fileprep.ReadableLocalFile(p)
		if !readable {
			failMsgs = append(failMsgs, fmt.Sprintf("%s: not a readable file", filepath.Base(p)))
			continue
		}
		add(local, fileprep.SanitizeFileName(filepath.Base(local)), nil)
	}
	if len(failMsgs) > 0 {
		fyne.Do(func() {
			dialog.ShowError(
				fmt.Errorf("%d path(s) could not be added:\n%s", len(failMsgs), strings.Join(failMsgs, "\n")),
				MyApp.Window,
			)
		})
	}
}

// filesFromArgs returns the arguments that look like existing file paths.
// Flags (starting with "-") are ignored.
func filesFromArgs(args []string) []string {
	var out []string
	for _, a := range args {
		a = strings.Trim(strings.TrimSpace(a), `"`)
		if a == "" || strings.HasPrefix(a, "-") {
			continue
		}
		if info, err := os.Stat(a); err == nil && !info.IsDir() {
			out = append(out, a)
		}
	}
	return out
}

// buildDropZone wraps the hint in a rounded, padded card with an upload icon.
// The hint is the empty-state content; the card is hidden when the queue has files.
func buildDropZone(hint *fyne.Container) *fyne.Container {
	icon := widget.NewIcon(theme.UploadIcon())
	iconBox := container.New(layout.NewCenterLayout(), icon)
	inner := container.NewPadded(container.NewVBox(
		layout.NewSpacer(),
		iconBox,
		hint,
		layout.NewSpacer(),
	))
	card := widget.NewCard("", "", inner)
	return container.NewPadded(card)
}

// buildAccountLine shows the current profile in small, muted text above the license line.
func buildAccountLine() *fyne.Container {
	usern := MyApp.App.Preferences().String(SettingUsername)
	if usern != "" {
		_ = MyApp.Settings.Username.Set(usern)
	}
	if usern == "" {
		usern = "не подключен"
	}

	// Small, muted text. canvas.Text supports TextSize; it is refreshed from the
	// bound username so the line stays current after Settings are saved.
	line := canvas.NewText("Аккаунт: "+usern, theme.DisabledColor())
	line.TextSize = 11
	MyApp.Settings.Username.AddListener(binding.NewDataListener(func() {
		name, _ := MyApp.Settings.Username.Get()
		line.Text = "Аккаунт: " + name
		fyne.Do(func() { line.Refresh() })
	}))

	return container.New(layout.NewCenterLayout(), line)
}

func buildCopyright() *fyne.Container {
	copyright := canvas.NewText(FooterText+Version, theme.DisabledColor())
	copyright.TextSize = 11
	containerEnd := container.New(layout.NewCenterLayout(), copyright)
	containerEnd.Resize(fyne.NewSize(0, 50))

	return containerEnd
}

// materializeForUpload returns a real local filesystem path for go-pd's
// PathToFile / SetFile upload path.
//
// Desktop file:// URIs already map to readable paths. On Android (and other SAF
// providers) the dialog yields content://document URIs such as
// /document/primary%3APictures%2Fphoto.png which cannot be opened with os.Open.
// In that case the URI reader is copied into a temporary file that the caller
// must remove via the returned cleanup function.
//
// The reader is always closed by this function.
func materializeForUpload(reader fyne.URIReadCloser) (localPath, fileName string, cleanup func(), err error) {
	cleanup = func() {}
	if reader == nil {
		return "", "", cleanup, errors.New("file reader is nil")
	}
	defer func() {
		// Ensure the picker stream is closed even if we already consumed it.
		_ = reader.Close()
	}()

	fileURI := reader.URI()
	fileName = fileprep.SanitizeFileName(fileURI.Name())

	if path, ok := fileprep.ReadableLocalFile(fileURI.Path()); ok {
		return path, fileName, cleanup, nil
	}

	// Android SAF / content URIs: stream into a temp file for go-pd SetFile.
	tmpPath, copyErr := fileprep.CopyReaderToTempFile(reader, fileName)
	if copyErr != nil {
		return "", "", cleanup, fmt.Errorf("copy from %q: %w", fileURI.String(), copyErr)
	}
	cleanup = func() { _ = os.Remove(tmpPath) }
	return tmpPath, fileName, cleanup, nil
}

// upload sends a local file to PixelDrain.
//
// go-pd UploadPOST always uses PathToFile with req.SetFile (the File reader is
// not used for the HTTP body). That is required for correct progress FileSize.
//
// fileIndex and fileTotal map this file's progress into a multi-file overall bar:
// overall = (fileIndex + fraction) / fileTotal. Pass fileTotal=1 for a single file.
func upload(filePath, fileName, key string, progressBar *widget.ProgressBar, fileIndex, fileTotal int) (string, error) {
	if filePath == "" {
		return "", errors.New("file path can not be empty")
	}
	if fileName == "" {
		fileName = filepath.Base(filePath)
	}
	if fileName == "" || fileName == "." || fileName == "/" {
		return "", errors.New("filename can not be empty")
	}
	if fileTotal < 1 {
		fileTotal = 1
	}
	if fileIndex < 0 {
		fileIndex = 0
	}

	// Streamed multipart upload with a 4 MiB io.CopyBuffer (see uploadutil.UploadFile).
	return uploadutil.UploadFile(filePath, fileName, key, func(written, total int64) {
		if total <= 0 {
			return
		}
		fraction := float64(written) / float64(total)
		overall := uploadutil.MultiFileProgress(fileIndex, fileTotal, fraction)
		fyne.Do(func() {
			progressBar.SetValue(overall)
		})
	})
}
