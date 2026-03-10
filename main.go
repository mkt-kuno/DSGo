package main

import (
	"fmt"
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
)

type channelDef struct {
	Index int
	Name  string
	Value string
}

func makeCell(ch channelDef) fyne.CanvasObject {
	label := canvas.NewText(fmt.Sprintf("%02d:%s", ch.Index, ch.Name), color.White)
	label.TextSize = 11

	value := canvas.NewText(ch.Value, color.White)
	value.TextSize = 24
	value.Alignment = fyne.TextAlignTrailing
	value.TextStyle = fyne.TextStyle{Bold: true}

	bg := canvas.NewRectangle(color.NRGBA{255, 255, 255, 0})
	bg.CornerRadius = 6
	bg.StrokeColor = color.NRGBA{255, 255, 255, 128}
	bg.StrokeWidth = 1.5

	valueBox := container.NewStack(bg, container.NewPadded(value))

	return container.NewBorder(label, nil, nil, nil, valueBox)
}

func makeSection(title string, channels []channelDef) fyne.CanvasObject {
	titleText := canvas.NewText(title, color.White)
	titleText.TextSize = 13
	titleText.TextStyle = fyne.TextStyle{Bold: true}

	cells := make([]fyne.CanvasObject, len(channels))
	for i, ch := range channels {
		cells[i] = makeCell(ch)
	}
	grid := container.NewGridWithColumns(8, cells...)
	sep := canvas.NewRectangle(color.White)
	sep.SetMinSize(fyne.NewSize(0, 1))
	return container.NewVBox(titleText, sep, grid)
}

func rawValueChannels() []channelDef {
	names := []string{
		"LoadCell(i16)", "LVDT(i16)", "LDT1(i16)", "LDT2(i16)",
		"none(i16)", "none(i16)", "none(i16)", "none(i16)",
		"HCDPT(i16)", "LCDPT(i16)", "none(i16)", "none(i16)",
		"none(i16)", "none(i16)", "none(i16)", "none(i16)",
	}
	chs := make([]channelDef, len(names))
	for i, n := range names {
		chs[i] = channelDef{Index: i, Name: n, Value: "0"}
	}
	return chs
}

func physicalValueChannels() []channelDef {
	names := []string{
		"Load(N)", "ExtDisp(mm)", "LDT1Disp(mm)", "LDT2Disp(mm)",
		"none", "none", "none", "none",
		"EffCellP(kPa)", "VolChange(mm3)", "none", "none",
		"none", "none", "none", "none",
	}
	chs := make([]channelDef, len(names))
	for i, n := range names {
		chs[i] = channelDef{Index: i, Name: n, Value: "0.0000"}
	}
	return chs
}

func parameterChannels() []channelDef {
	names := []string{
		"q(kPa)", "p'(kPa)", "sigma'(a)(kPa)", "sigma'(r)(kPa)",
		"AxialStrain(%)", "RadialStrain(%)", "VolumetricStrain(%)", "LDT1(mm)",
		"LDT2(mm)", "LocalAxialStrain(%)", "LDT1LocAxStrain(%)", "LDT2LocAxStrain(%)",
		"none", "none", "none", "none",
		"none", "none", "none", "none",
		"none", "none", "none", "none",
		"CurrentDiameter(mm)", "CurrentHeight(mm)", "CurrentArea(mm2)", "CurrentVolume(mm3)",
		"RefDiameter(mm)", "RefHeight(mm)", "RefArea(mm2)", "RefVolume(mm3)",
	}
	chs := make([]channelDef, len(names))
	for i, n := range names {
		chs[i] = channelDef{Index: i, Name: n, Value: "0.0000"}
	}
	return chs
}

func voltageOutChannels() []channelDef {
	return []channelDef{
		{0, "Motor ON/OFF", "0.0000"},
		{1, "Motor UP/DOWN", "0.0000"},
		{2, "Motor Speed", "0.0000"},
		{3, "EP Cell Pressure", "0.0000"},
		{4, "EP Axis Pressure", "0.0000"},
		{5, "Torsional ON/OFF", "0.0000"},
		{6, "Torsional CW/CCW", "0.0000"},
		{7, "Torsional Speed", "0.0000"},
	}
}

func main() {
	a := app.New()
	w := a.NewWindow("DigitShowGo")
	w.Resize(fyne.NewSize(1400, 800))

	// Status label for menu actions.
	status := widget.NewLabel("")

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

	// Build display sections.
	rawSection := makeSection("Raw Value (int16_t: -32768 to +32767)", rawValueChannels())
	physSection := makeSection("Physical Value", physicalValueChannels())
	paramSection := makeSection("Parameter", parameterChannels())
	voltSection := makeSection("Voltage Out", voltageOutChannels())

	sections := container.New(layout.NewVBoxLayout(),
		rawSection, physSection, paramSection, voltSection, status,
	)
	content := container.NewPadded(sections)
	scrollable := container.NewVScroll(content)

	w.SetContent(scrollable)
	w.ShowAndRun()
}
