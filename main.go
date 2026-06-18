package main

import (
	"encoding/binary"
	"fmt"
	"math"
	"sort"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	. "modernc.org/tk9.0"
	_ "modernc.org/tk9.0/extensions/eval"
	tkeval "modernc.org/tk9.0/extensions/eval"

	"go.bug.st/serial"
)

func toInt(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}

// ─── Colors ──────────────────────────────────────────────────────────────────
// MFC "Digital" theme: dark teal background with white text (matches screenshots)
const (
	bgMain   = "#0d3b3b" // dark teal background
	bgHdr    = "#0d3b3b"
	bgPanel  = "#0d3b3b"
	bgGroup  = "#0a2f2f"
	bgCell   = "#0a2f2f"
	bgCellHi = "#1a4a4a"
	bgBtn    = "#0a2f2f"
	bgBtnOn  = "#0d3b3b"
	bgGreen  = "#69f0ae"
	bgRed    = "#ef5350"
	fgText   = "#ffffff"
	fgLabel  = "#cccccc"
	fgDim    = "#888888"
	fgAccent = "#4fc3f7"
	fgGreen  = "#69f0ae"
	fgOrange = "#ffb74d"
	fgCrit   = "#ef9a9a"
	fgWarn   = "#fff176"
	fgTitle  = "#ffffff"
)

// ─── Channel tables ───────────────────────────────────────────────────────────
// Mirrors the MFC DigitShowBasicM reference.
var rawChNames = [16]string{
	"LoadCell", "LVDT", "LDT1", "LDT2",
	"none", "none", "none", "none",
	"HCDPT", "LCDPT", "none", "none",
	"none", "none", "none", "none",
}

var physChNames = [16]string{
	"Load", "ExtDisp", "LDT1Disp", "LDT2Disp",
	"none", "none", "none", "none",
	"EffCellP", "VolChange", "none", "none",
	"none", "none", "none", "none",
}

var physUnits = [16]string{
	"N", "mm", "mm", "mm",
	"--", "--", "--", "--",
	"kPa", "mm3", "--", "--",
	"--", "--", "--", "--",
}

var paramNames = [32]string{
	"q", "p'", "sigma'(a)", "sigma'(r)",
	"AxialStrain", "RadialStrain", "VolumetricStrain", "LDT1",
	"LDT2", "LocalAxialStrain", "LDT1LocAxStrain", "LDT2LocAxStrain",
	"none", "none", "none", "none",
	"none", "none", "none", "none",
	"none", "none", "none", "none",
	"CurrentDiameter", "CurrentHeight", "CurrentArea", "CurrentVolume",
	"RefDiameter", "RefHeight", "RefArea", "RefVolume",
}

var paramUnits = [32]string{
	"kPa", "kPa", "kPa", "kPa",
	"%", "%", "%", "mm",
	"mm", "%", "%", "%",
	"--", "--", "--", "--",
	"--", "--", "--", "--",
	"--", "--", "--", "--",
	"mm", "mm", "mm2", "mm3",
	"mm", "mm", "mm2", "mm3",
}

var voltChNames = [8]string{
	"Motor ON/OFF", "Motor UP/DOWN", "Motor Speed", "EP Cell Pressure",
	"EP Axis Pressure", "Torsional ON/OFF", "Torsional CW/CCW", "Torsional Speed",
}

var voltUnits = [8]string{
	"on/off", "on/off", "rpm", "kPa",
	"kPa", "on/off", "dir", "rpm",
}

// ─── Control type / sampling time options ─────────────────────────────────────
var controlTypes = []string{
	"None", "Stop", "MonotonicAxial", "CyclicAxialStress",
	"CyclicAxialStrain", "Creep", "LinearStressPath", "Consolidation",
}

var samplingTimes = []string{
	"0.1 sec", "0.2 sec", "0.5 sec", "1 sec", "2 sec", "5 sec", "10 sec",
}

// Plot axis choices
var plotAxisXChoices = []string{"time", "00", "01", "02", "03", "04", "05", "06", "07"}
var plotAxisYChoices = []string{"00", "01", "02", "03", "04", "05", "06", "07", "08", "09"}
var plotTargetChoices = []string{"Raw", "Phy", "Par"}

// ─── Shared application data ──────────────────────────────────────────────────
// Calibration: per-channel quadratic y = a*x^2 + b*x + c
type CalCoeff struct{ A, B, C float64 }

// Specimen stage: Initial / Present / Before / After consolidation
type SpecimenStage struct {
	Diameter float64 // mm
	Height   float64 // mm
	Area     float64 // mm2 (computed)
	Volume   float64 // mm3 (computed)
	LDT1     float64 // mm
	LDT2     float64 // mm
}

type SpecimenData struct {
	MembraneE  float64 // kPa
	MembraneT  float64 // mm
	CapWeight  float64 // N
	Present    SpecimenStage
	Initial    SpecimenStage
	BeforeCons SpecimenStage
	AfterCons  SpecimenStage
}

// PreConsolidation control parameters
type PreConParams struct {
	TargetQ   float64 // kPa
	QError    float64 // kPa
	MaxSpeed  float64 // rpm
}

// Step Control
type StepCtrl struct {
	StepNo    int
	ControlNo int
	CyclicNo  int
	Args      [16]float64
}

// Env variables (read from / written to os.Environ on Apply)
type EnvVars struct {
	Values [16]float64
	Names  [16]string
}

type AppData struct {
	mu sync.RWMutex

	// Live data
	raw    [16]int16
	phys   [16]float64
	params [32]float64
	volts  [8]float64

	// Connection state
	portStr string
	simMode bool

	// Control state
	controlOn   bool
	savingOn    bool
	saveFile    string
	saveElapsed time.Duration
	controlType string
	sampleTime  string
	stepNo      int
	controlNo   int
	cyclicNo    int

	// Persisted configuration
	cal       [16]CalCoeff
	specimen  SpecimenData
	preCon    PreConParams
	stepCtrl  StepCtrl
	envVars   EnvVars
}

var appData AppData

// ─── Log message channel (worker -> UI thread) ────────────────────────────────
var logCh = make(chan string, 256)

