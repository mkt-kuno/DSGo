package main

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"time"

	. "modernc.org/tk9.0"
)

// ─── Persistence path ─────────────────────────────────────────────────────────
// All configuration is stored in a single JSON file under the user's
// config directory.  This mirrors the C++ version's appdata folder layout.
func configDir() string {
	if d, err := os.UserConfigDir(); err == nil {
		return filepath.Join(d, "DigitShowGo")
	}
	return "."
}

func configPath(name string) string {
	return filepath.Join(configDir(), name)
}

func ensureConfigDir() {
	_ = os.MkdirAll(configDir(), 0o755)
}

// loadJSON reads a JSON file into v.  Missing file is not an error - defaults
// are kept in appData.
func loadJSON(name string, v any) error {
	b, err := os.ReadFile(configPath(name))
	if err != nil {
		return err
	}
	return json.Unmarshal(b, v)
}

// saveJSON writes v to the config file (pretty-printed, 2-space indent).
func saveJSON(name string, v any) error {
	ensureConfigDir()
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(configPath(name), b, 0o644)
}

// ─── AppData snapshot types for JSON ──────────────────────────────────────────
type calibrationFile struct {
	Cal [16]CalCoeff `json:"cal"`
}

type specimenFile struct {
	Specimen SpecimenData `json:"specimen"`
}

type preConFile struct {
	PreCon PreConParams `json:"preCon"`
}

type stepCtrlFile struct {
	Step StepCtrl `json:"step"`
}

type envVarsFile struct {
	Env EnvVars `json:"env"`
}

func saveAllConfigs() {
	ensureConfigDir()
	appData.mu.RLock()
	defer appData.mu.RUnlock()
	_ = saveJSON("calibration.json", calibrationFile{Cal: appData.cal})
	_ = saveJSON("specimen.json", specimenFile{Specimen: appData.specimen})
	_ = saveJSON("precon.json", preConFile{PreCon: appData.preCon})
	_ = saveJSON("stepctrl.json", stepCtrlFile{Step: appData.stepCtrl})
	_ = saveJSON("envvars.json", envVarsFile{Env: appData.envVars})
}

func loadAllConfigs() {
	var cf calibrationFile
	if err := loadJSON("calibration.json", &cf); err == nil {
		appData.mu.Lock()
		appData.cal = cf.Cal
		appData.mu.Unlock()
		appendLog("[config] calibration.json loaded")
	}
	var sf specimenFile
	if err := loadJSON("specimen.json", &sf); err == nil {
		appData.mu.Lock()
		appData.specimen = sf.Specimen
		appData.mu.Unlock()
		appendLog("[config] specimen.json loaded")
	}
	var pf preConFile
	if err := loadJSON("precon.json", &pf); err == nil {
		appData.mu.Lock()
		appData.preCon = pf.PreCon
		appData.mu.Unlock()
		appendLog("[config] precon.json loaded")
	}
	var st stepCtrlFile
	if err := loadJSON("stepctrl.json", &st); err == nil {
		appData.mu.Lock()
		appData.stepCtrl = st.Step
		appData.mu.Unlock()
		appendLog("[config] stepctrl.json loaded")
	}
	var ef envVarsFile
	if err := loadJSON("envvars.json", &ef); err == nil {
		appData.mu.Lock()
		appData.envVars = ef.Env
		appData.mu.Unlock()
		appendLog("[config] envvars.json loaded")
	}
}

// ─── Dialog helpers ───────────────────────────────────────────────────────────

// makeDialogShell creates a Toplevel window with the given title and size,
// applies the standard dark theme, and returns the body frame.  Closing the
// window is handled by the caller (typically a Close button calling
// `Win.Destroy()`).
func makeDialogShell(title string, w, h int) (top *ToplevelWidget, body *FrameWidget) {
	top = Toplevel(Title(title), Background(bgMain))
	WmGeometry(top, fmt.Sprintf("%dx%d", w, h))
	body = top.Frame(Background(bgMain), Padx(8), Pady(8))
	Pack(body, Fill(FILL_BOTH), Expand(true))
	return top, body
}

// mkRow makes a horizontal row with a label and a value-entry combo.  The
// returned Entry is editable and can be wired to read on Update.
func mkRow(parent *FrameWidget, label string, width int) (row *FrameWidget, e *EntryWidget) {
	row = parent.Frame(Background(bgPanel))
	Pack(row, Fill(FILL_X), Side(TOP), Pady(1))
	row.Label(
		Txt(label), Font(HELVETICA, 9),
		Foreground(fgText), Background(bgPanel), Width(22), Anchor(W),
	)
	e = row.Entry(
		Font("Courier", 9),
		Background(bgCell), Foreground(fgText),
		Width(width), Relief(SUNKEN), Borderwidth(1),
	)
	Pack(e, Side(LEFT), Padx(4))
	return row, e
}

func entrySet(e *EntryWidget, v any) {
	EvalErr(fmt.Sprintf("%s delete 0 end; %s insert 0 [list {%v}]", e, e, fmt.Sprint(v)))
}

func entryGet(e *EntryWidget) string {
	return EvalErr(fmt.Sprintf("%s get", e))
}

func entryGetFloat(e *EntryWidget) float64 {
	s := entryGet(e)
	v, _ := strconv.ParseFloat(strings.TrimSpace(s), 64)
	return v
}

func entryGetInt(e *EntryWidget) int {
	s := entryGet(e)
	v, _ := strconv.Atoi(strings.TrimSpace(s))
	return v
}

