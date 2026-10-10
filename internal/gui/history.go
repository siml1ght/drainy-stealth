package gui

import (
	"fmt"
	"image/color"
	"os"
	"sort"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/ManuelReschke/go-pd/pkg/pd"
)

// historyEntryURL builds the public PixelDrain link for an uploaded file.
func historyEntryURL(item pd.FileGetUser) string {
	return "https://pixeldrain.com/u/" + item.ID
}

// historyEntry renders one history item on the plain window background (no fill):
// file icon on the left, name and muted details in the middle, copy button on the
// right, all centred vertically. A thin separator follows each entry.
func historyEntry(item pd.FileGetUser, onCopy func(url string)) fyne.CanvasObject {
	// Icon and button are centred in the row by a Center wrapper.
	icon := widget.NewIcon(theme.FileIcon())
	iconBox := container.NewCenter(icon)

	name := widget.NewLabel(item.Name)
	name.TextStyle = fyne.TextStyle{Bold: true}
	name.Truncation = fyne.TextTruncateEllipsis

	subTitleLabel := widget.NewLabel(fmt.Sprintf(
		"%s • Просмотров: %d • Скачиваний: %d",
		item.DateUpload.Format("02.01.2006 15:04"),
		item.Views,
		item.Downloads,
	))
	// MediumImportance is the readable muted tone; LowImportance was too dark on the
	// plain background.
	subTitleLabel.Importance = widget.MediumImportance
	subTitleLabel.Truncation = fyne.TextTruncateEllipsis

	// Text block in the centre: it takes all the free width. It must not be wrapped
	// in NewCenter, which gives children their minimum width and makes labels
	// truncate to "...".
	textContainer := container.NewVBox(name, subTitleLabel)

	copyButton := widget.NewButtonWithIcon("", theme.ContentCopyIcon(), func() {
		onCopy(historyEntryURL(item))
	})
	copyButton.Importance = widget.LowImportance

	// Icon and button are wrapped only to centre them vertically; they keep their
	// minimum size, so the text gets the remaining width.
	rowContent := container.NewBorder(
		nil, nil,
		container.NewPadded(iconBox),
		container.NewCenter(copyButton),
		textContainer,
	)

	return container.NewVBox(
		container.NewPadded(rowContent),
		widget.NewSeparator(),
	)
}

// showHistoryWindow builds the frameless History window: custom header with a
// title, close button and drag area, padded body, and a small bottom margin.
func showHistoryWindow(window fyne.Window, body fyne.CanvasObject) {
	titleLabel := widget.NewLabel("История")
	titleLabel.TextStyle = fyne.TextStyle{Bold: true}
	titleLabel.Importance = widget.LowImportance

	closeButton := topBarButton(theme.WindowCloseIcon(), func() {
		window.Close()
	})

	header := container.NewPadded(container.NewBorder(
		nil, nil,
		titleLabel,
		closeButton,
		newDragArea(window),
	))

	bottomSpace := canvas.NewRectangle(color.Transparent)
	bottomSpace.SetMinSize(fyne.NewSize(0, 12))

	window.SetContent(container.NewBorder(
		container.NewVBox(header, widget.NewSeparator()),
		bottomSpace, nil, nil,
		container.NewPadded(body),
	))
	window.Resize(fyne.NewSize(600, 420))
	window.Show()

	// Frameless like the main and Settings windows.
	fyne.Do(func() { MakeWindowFrameless(window) })
}

// messageBody is a centred single-line status message (no key, API error).
func messageBody(msg string) fyne.CanvasObject {
	label := widget.NewLabel(msg)
	label.Alignment = fyne.TextAlignCenter
	label.Importance = widget.LowImportance
	return container.New(layout.NewCenterLayout(), label)
}

func BuildHistoryWindow() {
	window := MyApp.App.NewWindow("History")

	key := storedAPIKey(MyApp.App.Preferences())
	if key == "" {
		key = strings.TrimSpace(os.Getenv(EnvAPIKey))
	}
	if key == "" {
		// No API key means no connected account: skip the request entirely.
		showHistoryWindow(window, messageBody("Аккаунт не подключен. Добавьте API ключ в Settings."))
		return
	}

	req := &pd.RequestGetUserFiles{
		Auth: pd.Auth{
			APIKey: key,
		},
	}

	client := pd.New(nil, nil)
	rsp, err := client.GetUserFiles(req)
	if err != nil || rsp == nil || rsp.StatusCode != 200 {
		msg := "error occurred"
		if err != nil {
			msg = "error occurred: " + err.Error()
		} else if rsp == nil {
			msg = "error occurred: empty response"
		} else {
			msg = fmt.Sprintf("error occurred: HTTP %d", rsp.StatusCode)
		}
		showHistoryWindow(window, messageBody(msg))
		return
	}

	// Collect every entry (map keys are not guaranteed to be 0..n-1), newest first.
	ordered := make([]pd.FileGetUser, 0, len(rsp.Files))
	for _, item := range rsp.Files {
		ordered = append(ordered, item)
	}
	sort.Slice(ordered, func(i, j int) bool {
		return ordered[i].DateUpload.After(ordered[j].DateUpload)
	})

	if len(ordered) == 0 {
		showHistoryWindow(window, messageBody("Загрузок пока нет."))
		return
	}

	// Status line under the header; flashes after a copy.
	status := widget.NewLabel("")
	status.Alignment = fyne.TextAlignCenter
	status.Importance = widget.LowImportance

	cards := make([]fyne.CanvasObject, 0, len(ordered))
	for _, item := range ordered {
		cards = append(cards, historyEntry(item, func(url string) {
			MyApp.Window.Clipboard().SetContent(url)
			status.SetText("Ссылка скопирована: " + url)
		}))
	}

	list := container.NewVBox(cards...)
	scroll := container.NewVScroll(list)

	showHistoryWindow(window, container.NewBorder(status, nil, nil, nil, scroll))
}