// ─── UI widget references (updated in ticker) ─────────────────────────────────
var (
	rawValLbls    [16]*LabelWidget
	physValLbls   [16]*LabelWidget
	paramValLbls  [32]*LabelWidget
	voltValLbls   [8]*LabelWidget
	portStatusLbl *LabelWidget

	modeLbl        *LabelWidget
	saveElapsedLbl *LabelWidget

	// Current Settings
	ctrlTypeCurLbl *LabelWidget
	sampTimeCurLbl *LabelWidget
	stepNoLbl      *LabelWidget
	ctrlNoLbl      *LabelWidget
	elapsedLbl     *LabelWidget
	cyclicNoLbl    *LabelWidget
	saveFileLbl    *LabelWidget

	btnCtrlOn  *ButtonWidget
	btnCtrlOff *ButtonWidget
	btnSaveOn  *ButtonWidget
	btnSaveOff *ButtonWidget

	comboCtrlType  *TComboboxWidget
	comboSampTime  *TComboboxWidget
	comboXAxisA    *TComboboxWidget
	comboYAxisA    *TComboboxWidget
	comboTargetA   *TComboboxWidget
	comboXAxisB    *TComboboxWidget
	comboYAxisB    *TComboboxWidget
	comboTargetB   *TComboboxWidget

	plotA       *miniChart
	plotB       *miniChart

	logText *TextWidget
)

// ─── Main ─────────────────────────────────────────────────────────────────────
func main() {
	InitializeExtension("eval")

	appData.controlType = "00:None"
	appData.sampleTime = "1 sec"
	appData.saveFile = ""

	// Default calibration: identity (a=0, b=1, c=0) so uncalibrated data still
	// produces the original normalised value until the user dials something in.
	for i := range appData.cal {
		appData.cal[i] = CalCoeff{A: 0, B: 1, C: 0}
	}
	// Default specimen (50mm diameter × 100mm height cylinder)
	appData.specimen.Present.Diameter = 50
	appData.specimen.Present.Height = 100
	appData.specimen.Initial = appData.specimen.Present
	appData.specimen.BeforeCons = appData.specimen.Present
	appData.specimen.AfterCons = appData.specimen.Present
	appData.specimen.MembraneE = 0
	appData.specimen.MembraneT = 0.3
	appData.specimen.CapWeight = 0
	// Default pre-consolidation
	appData.preCon = PreConParams{TargetQ: 0, QError: 10, MaxSpeed: 1000}
	// Default step control
	appData.stepCtrl.StepNo = 0
	appData.stepCtrl.ControlNo = 0
	appData.stepCtrl.CyclicNo = 0
	// Default environmental variable names (mirrors C++ default)
	for i := range appData.envVars.Values {
		appData.envVars.Values[i] = 0
	}
	appData.envVars.Names = [16]string{
		"DA02:Motor Speed   a*(gradient)", "DA02:Motor Speed   b*(intercept)",
		"DA03:EP Cell Pres  a*(gradient)", "DA03:EP Cell Pres  b*(intercept)",
		"DA04:EP Axis Pres  a*(gradient)", "DA04:EP Axis Pres  b*(intercept)",
		"DA07:Tor   Speed   a*(gradient)", "DA07:Tor   Speed   b*(intercept)",
		"Error in Compressive Control of Deviator Stress (kPa)",
		"Error in Extensive Control of Deviator Stress (kPa)",
		"Error in Control of Cell Pressure (kPa)",
		"Error in Control of Axial Strain (%)",
		"Default Specimen Diameter (mm) only apply on start up",
		"Default Specimen Height (mm)  only apply on start up",
		"none", "none",
	}
	// Seed env var values from the C++ defaults (matches calibration/constants)
	appData.envVars.Values[0] = 0.000333333
	appData.envVars.Values[1] = 0
	appData.envVars.Values[2] = 0.001275
	appData.envVars.Values[3] = 0
	appData.envVars.Values[4] = 0.00511
	appData.envVars.Values[5] = 0
	appData.envVars.Values[6] = 0
	appData.envVars.Values[7] = 0
	appData.envVars.Values[8] = 0.5
	appData.envVars.Values[9] = -0.5
	appData.envVars.Values[10] = 0.5
	appData.envVars.Values[11] = 0.05
	appData.envVars.Values[12] = 50
	appData.envVars.Values[13] = 100
	appData.envVars.Values[14] = 0
	appData.envVars.Values[15] = 0

	buildMenu()

	App.WmTitle("DigitShowGo v0.1.0 release [Modbus RTU]")
	WmGeometry(App, "1600x900")
	WmMinSize(App, 1600, 900)
	App.Configure(Padx(0), Pady(0), Background(bgMain))

	buildUI()
	loadConfigsOnStartup()

	go modbusWorker()

	NewTicker(100*time.Millisecond, updateUI)
	App.Wait()
}

// ─── Menu bar ─────────────────────────────────────────────────────────────────
func buildMenu() {
	menubar := Menu()

	mApp := menubar.Menu()
	mApp.AddCommand(Lbl("Calibration Value"), Command(func() {
		openCalibrationDialog()
	}))
	menubar.AddCascade(Lbl("AD Input"), Mnu(mApp))

	mDA := menubar.Menu()
	mDA.AddCommand(Lbl("Voltage Output"), Command(func() {
		openVoltageOutDialog()
	}))
	menubar.AddCascade(Lbl("DA Output"), Mnu(mDA))

	mSP := menubar.Menu()
	mSP.AddCommand(Lbl("Config"), Command(func() {
		openSpecimenDialog()
	}))
	menubar.AddCascade(Lbl("Specimen"), Mnu(mSP))

	mCtrl := menubar.Menu()
	mCtrl.AddCommand(Lbl("Pre-Consolidation"), Command(func() {
		openPreConsolidationDialog()
	}))
	mCtrl.AddCommand(Lbl("Step Control"), Command(func() {
		openStepCtrlDialog()
	}))
	menubar.AddCascade(Lbl("Control"), Mnu(mCtrl))

	mOther := menubar.Menu()
	mOther.AddCommand(Lbl("Version"), Command(func() {
		openVersionDialog()
	}))
	mOther.AddCommand(Lbl("Environmental Variables"), Command(func() {
		openEnvVarDialog()
	}))
	mOther.AddCommand(Lbl("Web Server Info"), Command(func() {
		openWebServerInfoDialog()
	}))
	mOther.AddCommand(Lbl("Open Appdata/Log Folder"), Command(func() {
		openAppDataFolder()
	}))
	mOther.AddCommand(Lbl("Open Temporary Folder"), Command(func() {
		openTempFolder()
	}))
	menubar.AddCascade(Lbl("Other"), Mnu(mOther))

	App.Configure(Mnu(menubar))
}

