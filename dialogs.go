package main

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	. "modernc.org/tk9.0"
	"modernc.org/tk9.0/extensions/eval"
)

// ─── Persistence ──────────────────────────────────────────────────────────────

func configDir() string {
	if d, err := os.UserConfigDir(); err == nil {
		return filepath.Join(d, "DigitShowGo")
	}
	return "."
}

func configPath(name string) string { return filepath.Join(configDir(), name) }
func ensureConfigDir()               { _ = os.MkdirAll(configDir(), 0o755) }

func loadJSON(name string, v any) error {
	b, err := os.ReadFile(configPath(name))
	if err != nil {
		return err
	}
	return json.Unmarshal(b, v)
}

func saveJSON(name string, v any) error {
	ensureConfigDir()
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(configPath(name), b, 0o644)
}

type calibrationFile struct{ Cal [16]CalCoeff `json:"cal"` }
type specimenFile struct{ Specimen SpecimenData `json:"specimen"` }
type preConFile struct{ PreCon PreConParams `json:"preCon"` }
type stepCtrlFile struct{ Step StepCtrl `json:"step"` }
type envVarsFile struct{ Env EnvVars `json:"env"` }

func saveAllConfigs() {
	ensureConfigDir()
	appData.mu.RLock()
	_ = saveJSON("calibration.json", calibrationFile{Cal: appData.cal})
	_ = saveJSON("specimen.json", specimenFile{Specimen: appData.specimen})
	_ = saveJSON("precon.json", preConFile{PreCon: appData.preCon})
	_ = saveJSON("stepctrl.json", stepCtrlFile{Step: appData.stepCtrl})
	_ = saveJSON("envvars.json", envVarsFile{Env: appData.envVars})
	appData.mu.RUnlock()
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

// makeDialogShell creates a Toplevel window with title and size, returning
// the top-level widget (for use with `Destroy(top)`) and the body frame.
func makeDialogShell(title string, w, h int) (top *ToplevelWidget, body *FrameWidget) {
	top = Toplevel(Title(title), Background(bgMain))
	WmGeometry(top.Window, fmt.Sprintf("%dx%d", w, h))
	body = top.Frame(Background(bgMain), Padx(8), Pady(8))
	Pack(body, Fill(FILL_BOTH), Expand(true))
	return top, body
}

func mkRow(parent *FrameWidget, label string, width int) (row *FrameWidget, e *EntryWidget) {
	row = parent.Frame(Background(bgPanel))
	Pack(row, Fill(FILL_X), Side(TOP), Pady(1))
	lbl := row.Label(
		Txt(label), Font(HELVETICA, 9),
		Foreground(fgText), Background(bgPanel), Width(22), Anchor(W),
	)
	Pack(lbl, Side(LEFT), Padx(2))
	e = row.Entry(
		Font("Courier", 9), Background(bgCell), Foreground(fgText),
		Width(width), Relief(SUNKEN), Borderwidth(1),
	)
	Pack(e, Side(LEFT), Padx(4))
	return row, e
}

func entrySet(e *EntryWidget, v any) {
	eval.EvalErr(fmt.Sprintf("%s delete 0 end; %s insert 0 {%v}", e, e, fmt.Sprint(v)))
}

// entrySetRO writes a value into a read-only (State("disabled")) Entry widget
// by temporarily toggling it to the normal state, mirroring the pattern used
// by appendTextWidget for the disabled Text widget.
func entrySetRO(e *EntryWidget, v any) {
	s := fmt.Sprint(v)
	eval.EvalErr(fmt.Sprintf("%s configure -state normal", e))
	eval.EvalErr(fmt.Sprintf("%s delete 0 end; %s insert 0 {%s}", e, e, s))
	eval.EvalErr(fmt.Sprintf("%s configure -state disabled", e))
}

func entryGet(e *EntryWidget) string {
	return eval.EvalErr(fmt.Sprintf("%s get", e))
}

func entryGetFloat(e *EntryWidget) float64 {
	v, _ := strconv.ParseFloat(strings.TrimSpace(entryGet(e)), 64)
	return v
}

func entryGetInt(e *EntryWidget) int {
	v, _ := strconv.Atoi(strings.TrimSpace(entryGet(e)))
	return v
}

// ─── Calibration Value dialog ─────────────────────────────────────────────────
func openCalibrationDialog() {
	top, body := makeDialogShell("Calibration Value", 700, 620)

	// Header
	hdr := body.Frame(Background(bgPanel))
	Pack(hdr, Fill(FILL_X), Side(TOP), Pady(2))
	hdrTitles := []string{"x : Raw Value", "y : Physical Value", "a*x^2", "b*x", "c", "y"}
	hdrWidths := []int{22, 22, 8, 8, 8, 8}
	for i, t := range hdrTitles {
		lbl := hdr.Label(
			Txt(t), Font(HELVETICA, 9, BOLD),
			Foreground(fgAccent), Background(bgPanel),
			Width(hdrWidths[i]), Anchor(W),
		)
		Pack(lbl, Side(LEFT), Padx(2))
	}

	appData.mu.RLock()
	cal := appData.cal
	phys := appData.phys
	appData.mu.RUnlock()

	aEntries := [16]*EntryWidget{}
	bEntries := [16]*EntryWidget{}
	cEntries := [16]*EntryWidget{}
	yEntries := [16]*EntryWidget{}

	for i := 0; i < 16; i++ {
		row := body.Frame(Background(bgPanel))
		Pack(row, Fill(FILL_X), Side(TOP), Pady(0))

		lbl := row.Label(
			Txt(fmt.Sprintf("%02d:%s(i16)", i, rawChNames[i])),
			Font(HELVETICA, 8), Foreground(fgLabel),
			Background(bgPanel), Width(22), Anchor(W),
		)
		Pack(lbl, Side(LEFT), Padx(2))
		lbl = row.Label(
			Txt("-->"), Font(HELVETICA, 8), Foreground(fgDim),
			Background(bgPanel), Width(3), Anchor(W),
		)
		Pack(lbl, Side(LEFT), Padx(2))
		lbl = row.Label(
			Txt(fmt.Sprintf("%02d:%s(%s)", i, physChNames[i], physUnits[i])),
			Font(HELVETICA, 8), Foreground(fgLabel),
			Background(bgPanel), Width(22), Anchor(W),
		)
		Pack(lbl, Side(LEFT), Padx(2))

		aEntries[i] = row.Entry(Font("Courier", 9), Background(bgCell), Foreground(fgText),
			Width(8), Relief(SUNKEN), Borderwidth(1))
		bEntries[i] = row.Entry(Font("Courier", 9), Background(bgCell), Foreground(fgText),
			Width(8), Relief(SUNKEN), Borderwidth(1))
		cEntries[i] = row.Entry(Font("Courier", 9), Background(bgCell), Foreground(fgText),
			Width(8), Relief(SUNKEN), Borderwidth(1))
		yEntries[i] = row.Entry(Font("Courier", 9), Background(bgCell), Foreground(fgText),
			Width(8), Relief(SUNKEN), Borderwidth(1))
		entrySet(aEntries[i], cal[i].A)
		entrySet(bEntries[i], cal[i].B)
		entrySet(cEntries[i], cal[i].C)
		entrySet(yEntries[i], phys[i])
		Pack(aEntries[i], Side(LEFT), Padx(1))
		Pack(bEntries[i], Side(LEFT), Padx(1))
		Pack(cEntries[i], Side(LEFT), Padx(1))
		Pack(yEntries[i], Side(LEFT), Padx(1))

		idx := i
		zb := row.Button(
			Txt(fmt.Sprintf("%02d:Zero", i)), Font(HELVETICA, 8),
			Background(bgBtn), Foreground(fgText),
			Width(9), Command(func() { onZeroCalibration(idx, aEntries, bEntries, cEntries, yEntries) }),
		)
		Pack(zb, Side(LEFT), Padx(2))
	}

	// Footer
	foot := body.Frame(Background(bgPanel))
	Pack(foot, Fill(FILL_X), Side(TOP), Pady(4))
	updateBtn := foot.Button(
		Txt("Update Variants"), Font(HELVETICA, 9, BOLD),
		Background(bgBtn), Foreground(fgText),
		Width(14), Command(func() { onUpdateCalibration(aEntries, bEntries, cEntries) }),
	)
	loadBtn := foot.Button(
		Txt("Load from a file"), Font(HELVETICA, 9),
		Background(bgBtn), Foreground(fgText),
		Width(14), Command(func() { onLoadCalibration(aEntries, bEntries, cEntries) }),
	)
	saveBtn := foot.Button(
		Txt("Save to a file"), Font(HELVETICA, 9),
		Background(bgBtn), Foreground(fgText),
		Width(14), Command(func() { onSaveCalibration(aEntries, bEntries, cEntries) }),
	)
	closeBtn := foot.Button(
		Txt("Close"), Font(HELVETICA, 9, BOLD),
		Background(bgRed), Foreground("#ffffff"),
		Width(10), Command(func() { Destroy(top) }),
	)
	Pack(updateBtn, Side(LEFT), Padx(2))
	Pack(loadBtn, Side(LEFT), Padx(2))
	Pack(saveBtn, Side(LEFT), Padx(2))
	Pack(closeBtn, Side(RIGHT), Padx(2))
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
	cal := appData.cal
	appData.mu.Unlock()
	_ = saveJSON("calibration.json", calibrationFile{Cal: cal})
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
// C++ `c = c - phy` behaviour in IDD_Calibration_Factor.
func onZeroCalibration(idx int, a, b, c, y [16]*EntryWidget) {
	onUpdateCalibration(a, b, c)
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
	entrySet(y[idx], 0)
	appendLog(fmt.Sprintf("[calib] zero CH%02d: c <- %.6f", idx, newC))
}

// ─── VoltageOut dialog ────────────────────────────────────────────────────────
func openVoltageOutDialog() {
	top, body := makeDialogShell("Voltage Output on DA Board", 380, 400)

	head := body.Frame(Background(bgPanel))
	Pack(head, Fill(FILL_X), Side(TOP))
	lbl := head.Label(
		Txt(""), Font(HELVETICA, 9), Background(bgPanel), Width(20), Anchor(W),
	)
	Pack(lbl, Side(LEFT), Padx(2))
	lbl = head.Label(
		Txt("Voltage (V)"), Font(HELVETICA, 9, BOLD),
		Foreground(fgAccent), Background(bgPanel), Width(12), Anchor(E),
	)
	Pack(lbl, Side(LEFT), Padx(2))

	// Local voltEntries so the dialog instances don't share a global (otherwise
	// opening the dialog twice would cross-wire their buttons).
	var voltEntries [8]*EntryWidget

	right := body.Frame(Background(bgPanel))
	Pack(right, Side(RIGHT), Fill(FILL_Y), Padx(4))
	refBtn := right.Button(
		Txt("Refresh"), Font(HELVETICA, 9),
		Background(bgBtn), Foreground(fgText), Width(8),
		Command(func() { onRefreshVoltage(voltEntries) }),
	)
	outBtn := right.Button(
		Txt("Output"), Font(HELVETICA, 9),
		Background(bgBtn), Foreground(fgText), Width(8),
		Command(func() { onOutputVoltage(voltEntries) }),
	)
	closeBtn := right.Button(
		Txt("Close"), Font(HELVETICA, 9, BOLD),
		Background(bgRed), Foreground("#ffffff"), Width(8),
		Command(func() { Destroy(top) }),
	)
	Pack(refBtn, Side(TOP), Pady(2))
	Pack(outBtn, Side(TOP), Pady(2))
	Pack(closeBtn, Side(TOP), Pady(20))

	left := body.Frame(Background(bgCell), Relief(SUNKEN), Borderwidth(1))
	Pack(left, Side(LEFT), Fill(FILL_BOTH), Expand(true), Padx(2))

	appData.mu.RLock()
	volts := appData.volts
	appData.mu.RUnlock()

	for i := 0; i < 8; i++ {
		row := left.Frame(Background(bgCell))
		Pack(row, Fill(FILL_X), Side(TOP), Pady(1))
		lbl := row.Label(
			Txt(fmt.Sprintf("%02d:%s", i, voltChNames[i])),
			Font(HELVETICA, 8), Foreground(fgLabel),
			Background(bgCell), Width(22), Anchor(W),
		)
		Pack(lbl, Side(LEFT), Padx(2))
		voltEntries[i] = row.Entry(
			Font("Courier", 9), Background(bgPanel), Foreground(fgText),
			Width(8), Relief(SUNKEN), Borderwidth(1), Justify(RIGHT),
		)
		entrySet(voltEntries[i], volts[i])
		Pack(voltEntries[i], Side(LEFT), Padx(4))
	}
}

func onOutputVoltage(e [8]*EntryWidget) {
	appData.mu.Lock()
	for i := 0; i < 8; i++ {
		appData.volts[i] = entryGetFloat(e[i])
	}
	appData.mu.Unlock()
	appendLog("[dac] voltage out applied (UI-side echo)")
}

func onRefreshVoltage(e [8]*EntryWidget) {
	appData.mu.RLock()
	volts := appData.volts
	appData.mu.RUnlock()
	for i := 0; i < 8; i++ {
		entrySet(e[i], volts[i])
	}
	appendLog("[dac] refreshed")
}

// ─── Specimen Data dialog ──────────────────────────────────────────────────────
func openSpecimenDialog() {
	top, body := makeDialogShell("Specimen Data", 760, 660)

	// Apparatus group
	appGrp := body.Frame(Background(bgPanel), Relief(SUNKEN), Borderwidth(1))
	Pack(appGrp, Fill(FILL_X), Side(TOP), Pady(2))
	applbl := appGrp.Label(
		Txt(" Parameters of Test Apparatus [only available for torsional shear]"),
		Font(HELVETICA, 9, BOLD), Foreground(fgAccent),
		Background(bgPanel), Anchor(W), Pady(3),
	)
	Pack(applbl, Fill(FILL_X))
	apparatus := appGrp.Frame(Background(bgPanel))
	Pack(apparatus, Fill(FILL_X), Side(TOP), Pady(1))
	_, memE := mkRow(apparatus, "Young's Modulus of membrane (kPa)", 8)
	_, memT := mkRow(apparatus, "Thickness of membrane (mm)", 8)
	_, capW := mkRow(apparatus, "Cap Weight (N)", 8)

	appData.mu.RLock()
	spec := appData.specimen
	appData.mu.RUnlock()
	entrySet(memE, spec.MembraneE)
	entrySet(memT, spec.MembraneT)
	entrySet(capW, spec.CapWeight)
	// Apparatus params are read-only (C++ ES_READONLY on the IDC_EDIT_* controls).
	eval.EvalErr(fmt.Sprintf("%s configure -state disabled", memE))
	eval.EvalErr(fmt.Sprintf("%s configure -state disabled", memT))
	eval.EvalErr(fmt.Sprintf("%s configure -state disabled", capW))

	// Note explaining the * marker on Volume and Area rows.
	note := body.Label(
		Txt(" (*: Automatically calculated)"),
		Font(HELVETICA, 8), Foreground(fgDim),
		Background(bgMain), Anchor(E),
	)
	Pack(note, Fill(FILL_X), Side(TOP), Padx(2))

	// 4-column grid
	grp := body.Frame(Background(bgPanel), Relief(SUNKEN), Borderwidth(1))
	Pack(grp, Fill(FILL_BOTH), Expand(true), Side(TOP), Pady(4))
	hdr := grp.Frame(Background(bgPanel))
	Pack(hdr, Fill(FILL_X), Side(TOP), Pady(2))
	hdrs := []string{"", "Present", "Initial", "Before consol.", "After consol."}
	widths := []int{16, 9, 9, 14, 14}
	for i, h := range hdrs {
		lbl := hdr.Label(
			Txt(h), Font(HELVETICA, 9, BOLD),
			Foreground(fgAccent), Background(bgPanel),
			Width(widths[i]), Anchor(anchorForHeader(i)),
		)
		Pack(lbl, Side(LEFT), Padx(2))
	}

	stages := [4]specEditorStage{}
	stageKeys := [4]*SpecimenStage{&spec.Present, &spec.Initial, &spec.BeforeCons, &spec.AfterCons}
	rowDefs := []string{"Diameter (mm)", "Height (mm)", "Volume * (mm3)", "Area * (mm2)", "LDT1 (mm)", "LDT2 (mm)"}

	for ri, rname := range rowDefs {
		row := grp.Frame(Background(bgPanel))
		Pack(row, Fill(FILL_X), Side(TOP), Pady(0))
		lbl := row.Label(
			Txt(rname), Font(HELVETICA, 9),
			Foreground(fgText), Background(bgPanel), Width(16), Anchor(W),
		)
		Pack(lbl, Side(LEFT))
		for si := 0; si < 4; si++ {
			// Read-only conditions (C++ ES_READONLY):
			//   - si == 0  : Present column
			//   - ri == 2  : Volume row
			//   - ri == 3  : Area row
			ro := si == 0 || ri == 2 || ri == 3
			var entry *EntryWidget
			if ro {
				entry = row.Entry(
					Font("Courier", 9), Background(bgCell), Foreground(fgText),
					Width(9), Relief(SUNKEN), Borderwidth(1), Justify(RIGHT),
					State("disabled"),
				)
			} else {
				entry = row.Entry(
					Font("Courier", 9), Background(bgCell), Foreground(fgText),
					Width(9), Relief(SUNKEN), Borderwidth(1), Justify(RIGHT),
				)
			}
			entrySet(entry, stageValueByIdx(stageKeys[si], ri))
			Pack(entry, Side(LEFT), Padx(2))
			setSpecEditorField(&stages[si], ri, entry)
		}
	}

	// "Copy to present" buttons (C++ IDC_BUTTON_ToPresent1/2/3).
	// Aligned with the Initial / BeforeCons / AfterCons columns; the Present
	// column gets a blank spacer instead.
	copyRow := grp.Frame(Background(bgPanel))
	Pack(copyRow, Fill(FILL_X), Side(TOP), Pady(2))
	spacerLbl := copyRow.Label(
		Txt(""), Font(HELVETICA, 9),
		Background(bgPanel), Width(16), Anchor(W),
	)
	Pack(spacerLbl, Side(LEFT))
	presentSpacer := copyRow.Label(
		Txt(""), Font("Courier", 9),
		Background(bgPanel), Width(9),
	)
	Pack(presentSpacer, Side(LEFT), Padx(2))
	for si := 1; si < 4; si++ {
		idx := si
		btn := copyRow.Button(
			Txt("copy to present"), Font(HELVETICA, 7),
			Background(bgBtn), Foreground(fgText), Width(14),
			Command(func() { copyStageToPresent(&stages[idx], &stages[0]) }),
		)
		Pack(btn, Side(LEFT), Padx(2))
	}

	// Footer: Before/After Consolidation
	footGrp := body.Frame(Background(bgPanel), Relief(SUNKEN), Borderwidth(1))
	Pack(footGrp, Fill(FILL_X), Side(TOP), Pady(4))
	footLbl := footGrp.Label(
		Txt(" Update Reference Specimen Size and Initialize Strains"),
		Font(HELVETICA, 9, BOLD), Foreground(fgAccent),
		Background(bgPanel), Anchor(W), Pady(2),
	)
	Pack(footLbl, Fill(FILL_X))
	beBtn := footGrp.Button(
		Txt("Before Consolidation"), Font(HELVETICA, 9),
		Background(bgBtn), Foreground(fgText), Width(18),
		Command(func() { appendLog("[specimen] Before Consolidation pressed (no-op stub)") }),
	)
	afBtn := footGrp.Button(
		Txt("After Consolidation"), Font(HELVETICA, 9),
		Background(bgBtn), Foreground(fgText), Width(18),
		Command(func() { appendLog("[specimen] After Consolidation pressed (no-op stub)") }),
	)
	Pack(beBtn, Side(LEFT), Padx(2))
	Pack(afBtn, Side(LEFT), Padx(2))

	// Descriptive text (C++ IDC_STATIC_* captions at the bottom of IDD_SpecimenData).
	desc := body.Frame(Background(bgPanel), Relief(SUNKEN), Borderwidth(1))
	Pack(desc, Fill(FILL_X), Side(TOP), Pady(4))
	descLbl1 := desc.Label(
		Txt("Update reference specimen size from current specimen strains."),
		Font(HELVETICA, 9, BOLD), Foreground(fgAccent),
		Background(bgPanel), Anchor(W), Pady(2),
	)
	Pack(descLbl1, Fill(FILL_X))
	descLbl2 := desc.Label(
		Txt("Assuming isotropic deformation (where the volumetric strain is three times the axial strain), the reference size of the specimen is updated based on the PRESENT axial displacement data."),
		Font(HELVETICA, 8), Foreground(fgText),
		Background(bgPanel), Anchor(W), Pady(2),
	)
	Pack(descLbl2, Fill(FILL_X))

	footRow2 := body.Frame(Background(bgPanel))
	Pack(footRow2, Fill(FILL_X), Side(TOP), Pady(2))
	updateBtn := footRow2.Button(
		Txt("Update Params"), Font(HELVETICA, 9, BOLD),
		Background(bgBtn), Foreground(fgText), Width(14),
		State("disabled"),
		Command(func() { onUpdateSpecimen(memE, memT, capW, stages) }),
	)
	saveBtn := footRow2.Button(
		Txt("Save to file"), Font(HELVETICA, 9),
		Background(bgBtn), Foreground(fgText), Width(14),
		Command(func() {
			onUpdateSpecimen(memE, memT, capW, stages)
			appendLog("[specimen] saved to " + configPath("specimen.json"))
		}),
	)
	closeBtn := footRow2.Button(
		Txt("Close"), Font(HELVETICA, 9, BOLD),
		Background(bgRed), Foreground("#ffffff"), Width(10),
		Command(func() { Destroy(top) }),
	)
	Pack(updateBtn, Side(LEFT), Padx(2))
	Pack(saveBtn, Side(LEFT), Padx(2))
	Pack(closeBtn, Side(RIGHT), Padx(2))
}

func anchorForHeader(i int) any {
	if i == 0 {
		return W
	}
	return CENTER
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

type specEditorStage struct {
	diameter, height, volume, area, ldt1, ldt2 *EntryWidget
}

func setSpecEditorField(s *specEditorStage, idx int, e *EntryWidget) {
	switch idx {
	case 0:
		s.diameter = e
	case 1:
		s.height = e
	case 2:
		s.volume = e
	case 3:
		s.area = e
	case 4:
		s.ldt1 = e
	case 5:
		s.ldt2 = e
	}
}

// copyStageToPresent copies Diameter/Height/LDT1/LDT2 from src into the
// Present stage (which is read-only) and recomputes Present's Volume and
// Area from the new Diameter/Height using the standard cylinder formulas:
//   Area   = π · D² / 4
//   Volume = Area · H
func copyStageToPresent(src, dst *specEditorStage) {
	d := entryGetFloat(src.diameter)
	h := entryGetFloat(src.height)
	entrySetRO(dst.diameter, d)
	entrySetRO(dst.height, h)
	entrySetRO(dst.ldt1, entryGetFloat(src.ldt1))
	entrySetRO(dst.ldt2, entryGetFloat(src.ldt2))
	area := math.Pi * d * d / 4.0
	volume := area * h
	entrySetRO(dst.area, area)
	entrySetRO(dst.volume, volume)
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
		d := entryGetFloat(s.diameter)
		h := entryGetFloat(s.height)
		area := math.Pi * d * d / 4.0
		volume := area * h
		keys[i].Diameter = d
		keys[i].Height = h
		keys[i].Area = area
		keys[i].Volume = volume
		keys[i].LDT1 = entryGetFloat(s.ldt1)
		keys[i].LDT2 = entryGetFloat(s.ldt2)
		entrySetRO(s.area, area)
		entrySetRO(s.volume, volume)
	}
	snap := appData.specimen
	appData.mu.Unlock()
	_ = saveJSON("specimen.json", specimenFile{Specimen: snap})
	appendLog("[specimen] updated")
}

// ─── Pre-Consolidation dialog ─────────────────────────────────────────────────
func openPreConsolidationDialog() {
	top, body := makeDialogShell("Control Parameters in Pre-Consolidation Process", 400, 240)

	grp := body.Frame(Background(bgPanel), Relief(SUNKEN), Borderwidth(1))
	Pack(grp, Fill(FILL_X), Side(TOP), Pady(2))
	lbl := grp.Label(
		Txt(" Settings of pre-consolidation"),
		Font(HELVETICA, 9, BOLD), Foreground(fgAccent),
		Background(bgPanel), Anchor(W), Pady(3),
	)
	Pack(lbl, Fill(FILL_X))

	appData.mu.RLock()
	pc := appData.preCon
	appData.mu.RUnlock()

	_, tgt := mkRow(grp, "Target Deviator Stress, q (kPa)", 8)
	_, qErr := mkRow(grp, "q error at max motor speed (kPa)", 8)
	_, spd := mkRow(grp, "Max Motor Speed (rpm)", 8)
	entrySet(tgt, pc.TargetQ)
	entrySet(qErr, pc.QError)
	entrySet(spd, pc.MaxSpeed)

	foot := body.Frame(Background(bgPanel))
	Pack(foot, Fill(FILL_X), Side(TOP), Pady(4))
	updateBtn := foot.Button(
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
	)
	closeBtn := foot.Button(
		Txt("Close"), Font(HELVETICA, 9, BOLD),
		Background(bgRed), Foreground("#ffffff"), Width(10),
		Command(func() { Destroy(top) }),
	)
	Pack(updateBtn, Side(LEFT), Padx(2))
	Pack(closeBtn, Side(RIGHT), Padx(2))
}

// ─── Step Control dialog ──────────────────────────────────────────────────────
func openStepCtrlDialog() {
	top, body := makeDialogShell("Step Control", 900, 480)

	// Header: read-only step/control no + checkbox + <-> arrows
	topRow := body.Frame(Background(bgPanel), Relief(SUNKEN), Borderwidth(1))
	Pack(topRow, Fill(FILL_X), Side(TOP), Pady(2))
	lbl := topRow.Label(
		Txt(" Step Control"),
		Font(HELVETICA, 9, BOLD), Foreground(fgAccent),
		Background(bgPanel), Anchor(W), Pady(3),
	)
	Pack(lbl, Fill(FILL_X))

	ctrlRow := body.Frame(Background(bgPanel))
	Pack(ctrlRow, Fill(FILL_X), Side(TOP), Pady(1))
	lbl = ctrlRow.Label(
		Txt("Current Step No."), Font(HELVETICA, 9),
		Foreground(fgText), Background(bgPanel), Width(16), Anchor(W),
	)
	Pack(lbl, Side(LEFT), Padx(2))
	stepEntry := ctrlRow.Entry(
		Font("Courier", 9), Background(bgCell), Foreground(fgText),
		Width(8), Relief(SUNKEN), Borderwidth(1),
		State("disabled"),
	)
	Pack(stepEntry, Side(LEFT), Padx(4))
	lbl = ctrlRow.Label(
		Txt("Current Control No."), Font(HELVETICA, 9),
		Foreground(fgText), Background(bgPanel), Width(18), Anchor(W),
	)
	Pack(lbl, Side(LEFT), Padx(2))
	ctrlEntry := ctrlRow.Entry(
		Font("Courier", 9), Background(bgCell), Foreground(fgText),
		Width(8), Relief(SUNKEN), Borderwidth(1),
		State("disabled"),
	)
	Pack(ctrlEntry, Side(LEFT), Padx(4))
	var decBtn, incBtn *ButtonWidget
	changeEnabled := false
	changeChk := ctrlRow.Checkbutton(
		Txt("ChangeNo"), Font(HELVETICA, 9),
		Background(bgPanel), Foreground(fgText),
		Command(func() {
			changeEnabled = !changeEnabled
			st := State("normal")
			if !changeEnabled {
				st = State("disabled")
			}
			decBtn.Configure(st)
			incBtn.Configure(st)
		}),
	)
	Pack(changeChk, Side(LEFT), Padx(4))
	decBtn = ctrlRow.Button(
		Txt("<-"), Font(HELVETICA, 9),
		Background(bgBtn), Foreground(fgText), Width(4),
		State("disabled"),
		Command(func() { appendLog("[step] step-- (no-op stub)") }),
	)
	incBtn = ctrlRow.Button(
		Txt("->"), Font(HELVETICA, 9),
		Background(bgBtn), Foreground(fgText), Width(4),
		State("disabled"),
		Command(func() { appendLog("[step] step++ (no-op stub)") }),
	)
	Pack(decBtn, Side(LEFT), Padx(2))
	Pack(incBtn, Side(LEFT), Padx(2))

	// Editable
	editRow := body.Frame(Background(bgPanel), Relief(SUNKEN), Borderwidth(1))
	Pack(editRow, Fill(FILL_X), Side(TOP), Pady(2))
	lbl = editRow.Label(
		Txt(" Control Arguments"),
		Font(HELVETICA, 9, BOLD), Foreground(fgAccent),
		Background(bgPanel), Anchor(W), Pady(3),
	)
	Pack(lbl, Fill(FILL_X))

	idxRow := editRow.Frame(Background(bgPanel))
	Pack(idxRow, Fill(FILL_X), Side(TOP), Pady(1))
	lbl = idxRow.Label(
		Txt("Step No."), Font(HELVETICA, 9),
		Foreground(fgText), Background(bgPanel), Width(8), Anchor(W),
	)
	Pack(lbl, Side(LEFT), Padx(2))
	editStep := idxRow.Entry(
		Font("Courier", 9), Background(bgCell), Foreground(fgText),
		Width(6), Relief(SUNKEN), Borderwidth(1),
	)
	Pack(editStep, Side(LEFT), Padx(2))
	lbl = idxRow.Label(
		Txt("Control No."), Font(HELVETICA, 9),
		Foreground(fgText), Background(bgPanel), Width(10), Anchor(W),
	)
	Pack(lbl, Side(LEFT), Padx(2))
	editCtrl := idxRow.Entry(
		Font("Courier", 9), Background(bgCell), Foreground(fgText),
		Width(6), Relief(SUNKEN), Borderwidth(1),
	)
	Pack(editCtrl, Side(LEFT), Padx(2))
	loadBtn := idxRow.Button(
		Txt("Load"), Font(HELVETICA, 9),
		Background(bgBtn), Foreground(fgText), Width(8),
		Command(func() { appendLog("[step] Load (no-op stub)") }),
	)
	var args [16]*EntryWidget
	updArgs := idxRow.Button(
		Txt("Update"), Font(HELVETICA, 9),
		Background(bgBtn), Foreground(fgText), Width(8),
		Command(func() {
			appData.mu.Lock()
			appData.stepCtrl.StepNo = entryGetInt(editStep)
			appData.stepCtrl.ControlNo = entryGetInt(editCtrl)
			for i := 0; i < 16; i++ {
				appData.stepCtrl.Args[i] = entryGetFloat(args[i])
			}
			snap := appData.stepCtrl
			appData.mu.Unlock()
			_ = saveJSON("stepctrl.json", stepCtrlFile{Step: snap})
			appendLog("[step] updated")
		}),
	)
	Pack(loadBtn, Side(LEFT), Padx(2))
	Pack(updArgs, Side(LEFT), Padx(2))

	// 16 Args entries
	argRow := editRow.Frame(Background(bgPanel))
	Pack(argRow, Fill(FILL_X), Side(TOP), Pady(1))
	appData.mu.RLock()
	argVals := appData.stepCtrl.Args
	appData.mu.RUnlock()
	for i := 0; i < 16; i++ {
		lbl := argRow.Label(
			Txt(fmt.Sprintf("Args[%02d]", i)), Font(HELVETICA, 7),
			Foreground(fgDim), Background(bgPanel), Width(6), Anchor(W),
		)
		Pack(lbl, Side(LEFT), Padx(1))
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
		Height(8), Wrap("word"), State("disabled"),
	)
	Pack(descLbl, Fill(FILL_BOTH), Expand(true))
	appendTextWidget(descLbl, stepCtrlHelp)

	// Footer
	foot := body.Frame(Background(bgPanel))
	Pack(foot, Fill(FILL_X), Side(TOP), Pady(2))
	readBtn := foot.Button(
		Txt("Read from file"), Font(HELVETICA, 9),
		Background(bgBtn), Foreground(fgText), Width(14),
		Command(func() { appendLog("[step] Read from file (no-op stub)") }),
	)
	writeBtn := foot.Button(
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
	)
	closeBtn := foot.Button(
		Txt("Close"), Font(HELVETICA, 9, BOLD),
		Background(bgRed), Foreground("#ffffff"), Width(10),
		Command(func() { Destroy(top) }),
	)
	Pack(readBtn, Side(LEFT), Padx(2))
	Pack(writeBtn, Side(LEFT), Padx(2))
	Pack(closeBtn, Side(RIGHT), Padx(2))

	appData.mu.RLock()
	curStep := appData.stepCtrl.StepNo
	curCtrl := appData.stepCtrl.ControlNo
	appData.mu.RUnlock()
	entrySetRO(stepEntry, curStep)
	entrySetRO(ctrlEntry, curCtrl)
	entrySet(editStep, curStep)
	entrySet(editCtrl, curCtrl)
}

const stepCtrlHelp = `Control No.  Unit: Stress (kPa), Stress_rate (kPa/min), Motor_Speed (RPM), Strain (%), Time (min)
0: Stop
1: Monotonic Axial Loading  ([0] 0:compression/1:extension, [1] motor_speed, [2] eff_rad_stress*, [3] enable_axial_strain_limiter?, [4] axial_strain_limit, [5] enable_q_limiter?, [6] q_limit)
2: Cyclic Axial Loading Between Specified STRESS Limits
3: Cyclic Axial Loading Between Specified STRAIN Limits
4: Creep  ([0] q, [1] q_error_at_max_motor_speed, [2] max_motor_speed, [3] duration_time, [4] eff_rad_stress*)
5: Linear Stress Path Loading
* Effective radial stress is controlled only when a POSITIVE value is entered.`

// ─── Environmental Variables dialog ────────────────────────────────────────────
func openEnvVarDialog() {
	top, body := makeDialogShell("Environmental Variables", 540, 540)

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
		lbl := hdr.Label(
			Txt(h), Font(HELVETICA, 9, BOLD),
			Foreground(fgAccent), Background(bgPanel), Width(20), Anchor(W),
		)
		Pack(lbl, Side(LEFT), Padx(2))
	}

	appData.mu.RLock()
	env := appData.envVars
	appData.mu.RUnlock()

	entries := [16]*EntryWidget{}
	for i := 0; i < 16; i++ {
		row := body.Frame(Background(bgPanel))
		Pack(row, Fill(FILL_X), Side(TOP), Pady(0))
		lbl := row.Label(
			Txt(env.Names[i]), Font(HELVETICA, 8),
			Foreground(fgLabel), Background(bgPanel), Width(30), Anchor(W),
		)
		Pack(lbl, Side(LEFT), Padx(2))
		lbl = row.Label(
			Txt(fmt.Sprintf("%g", env.Values[i])), Font(HELVETICA, 8),
			Foreground(fgDim), Background(bgPanel), Width(12), Anchor(W),
		)
		Pack(lbl, Side(LEFT), Padx(2))
		entries[i] = row.Entry(
			Font("Courier", 9), Background(bgCell), Foreground(fgText),
			Width(10), Relief(SUNKEN), Borderwidth(1),
		)
		entrySet(entries[i], env.Values[i])
		Pack(entries[i], Side(LEFT), Padx(2))
		idx := i
		upBtn := row.Button(
			Txt("Update"), Font(HELVETICA, 8),
			Background(bgBtn), Foreground(fgText), Width(7),
			Command(func() {
				appData.mu.Lock()
				appData.envVars.Values[idx] = entryGetFloat(entries[idx])
				snap := appData.envVars
				appData.mu.Unlock()
				_ = saveJSON("envvars.json", envVarsFile{Env: snap})
				appendLog(fmt.Sprintf("[env] %s = %g", snap.Names[idx], snap.Values[idx]))
			}),
		)
		Pack(upBtn, Side(LEFT), Padx(2))
	}
	_ = acceptChk // Accept Risks toggle is purely visual in this stub.

	foot := body.Frame(Background(bgPanel))
	Pack(foot, Fill(FILL_X), Side(TOP), Pady(4))
	upAllBtn := foot.Button(
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
	)
	closeBtn := foot.Button(
		Txt("Close"), Font(HELVETICA, 9, BOLD),
		Background(bgRed), Foreground("#ffffff"), Width(10),
		Command(func() { Destroy(top) }),
	)
	Pack(upAllBtn, Side(LEFT), Padx(2))
	Pack(closeBtn, Side(RIGHT), Padx(2))
}

// ─── Web Server Info dialog ────────────────────────────────────────────────────
func openWebServerInfoDialog() {
	top, body := makeDialogShell("Web Server Info", 460, 220)

	hdr := body.Frame(Background(bgPanel))
	Pack(hdr, Fill(FILL_X), Side(TOP), Pady(4))
	lbl := hdr.Label(
		Txt("Web Server Info"),
		Font(HELVETICA, 11, BOLD), Foreground(fgAccent),
		Background(bgPanel), Anchor(W),
	)
	Pack(lbl, Side(LEFT), Padx(8))
	okBtn := hdr.Button(
		Txt("OK"), Font(HELVETICA, 9, BOLD),
		Background(bgBtn), Foreground(fgText), Width(8),
		Command(func() { Destroy(top) }),
	)
	Pack(okBtn, Side(RIGHT), Padx(8))

	txt := body.Text(
		Font("Courier", 9), Background(bgCell), Foreground(fgText),
		Height(10), Wrap("word"), State("disabled"),
	)
	Pack(txt, Fill(FILL_BOTH), Expand(true), Pady(4))
	appendTextWidget(txt, "Web Server Info not implemented")
}

// ─── Open Appdata / Log Folder ─────────────────────────────────────────────────
func openAppDataFolder() {
	dir := configDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		appendLog("[folder] mkdir failed: " + err.Error())
		return
	}
	if err := exec.Command("explorer", dir).Start(); err != nil {
		appendLog("[folder] open appdata failed: " + err.Error())
		return
	}
	appendLog("[folder] opened " + dir)
}

