package main

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

func main() {
	a := app.New()
	w := a.NewWindow("DigitShowGo")
	w.Resize(fyne.NewSize(800, 600))

	// Content label.
	status := widget.NewLabel("Hello, World!")
	status.Alignment = fyne.TextAlignCenter

	// Helper to create a menu item that updates the status label.
	menuAction := func(label string) *fyne.MenuItem {
		return fyne.NewMenuItem(label, func() {
			status.SetText("Clicked: " + label)
		})
	}

	// Build menus.
	appMenu := fyne.NewMenu("App",
		menuAction("Version"),
		menuAction("Env Variables"),
		menuAction("WebServer Info"),
		menuAction("Open Log Folder"),
		menuAction("Open Temp Folder"),
	)
	aioMenu := fyne.NewMenu("AI/AO", 
		menuAction("AI Calibration"),
		menuAction("AO VoltageOut"),
	)
	speMenu := fyne.NewMenu("Specimen", 
		menuAction("Config"),
	)
	ctlMenu := fyne.NewMenu("Control",
		menuAction("PreConsolidation"),
		menuAction("StepControl"),
	)

	mainMenu := fyne.NewMainMenu(appMenu, aioMenu, speMenu, ctlMenu)
	w.SetMainMenu(mainMenu)
	w.SetContent(container.NewCenter(status))
	w.ShowAndRun()
}