// ─── UI construction ──────────────────────────────────────────────────────────
func buildUI() {
	bodyFrame := App.Frame(Background(bgMain))
	Pack(bodyFrame, Fill(FILL_BOTH), Expand(true))

	buildBody(bodyFrame)
}

func buildBody(parent *FrameWidget) {
	// Top: data sections (Raw / Physical / Parameter)
	dataFrame := parent.Frame(Background(bgMain))
	Pack(dataFrame, Fill(FILL_X), Side(TOP))

	buildRawSection(dataFrame)
	buildPhysSection(dataFrame)
	buildParamSection(dataFrame)
	buildVoltSection(dataFrame) // Voltage Out is the 4th group under Parameter (matches MFC reference)

	// Bottom row: Settings (left), Plot (right)
	bottomFrame := parent.Frame(Background(bgMain))
	Pack(bottomFrame, Fill(FILL_BOTH), Expand(true), Side(TOP), Pady(2))

	buildBottomRow(bottomFrame)
}

func buildBottomRow(parent *FrameWidget) {
	// Bottom row: Plot (left) | Center spdlog+Mode+Save (middle) | Settings (right)
	plotCol := parent.Frame(Background(bgPanel))
	centerCol := parent.Frame(Background(bgPanel))
	settingsCol := parent.Frame(Background(bgPanel))

	Pack(plotCol, Side(LEFT), Fill(FILL_BOTH), Expand(true), Padx(2))
	Pack(centerCol, Side(LEFT), Fill(FILL_Y), Padx(2))
	Pack(settingsCol, Side(LEFT), Fill(FILL_Y), Padx(2))

	buildPlotPanel(plotCol)
	buildCenterPanel(centerCol)
	buildSettingsPanel(settingsCol)
}

// ─── Section / row helpers ────────────────────────────────────────────────────

// makeGroup creates a labelled group (just a title label + child container).
func makeGroup(parent *FrameWidget, title string) *FrameWidget {
	gr := parent.Frame(Background(bgMain), Padx(0), Pady(0))
	Pack(gr, Fill(FILL_X), Side(TOP), Pady(1))

	if title != "" {
		titleLbl := gr.Label(
			Txt(" "+title),
			Font(HELVETICA, 9, BOLD),
			Foreground(fgAccent),
			Background(bgMain),
			Anchor(W),
			Relief(FLAT),
			Pady(2),
		)
		Pack(titleLbl, Fill(FILL_X))
	}
	return gr
}

// makeDataGrid creates an 8-col-per-row value grid for N rows and returns
// the slice of value labels in left-to-right, top-to-bottom order.  All 8
// cells in a row get equal width via Grid + Uniform so the layout is
// perfectly column-aligned regardless of header text length.
func makeDataGrid(parent *FrameWidget, total int, header func(i int) string, initVal string) []*LabelWidget {
	rows := total / 8
	out := make([]*LabelWidget, 0, total)
	for r := 0; r < rows; r++ {
		rowFr := parent.Frame(Background(bgMain))
		Pack(rowFr, Fill(FILL_X), Side(TOP), Pady(0))
		gridTag := "uGrid" + fmt.Sprint(r)
		// Configure all 8 columns up-front with the same uniform group so the
		// grid splits the available width equally between them.
		for c := 0; c < 8; c++ {
			GridColumnConfigure(rowFr, c, Weight(1), Uniform(gridTag))
		}
		for c := 0; c < 8; c++ {
			idx := r*8 + c
			cell := rowFr.Frame(Background(bgCell), Relief(FLAT), Borderwidth(1))
			Grid(cell, In(rowFr), Row(r), Column(c), Sticky("nsew"))

			hdr := cell.Label(
				Txt(header(idx)),
				Font(HELVETICA, 8),
				Foreground(fgLabel),
				Background(bgCell),
				Anchor(W),
				Padx(3), Pady(1),
			)
			Pack(hdr, Fill(FILL_X))

			v := cell.Label(
				Txt(initVal),
				Font(HELVETICA, 11, BOLD),
				Foreground(fgText),
				Background(bgCell),
				Anchor(E),
				Padx(3), Pady(1),
			)
			Pack(v, Fill(FILL_X))
			out = append(out, v)
		}
	}
	return out
}

// ─── Raw AI section (16 ch) ───────────────────────────────────────────────────
func buildRawSection(parent *FrameWidget) {
	gr := makeGroup(parent, "Raw Value (int16_t: -32768 to +32767)")
	values := makeDataGrid(gr, 16, func(i int) string {
		return fmt.Sprintf("%02d:%s(i16)", i, rawChNames[i])
	}, "0")
	for i, v := range values {
		rawValLbls[i] = v
	}
}

// ─── Physical Value section (16 ch) ───────────────────────────────────────────
func buildPhysSection(parent *FrameWidget) {
	gr := makeGroup(parent, "Physical Value")
	values := makeDataGrid(gr, 16, func(i int) string {
		u := physUnits[i]
		return fmt.Sprintf("%02d:%s(%s)", i, physChNames[i], u)
	}, "0.0000")
	for i, v := range values {
		physValLbls[i] = v
	}
}

// ─── Parameter section (32 ch) ────────────────────────────────────────────────
func buildParamSection(parent *FrameWidget) {
	gr := makeGroup(parent, "Parameter")
	values := makeDataGrid(gr, 32, func(i int) string {
		u := paramUnits[i]
		return fmt.Sprintf("%02d:%s(%s)", i, paramNames[i], u)
	}, "0.0000")
	for i, v := range values {
		paramValLbls[i] = v
	}
}