// ─── Open Temporary Folder ─────────────────────────────────────────────────────
func openTempFolder() {
	dir := os.TempDir()
	if err := exec.Command("explorer", dir).Start(); err != nil {
		appendLog("[folder] open temp failed: " + err.Error())
		return
	}
	appendLog("[folder] opened " + dir)
}

// ─── Version dialog ───────────────────────────────────────────────────────────
func openVersionDialog() {
	top, body := makeDialogShell("DigitShowGo", 480, 380)

	row := body.Frame(Background(bgPanel))
	Pack(row, Fill(FILL_X), Side(TOP), Pady(4))
	lbl := row.Label(
		Txt("DigitShowGo Information"),
		Font(HELVETICA, 11, BOLD), Foreground(fgAccent),
		Background(bgPanel), Anchor(W),
	)
	Pack(lbl, Side(LEFT), Padx(8))
	okBtn := row.Button(
		Txt("OK"), Font(HELVETICA, 9, BOLD),
		Background(bgBtn), Foreground(fgText), Width(8),
		Command(func() { Destroy(top) }),
	)
	Pack(okBtn, Side(RIGHT), Padx(8))

	txt := body.Text(
		Font("Courier", 9), Background(bgCell), Foreground(fgText),
		Height(18), Wrap("word"), State("disabled"),
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
  - 8-ch voltage out (UI side)
  - Calibration Value dialog (a*x^2 + b*x + c)
  - Voltage Output dialog
  - Specimen Data dialog (4 stages)
  - Pre-Consolidation parameter dialog
  - Step Control dialog
  - Environmental Variables dialog
  - Live mini-plot (Chart panel)
  - JSON-backed persistence under os.UserConfigDir

Open TODOs:
  - Real board output for DA voltage
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
	eval.EvalErr(fmt.Sprintf("%s configure -state normal", t))
	eval.EvalErr(fmt.Sprintf("%s insert end {%s}", t, body))
	eval.EvalErr(fmt.Sprintf("%s see end", t))
	eval.EvalErr(fmt.Sprintf("%s configure -state disabled", t))
}

// loadConfigsOnStartup is invoked once from main() at startup.
func loadConfigsOnStartup() { loadAllConfigs() }