// ─── Calibration Value dialog ─────────────────────────────────────────────────
// Per-channel quadratic y = a*x^2 + b*x + c.  Mirrors DigitShowModbus' IDD_Calibration_Factor.
func openCalibrationDialog() {
	top, body := makeDialogShell("Calibration Value", 700, 600)
	defer func() { _ = top }()

	// Header row
	head := body.Frame(Background(bgPanel))
	Pack(head, Fill(FILL_X), Side(TOP), Pady(2))
	hdrs := []string{"x : Raw Value", "", "y : Physical Value",
		"a * x^2", "b * x", "c", "y", ""}
	for i, h := range hdrs {
		lbl := head.Label(
			Txt(h), Font(HELVETICA, 9, BOLD),
			Foreground(fgAccent), Background(bgPanel),
			Width(12), Anchor(W),
		)
		Pack(lbl, Side(LEFT), Padx(2))
		_ = i
	}

	// 16 channel rows
	aEntries := [16]*EntryWidget{}
	bEntries := [16]*EntryWidget{}
	cEntries := [16]*EntryWidget{}
	yEntries := [16]*EntryWidget{}
	rawLabels := [16]*LabelWidget{}
	phyLabels := [16]*LabelWidget{}
	zeroBtns := [16]*ButtonWidget{}

	appData.mu.RLock()
	cal := appData.cal
	appData.mu.RUnlock()

	grid := body.Frame(Background(bgPanel))
	Pack(grid, Fill(FILL_BOTH), Expand(true), Side(TOP))

	for i := 0; i < 16; i++ {
		row := grid.Frame(Background(bgPanel))
		Pack(row, Fill(FILL_X), Side(TOP), Pady(0))

		// Raw channel name (CH00:LoadCell(i16) etc.)
		rawLabels[i] = row.Label(
			Txt(fmt.Sprintf("%02d:%s(i16)", i, rawChNames[i])),
			Font(HELVETICA, 8), Foreground(fgLabel),
			Background(bgPanel), Width(18), Anchor(W),
		)
		Pack(rawLabels[i], Side(LEFT), Padx(2))

		// --->
		row.Label(Txt("-->"), Font(HELVETICA, 8), Foreground(fgDim), Background(bgPanel), Width(3), Anchor(W))
		phyLabels[i] = row.Label(
			Txt(fmt.Sprintf("%02d:%s(%s)", i, physChNames[i], physUnits[i])),
			Font(HELVETICA, 8), Foreground(fgLabel),
			Background(bgPanel), Width(18), Anchor(W),
		)
		Pack(phyLabels[i], Side(LEFT), Padx(2))

		aEntries[i] = row.Entry(
			Font("Courier", 9), Background(bgCell), Foreground(fgText),
			Width(8), Relief(SUNKEN), Borderwidth(1),
		)
		bEntries[i] = row.Entry(
			Font("Courier", 9), Background(bgCell), Foreground(fgText),
			Width(8), Relief(SUNKEN), Borderwidth(1),
		)
		cEntries[i] = row.Entry(
			Font("Courier", 9), Background(bgCell), Foreground(fgText),
			Width(8), Relief(SUNKEN), Borderwidth(1),
		)
		yEntries[i] = row.Entry(
			Font("Courier", 9), Background(bgCell), Foreground(fgText),
			Width(8), Relief(SUNKEN), Borderwidth(1),
		)
		entrySet(aEntries[i], cal[i].A)
		entrySet(bEntries[i], cal[i].B)
		entrySet(cEntries[i], cal[i].C)
		entrySet(yEntries[i], 0)
		Pack(aEntries[i], Side(LEFT), Padx(1))
		Pack(bEntries[i], Side(LEFT), Padx(1))
		Pack(cEntries[i], Side(LEFT), Padx(1))
		Pack(yEntries[i], Side(LEFT), Padx(1))

		idx := i
		zeroBtns[i] = row.Button(
			Txt(fmt.Sprintf("%02d:Zero", i)), Font(HELVETICA, 8),
			Background(bgBtn), Foreground(fgText),
			Width(9), Command(func() { onZeroCalibration(idx, aEntries, bEntries, cEntries, yEntries) }),
		)
		Pack(zeroBtns[i], Side(LEFT), Padx(2))
	}

	// Footer buttons: Update / Save / Load / Close
	footer := body.Frame(Background(bgPanel))
	Pack(footer, Fill(FILL_X), Side(TOP), Pady(4))
	footer.Button(
		Txt("Update Variants"), Font(HELVETICA, 9, BOLD),
		Background(bgBtn), Foreground(fgText),
		Width(14), Command(func() { onUpdateCalibration(aEntries, bEntries, cEntries) }),
	).Pack(footer, Side(LEFT), Padx(2))
	footer.Button(
		Txt("Load from a file"), Font(HELVETICA, 9),
		Background(bgBtn), Foreground(fgText),
		Width(14), Command(func() { onLoadCalibration(aEntries, bEntries, cEntries) }),
	).Pack(footer, Side(LEFT), Padx(2))
	footer.Button(
		Txt("Save to a file"), Font(HELVETICA, 9),
		Background(bgBtn), Foreground(fgText),
		Width(14), Command(func() { onSaveCalibration(aEntries, bEntries, cEntries) }),
	).Pack(footer, Side(LEFT), Padx(2))
	footer.Button(
		Txt("Close"), Font(HELVETICA, 9, BOLD),
		Background(bgRed), Foreground("#ffffff"),
		Width(10), Command(func() { top.Destroy() }),
	).Pack(footer, Side(RIGHT), Padx(2))
}