// ─── Bottom-left: Settings + Step Control + Start/Stop buttons ────────────────
func buildSettingsPanel(parent *FrameWidget) {
	// Port status row (top of settings column)
	portGr := makeGroup(parent, "Port")
	portRow := portGr.Frame(Background(bgPanel))
	Pack(portRow, Fill(FILL_X), Side(TOP))
	statusLbl := portRow.Label(
		Txt("Status"), Font(HELVETICA, 9),
		Foreground(fgText), Background(bgPanel), Width(11), Anchor(W),
	)
	portStatusLbl = portRow.Label(
		Txt("probing..."), Font(HELVETICA, 9, BOLD),
		Foreground(fgOrange), Background(bgPanel), Anchor(W),
	)
	reconnectBtn := portRow.Button(
		Txt("Reconnect"), Font(HELVETICA, 8),
		Width(8), Command(func() { requestReconnect() }),
	)
	Pack(statusLbl, Side(LEFT))
	Pack(portStatusLbl, Side(LEFT), Padx(2))
	Pack(reconnectBtn, Side(LEFT), Padx(4))

	// Basic Settings group
	bsGr := makeGroup(parent, "Basic")
	bs := bsGr.Frame(Background(bgPanel))
	Pack(bs, Fill(FILL_X), Side(TOP))

	// ControlType row
	row1 := bs.Frame(Background(bgPanel))
	Pack(row1, Fill(FILL_X), Side(TOP))
	ctrlTypeLbl := row1.Label(
		Txt("ControlType"), Font(HELVETICA, 9),
		Foreground(fgText), Background(bgPanel), Width(11), Anchor(W),
	)
	comboCtrlType = row1.TCombobox(
		Values(controlTypes),
		Textvariable(appData.controlType),
		Width(14), Font(HELVETICA, 9),
		State("readonly"),
	)
	apply1 := row1.Button(
		Txt("Apply"), Font(HELVETICA, 8),
		Width(6), Command(func() { onApplyCtrlType() }),
	)
	Pack(ctrlTypeLbl, Side(LEFT))
	Pack(comboCtrlType, Side(LEFT))
	Pack(apply1, Side(LEFT), Padx(2))

	// SamplingTime row
	row2 := bs.Frame(Background(bgPanel))
	Pack(row2, Fill(FILL_X), Side(TOP))
	sampTimeLbl := row2.Label(
		Txt("SamplingTime"), Font(HELVETICA, 9),
		Foreground(fgText), Background(bgPanel), Width(11), Anchor(W),
	)
	comboSampTime = row2.TCombobox(
		Values(samplingTimes),
		Textvariable(appData.sampleTime),
		Width(14), Font(HELVETICA, 9),
		State("readonly"),
	)
	apply2 := row2.Button(
		Txt("Apply"), Font(HELVETICA, 8),
		Width(6), Command(func() { onApplySampTime() }),
	)
	Pack(sampTimeLbl, Side(LEFT))
	Pack(comboSampTime, Side(LEFT))
	Pack(apply2, Side(LEFT), Padx(2))

	// Current Settings group
	csGr := makeGroup(parent, "Current")
	cs := csGr.Frame(Background(bgPanel))
	Pack(cs, Fill(FILL_X), Side(TOP))

	mkRow := func(label string, lblRef **LabelWidget, init string) {
		r := cs.Frame(Background(bgPanel))
		Pack(r, Fill(FILL_X), Side(TOP))
		lbl := r.Label(
			Txt(label), Font(HELVETICA, 9),
			Foreground(fgText), Background(bgPanel), Width(11), Anchor(W),
		)
		*lblRef = r.Label(
			Txt(init), Font(HELVETICA, 9, BOLD),
			Foreground(fgGreen), Background(bgPanel), Anchor(W),
		)
		Pack(lbl, Side(LEFT))
		Pack(*lblRef, Side(LEFT), Padx(2))
	}
	mkRow("ControlType", &ctrlTypeCurLbl, "00:None")
	mkRow("SamplingTime", &sampTimeCurLbl, "1")

	// Step Control group
	scGr := makeGroup(parent, "Step Control")
	sc := scGr.Frame(Background(bgPanel))
	Pack(sc, Fill(FILL_X), Side(TOP))

	scRow1 := sc.Frame(Background(bgPanel))
	Pack(scRow1, Fill(FILL_X), Side(TOP))
	stepNoHdr := scRow1.Label(
		Txt("Step No"), Font(HELVETICA, 9),
		Foreground(fgText), Background(bgPanel), Width(11), Anchor(W),
	)
	stepNoLbl = scRow1.Label(
		Txt("0"), Font(HELVETICA, 9, BOLD),
		Foreground(fgGreen), Background(bgPanel), Anchor(W),
	)
	Pack(stepNoHdr, Side(LEFT))
	Pack(stepNoLbl, Side(LEFT), Padx(2))

	scRow2 := sc.Frame(Background(bgPanel))
	Pack(scRow2, Fill(FILL_X), Side(TOP))
	ctrlNoHdr := scRow2.Label(
		Txt("Control No"), Font(HELVETICA, 9),
		Foreground(fgText), Background(bgPanel), Width(11), Anchor(W),
	)
	ctrlNoLbl = scRow2.Label(
		Txt("0"), Font(HELVETICA, 9, BOLD),
		Foreground(fgGreen), Background(bgPanel), Anchor(W),
	)
	Pack(ctrlNoHdr, Side(LEFT))
	Pack(ctrlNoLbl, Side(LEFT), Padx(2))

	scRow3 := sc.Frame(Background(bgPanel))
	Pack(scRow3, Fill(FILL_X), Side(TOP))
	elapsedHdr := scRow3.Label(
		Txt("Elapsed"), Font(HELVETICA, 9),
		Foreground(fgText), Background(bgPanel), Width(11), Anchor(W),
	)
	elapsedLbl = scRow3.Label(
		Txt("0 [s]"), Font(HELVETICA, 9, BOLD),
		Foreground(fgGreen), Background(bgPanel), Anchor(W),
	)
	Pack(elapsedHdr, Side(LEFT))
	Pack(elapsedLbl, Side(LEFT), Padx(2))

	scRow4 := sc.Frame(Background(bgPanel))
	Pack(scRow4, Fill(FILL_X), Side(TOP))
	cyclicNoHdr := scRow4.Label(
		Txt("Cyclic No"), Font(HELVETICA, 9),
		Foreground(fgText), Background(bgPanel), Width(11), Anchor(W),
	)
	cyclicNoLbl = scRow4.Label(
		Txt("0"), Font(HELVETICA, 9, BOLD),
		Foreground(fgGreen), Background(bgPanel), Anchor(W),
	)
	Pack(cyclicNoHdr, Side(LEFT))
	Pack(cyclicNoLbl, Side(LEFT), Padx(2))

	// Start/Stop buttons in 2 rows
	btnGr := makeGroup(parent, "")
	btnFr1 := btnGr.Frame(Background(bgPanel))
	Pack(btnFr1, Fill(FILL_X), Side(TOP), Pady(1))
	btnCtrlOn = btnFr1.Button(
		Txt("Start Control"),
		Font(HELVETICA, 9, BOLD),
		Background(bgBtn), Foreground(fgText),
		Activebackground(bgGreen),
		Width(13), Command(func() { onStartControl() }),
	)
	btnCtrlOff = btnFr1.Button(
		Txt("Stop Control"),
		Font(HELVETICA, 9, BOLD),
		Background(bgBtn), Foreground(fgDim),
		Width(13), Command(func() { onStopControl() }),
	)
	Pack(btnCtrlOn, Side(LEFT), Padx(2))
	Pack(btnCtrlOff, Side(LEFT), Padx(2))

	btnFr2 := btnGr.Frame(Background(bgPanel))
	Pack(btnFr2, Fill(FILL_X), Side(TOP), Pady(1))
	btnSaveOn = btnFr2.Button(
		Txt("Start Saving"),
		Font(HELVETICA, 9, BOLD),
		Background(bgBtn), Foreground(fgText),
		Activebackground(bgGreen),
		Width(13), Command(func() { onStartSaving() }),
	)
	btnSaveOff = btnFr2.Button(
		Txt("Stop Saving"),
		Font(HELVETICA, 9, BOLD),
		Background(bgBtn), Foreground(fgDim),
		Width(13), Command(func() { onStopSaving() }),
	)
	Pack(btnSaveOn, Side(LEFT), Padx(2))
	Pack(btnSaveOff, Side(LEFT), Padx(2))
}

