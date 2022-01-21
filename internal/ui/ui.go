// Package ui provides primitives that initialises the UI.
package ui

import (
	"fmt"
	"runtime"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"

	appApp "github.com/beebeeoii/lominus/internal/app"
	appAuth "github.com/beebeeoii/lominus/internal/app/auth"
	intTelegram "github.com/beebeeoii/lominus/internal/app/integrations/telegram"
	appPref "github.com/beebeeoii/lominus/internal/app/pref"
	"github.com/beebeeoii/lominus/internal/cron"
	"github.com/beebeeoii/lominus/internal/file"
	logs "github.com/beebeeoii/lominus/internal/log"
	"github.com/beebeeoii/lominus/internal/lominus"
	"github.com/beebeeoii/lominus/internal/notifications"
	"github.com/beebeeoii/lominus/pkg/auth"
	"github.com/beebeeoii/lominus/pkg/integrations/telegram"
	"github.com/getlantern/systray"
	fileDialog "github.com/sqweek/dialog"
)

const (
	FREQUENCY_DISABLED    = "Disabled"
	FREQUENCY_ONE_HOUR    = "1 hour"
	FREQUENCY_TWO_HOUR    = "2 hour"
	FREQUENCY_FOUR_HOUR   = "4 hour"
	FREQUENCY_SIX_HOUR    = "6 hour"
	FREQUENCY_TWELVE_HOUR = "12 hour"
)

var frequencyMap = map[int]string{
	1:  FREQUENCY_ONE_HOUR,
	2:  FREQUENCY_TWO_HOUR,
	4:  FREQUENCY_FOUR_HOUR,
	6:  FREQUENCY_SIX_HOUR,
	12: FREQUENCY_TWELVE_HOUR,
	-1: FREQUENCY_DISABLED,
}

var mainApp fyne.App
var w fyne.Window

// Init builds and initialises the UI.
func Init() error {
	if runtime.GOOS == "windows" {
		systray.Register(onReady, onExit)
	}
	mainApp = app.NewWithID(lominus.APP_NAME)
	mainApp.SetIcon(resourceAppIconPng)

	go func() {
		for {
			notification := <-notifications.NotificationChannel
			mainApp.SendNotification(fyne.NewNotification(notification.Title, notification.Content))
		}
	}()

	w = mainApp.NewWindow(fmt.Sprintf("%s v%s", lominus.APP_NAME, lominus.APP_VERSION))

	credentialsTab, credentialsUiErr := getCredentialsTab(w)
	if credentialsUiErr != nil {
		return credentialsUiErr
	}

	preferencesTab, preferencesErr := getPreferencesTab(w)
	if preferencesErr != nil {
		return preferencesErr
	}

	integrationsTab, integrationsErr := getIntegrationsTab(w)
	if integrationsErr != nil {
		return integrationsErr
	}

	tabsContainer := container.NewAppTabs(credentialsTab, preferencesTab, integrationsTab)
	content := container.NewVBox(
		tabsContainer,
		layout.NewSpacer(),
		getSyncButton(w),
		getQuitButton(),
	)

	w.SetContent(content)
	w.Resize(fyne.NewSize(600, 600))
	w.SetFixedSize(true)
	w.SetMaster()
	w.SetCloseIntercept(func() {
		w.Hide()
		notifications.NotificationChannel <- notifications.Notification{Title: "Lominus", Content: "Lominus is still running in the background to keep your files synced"}
	})
	mainApp.Lifecycle().SetOnEnteredForeground(func() {
		w.Show()
	})
	w.ShowAndRun()
	return nil
}