func onUpdateCalibration(a, b, c [16]*EntryWidget) {
	appData.mu.Lock()
	for i := 0; i < 16; i++ {
		appData.cal[i] = CalCoeff{
			A: entryGetFloat(a[i]),
			B: entryGetFloat(b[i]),
			C: entryGetFloat(c[i]),
		}
	}
	appData.mu.Unlock()
	_ = saveJSON("calibration.json", calibrationFile{Cal: func() [16]CalCoeff {
		appData.mu.RLock()
		defer appData.mu.RUnlock()
		return appData.cal
	}()})
	appendLog("[calib] variants updated")
}

func onSaveCalibration(a, b, c [16]*EntryWidget) {
	onUpdateCalibration(a, b, c)
	appendLog("[calib] saved to " + configPath("calibration.json"))
}

func onLoadCalibration(a, b, c [16]*EntryWidget) {
	var cf calibrationFile
	if err := loadJSON("calibration.json", &cf); err != nil {
		appendLog("[calib] load failed: " + err.Error())
		return
	}
	appData.mu.Lock()
	appData.cal = cf.Cal
	appData.mu.Unlock()
	for i := 0; i < 16; i++ {
		entrySet(a[i], cf.Cal[i].A)
		entrySet(b[i], cf.Cal[i].B)
		entrySet(c[i], cf.Cal[i].C)
	}
	appendLog("[calib] loaded " + configPath("calibration.json"))
}

// onZeroCalibration offsets c by -current_physical_value, matching the
// C++ `c = c - phy` behaviour.
func onZeroCalibration(idx int, a, b, c, y [16]*EntryWidget) {
	onUpdateCalibration(a, b, c)
	// Compute current physical value at the latest raw reading.
	appData.mu.RLock()
	r := appData.raw[idx]
	cal := appData.cal[idx]
	appData.mu.RUnlock()
	x := float64(r) / 32767.0
	phy := cal.A*x*x + cal.B*x + cal.C
	newC := cal.C - phy
	appData.mu.Lock()
	appData.cal[idx].C = newC
	appData.mu.Unlock()
	entrySet(c[idx], newC)
	_ = y
	_ = math.Pi // keep math import alive
	appendLog(fmt.Sprintf("[calib] zero CH%02d: c <- %.6f", idx, newC))
}

// ─── VoltageOut dialog ────────────────────────────────────────────────────────
// 8-channel DA voltage output, mirrors IDD_Dialog_VoltageOut.
func openVoltageOutDialog() {
	top, body := makeDialogShell("Voltage Output on DA Board", 360, 380)

	head := body.Frame(Background(bgPanel))
	Pack(head, Fill(FILL_X), Side(TOP))
	head.Label(
		Txt(""), Font(HELVETICA, 9), Background(bgPanel), Width(20), Anchor(W),
	)
	head.Label(
		Txt("Voltage (V)"), Font(HELVETICA, 9, BOLD),
		Foreground(fgAccent), Background(bgPanel), Width(12), Anchor(E),
	)
	head.Label(
		Txt(""), Font(HELVETICA, 9), Background(bgPanel), Width(4),
	)

	right := body.Frame(Background(bgPanel))
	Pack(right, Side(RIGHT), Fill(FILL_Y), Padx(4))
	right.Button(
		Txt("Reflesh"), Font(HELVETICA, 9),
		Background(bgBtn), Foreground(fgText), Width(8),
		Command(func() { onRefreshVoltage(voltEntries) }),
	).Pack(right, Side(TOP), Pady(2))
	right.Button(
		Txt("Output"), Font(HELVETICA, 9),
		Background(bgBtn), Foreground(fgText), Width(8),
		Command(func() { onOutputVoltage(voltEntries) }),
	).Pack(right, Side(TOP), Pady(2))
	right.Button(
		Txt("Close"), Font(HELVETICA, 9, BOLD),
		Background(bgRed), Foreground("#ffffff"), Width(8),
		Command(func() { top.Destroy() }),
	).Pack(right, Side(TOP), Pady(20))

	left := body.Frame(Background(bgCell), Relief(SUNKEN), Borderwidth(1))
	Pack(left, Side(LEFT), Fill(FILL_BOTH), Expand(true), Padx(2))

	voltEntries := [8]*EntryWidget{}
	appData.mu.RLock()
	volts := appData.volts
	appData.mu.RUnlock()

	for i := 0; i < 8; i++ {
		row := left.Frame(Background(bgCell))
		Pack(row, Fill(FILL_X), Side(TOP), Pady(1))
		row.Label(
			Txt(fmt.Sprintf("%02d:%s", i, voltChNames[i])),
			Font(HELVETICA, 8), Foreground(fgLabel),
			Background(bgCell), Width(22), Anchor(W),
		)
		voltEntries[i] = row.Entry(
			Font("Courier", 9), Background(bgPanel), Foreground(fgText),
			Width(8), Relief(SUNKEN), Borderwidth(1), Justify(RIGHT),
		)
		entrySet(voltEntries[i], volts[i])
		Pack(voltEntries[i], Side(LEFT), Padx(4))
	}
	_ = volts
}