// ─── Center column: spdlog + Mode indicator + Save filename + elapsed ─────────
func buildCenterPanel(parent *FrameWidget) {
	gr := makeGroup(parent, "spdlog (latest 4 lines)")

	logFr := gr.Frame(Background(bgCell), Relief(SUNKEN), Borderwidth(1))
	Pack(logFr, Fill(FILL_BOTH), Expand(true), Pady(1))
	logText = logFr.Text(
		Font("Courier", 8),
		Background(bgCell),
		Foreground(fgText),
		Wrap("word"),
		State("disabled"),
	)
	scr := logFr.Scrollbar(Orient(VERTICAL), Command(func(e *Event) { e.Yview(logText) }))
	logText.Configure(Yscrollcommand(func(e *Event) { e.ScrollSet(scr) }))
	Pack(logText, Side(LEFT), Fill(FILL_BOTH), Expand(true))
	Pack(scr, Side(RIGHT), Fill(FILL_Y))

	// Mode indicator
	modeRow := gr.Frame(Background(bgPanel))
	Pack(modeRow, Fill(FILL_X), Side(TOP), Pady(2))
	modeHdr := modeRow.Label(
		Txt("Mode:"), Font(HELVETICA, 9, BOLD),
		Foreground(fgText), Background(bgPanel), Anchor(W),
	)
	modeLbl = modeRow.Label(
		Txt("None"), Font(HELVETICA, 9, BOLD),
		Foreground(fgGreen), Background(bgPanel), Anchor(W),
	)
	Pack(modeHdr, Side(LEFT))
	Pack(modeLbl, Side(LEFT), Padx(2))

	// Save filename + elapsed
	saveRow := gr.Frame(Background(bgPanel))
	Pack(saveRow, Fill(FILL_X), Side(TOP), Pady(2))
	saveFileHdr := saveRow.Label(
		Txt("Save: Filename"), Font(HELVETICA, 9, BOLD),
		Foreground(fgText), Background(bgPanel), Anchor(W),
	)
	saveFileLbl = saveRow.Label(
		Txt("(none)"), Font(HELVETICA, 9),
		Foreground(fgText), Background(bgCell), Anchor(W),
		Relief(SUNKEN), Borderwidth(1),
	)
	saveElapsedLbl = saveRow.Label(
		Txt("0 [sec]"), Font(HELVETICA, 9, BOLD),
		Foreground(fgOrange), Background(bgPanel), Anchor(E),
	)
	Pack(saveFileHdr, Side(LEFT))
	Pack(saveFileLbl, Side(LEFT), Padx(2), Expand(true), Fill(FILL_X))
	Pack(saveElapsedLbl, Side(LEFT), Padx(2))

	appendLog("DigitShowGo started.")
}

// ─── Voltage Out section (8 ch) - top data group, below Parameter ─────────────
func buildVoltSection(parent *FrameWidget) {
	gr := makeGroup(parent, "Voltage Out")
	values := makeDataGrid(gr, 8, func(i int) string {
		return fmt.Sprintf("%02d:%s(%s)", i, voltChNames[i], voltUnits[i])
	}, "0.0000")
	for i, v := range values {
		voltValLbls[i] = v
	}
}

// ─── Plot panel (right column) with X/Y/Target axis selectors ───────────────
type miniChart struct {
	canvas *CanvasWidget
	data   []float64
	maxLen int
	bg, fg, axis string
}

func newMiniChart(parent *FrameWidget, title, bg, fg, axis string) *miniChart {
	fr := parent.Frame(Background(bgMain), Padx(0), Pady(0))
	Pack(fr, Side(LEFT), Fill(FILL_BOTH), Expand(true), Padx(2))

	if title != "" {
		titleLbl := fr.Label(
			Txt(" "+title),
			Font(HELVETICA, 9, BOLD),
			Foreground(fgAccent),
			Background(bgMain),
			Anchor(W), Pady(2),
		)
		Pack(titleLbl, Fill(FILL_X))
	}

	cvs := fr.Canvas(
		Background(bgCell),
		Highlightthickness(0),
	)
	Pack(cvs, Fill(FILL_BOTH), Expand(true), Padx(0), Pady(0))

	mc := &miniChart{canvas: cvs, maxLen: 200, bg: bg, fg: fg, axis: axis}
	Bind(cvs.Window, "<Configure>", Command(func(e *Event) { mc.redraw() }))
	return mc
}

func (mc *miniChart) push(v float64) {
	mc.data = append(mc.data, v)
	if len(mc.data) > mc.maxLen {
		mc.data = mc.data[len(mc.data)-mc.maxLen:]
	}
	mc.redraw()
}

func (mc *miniChart) redraw() {
	if mc == nil || mc.canvas == nil {
		return
	}
	mc.canvas.Delete("all")
	wf := float64(toInt(mc.canvas.Width()))
	hf := float64(toInt(mc.canvas.Height()))
	w := int(wf)
	h := int(hf)
	if w < 10 || h < 10 {
		return
	}
	mc.canvas.CreateLine(0, h-1, w, h-1, Fill(mc.axis))
	mc.canvas.CreateLine(0, 0, 0, h-1, Fill(mc.axis))
	if len(mc.data) < 2 {
		return
	}
	vMin, vMax := mc.data[0], mc.data[0]
	for _, v := range mc.data {
		if v < vMin {
			vMin = v
		}
		if v > vMax {
			vMax = v
		}
	}
	if vMax-vMin < 1e-9 {
		vMin, vMax = vMin-1, vMax+1
	}
	step := wf / float64(mc.maxLen-1)
	for i := 1; i < len(mc.data); i++ {
		x0 := float64(mc.maxLen-len(mc.data)+i-1) * step
		x1 := float64(mc.maxLen-len(mc.data)+i) * step
		y0 := hf - (mc.data[i-1]-vMin)/(vMax-vMin)*hf
		y1 := hf - (mc.data[i]-vMin)/(vMax-vMin)*hf
		mc.canvas.CreateLine(x0, y0, x1, y1, Fill(mc.fg))
	}
}