// getCredentialsTab builds the credentials tab in the main UI.
func getCredentialsTab(parentWindow fyne.Window) (*container.TabItem, error) {
	tab := container.NewTabItem("Login Info", container.NewVBox())

	label := widget.NewLabelWithStyle("Your Credentials", fyne.TextAlignLeading, fyne.TextStyle{Bold: true, Italic: false, Monospace: false, TabWidth: 0})
	subLabel := widget.NewRichTextFromMarkdown("Credentials are saved **locally**. It is **only** used to login to your [Luminus](https://luminus.nus.edu.sg) account.")
	subLabel.Wrapping = fyne.TextWrapBreak

	usernameEntry := widget.NewEntry()
	usernameEntry.SetPlaceHolder("Eg: nusstu\\e0123456")
	passwordEntry := widget.NewPasswordEntry()
	passwordEntry.SetPlaceHolder("Password")

	credentialsPath := appAuth.GetCredentialsPath()

	if file.Exists(credentialsPath) {
		credentials, err := auth.LoadCredentials(credentialsPath)
		if err != nil {
			return tab, err
		}

		usernameEntry.SetText(credentials.Username)
		passwordEntry.SetText(credentials.Password)
	}

	credentialsForm := widget.NewForm(widget.NewFormItem("Username", usernameEntry), widget.NewFormItem("Password", passwordEntry))

	saveButtonText := "Save Credentials"
	if usernameEntry.Text != "" && passwordEntry.Text != "" {
		saveButtonText = "Update Credentials"
	}

	saveButton := widget.NewButton(saveButtonText, func() {
		credentials := auth.Credentials{Username: usernameEntry.Text, Password: passwordEntry.Text}

		status := widget.NewLabel("Please wait while we verify your credentials...")
		progressBar := widget.NewProgressBarInfinite()

		mainDialog := dialog.NewCustom(lominus.APP_NAME, "Cancel", container.NewVBox(status, progressBar), parentWindow)
		mainDialog.Show()

		_, err := auth.RetrieveJwtToken(credentials, true)
		mainDialog.Hide()
		if err != nil {
			dialog.NewInformation(lominus.APP_NAME, "Verification failed. Please check your credentials.", parentWindow).Show()
		} else {
			auth.SaveCredentials(credentialsPath, credentials)
			dialog.NewInformation(lominus.APP_NAME, "Verification successful.", parentWindow).Show()
		}
	})

	tab.Content = container.NewVBox(
		label,
		widget.NewSeparator(),
		subLabel,
		credentialsForm,
		saveButton,
	)

	return tab, nil
}

// getPreferencesTab builds the preferences tab in the main UI.
func getPreferencesTab(parentWindow fyne.Window) (*container.TabItem, error) {
	tab := container.NewTabItem("Preferences", container.NewVBox())

	fileDirHeader := widget.NewLabelWithStyle("File Directory", fyne.TextAlignLeading, fyne.TextStyle{Bold: true, Italic: false, Monospace: false, TabWidth: 0})
	fileDirSubHeader := widget.NewLabel("Root directory for your Luminus files:")

	dir := getPreferences().Directory
	if dir == "" {
		dir = "Not set"
	}

	fileDirLabel := widget.NewLabel(dir)
	fileDirLabel.Wrapping = fyne.TextWrapBreak
	chooseDirButton := widget.NewButton("Choose directory", func() {
		dir, dirErr := fileDialog.Directory().Title("Choose directory").Browse()
		if dirErr != nil {
			if dirErr.Error() != "Cancelled" {
				dialog.NewInformation(lominus.APP_NAME, "An error has occurred :( Please try again or contact us.", parentWindow).Show()
				logs.Logger.Errorln(dirErr)
			}
			return
		}

		preferences := getPreferences()
		preferences.Directory = dir

		savePrefErr := appPref.SavePreferences(appPref.GetPreferencesPath(), preferences)
		if savePrefErr != nil {
			dialog.NewInformation(lominus.APP_NAME, "An error has occurred :( Please try again or contact us.", parentWindow).Show()
			logs.Logger.Errorln(savePrefErr)
			return
		}
		fileDirLabel.SetText(preferences.Directory)
	})

	frequencyHeader := widget.NewLabelWithStyle("Sync Frequency", fyne.TextAlignLeading, fyne.TextStyle{Bold: true, Italic: false, Monospace: false, TabWidth: 0})
	frequencySubHeader1 := widget.NewRichTextFromMarkdown("Lominus helps to sync files and more from [Luminus](https://luminus.nus.edu.sg) **automatically**.")
	frequencySubHeader2 := widget.NewRichTextFromMarkdown("Frequency denotes the number of **hours** between each sync.")

	frequencySelect := widget.NewSelect([]string{FREQUENCY_DISABLED, FREQUENCY_ONE_HOUR, FREQUENCY_TWO_HOUR, FREQUENCY_FOUR_HOUR, FREQUENCY_SIX_HOUR, FREQUENCY_TWELVE_HOUR}, func(s string) {
		preferences := getPreferences()
		switch s {
		case FREQUENCY_DISABLED:
			preferences.Frequency = -1
		case FREQUENCY_ONE_HOUR:
			preferences.Frequency = 1
		case FREQUENCY_TWO_HOUR:
			preferences.Frequency = 2
		case FREQUENCY_FOUR_HOUR:
			preferences.Frequency = 4
		case FREQUENCY_SIX_HOUR:
			preferences.Frequency = 6
		case FREQUENCY_TWELVE_HOUR:
			preferences.Frequency = 12
		default:
			preferences.Frequency = 1
		}

		savePrefErr := appPref.SavePreferences(appPref.GetPreferencesPath(), preferences)
		if savePrefErr != nil {
			dialog.NewInformation(lominus.APP_NAME, "An error has occurred :( Please try again or contact us.", parentWindow).Show()
			logs.Logger.Errorln(savePrefErr)
			return
		}
	})
	frequencySelect.Selected = frequencyMap[getPreferences().Frequency]

	tab.Content = container.NewVBox(
		fileDirHeader,
		widget.NewSeparator(),
		fileDirSubHeader,
		fileDirLabel,
		chooseDirButton,
		frequencyHeader,
		widget.NewSeparator(),
		frequencySubHeader1,
		frequencySubHeader2,
		frequencySelect,
	)

	return tab, nil
}