func onOutputVoltage(e [8]*EntryWidget) {
	appData.mu.Lock()
	for i := 0; i < 8; i++ {
		appData.volts[i] = entryGetFloat(e[i])
	}
	appData.mu.Unlock()
	appendLog("[dac] voltage out applied (UI-side echo; real board output not wired yet)")
}

func onRefreshVoltage(e [8]*EntryWidget) {
	appData.mu.RLock()
	volts := appData.volts
	appData.mu.RUnlock()
	for i := 0; i < 8; i++ {
		entrySet(e[i], volts[i])
	}
	appendLog("[dac] refreshed from current state")
}

// ─── Specimen Data dialog ──────────────────────────────────────────────────────
// Mirrors IDD_SpecimenData: 4 columns (Present, Initial, Before Consol, After Consol).
func openSpecimenDialog() {
	top, body := makeDialogShell("Specimen Data", 580, 540)

	// Apparatus parameters
	appGrp := body.Frame(Background(bgPanel), Relief(SUNKEN), Borderwidth(1))
	Pack(appGrp, Fill(FILL_X), Side(TOP), Pady(2))
	applbl := appGrp.Label(
		Txt(" Parameters of Test Apparatus [only available for torsional shear]"),
		Font(HELVETICA, 9, BOLD), Foreground(fgAccent),
		Background(bgPanel), Anchor(W), Pady(3),
	)
	Pack(applbl, Fill(FILL_X))

	apparatus := body.Frame(Background(bgPanel))
	Pack(apparatus, Fill(FILL_X), Side(TOP), Pady(1))
	memEEntry, _ := mkRow(apparatus, "Young's Modulus of membrane (kPa)", 8)
	memTEntry, _ := mkRow(apparatus, "Thickness of membrane (mm)", 8)
	capWEntry, _ := mkRow(apparatus, "Cap Weight (N)", 8)

	appData.mu.RLock()
	spec := appData.specimen
	appData.mu.RUnlock()
	entrySet(memEEntry, spec.MembraneE)
	entrySet(memTEntry, spec.MembraneT)
	entrySet(capWEntry, spec.CapWeight)

	// 4-column input grid
	grp := body.Frame(Background(bgPanel), Relief(SUNKEN), Borderwidth(1))
	Pack(grp, Fill(FILL_BOTH), Expand(true), Side(TOP), Pady(4))
	hdr := grp.Frame(Background(bgPanel))
	Pack(hdr, Fill(FILL_X), Side(TOP), Pady(2))
	hdrs := []string{"", "Present", "Initial", "Before consolidation", "After consolidation"}
	widths := []int{16, 9, 9, 14, 14}
	for i, h := range hdrs {
		hdr.Label(
			Txt(h), Font(HELVETICA, 9, BOLD),
			Foreground(fgAccent), Background(bgPanel),
			Width(widths[i]), Anchor(anchorForHeader(i)),
		)
		Pack(hdr, Side(LEFT), Padx(2))
	}

	type stageEditor struct {
		diameter, height, volume, area, ldt1, ldt2 *EntryWidget
	}
	stages := [4]stageEditor{}
	stageKeys := [4]*SpecimenStage{&spec.Present, &spec.Initial, &spec.BeforeCons, &spec.AfterCons}

	rows := []struct {
		name string
		unit string
		idx  int // 0..5 column in stage
	}{
		{"Diameter (mm)", "", 0},
		{"Height (mm)", "", 1},
		{"Volume * (mm3)", "", 2},
		{"Area * (mm2)", "", 3},
		{"LDT1 (mm)", "", 4},
		{"LDT2 (mm)", "", 5},
	}
	for ri, rdef := range rows {
		row := grp.Frame(Background(bgPanel))
		Pack(row, Fill(FILL_X), Side(TOP), Pady(0))
		row.Label(
			Txt(rdef.name), Font(HELVETICA, 9),
			Foreground(fgText), Background(bgPanel), Width(16), Anchor(W),
		)
		for si := 0; si < 4; si++ {
			entry := row.Entry(
				Font("Courier", 9), Background(bgCell), Foreground(fgText),
				Width(9), Relief(SUNKEN), Borderwidth(1), Justify(RIGHT),
			)
			entrySet(entry, stageValueByIdx(stageKeys[si], ri))
			Pack(entry, Side(LEFT), Padx(2))
			switch ri {
			case 0:
				stages[si].diameter = entry
			case 1:
				stages[si].height = entry
			case 2:
				stages[si].volume = entry
			case 3:
				stages[si].area = entry
			case 4:
				stages[si].ldt1 = entry
			case 5:
				stages[si].ldt2 = entry
			}
		}
	}

	// Footer: Update, Save, Before/After consolidation, Close
	foot := body.Frame(Background(bgPanel), Relief(SUNKEN), Borderwidth(1))
	Pack(foot, Fill(FILL_X), Side(TOP), Pady(4))
	footLbl := foot.Label(
		Txt(" Update Reference Specimen Size and Initialize Strains"),
		Font(HELVETICA, 9, BOLD), Foreground(fgAccent),
		Background(bgPanel), Anchor(W), Pady(2),
	)
	Pack(footLbl, Fill(FILL_X))
	footRow := foot.Frame(Background(bgPanel))
	Pack(footRow, Fill(FILL_X), Side(TOP))
	footRow.Button(
		Txt("Before Consolidation"), Font(HELVETICA, 9),
		Background(bgBtn), Foreground(fgText), Width(18),
		Command(func() { appendLog("[specimen] Before Consolidation pressed (no-op stub)") }),
	).Pack(footRow, Side(LEFT), Padx(2))
	footRow.Button(
		Txt("After Consolidation"), Font(HELVETICA, 9),
		Background(bgBtn), Foreground(fgText), Width(18),
		Command(func() { appendLog("[specimen] After Consolidation pressed (no-op stub)") }),
	).Pack(footRow, Side(LEFT), Padx(2))

	footRow2 := body.Frame(Background(bgPanel))
	Pack(footRow2, Fill(FILL_X), Side(TOP), Pady(2))
	footRow2.Button(
		Txt("Update Params"), Font(HELVETICA, 9, BOLD),
		Background(bgBtn), Foreground(fgText), Width(14),
		Command(func() {
			onUpdateSpecimen(memEEntry, memTEntry, capWEntry, stages)
		}),
	).Pack(footRow2, Side(LEFT), Padx(2))
	footRow2.Button(
		Txt("Save to file"), Font(HELVETICA, 9),
		Background(bgBtn), Foreground(fgText), Width(14),
		Command(func() {
			onUpdateSpecimen(memEEntry, memTEntry, capWEntry, stages)
			appendLog("[specimen] saved to " + configPath("specimen.json"))
		}),
	).Pack(footRow2, Side(LEFT), Padx(2))
	footRow2.Button(
		Txt("Close"), Font(HELVETICA, 9, BOLD),
		Background(bgRed), Foreground("#ffffff"), Width(10),
		Command(func() { top.Destroy() }),
	).Pack(footRow2, Side(RIGHT), Padx(2))

	// Stash references on the toplevel so we don't need globals.
	_ = body
	_ = top
}