func buildPlotPanel(parent *FrameWidget) {
	gr := makeGroup(parent, "Plot")

	// Chart row
	row := gr.Frame(Background(bgMain))
	Pack(row, Fill(FILL_BOTH), Expand(true))

	// Chart A: 3 axis selectors on the left, chart on the right
	frameA := row.Frame(Background(bgMain))
	Pack(frameA, Side(LEFT), Fill(FILL_BOTH), Expand(true), Padx(2))

	ctrlA := frameA.Frame(Background(bgMain))
	Pack(ctrlA, Side(LEFT), Fill(FILL_Y))
	comboXAxisA = ctrlA.TCombobox(Values(plotAxisXChoices), Width(7), State("readonly"))
	comboYAxisA = ctrlA.TCombobox(Values(plotAxisYChoices), Width(7), State("readonly"))
	comboTargetA = ctrlA.TCombobox(Values(plotTargetChoices), Width(7), State("readonly"))
	Pack(comboXAxisA, Side(TOP), Pady(1))
	Pack(comboYAxisA, Side(TOP), Pady(1))
	Pack(comboTargetA, Side(TOP), Pady(1))

	xLblA := ctrlA.Label(Txt("X-axis"), Font(HELVETICA, 8), Foreground(fgLabel), Background(bgMain))
	yLblA := ctrlA.Label(Txt("Y-axis"), Font(HELVETICA, 8), Foreground(fgLabel), Background(bgMain))
	tLblA := ctrlA.Label(Txt("Target"), Font(HELVETICA, 8), Foreground(fgLabel), Background(bgMain))
	Pack(xLblA, Side(TOP))
	Pack(yLblA, Side(TOP))
	Pack(tLblA, Side(TOP))

	plotA = newMiniChart(frameA, "", bgMain, fgGreen, fgDim)

	// Chart B
	frameB := row.Frame(Background(bgMain))
	Pack(frameB, Side(LEFT), Fill(FILL_BOTH), Expand(true), Padx(2))

	ctrlB := frameB.Frame(Background(bgMain))
	Pack(ctrlB, Side(LEFT), Fill(FILL_Y))
	comboXAxisB = ctrlB.TCombobox(Values(plotAxisXChoices), Width(7), State("readonly"))
	comboYAxisB = ctrlB.TCombobox(Values(plotAxisYChoices), Width(7), State("readonly"))
	comboTargetB = ctrlB.TCombobox(Values(plotTargetChoices), Width(7), State("readonly"))
	Pack(comboXAxisB, Side(TOP), Pady(1))
	Pack(comboYAxisB, Side(TOP), Pady(1))
	Pack(comboTargetB, Side(TOP), Pady(1))
	xLblB := ctrlB.Label(Txt("X-axis"), Font(HELVETICA, 8), Foreground(fgLabel), Background(bgMain))
	yLblB := ctrlB.Label(Txt("Y-axis"), Font(HELVETICA, 8), Foreground(fgLabel), Background(bgMain))
	tLblB := ctrlB.Label(Txt("Target"), Font(HELVETICA, 8), Foreground(fgLabel), Background(bgMain))
	Pack(xLblB, Side(TOP))
	Pack(yLblB, Side(TOP))
	Pack(tLblB, Side(TOP))

	plotB = newMiniChart(frameB, "", bgMain, fgOrange, fgDim)
}

// ─── Control button handlers ──────────────────────────────────────────────────
// reconnectCh is a thread-safe flag the worker can observe to drop its port and
// re-probe.  We avoid a Go channel/select here because the worker is on its own
// goroutine and we just want a one-shot hint.
var reconnectRequested atomic.Bool

func requestReconnect() {
	reconnectRequested.Store(true)
	appendLog("[ui] reconnect requested by user")
}

func consumeReconnect() bool {
	return reconnectRequested.Swap(false)
}

func onStartControl() {
	appData.mu.Lock()
	appData.controlOn = true
	appData.mu.Unlock()
	appendLog("[control] Start requested. ControlType=" + appData.controlType)
}

func onStopControl() {
	appData.mu.Lock()
	appData.controlOn = false
	appData.mu.Unlock()
	appendLog("[control] Stop requested.")
}

func onStartSaving() {
	fn := fmt.Sprintf("data_%s.tsv", time.Now().Format("20060102_150405"))
	appData.mu.Lock()
	appData.savingOn = true
	appData.saveFile = fn
	appData.saveElapsed = 0
	appData.mu.Unlock()
	appendLog("[save] Start requested. File=" + fn)
}

func onStopSaving() {
	appData.mu.Lock()
	appData.savingOn = false
	appData.mu.Unlock()
	appendLog("[save] Stop requested.")
}

func onApplyCtrlType() {
	v := appData.controlType
	appendLog("[settings] ControlType -> " + v)
}

func onApplySampTime() {
	v := appData.sampleTime
	appendLog("[settings] SamplingTime -> " + v)
}

// ─── Log helper ───────────────────────────────────────────────────────────────
// appendLog can be called from any goroutine; messages are pushed into a channel
// and rendered by the main-thread ticker.
func appendLog(msg string) {
	select {
	case logCh <- msg:
	default:
		// Channel full - drop oldest by skipping
	}
}

func flushLogs() {
	for {
		select {
		case msg := <-logCh:
			if logText == nil {
				continue
			}
			ts := time.Now().Format("15:04:05.0")
			line := fmt.Sprintf("%s  %s", ts, msg)
			tkeval.EvalErr(fmt.Sprintf("%s configure -state normal", logText))
			tkeval.EvalErr(fmt.Sprintf("%s insert end {%s\n}", logText, line))
			tkeval.EvalErr(fmt.Sprintf("%s see end", logText))
			tkeval.EvalErr(fmt.Sprintf("%s configure -state disabled", logText))
		default:
			return
		}
	}
}