// getIntegrationsTab builds the integrations tab in the main UI.
func getIntegrationsTab(parentWindow fyne.Window) (*container.TabItem, error) {
	tab := container.NewTabItem("Integrations", container.NewVBox())

	label := widget.NewLabelWithStyle("Telegram", fyne.TextAlignLeading, fyne.TextStyle{Bold: true, Italic: false, Monospace: false, TabWidth: 0})
	subLabel := widget.NewRichTextFromMarkdown("Lominus can be linked to your Telegram bot to notify you when new grades are released.")
	subLabel.Wrapping = fyne.TextWrapBreak

	botApiEntry := widget.NewPasswordEntry()
	botApiEntry.SetPlaceHolder("Your bot's API token")
	userIdEntry := widget.NewEntry()
	userIdEntry.SetPlaceHolder("Your account's ID")

	telegramInfoPath := intTelegram.GetTelegramInfoPath()

	if file.Exists(telegramInfoPath) {
		telegramInfo, err := telegram.LoadTelegramData(telegramInfoPath)
		if err != nil {
			return tab, err
		}

		botApiEntry.SetText(telegramInfo.BotApi)
		userIdEntry.SetText(telegramInfo.UserId)
	}

	telegramForm := widget.NewForm(widget.NewFormItem("Bot API Token", botApiEntry), widget.NewFormItem("User ID", userIdEntry))

	saveButtonText := "Save Telegram Info"
	if botApiEntry.Text != "" && userIdEntry.Text != "" {
		saveButtonText = "Update Telegram Info"
	}

	saveButton := widget.NewButton(saveButtonText, func() {
		botApi := botApiEntry.Text
		userId := userIdEntry.Text

		status := widget.NewLabel("Please wait while we send you a test message...")
		progressBar := widget.NewProgressBarInfinite()

		mainDialog := dialog.NewCustom(lominus.APP_NAME, "Cancel", container.NewVBox(status, progressBar), parentWindow)
		mainDialog.Show()

		err := telegram.SendMessage(botApi, userId, "Thank you for using Lominus! You have succesfully integrated Telegram with Lominus!\n\nBy integrating Telegram with Lominus, you will be notified of the following whenever Lominus polls for new update based on the intervals set:\n💥 new grades releases\n💥 new announcements (TBC)")
		mainDialog.Hide()
		if err != nil {
			errMessage := fmt.Sprintf("%s: %s", err.Error()[:13], err.Error()[strings.Index(err.Error(), "description")+14:len(err.Error())-2])
			dialog.NewInformation(lominus.APP_NAME, errMessage, parentWindow).Show()
		} else {
			telegram.SaveTelegramData(telegramInfoPath, telegram.TelegramInfo{BotApi: botApi, UserId: userId})
			dialog.NewInformation(lominus.APP_NAME, "Test message sent!\nTelegram info saved successfully.", parentWindow).Show()
		}
	})

	tab.Content = container.NewVBox(
		label,
		widget.NewSeparator(),
		subLabel,
		telegramForm,
		saveButton,
	)

	return tab, nil
}

// getPreferences is a util function that retrieves the user's preferences.
func getPreferences() appPref.Preferences {
	preference, err := appPref.LoadPreferences(appPref.GetPreferencesPath())
	if err != nil {
		logs.Logger.Fatalln(err)
	}

	return preference
}

// getSyncButton builds the sync button in the main UI.
func getSyncButton(parentWindow fyne.Window) *widget.Button {
	return widget.NewButton("Sync Now", func() {
		preferences := getPreferences()
		if preferences.Directory == "" {
			dialog.NewInformation(lominus.APP_NAME, "Please set the directory to store your Luminus files", parentWindow).Show()
			return
		}
		if preferences.Frequency == -1 {
			dialog.NewInformation(lominus.APP_NAME, "Sync is currently disabled. Please choose a sync frequency to sync now.", parentWindow).Show()
			return
		}
		cron.Rerun(getPreferences().Frequency)
	})
}

// getQuitButton builds the quit button in the main UI.
func getQuitButton() *widget.Button {
	return widget.NewButton("Quit Lominus", func() {
		if appApp.GetOs() == "windows" {
			logs.Logger.Infoln("systray quit")
			systray.Quit()
		}
		logs.Logger.Infoln("lominus quit")
		mainApp.Quit()
	})
}