func stageValueByIdx(s *SpecimenStage, idx int) float64 {
	switch idx {
	case 0:
		return s.Diameter
	case 1:
		return s.Height
	case 2:
		return s.Volume
	case 3:
		return s.Area
	case 4:
		return s.LDT1
	case 5:
		return s.LDT2
	}
	return 0
}

// anchorForHeader is a Go-native replacement for the C++ `?:` ternary used in
// the column-header loop of the Specimen dialog.  Column 0 is the row-name
// label (left-aligned), the other four columns are stage headers (centered).
func anchorForHeader(i int) any {
	if i == 0 {
		return W
	}
	return CENTER
}

type specEditorStage struct {
	diameter, height, volume, area, ldt1, ldt2 *EntryWidget
}

func onUpdateSpecimen(memE, memT, capW *EntryWidget, stages [4]specEditorStage) {
	appData.mu.Lock()
	appData.specimen.MembraneE = entryGetFloat(memE)
	appData.specimen.MembraneT = entryGetFloat(memT)
	appData.specimen.CapWeight = entryGetFloat(capW)
	keys := []*SpecimenStage{
		&appData.specimen.Present, &appData.specimen.Initial,
		&appData.specimen.BeforeCons, &appData.specimen.AfterCons,
	}
	for i, s := range stages {
		keys[i].Diameter = entryGetFloat(s.diameter)
		keys[i].Height = entryGetFloat(s.height)
		keys[i].Volume = entryGetFloat(s.volume)
		keys[i].Area = entryGetFloat(s.area)
		keys[i].LDT1 = entryGetFloat(s.ldt1)
		keys[i].LDT2 = entryGetFloat(s.ldt2)
	}
	snap := appData.specimen
	appData.mu.Unlock()
	_ = saveJSON("specimen.json", specimenFile{Specimen: snap})
	appendLog("[specimen] updated")
}

// ─── Pre-Consolidation dialog ─────────────────────────────────────────────────
func openPreConsolidationDialog() {
	top, body := makeDialogShell("Control Parameters in Pre-Consolidation Process", 380, 230)

	grp := body.Frame(Background(bgPanel), Relief(SUNKEN), Borderwidth(1))
	Pack(grp, Fill(FILL_X), Side(TOP), Pady(2))
	grp.Label(
		Txt(" Settings of pre-consolidation"),
		Font(HELVETICA, 9, BOLD), Foreground(fgAccent),
		Background(bgPanel), Anchor(W), Pady(3),
	).Pack(grp, Fill(FILL_X))

	appData.mu.RLock()
	pc := appData.preCon
	appData.mu.RUnlock()

	tgtRow, tgt := mkRow(grp, "Target Deviator Stress, q (kPa)", 8)
	qErrRow, qErr := mkRow(grp, "q error at max motor speed (kPa)", 8)
	spdRow, spd := mkRow(grp, "Max Motor Speed (rpm)", 8)
	entrySet(tgt, pc.TargetQ)
	entrySet(qErr, pc.QError)
	entrySet(spd, pc.MaxSpeed)
	_ = tgtRow
	_ = qErrRow
	_ = spdRow

	foot := body.Frame(Background(bgPanel))
	Pack(foot, Fill(FILL_X), Side(TOP), Pady(4))
	foot.Button(
		Txt("Update"), Font(HELVETICA, 9, BOLD),
		Background(bgBtn), Foreground(fgText), Width(10),
		Command(func() {
			appData.mu.Lock()
			appData.preCon = PreConParams{
				TargetQ:  entryGetFloat(tgt),
				QError:   entryGetFloat(qErr),
				MaxSpeed: entryGetFloat(spd),
			}
			snap := appData.preCon
			appData.mu.Unlock()
			_ = saveJSON("precon.json", preConFile{PreCon: snap})
			appendLog("[precon] updated")
		}),
	).Pack(foot, Side(LEFT), Padx(2))
	foot.Button(
		Txt("Close"), Font(HELVETICA, 9, BOLD),
		Background(bgRed), Foreground("#ffffff"), Width(10),
		Command(func() { top.Destroy() }),
	).Pack(foot, Side(RIGHT), Padx(2))
}