// ─── UI update ticker (runs on main Tk thread) ────────────────────────────────
func updateUI() {
	flushLogs()

	appData.mu.RLock()
	rawSnap := appData.raw
	physSnap := appData.phys
	paramsSnap := appData.params
	voltsSnap := appData.volts
	portStr := appData.portStr
	simMode := appData.simMode
	controlOn := appData.controlOn
	savingOn := appData.savingOn
	saveFile := appData.saveFile
	ctrlType := appData.controlType
	sampleTime := appData.sampleTime
	stepNo := appData.stepNo
	controlNo := appData.controlNo
	cyclicNo := appData.cyclicNo
	appData.mu.RUnlock()

	// Update port status label
	if portStatusLbl != nil {
		portFg := fgOrange
		portTxt := "SIM"
		if !simMode && portStr != "" {
			portFg = fgGreen
			portTxt = portStr
		}
		portStatusLbl.Configure(Txt(portTxt), Foreground(portFg))
	}

	// Update raw + physical value labels
	for i := 0; i < 16; i++ {
		r := rawSnap[i]
		p := physSnap[i]

		absR := r
		if absR < 0 {
			absR = -absR
			if absR < 0 {
				absR = 32767
			}
		}
		rawFg := fgText
		if absR >= 30000 {
			rawFg = fgCrit
		} else if absR >= 25000 {
			rawFg = fgWarn
		}

		rawValLbls[i].Configure(Txt(fmt.Sprintf("%6d", r)), Foreground(rawFg))
		physValLbls[i].Configure(Txt(fmt.Sprintf("%9.4f", p)))
	}

	// Update parameter labels
	for i := 0; i < 32; i++ {
		paramValLbls[i].Configure(Txt(fmt.Sprintf("%9.4f", paramsSnap[i])))
	}

	// Update voltage labels
	for i := 0; i < 8; i++ {
		voltValLbls[i].Configure(Txt(fmt.Sprintf("%9.4f", voltsSnap[i])))
	}

	// Update settings panel
	ctrlTypeCurLbl.Configure(Txt(ctrlType))
	sampTimeCurLbl.Configure(Txt(sampleTime))
	stepNoLbl.Configure(Txt(fmt.Sprintf("%d", stepNo)))
	ctrlNoLbl.Configure(Txt(fmt.Sprintf("%d", controlNo)))
	cyclicNoLbl.Configure(Txt(fmt.Sprintf("%d", cyclicNo)))

	if savingOn {
		appData.mu.Lock()
		appData.saveElapsed += 100 * time.Millisecond
		appData.mu.Unlock()
	}
	appData.mu.RLock()
	elSec := appData.saveElapsed.Seconds()
	appData.mu.RUnlock()
	elapsedLbl.Configure(Txt(fmt.Sprintf("%.1f [s]", elSec)))
	if saveElapsedLbl != nil {
		saveElapsedLbl.Configure(Txt(fmt.Sprintf("%.1f [sec]", elSec)))
	}
	if modeLbl != nil {
		modeTxt := "None"
		if controlOn {
			modeTxt = ctrlType
		}
		modeLbl.Configure(Txt(modeTxt))
	}

	if saveFile != "" {
		saveFileLbl.Configure(Txt(saveFile))
	} else {
		saveFileLbl.Configure(Txt("(none)"))
	}

	// Update button states
	if controlOn {
		btnCtrlOn.Configure(Background(bgGreen), State("disabled"))
		btnCtrlOff.Configure(Background(bgRed), State("normal"))
	} else {
		btnCtrlOn.Configure(Background(bgBtn), State("normal"))
		btnCtrlOff.Configure(Background(bgBtn), State("disabled"))
	}
	if savingOn {
		btnSaveOn.Configure(Background(bgGreen), State("disabled"))
		btnSaveOff.Configure(Background(bgRed), State("normal"))
	} else {
		btnSaveOn.Configure(Background(bgBtn), State("normal"))
		btnSaveOff.Configure(Background(bgBtn), State("disabled"))
	}

	// Push to charts - based on Target (Raw/Phy/Par) and Y-axis selection
	plotA.push(physSnap[0])  // chart A: physical CH00 (load)
	plotB.push(paramsSnap[4]) // chart B: axial strain
}

