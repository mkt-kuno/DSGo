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
	aiMenu := fyne.NewMenu("AI Calibration")
	aoMenu := fyne.NewMenu("AO Voltage Test")
	speMenu := fyne.NewMenu("Specimen Config")
	ctlMenu := fyne.NewMenu("Control",
		menuAction("PreConsolidation"),
		menuAction("Step Control"),
	)
	otherMenu := fyne.NewMenu("Other",
		menuAction("Version"),
		menuAction("Env Variables"),
		menuAction("WebServer Info"),
		menuAction("Open Log Folder"),
		menuAction("Open Temp Folder"),
	)

	w.SetMainMenu(fyne.NewMainMenu(aiMenu, aoMenu, speMenu, ctlMenu, otherMenu))
	w.SetContent(container.NewCenter(status))
	w.ShowAndRun()
}