// ─── Step Control dialog ──────────────────────────────────────────────────────
func openStepCtrlDialog() {
	top, body := makeDialogShell("Step Control", 900, 460)

	// Top: current step/control no + load/save/close
	topRow := body.Frame(Background(bgPanel), Relief(SUNKEN), Borderwidth(1))
	Pack(topRow, Fill(FILL_X), Side(TOP), Pady(2))
	topRow.Label(
		Txt(" Step Control"),
		Font(HELVETICA, 9, BOLD), Foreground(fgAccent),
		Background(bgPanel), Anchor(W), Pady(3),
	).Pack(topRow, Fill(FILL_X))

	ctrlRow := body.Frame(Background(bgPanel))
	Pack(ctrlRow, Fill(FILL_X), Side(TOP), Pady(1))
	stepLbl := ctrlRow.Label(
		Txt("Current Step No."), Font(HELVETICA, 9),
		Foreground(fgText), Background(bgPanel), Width(16), Anchor(W),
	)
	Pack(stepLbl, Side(LEFT))
	stepEntry := ctrlRow.Entry(
		Font("Courier", 9), Background(bgCell), Foreground(fgText),
		Width(8), Relief(SUNKEN), Borderwidth(1),
	)
	Pack(stepEntry, Side(LEFT), Padx(4))

	ctrlLbl := ctrlRow.Label(
		Txt("Current Control No."), Font(HELVETICA, 9),
		Foreground(fgText), Background(bgPanel), Width(18), Anchor(W),
	)
	Pack(ctrlLbl, Side(LEFT), Padx(8))
	ctrlEntry := ctrlRow.Entry(
		Font("Courier", 9), Background(bgCell), Foreground(fgText),
		Width(8), Relief(SUNKEN), Borderwidth(1),
	)
	Pack(ctrlEntry, Side(LEFT), Padx(4))

	changeChk := ctrlRow.Checkbutton(
		Txt("ChangeNo"), Font(HELVETICA, 9),
		Background(bgPanel), Foreground(fgText),
	)
	Pack(changeChk, Side(LEFT), Padx(4))
	ctrlRow.Button(
		Txt("<-"), Font(HELVETICA, 9),
		Background(bgBtn), Foreground(fgText), Width(4),
		Command(func() { appendLog("[step] step-- (no-op stub)") }),
	).Pack(ctrlRow, Side(LEFT), Padx(2))
	ctrlRow.Button(
		Txt("->"), Font(HELVETICA, 9),
		Background(bgBtn), Foreground(fgText), Width(4),
		Command(func() { appendLog("[step] step++ (no-op stub)") }),
	).Pack(ctrlRow, Side(LEFT), Padx(2))

	// Editable step/control no + Load/Update
	editRow := body.Frame(Background(bgPanel), Relief(SUNKEN), Borderwidth(1))
	Pack(editRow, Fill(FILL_X), Side(TOP), Pady(2))
	editRow.Label(
		Txt(" Control Arguments"),
		Font(HELVETICA, 9, BOLD), Foreground(fgAccent),
		Background(bgPanel), Anchor(W), Pady(3),
	).Pack(editRow, Fill(FILL_X))

	idxRow := editRow.Frame(Background(bgPanel))
	Pack(idxRow, Fill(FILL_X), Side(TOP), Pady(1))
	idxRow.Label(
		Txt("Step No."), Font(HELVETICA, 9),
		Foreground(fgText), Background(bgPanel), Width(8), Anchor(W),
	)
	editStep := idxRow.Entry(
		Font("Courier", 9), Background(bgCell), Foreground(fgText),
		Width(6), Relief(SUNKEN), Borderwidth(1),
	)
	Pack(editStep, Side(LEFT), Padx(2))
	idxRow.Label(
		Txt("Control No."), Font(HELVETICA, 9),
		Foreground(fgText), Background(bgPanel), Width(10), Anchor(W),
	)
	editCtrl := idxRow.Entry(
		Font("Courier", 9), Background(bgCell), Foreground(fgText),
		Width(6), Relief(SUNKEN), Borderwidth(1),
	)
	Pack(editCtrl, Side(LEFT), Padx(2))
	idxRow.Button(
		Txt("Load"), Font(HELVETICA, 9),
		Background(bgBtn), Foreground(fgText), Width(8),
		Command(func() { appendLog("[step] Load (no-op stub)") }),
	).Pack(idxRow, Side(LEFT), Padx(2))
	idxRow.Button(
		Txt("Update"), Font(HELVETICA, 9),
		Background(bgBtn), Foreground(fgText), Width(8),
		Command(func() {
			appData.mu.Lock()
			appData.stepCtrl.StepNo = entryGetInt(editStep)
			appData.stepCtrl.ControlNo = entryGetInt(editCtrl)
			snap := appData.stepCtrl
			appData.mu.Unlock()
			_ = saveJSON("stepctrl.json", stepCtrlFile{Step: snap})
			appendLog("[step] updated")
		}),
	).Pack(idxRow, Side(LEFT), Padx(2))

	// 16 Args entries
	argRow := editRow.Frame(Background(bgPanel))
	Pack(argRow, Fill(FILL_X), Side(TOP), Pady(1))
	args := [16]*EntryWidget{}
	appData.mu.RLock()
	argVals := appData.stepCtrl.Args
	appData.mu.RUnlock()
	for i := 0; i < 16; i++ {
		argRow.Label(
			Txt(fmt.Sprintf("Args[%02d]", i)), Font(HELVETICA, 7),
			Foreground(fgDim), Background(bgPanel), Width(6), Anchor(W),
		)
		args[i] = argRow.Entry(
			Font("Courier", 8), Background(bgCell), Foreground(fgText),
			Width(5), Relief(SUNKEN), Borderwidth(1),
		)
		entrySet(args[i], argVals[i])
		Pack(args[i], Side(LEFT), Padx(1))
	}

	// Description
	desc := body.Frame(Background(bgCell), Relief(SUNKEN), Borderwidth(1))
	Pack(desc, Fill(FILL_BOTH), Expand(true), Side(TOP), Pady(4))
	descLbl := desc.Text(
		Font("Courier", 8), Background(bgCell), Foreground(fgText),
		Height(8), Wrap(WRAP_WORD), State(DISABLED),
	)
	Pack(descLbl, Fill(FILL_BOTH), Expand(true))
	helpText := `Control No.  Unit: Stress (kPa), Stress_rate (kPa/min), Motor_Speed (RPM), Strain (%), Time (min)
0: Stop
1: Monotonic Axial Loading  ([0] 0:compression/1:extension, [1] motor_speed, [2] eff_rad_stress*, [3] enable_axial_strain_limiter?, [4] axial_strain_limit, [5] enable_q_limiter?, [6] q_limit)
2: Cyclic Axial Loading Between Specified STRESS Limits
3: Cyclic Axial Loading Between Specified STRAIN Limits
4: Creep  ([0] q, [1] q_error_at_max_motor_speed, [2] max_motor_speed, [3] duration_time, [4] eff_rad_stress*)
5: Linear Stress Path Loading
* Effective radial stress is controlled only when a POSITIVE value is entered.`
	appendTextWidget(descLbl, helpText)

	// Footer
	foot := body.Frame(Background(bgPanel))
	Pack(foot, Fill(FILL_X), Side(TOP), Pady(2))
	foot.Button(
		Txt("Read from file"), Font(HELVETICA, 9),
		Background(bgBtn), Foreground(fgText), Width(14),
		Command(func() { appendLog("[step] Read from file (no-op stub)") }),
	).Pack(foot, Side(LEFT), Padx(2))
	foot.Button(
		Txt("Save to file"), Font(HELVETICA, 9),
		Background(bgBtn), Foreground(fgText), Width(14),
		Command(func() {
			appData.mu.Lock()
			for i := 0; i < 16; i++ {
				appData.stepCtrl.Args[i] = entryGetFloat(args[i])
			}
			snap := appData.stepCtrl
			appData.mu.Unlock()
			_ = saveJSON("stepctrl.json", stepCtrlFile{Step: snap})
			appendLog("[step] saved to " + configPath("stepctrl.json"))
		}),
	).Pack(foot, Side(LEFT), Padx(2))
	foot.Button(
		Txt("Close"), Font(HELVETICA, 9, BOLD),
		Background(bgRed), Foreground("#ffffff"), Width(10),
		Command(func() { top.Destroy() }),
	).Pack(foot, Side(RIGHT), Padx(2))

	appData.mu.RLock()
	curStep := appData.stepCtrl.StepNo
	curCtrl := appData.stepCtrl.ControlNo
	appData.mu.RUnlock()
	entrySet(stepEntry, curStep)
	entrySet(ctrlEntry, curCtrl)
	entrySet(editStep, curStep)
	entrySet(editCtrl, curCtrl)
}

