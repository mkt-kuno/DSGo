package main

import (
	"fmt"
	"image/color"
	"math"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

const (
	int16Max     = 32767
	numRawCh     = 16
	numPhyCh     = 16
	mockInterval = 50 * time.Millisecond
)

var (
	rawThreshWarn  = int16(math.Floor(float64(int16Max) * 0.80)) // 80% of INT16_MAX
	rawThreshAlert = int16(math.Floor(float64(int16Max) * 0.95)) // 95% of INT16_MAX
)

var (
	colorWhite  = color.NRGBA{255, 255, 255, 255}
	colorYellow = color.NRGBA{255, 255, 0, 255}
	colorRed    = color.NRGBA{255, 60, 60, 255}
)

// rawValueColor returns the display color based on the absolute value of a raw int16.
func rawValueColor(v int16) color.Color {
	a := v
	if a < 0 {
		if a == math.MinInt16 {
			return colorRed
		}
		a = -a
	}
	switch {
	case a >= rawThreshAlert:
		return colorRed
	case a >= rawThreshWarn:
		return colorYellow
	default:
		return colorWhite
	}
}

// phyValueColor returns the display color for a physical/parameter float64 value.
// NaN → red, Inf → yellow, otherwise white.
func phyValueColor(v float64) color.Color {
	switch {
	case math.IsNaN(v):
		return colorRed
	case math.IsInf(v, 0):
		return colorYellow
	default:
		return colorWhite
	}
}

type channelDef struct {
	Index int
	Name  string
	Value string
}

// channelCell holds references to updatable UI elements for one channel.
type channelCell struct {
	valueText *canvas.Text
}

func makeCell(ch channelDef) (fyne.CanvasObject, *channelCell) {
	label := canvas.NewText(fmt.Sprintf("%02d:%s", ch.Index, ch.Name), color.White)
	label.TextSize = 16
	label.TextStyle = fyne.TextStyle{Monospace: true}

	value := canvas.NewText(ch.Value, color.White)
	value.TextSize = 32
	value.Alignment = fyne.TextAlignTrailing
	value.TextStyle = fyne.TextStyle{Bold: true, Monospace: true}

	bg := canvas.NewRectangle(color.NRGBA{255, 255, 255, 0})
	bg.CornerRadius = 4
	bg.StrokeColor = color.NRGBA{255, 255, 255, 128}
	bg.StrokeWidth = 1

	valueBox := container.NewStack(bg, value)

	obj := container.NewVBox(label, valueBox)
	return obj, &channelCell{valueText: value}
}

func makeSection(title string, channels []channelDef) (fyne.CanvasObject, []*channelCell) {
	titleText := canvas.NewText(title, color.White)
	titleText.TextSize = 16
	titleText.TextStyle = fyne.TextStyle{Bold: true}

	cells := make([]fyne.CanvasObject, len(channels))
	refs := make([]*channelCell, len(channels))
	for i, ch := range channels {
		cells[i], refs[i] = makeCell(ch)
	}
	grid := container.NewGridWithColumns(8, cells...)
	sep := canvas.NewRectangle(color.White)
	sep.SetMinSize(fyne.NewSize(0, 1))
	return container.NewVBox(titleText, sep, grid), refs
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

// makePlotArea creates a single plot area with axis selectors and a placeholder canvas.
func makePlotArea() fyne.CanvasObject {
	// Axis options (Y-axis uses channel indices as options)
	yOptions := make([]string, numPhyCh)
	for i := 0; i < numPhyCh; i++ {
		yOptions[i] = fmt.Sprintf("%02d", i)
	}
	xAxisSel := widget.NewSelect([]string{"time", "00", "01", "02", "03"}, nil)
	xAxisSel.SetSelected("time")
	yAxisSel := widget.NewSelect(yOptions, nil)
	yAxisSel.SetSelected("00")
	targetSel := widget.NewSelect([]string{"---", "Target1", "Target2"}, nil)
	targetSel.SetSelected("---")
	selectWidth := float32(75)
	selectHeight := float32(30)

	xLabel := canvas.NewText("X-axis", colorWhite)
	xLabel.TextSize = 12
	yLabel := canvas.NewText("Y-axis", colorWhite)
	yLabel.TextSize = 12
	targetLabel := canvas.NewText("Target", colorWhite)
	targetLabel.TextSize = 12

	selectors := container.NewVBox(
		xLabel, container.NewGridWrap(fyne.NewSize(selectWidth, selectHeight), xAxisSel),
		yLabel, container.NewGridWrap(fyne.NewSize(selectWidth, selectHeight), yAxisSel),
		targetLabel, container.NewGridWrap(fyne.NewSize(selectWidth, selectHeight), targetSel),
	)

	// Plot placeholder
	plotBg := canvas.NewRectangle(color.NRGBA{30, 30, 30, 255})
	plotBg.SetMinSize(fyne.NewSize(250, 150))

	plotArea := container.NewBorder(nil, nil, nil, nil, plotBg)

	return container.NewBorder(nil, nil, selectors, nil, plotArea)
}

// makeLogArea creates the log display area.
func makeLogArea(logText *widget.RichText) fyne.CanvasObject {
	bg := canvas.NewRectangle(color.NRGBA{20, 20, 60, 255})
	bg.SetMinSize(fyne.NewSize(170, 150))
	scrollLog := container.NewVScroll(logText)
	scrollLog.SetMinSize(fyne.NewSize(170, 150))
	logPanel := container.NewStack(bg, scrollLog)
	return container.NewGridWrap(fyne.NewSize(170, 150), logPanel)
}

// makeControlStateArea creates the control mode display.
func makeControlStateArea(modeText *canvas.Text) fyne.CanvasObject {
	bg := canvas.NewRectangle(color.NRGBA{30, 30, 30, 255})
	bg.SetMinSize(fyne.NewSize(200, 50))
	modeLabel := canvas.NewText("Mode:", colorWhite)
	modeLabel.TextSize = 14
	row := container.NewHBox(modeLabel, modeText)
	return container.NewStack(bg, container.NewPadded(row))
}

// makeSaveArea creates the data save display with filename entry and elapsed seconds.
func makeSaveArea(filenameEntry *widget.Entry, elapsedLabel *canvas.Text) fyne.CanvasObject {
	saveLabel := canvas.NewText("Save:", colorWhite)
	saveLabel.TextSize = 14
	fileLabel := canvas.NewText("Filename", colorWhite)
	fileLabel.TextSize = 14
	secLabel := canvas.NewText("[sec]", colorWhite)
	secLabel.TextSize = 14
	filenameBox := container.NewGridWrap(fyne.NewSize(150, 30), filenameEntry)

	row := container.NewHBox(
		saveLabel, fileLabel, filenameBox,
		elapsedLabel, secLabel,
	)
	return row
}

// makeCurrentSettings creates the read-only current settings display.
func makeCurrentSettings(ctrlTypeLabel, sampTimeLabel *canvas.Text) fyne.CanvasObject {
	title := canvas.NewText("Current Settings", colorWhite)
	title.TextSize = 14
	title.TextStyle = fyne.TextStyle{Bold: true}

	ctLabel := canvas.NewText("ControlType", colorWhite)
	ctLabel.TextSize = 12
	stLabel := canvas.NewText("SamplingTime", colorWhite)
	stLabel.TextSize = 12

	ctrlTypeBg := canvas.NewRectangle(color.NRGBA{40, 40, 40, 255})
	ctrlTypeBox := container.NewStack(ctrlTypeBg, ctrlTypeLabel)
	sampTimeBg := canvas.NewRectangle(color.NRGBA{40, 40, 40, 255})
	sampTimeBox := container.NewStack(sampTimeBg, sampTimeLabel)

	row1 := container.NewHBox(ctLabel, ctrlTypeBox)
	row2 := container.NewHBox(stLabel, sampTimeBox)

	sep := canvas.NewRectangle(colorWhite)
	sep.SetMinSize(fyne.NewSize(0, 1))
	return container.NewVBox(title, sep, row1, row2)
}

// makeBasicSettings creates the control settings area with dropdowns and Apply buttons.
func makeBasicSettings(
	ctrlTypeSel *widget.Select,
	sampTimeSel *widget.Select,
	ctrlApplyBtn *widget.Button,
	sampApplyBtn *widget.Button,
) fyne.CanvasObject {
	title := canvas.NewText("Basic Settings", colorWhite)
	title.TextSize = 14
	title.TextStyle = fyne.TextStyle{Bold: true}

	ctLabel := canvas.NewText("ControlType", colorWhite)
	ctLabel.TextSize = 12
	stLabel := canvas.NewText("SamplingTime", colorWhite)
	stLabel.TextSize = 12

	sep := canvas.NewRectangle(colorWhite)
	sep.SetMinSize(fyne.NewSize(0, 1))

	row1 := container.NewHBox(ctLabel, ctrlTypeSel, ctrlApplyBtn)
	row2 := container.NewHBox(stLabel, sampTimeSel, sampApplyBtn)
	return container.NewVBox(title, sep, row1, row2)
}

func main() {
	a := app.New()
	a.Settings().SetTheme(theme.DarkTheme())
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
	rawSection, rawCells := makeSection("Raw Value (int16_t: -32768 to +32767)", rawValueChannels())
	physSection, phyCells := makeSection("Physical Value", physicalValueChannels())
	paramSection, _ := makeSection("Parameter", parameterChannels())
	voltSection, _ := makeSection("Voltage Out", voltageOutChannels())

	// --- Bottom area ---

	// Plot areas (2 plots side by side)
	plotTitle := canvas.NewText("Plot", colorWhite)
	plotTitle.TextSize = 16
	plotTitle.TextStyle = fyne.TextStyle{Bold: true}
	plotSep := canvas.NewRectangle(colorWhite)
	plotSep.SetMinSize(fyne.NewSize(0, 1))
	plot1 := makePlotArea()
	plot2 := makePlotArea()
	plotRow := container.NewGridWithColumns(2, plot1, plot2)
	plotSection := container.NewVBox(plotTitle, plotSep, plotRow)

	// Log display area
	nowStr := time.Now().Format("2006-01-02 15:04:05.000")
	logText := widget.NewRichTextFromMarkdown(
		fmt.Sprintf("[%s] [default] [info] config.yaml is not found.\n[%s] [default] [error] modbus_connect failed port:\\\\.\\COM9 slave:1", nowStr, nowStr),
	)
	logArea := makeLogArea(logText)

	// Control state display
	modeText := canvas.NewText("None", colorWhite)
	modeText.TextSize = 14
	controlStateArea := makeControlStateArea(modeText)

	// Data save area
	filenameEntry := widget.NewEntry()
	filenameEntry.SetPlaceHolder("")
	elapsedLabel := canvas.NewText("0", colorWhite)
	elapsedLabel.TextSize = 14
	saveArea := makeSaveArea(filenameEntry, elapsedLabel)

	// Current Settings display
	curCtrlType := canvas.NewText("00:None", colorWhite)
	curCtrlType.TextSize = 14
	curSampTime := canvas.NewText("1", colorWhite)
	curSampTime.TextSize = 14
	currentSettings := makeCurrentSettings(curCtrlType, curSampTime)

	// Basic Settings with controls
	ctrlTypeSel := widget.NewSelect([]string{"None", "Stress", "Strain", "Volume"}, nil)
	ctrlTypeSel.SetSelected("None")
	sampTimeSel := widget.NewSelect([]string{"1 sec", "2 sec", "5 sec", "10 sec"}, nil)
	sampTimeSel.SetSelected("1 sec")

	ctrlApplyBtn := widget.NewButton("Apply", func() {
		curCtrlType.Text = ctrlTypeSel.Selected
		curCtrlType.Refresh()
		status.SetText("Applied ControlType: " + ctrlTypeSel.Selected)
	})
	sampApplyBtn := widget.NewButton("Apply", func() {
		curSampTime.Text = sampTimeSel.Selected
		curSampTime.Refresh()
		status.SetText("Applied SamplingTime: " + sampTimeSel.Selected)
	})
	basicSettings := makeBasicSettings(ctrlTypeSel, sampTimeSel, ctrlApplyBtn, sampApplyBtn)

	// 4 Buttons
	startControlBtn := widget.NewButton("Start Control", func() {
		modeText.Text = ctrlTypeSel.Selected
		modeText.Refresh()
		status.SetText("Control started: " + ctrlTypeSel.Selected)
	})
	stopControlBtn := widget.NewButton("Stop Control", func() {
		modeText.Text = "None"
		modeText.Refresh()
		status.SetText("Control stopped")
	})
	startSavingBtn := widget.NewButton("Start Saving", func() {
		status.SetText("Saving started: " + filenameEntry.Text)
	})
	stopSavingBtn := widget.NewButton("Stop Saving", func() {
		status.SetText("Saving stopped")
	})

	// Layout: control buttons in 2x2 grid
	buttonGrid := container.NewGridWithColumns(2,
		startControlBtn, stopControlBtn,
		startSavingBtn, stopSavingBtn,
	)

	// Right column: Current Settings + Basic Settings + Buttons
	rightCol := container.NewVBox(
		currentSettings,
		basicSettings,
		buttonGrid,
	)

	// Middle-bottom area: log + control state stacked
	midBottomCol := container.NewVBox(
		logArea,
		controlStateArea,
		saveArea,
	)

	// Bottom row: plots | log+state | settings+buttons
	bottomRow := container.NewHBox(
		plotSection,
		midBottomCol,
		rightCol,
	)

	sections := container.New(layout.NewVBoxLayout(),
		rawSection, physSection, paramSection, voltSection,
		bottomRow,
		status,
	)
	content := container.NewPadded(sections)
	scrollable := container.NewVScroll(content)

	w.SetContent(scrollable)

	// Mock AD/DA: each channel gets a sine wave with a different phase offset.
	mock := NewMockADDA(numRawCh)
	go func() {
		for {
			time.Sleep(mockInterval)
			samples := mock.Next(0.05)
			for ch, s := range samples {
				// Update Raw
				rawCells[ch].valueText.Text = fmt.Sprintf("%d", s.Raw)
				rawCells[ch].valueText.Color = rawValueColor(s.Raw)
				rawCells[ch].valueText.Refresh()

				// Update Physical (same channels)
				if ch < numPhyCh {
					phyCells[ch].valueText.Text = fmt.Sprintf("%.4f", s.Phy)
					phyCells[ch].valueText.Color = phyValueColor(s.Phy)
					phyCells[ch].valueText.Refresh()
				}
			}
		}
	}()

	w.ShowAndRun()
}