// ─── Background Modbus / simulation worker ────────────────────────────────────
func modbusWorker() {
	appendLog(fmt.Sprintf("[worker] probing serial ports (preferred: %s)...", preferredPort))
	port, portName := findPort()
	if port == nil {
		appData.mu.Lock()
		appData.simMode = true
		appData.portStr = "SIM"
		appData.mu.Unlock()
		appendLog("[worker] no usable port - simulation mode")
		simLoop()
		return
	}
	defer port.Close()

	appData.mu.Lock()
	appData.simMode = false
	appData.portStr = portName
	appData.mu.Unlock()

	consecutiveErrors := 0
	tick := 0
	for {
		if consumeReconnect() {
			appendLog("[modbus] user-requested reconnect")
			port.Close()
			time.Sleep(200 * time.Millisecond)
			newPort, newName := findPort()
			if newPort == nil {
				appData.mu.Lock()
				appData.simMode = true
				appData.portStr = "SIM (reconnect)"
				appData.mu.Unlock()
				simLoop()
				return
			}
			port = newPort
			portName = newName
			consecutiveErrors = 0
			appData.mu.Lock()
			appData.simMode = false
			appData.portStr = portName
			appData.mu.Unlock()
			continue
		}

		raw, err := readModbus(port)
		if err != nil {
			consecutiveErrors++
			if consecutiveErrors <= 3 || consecutiveErrors%20 == 0 {
				appendLog(fmt.Sprintf("[modbus] read err (n=%d): %v", consecutiveErrors, err))
			}
			if consecutiveErrors >= 50 {
				appendLog("[modbus] too many errors, reconnecting...")
				port.Close()
				time.Sleep(500 * time.Millisecond)
				newPort, newName := findPort()
				if newPort == nil {
					appData.mu.Lock()
					appData.simMode = true
					appData.portStr = "SIM (err)"
					appData.mu.Unlock()
					simLoop()
					return
				}
				port = newPort
				portName = newName
				consecutiveErrors = 0
				appData.mu.Lock()
				appData.simMode = false
				appData.portStr = portName
				appData.mu.Unlock()
				continue
			}
			time.Sleep(100 * time.Millisecond)
			continue
		}
		consecutiveErrors = 0
		phys := computePhys(raw)
		params := computeParams(phys)

		appData.mu.Lock()
		appData.raw = raw
		appData.phys = phys
		appData.params = params
		for i := range appData.volts {
			appData.volts[i] = phys[i%16] * 5.0
		}
		appData.mu.Unlock()

		tick++
		if tick%50 == 0 {
			appendLog(fmt.Sprintf("[modbus] tick=%d ok raws=%d", tick, len(raw)))
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func simLoop() {
	t0 := time.Now()
	tick := 0
	for {
		t := time.Since(t0).Seconds()
		var raw [16]int16
		for i := range raw {
			v := math.Sin(t*0.3+float64(i)*0.45) * float64(26000+i*400)
			raw[i] = int16(v)
		}
		phys := computePhys(raw)
		params := computeParams(phys)

		appData.mu.Lock()
		appData.raw = raw
		appData.phys = phys
		appData.params = params
		for i := range appData.volts {
			appData.volts[i] = phys[i%16] * 5.0
		}
		appData.mu.Unlock()

		tick++
		if tick == 10 {
			appendLog("[sim] simulation stabilised.")
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// ─── Physical / parameter computation ────────────────────────────────────────
// computePhys applies the per-channel quadratic calibration y = a*x^2 + b*x + c
// to the raw int16 sample, where x is the normalised value (-1..+1).
func computePhys(raw [16]int16) [16]float64 {
	appData.mu.RLock()
	cal := appData.cal
	appData.mu.RUnlock()

	var phys [16]float64
	for i, r := range raw {
		x := float64(r) / 32767.0
		c := cal[i]
		phys[i] = c.A*x*x + c.B*x + c.C
	}
	return phys
}

func computeParams(phys [16]float64) [32]float64 {
	var p [32]float64
	p[0] = phys[0] * 100
	p[1] = phys[4] * 100
	p[2] = phys[0] * 100
	p[3] = phys[4] * 100
	p[4] = phys[1] * 100
	p[5] = phys[2] * 100
	p[6] = (phys[1] + 2*phys[2]) * 100
	p[7] = phys[2] * 10
	p[8] = phys[3] * 10
	p[9] = (phys[2] + phys[3]) * 50
	p[10] = phys[2] * 50
	p[11] = phys[3] * 50
	p[24] = 50.0 + phys[1]*5
	p[25] = 100.0 + phys[1]*2
	p[26] = math.Pi * p[24] * p[24] / 4.0
	p[27] = p[24] * p[24] * p[25] * math.Pi / 4.0
	p[28] = 50.0
	p[29] = 100.0
	p[30] = math.Pi * 50.0 * 50.0 / 4.0
	p[31] = 50.0 * 50.0 * 100.0 * math.Pi / 4.0
	return p
}

// ─── Port detection ───────────────────────────────────────────────────────────
// preferredPort is tried first; if it fails, every other detected port is tried
// in order, then we fall back to simulation mode.
const preferredPort = "COM12"

func findPort() (serial.Port, string) {
	ports, err := serial.GetPortsList()
	if err != nil || len(ports) == 0 {
		return nil, ""
	}

	// Build ordered candidate list: preferred first, then the rest sorted.
	candidates := []string{}
	seen := map[string]bool{}
	if containsString(ports, preferredPort) {
		candidates = append(candidates, preferredPort)
		seen[preferredPort] = true
	}
	sorted := append([]string(nil), ports...)
	sort.Strings(sorted)
	for _, p := range sorted {
		if !seen[p] {
			candidates = append(candidates, p)
			seen[p] = true
		}
	}

	mode := &serial.Mode{
		BaudRate: 38400,
		DataBits: 8,
		Parity:   serial.NoParity,
		StopBits: serial.OneStopBit,
	}

	for _, name := range candidates {
		p, err := serial.Open(name, mode)
		if err != nil {
			appendLog(fmt.Sprintf("[port] open %s: %v", name, err))
			continue
		}
		if err := p.SetReadTimeout(1000 * time.Millisecond); err != nil {
			p.Close()
			appendLog(fmt.Sprintf("[port] set timeout %s: %v", name, err))
			continue
		}
		appendLog(fmt.Sprintf("[port] opened %s @ 38400 8N1", name))
		return p, name
	}
	return nil, ""
}

func containsString(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

// ─── Modbus RTU FC04 read ─────────────────────────────────────────────────────
const (
	modbusSlaveID = 1
	modbusStart   = 0
	modbusCount   = 16
)

func readModbus(port serial.Port) ([16]int16, error) {
	req := [8]byte{modbusSlaveID, 0x04, 0, modbusStart, 0, modbusCount}
	crc := crc16(req[:6])
	req[6] = byte(crc)
	req[7] = byte(crc >> 8)

	if _, err := port.Write(req[:]); err != nil {
		return [16]int16{}, fmt.Errorf("write: %w", err)
	}

	expected := 5 + modbusCount*2
	buf := make([]byte, expected)
	n, err := readFull(port, buf)
	if err != nil || n != expected {
		return [16]int16{}, fmt.Errorf("read: got %d/%d bytes, err=%v", n, expected, err)
	}
	if buf[0] != modbusSlaveID || buf[1] != 0x04 || int(buf[2]) != modbusCount*2 {
		return [16]int16{}, fmt.Errorf("invalid response header: %02x %02x %02x", buf[0], buf[1], buf[2])
	}
	rxCRC := uint16(buf[expected-2]) | uint16(buf[expected-1])<<8
	calcCRC := crc16(buf[:expected-2])
	if rxCRC != calcCRC {
		return [16]int16{}, fmt.Errorf("CRC mismatch: got %04x, calc %04x", rxCRC, calcCRC)
	}
	var raw [16]int16
	for i := 0; i < modbusCount; i++ {
		raw[i] = int16(binary.BigEndian.Uint16(buf[3+i*2 : 5+i*2]))
	}
	return raw, nil
}

func readFull(port serial.Port, buf []byte) (int, error) {
	total := 0
	for total < len(buf) {
		n, err := port.Read(buf[total:])
		total += n
		if err != nil {
			return total, err
		}
	}
	return total, nil
}

// ─── CRC16 (Modbus, poly 0xA001) ──────────────────────────────────────────────
func crc16(data []byte) uint16 {
	crc := uint16(0xFFFF)
	for _, b := range data {
		crc ^= uint16(b)
		for i := 0; i < 8; i++ {
			if crc&1 != 0 {
				crc = (crc >> 1) ^ 0xA001
			} else {
				crc >>= 1
			}
		}
	}
	return crc
}