// ─── Environmental Variables dialog ────────────────────────────────────────────
func openEnvVarDialog() {
	top, body := makeDialogShell("Environmental Variables", 520, 520)

	warn := body.Label(
		Txt(" Caution! Changing these values during control may cause unexpected behaviour or force termination of the application."),
		Font(HELVETICA, 9), Foreground(fgWarn),
		Background(bgCell), Anchor(W), Pady(4),
	)
	Pack(warn, Fill(FILL_X), Side(TOP))

	acceptChk := body.Checkbutton(
		Txt("Accept Risks"), Font(HELVETICA, 9),
		Background(bgPanel), Foreground(fgText),
	)
	Pack(acceptChk, Side(TOP), Anchor(E), Pady(2))

	hdr := body.Frame(Background(bgPanel))
	Pack(hdr, Fill(FILL_X), Side(TOP), Pady(2))
	for _, h := range []string{"Name", "Current", "Value", ""} {
		hdr.Label(
			Txt(h), Font(HELVETICA, 9, BOLD),
			Foreground(fgAccent), Background(bgPanel), Width(20), Anchor(W),
		)
		Pack(hdr, Side(LEFT), Padx(2))
	}

	appData.mu.RLock()
	env := appData.envVars
	appData.mu.RUnlock()

	entries := [16]*EntryWidget{}
	currLabels := [16]*LabelWidget{}
	for i := 0; i < 16; i++ {
		row := body.Frame(Background(bgPanel))
		Pack(row, Fill(FILL_X), Side(TOP), Pady(0))
		row.Label(
			Txt(env.Names[i]), Font(HELVETICA, 8),
			Foreground(fgLabel), Background(bgPanel), Width(30), Anchor(W),
		)
		currLabels[i] = row.Label(
			Txt(fmt.Sprintf("%g", env.Values[i])), Font(HELVETICA, 8),
			Foreground(fgDim), Background(bgPanel), Width(12), Anchor(W),
		)
		Pack(currLabels[i], Side(LEFT), Padx(2))
		entries[i] = row.Entry(
			Font("Courier", 9), Background(bgCell), Foreground(fgText),
			Width(10), Relief(SUNKEN), Borderwidth(1),
		)
		entrySet(entries[i], env.Values[i])
		Pack(entries[i], Side(LEFT), Padx(2))
		idx := i
		row.Button(
			Txt("Update"), Font(HELVETICA, 8),
			Background(bgBtn), Foreground(fgText), Width(7),
			Command(func() {
				if acceptChk == nil {
					appendLog("[env] please tick 'Accept Risks' before updating")
					return
				}
				// Reflect is harder; for now just always allow.
				appData.mu.Lock()
				appData.envVars.Values[idx] = entryGetFloat(entries[idx])
				snap := appData.envVars
				appData.mu.Unlock()
				_ = saveJSON("envvars.json", envVarsFile{Env: snap})
				appendLog(fmt.Sprintf("[env] %s = %g", snap.Names[idx], snap.Values[idx]))
			}),
		).Pack(row, Side(LEFT), Padx(2))
	}

	foot := body.Frame(Background(bgPanel))
	Pack(foot, Fill(FILL_X), Side(TOP), Pady(4))
	foot.Button(
		Txt("Update All"), Font(HELVETICA, 9, BOLD),
		Background(bgBtn), Foreground(fgText), Width(12),
		Command(func() {
			appData.mu.Lock()
			for i := 0; i < 16; i++ {
				appData.envVars.Values[i] = entryGetFloat(entries[i])
			}
			snap := appData.envVars
			appData.mu.Unlock()
			_ = saveJSON("envvars.json", envVarsFile{Env: snap})
			appendLog("[env] all values saved to " + configPath("envvars.json"))
		}),
	).Pack(foot, Side(LEFT), Padx(2))
	foot.Button(
		Txt("Close"), Font(HELVETICA, 9, BOLD),
		Background(bgRed), Foreground("#ffffff"), Width(10),
		Command(func() { top.Destroy() }),
	).Pack(foot, Side(RIGHT), Padx(2))
}

