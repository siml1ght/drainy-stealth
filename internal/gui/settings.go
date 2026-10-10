package gui

import (
	"encoding/json"
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/ManuelReschke/go-pd/pkg/pd"
)

const (
	FormLabel               = "API KEY:"
	FormLabelInput          = "*optional"
	FormLabelTheme          = "Theme:"
	FormLabelFilePicker     = "Filepicker:"
	ThemeOptionDark         = "Dark"
	ThemeOptionLight        = "Light"
	SettingAPIKey           = "setting.apikey"
	SettingUserInfoJSON     = "setting.user.info.json"
	SettingUsername         = "setting.user.name"
	SettingUserSubscription = "setting.user.subscription"

	// EnvAPIKey is an optional environment variable used when no key is saved in Settings.
	EnvAPIKey = "PIXELDRAIN_API_KEY"
)

func BuildSettingsWindow() {
	w := MyApp.App.NewWindow("Settings")

	// FORM INPUT API-KEY
	formLabelAPIKey := widget.NewLabel(FormLabel)

	// Password entry: Fyne adds its own eye icon (show/hide) inside the field.
	inputAPIKey := widget.NewPasswordEntry()
	inputAPIKey.SetPlaceHolder(FormLabelInput)
	// MyApp.Settings.APIKey = binding.NewString()
	_ = MyApp.Settings.APIKey.Set(storedAPIKey(MyApp.App.Preferences()))
	inputAPIKey.Bind(MyApp.Settings.APIKey)

	// Explains what the key changes (without a key uploads are anonymous).
	keyHint := widget.NewLabel("Без ключа файлы загружаются анонимно. С ключом файлы привязываются к аккаунту и появляются в истории.")
	keyHint.Wrapping = fyne.TextWrapWord
	keyHint.Importance = widget.LowImportance

	// Enter in the key field stores the value immediately.
	inputAPIKey.OnSubmitted = func(s string) {
		_ = persistAPIKey(MyApp.App.Preferences(), s)
	}

	userData := getUserDataFromSettings()
	displayUsername := userData.Username
	if displayUsername != "" {
		_ = MyApp.Settings.Username.Set(displayUsername)
	}
	if displayUsername == "" {
		displayUsername = "Invalid API Key or PixelDrain is not available"
	}
	displaySubscription := userData.Subscription.Name
	if displaySubscription != "" {
		_ = MyApp.Settings.Subscription.Set(displaySubscription)
	}
	displaySubscription, _ = MyApp.Settings.Subscription.Get()
	profileLabel := widget.NewLabel("Аккаунт: " + displayUsername + " · " + displaySubscription)
	profileLabel.Importance = widget.LowImportance
	profileLabel.TextStyle = fyne.TextStyle{Monospace: false}

	// Theme switch (Dark / Light)
	formLabelTheme := widget.NewLabelWithStyle(FormLabelTheme, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	// Set the initial selection BEFORE attaching OnChanged. Otherwise SetSelected
	// fires the callback, applyTheme runs on a window whose native handle does not
	// exist yet, and the app panics (nil pointer in glfw GetWin32Window).
	themeSelect := widget.NewRadioGroup([]string{ThemeOptionDark, ThemeOptionLight}, nil)
	themeSelect.Horizontal = true
	themeSelect.Required = true
	if currentThemeName() == ThemeLight {
		themeSelect.SetSelected(ThemeOptionLight)
	} else {
		themeSelect.SetSelected(ThemeOptionDark)
	}
	themeSelect.OnChanged = func(selected string) {
		switch selected {
		case ThemeOptionLight:
			applyTheme(ThemeLight)
		case ThemeOptionDark:
			applyTheme(ThemeDark)
		}
	}

	// File picker: Fyne single-file vs OS native multi-select
	formLabelFilePicker := widget.NewLabelWithStyle(FormLabelFilePicker, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	filePickerSelect := widget.NewRadioGroup(
		[]string{FilePickerOptionFyne, FilePickerOptionNative},
		nil,
	)
	// Vertical so long labels stay readable in the form layout.
	filePickerSelect.Horizontal = false
	filePickerSelect.Required = true
	filePickerSelect.SetSelected(filePickerLabelFromMode(currentFilePickerMode()))
	filePickerSelect.OnChanged = func(selected string) {
		if selected == "" {
			return
		}
		setFilePickerMode(filePickerModeFromLabel(selected))
	}

	// FormLayout: labels in the left column, controls in the right column.
	formLabelAPIKey.TextStyle = fyne.TextStyle{Bold: true}
	formBody := container.New(
		layout.NewFormLayout(),
		formLabelAPIKey, inputAPIKey,
		layout.NewSpacer(), keyHint,
		formLabelTheme, themeSelect,
		formLabelFilePicker, filePickerSelect,
	)

	saveButton := widget.NewButtonWithIcon("Save", theme.ConfirmIcon(), func() {
		key, _ := MyApp.Settings.APIKey.Get()

		// check if the key is valid fetch data from pixeldrain /user
		userInfos := ""
		req := &pd.RequestGetUser{
			Auth: pd.Auth{
				APIKey: key,
			},
		}

		client := pd.New(nil, nil)
		rsp, err := client.GetUser(req)
		if err != nil || rsp.StatusCode != 200 {
			// nothing
		} else {
			b, _ := json.Marshal(rsp)
			userInfos = string(b)
		}

		MyApp.App.Preferences().SetString(SettingUserInfoJSON, userInfos)
		userData := getUserDataFromSettings()
		MyApp.App.Preferences().SetString(SettingUsername, userData.Username)
		MyApp.App.Preferences().SetString(SettingUserSubscription, userData.Subscription.Name)
		_ = MyApp.Settings.Username.Set(userData.Username)
		_ = MyApp.Settings.Subscription.Set(userData.Subscription.Name)
		_ = persistAPIKey(MyApp.App.Preferences(), key) // store user input in the keystore

		w.Hide()
	})
	saveButton.Importance = widget.HighImportance

	content := container.NewVBox(
		container.NewPadded(formBody),
		container.NewPadded(container.NewCenter(profileLabel)),
		container.NewPadded(saveButton),
	)

	// Custom header: grey title on the left, a single close button on the right.
	// The header is also the drag handle, same as the main window.
	titleLabel := widget.NewLabel("Настройки")
	titleLabel.TextStyle = fyne.TextStyle{Bold: true}
	titleLabel.Importance = widget.LowImportance
	closeButton := topBarButton(theme.WindowCloseIcon(), func() {
		w.Close()
	})
	header := container.NewPadded(container.NewBorder(
		nil, nil,
		titleLabel,
		closeButton,
		newDragArea(w),
	))

	// Breathing room under the Save button, so it does not touch the bottom edge.
	bottomSpace := canvas.NewRectangle(color.Transparent)
	bottomSpace.SetMinSize(fyne.NewSize(0, 12))

	w.SetContent(container.NewBorder(
		container.NewVBox(header, widget.NewSeparator()),
		bottomSpace, nil, nil,
		container.NewPadded(content),
	))
	w.Resize(fyne.NewSize(420, 420))
	w.Show()

	// Frameless like the main window: remove the system frame after the native window exists.
	fyne.Do(func() { MakeWindowFrameless(w) })
}

func getUserDataFromSettings() *pd.ResponseGetUser {
	userSettingJson := MyApp.App.Preferences().StringWithFallback(SettingUserInfoJSON, "")

	userData := &pd.ResponseGetUser{}
	err := json.Unmarshal([]byte(userSettingJson), userData)
	if err != nil {
		return userData
	}

	return userData
}