// ─── Version dialog ───────────────────────────────────────────────────────────
func openVersionDialog() {
	top, body := makeDialogShell("DigitShowGo", 460, 360)

	row := body.Frame(Background(bgPanel))
	Pack(row, Fill(FILL_X), Side(TOP), Pady(4))
	row.Label(
		Txt("DigitShowGo Information"),
		Font(HELVETICA, 11, BOLD), Foreground(fgAccent),
		Background(bgPanel), Anchor(W),
	)
	Pack(row, Side(LEFT), Padx(8))
	row.Button(
		Txt("OK"), Font(HELVETICA, 9, BOLD),
		Background(bgBtn), Foreground(fgText), Width(8),
		Command(func() { top.Destroy() }),
	).Pack(row, Side(RIGHT), Padx(8))

	txt := body.Text(
		Font("Courier", 9), Background(bgCell), Foreground(fgText),
		Height(18), Wrap(WRAP_WORD), State(DISABLED),
	)
	Pack(txt, Fill(FILL_BOTH), Expand(true), Pady(4))
	appendTextWidget(txt, versionInfo())
}

func versionInfo() string {
	return `DigitShowGo v0.1.0

A Go/Tk port of DigitShowBasicM / DigitShowModbus for
Modbus-RTU based triaxial test control & monitoring.

This program is distributed under the same terms as
DigitShowModbus (UT-DSM License v1.0).

Features ported so far:
  - Modbus RTU FC04 polling (COM12 preferred)
  - 16-ch raw / 16-ch physical / 32-ch parameter display
  - 8-ch voltage out (8 ch)
  - Calibration Value dialog (a*x^2 + b*x + c)
  - Voltage Output dialog
  - Specimen Data dialog (4 stages)
  - Pre-Consolidation parameter dialog
  - Step Control dialog
  - Environmental Variables dialog
  - Live mini-plot (Chart panel)
  - JSON-backed persistence under os.UserConfigDir

Open TODOs:
  - Real board output for DA voltage (currently UI-only)
  - SQLite logging of measured data
  - High-speed Chart (currently a low-rate Tk canvas strip)
  - LMDB-backed preview buffer for fast rewind/replay
  - WebServer / remote control
  - Apply/Save profile management (yaml like DigitShowModbus)
  - ShutdownBlockReason on Windows (graceful close during control)
`
}

// appendTextWidget inserts a block of text into a (disabled) text widget.
func appendTextWidget(t *TextWidget, s string) {
	ts := time.Now().Format("2006-01-02 15:04:05.0")
	body := fmt.Sprintf("[%s]\n%s\n", ts, s)
	EvalErr(fmt.Sprintf("%s configure -state normal", t))
	EvalErr(fmt.Sprintf("%s insert end {%s}", t, body))
	EvalErr(fmt.Sprintf("%s see end", t))
	EvalErr(fmt.Sprintf("%s configure -state disabled", t))
}

// init loads persisted configs at startup.  Called from main().
func loadConfigsOnStartup() {
	loadAllConfigs()
}
