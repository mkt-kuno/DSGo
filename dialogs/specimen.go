package dialogs

// ─── Specimen Data ダイアログ ────────────────────────────────────────────────
//
// C++ の IDD_SpecimenData 相当。
//   - 装置定数（Membrane E / Thickness / Cap Weight）は読み込み専用
//   - 試験片の 4 ステージ (Present / Initial / Before consol. / After consol.)
//     × 6 行 (Diameter / Height / Volume* / Area* / LDT1 / LDT2) を編集
//   - Volume と Area は直径と高さから自動計算（"*" マーカー）
//   - Present 列は読み込み専用（自動計算または「copy to present」経由）
//   - 「Before/After Consolidation」ボタンで C++ OnBUTTONBeConsol /
//     OnBUTTONAfConsolidation 相当のひずみ補正を実行し、CH01/CH02/CH03/CH09
//     の校正 c 係数を 0 シフト

import (
	"fmt"
	"math"

	. "modernc.org/tk9.0"
	"modernc.org/tk9.0/extensions/eval"
)

// specEditorStage は SpecimenStage の 6 フィールドを編集する
// 6 つの Entry ウィジェットへのポインタを束ねる。specimen.json には
// 書き出さない一時構造体。
type specEditorStage struct {
	diameter, height, volume, area, ldt1, ldt2 *EntryWidget
}

// setSpecEditorField は row index に応じて specEditorStage の対応する
// フィールドに Entry ウィジェットを格納する。
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

// stageValueByIdx は SpecimenStage の対応するフィールドの現在値を返す。
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

// OpenSpecimen は Specimen Data ダイアログを開く。
func OpenSpecimen() {
	top, body := makeDialogShell("Specimen Data", 760, 660)

	// ─── 装置パラメータグループ（Membrane E / T / Cap Weight）───
	appGrp := body.Frame(Background(BgPanel), Relief(SUNKEN), Borderwidth(1))
	Pack(appGrp, Fill(FILL_X), Side(TOP), Pady(2))
	applbl := appGrp.Label(
		Txt(" Parameters of Test Apparatus [only available for torsional shear]"),
		Font(HELVETICA, 9, BOLD), Foreground(FgAccent),
		Background(BgPanel), Anchor(W), Pady(3),
	)
	Pack(applbl, Fill(FILL_X))
	apparatus := appGrp.Frame(Background(BgPanel))
	Pack(apparatus, Fill(FILL_X), Side(TOP), Pady(1))
	_, memE := mkRow(apparatus, "Young's Modulus of membrane (kPa)", 8)
	_, memT := mkRow(apparatus, "Thickness of membrane (mm)", 8)
	_, capW := mkRow(apparatus, "Cap Weight (N)", 8)

	Store.RLock()
	spec := Store.Specimen
	Store.RUnlock()
	entrySet(memE, spec.MembraneE)
	entrySet(memT, spec.MembraneT)
	entrySet(capW, spec.CapWeight)
	// 装置定数は読み込み専用 (C++ ES_READONLY)
	eval.EvalErr(fmt.Sprintf("%s configure -state disabled", memE))
	eval.EvalErr(fmt.Sprintf("%s configure -state disabled", memT))
	eval.EvalErr(fmt.Sprintf("%s configure -state disabled", capW))

	// 「*」マーカー説明
	note := body.Label(
		Txt(" (*: Automatically calculated)"),
		Font(HELVETICA, 8), Foreground(FgDim),
		Background(BgMain), Anchor(E),
	)
	Pack(note, Fill(FILL_X), Side(TOP), Padx(2))

	// ─── 4 ステージ × 6 行の編集グリッド ───
	grp := body.Frame(Background(BgPanel), Relief(SUNKEN), Borderwidth(1))
	Pack(grp, Fill(FILL_BOTH), Expand(true), Side(TOP), Pady(4))
	grpLbl := grp.Label(
		Txt(" Input Specimen's Data"),
		Font(HELVETICA, 9, BOLD), Foreground(FgAccent),
		Background(BgPanel), Anchor(W), Pady(2),
	)
	Pack(grpLbl, Fill(FILL_X))
	hdr := grp.Frame(Background(BgPanel))
	Pack(hdr, Fill(FILL_X), Side(TOP), Pady(2))
	hdrs := []string{"", "Present", "Initial", "Before consol.", "After consol."}
	widths := []int{16, 9, 9, 14, 14}
	for i, h := range hdrs {
		lbl := hdr.Label(
			Txt(h), Font(HELVETICA, 9, BOLD),
			Foreground(FgAccent), Background(BgPanel),
			Width(widths[i]), Anchor(anchorForHeader(i)),
		)
		Pack(lbl, Side(LEFT), Padx(2))
	}

	var stages [4]specEditorStage
	stageKeys := [4]*SpecimenStage{&spec.Present, &spec.Initial, &spec.BeforeCons, &spec.AfterCons}
	rowDefs := []string{"Diameter (mm)", "Height (mm)", "Volume * (mm3)", "Area * (mm2)", "LDT1 (mm)", "LDT2 (mm)"}

	for ri, rname := range rowDefs {
		row := grp.Frame(Background(BgPanel))
		Pack(row, Fill(FILL_X), Side(TOP), Pady(0))
		lbl := row.Label(
			Txt(rname), Font(HELVETICA, 9),
			Foreground(FgText), Background(BgPanel), Width(16), Anchor(W),
		)
		Pack(lbl, Side(LEFT))
		for si := 0; si < 4; si++ {
			// 読み込み専用条件 (C++ ES_READONLY):
			//   si == 0 → Present 列
			//   ri == 2 → Volume 行
			//   ri == 3 → Area 行
			ro := si == 0 || ri == 2 || ri == 3
			var entry *EntryWidget
			if ro {
				entry = row.Entry(
					Font("Courier", 9), Background(BgCell), Foreground(FgText),
					Width(9), Relief(SUNKEN), Borderwidth(1), Justify(RIGHT),
					State("disabled"),
				)
			} else {
				entry = row.Entry(
					Font("Courier", 9), Background(BgCell), Foreground(FgText),
					Width(9), Relief(SUNKEN), Borderwidth(1), Justify(RIGHT),
				)
			}
			entrySet(entry, stageValueByIdx(stageKeys[si], ri))
			Pack(entry, Side(LEFT), Padx(2))
			setSpecEditorField(&stages[si], ri, entry)
		}
	}

	// ─── 「copy to present」ボタン行 (C++ IDC_BUTTON_ToPresent1/2/3) ───
	copyRow := grp.Frame(Background(BgPanel))
	Pack(copyRow, Fill(FILL_X), Side(TOP), Pady(2))
	spacerLbl := copyRow.Label(
		Txt(""), Font(HELVETICA, 9),
		Background(BgPanel), Width(16), Anchor(W),
	)
	Pack(spacerLbl, Side(LEFT))
	presentSpacer := copyRow.Label(
		Txt(""), Font("Courier", 9),
		Background(BgPanel), Width(9),
	)
	Pack(presentSpacer, Side(LEFT), Padx(2))
	for si := 1; si < 4; si++ {
		idx := si
		btn := copyRow.Button(
			Txt("copy to present"), Font(HELVETICA, 7),
			Background(BgBtn), Foreground(FgText), Width(14),
			Command(func() { copyStageToPresent(&stages[idx], &stages[0]) }),
		)
		Pack(btn, Side(LEFT), Padx(2))
	}

	// ─── Before/After Consolidation ボタン群 ───
	footGrp := body.Frame(Background(BgPanel), Relief(SUNKEN), Borderwidth(1))
	Pack(footGrp, Fill(FILL_X), Side(TOP), Pady(4))
	footLbl := footGrp.Label(
		Txt(" Update Reference Specimen Size and Initialize Strains"),
		Font(HELVETICA, 9, BOLD), Foreground(FgAccent),
		Background(BgPanel), Anchor(W), Pady(2),
	)
	Pack(footLbl, Fill(FILL_X))
	beBtn := footGrp.Button(
		Txt("Before Consolidation"), Font(HELVETICA, 9),
		Background(BgBtn), Foreground(FgText), Width(18),
		Command(func() { onBeforeConsolidation(stages) }),
	)
	afBtn := footGrp.Button(
		Txt("After Consolidation"), Font(HELVETICA, 9),
		Background(BgBtn), Foreground(FgText), Width(18),
		Command(func() { onAfterConsolidation(stages) }),
	)
	Pack(beBtn, Side(LEFT), Padx(2))
	Pack(afBtn, Side(LEFT), Padx(2))

	// ─── 説明文 ───
	desc := body.Frame(Background(BgPanel), Relief(SUNKEN), Borderwidth(1))
	Pack(desc, Fill(FILL_X), Side(TOP), Pady(4))
	descLbl1 := desc.Label(
		Txt("Update reference specimen size from PRESENT specimen strains."),
		Font(HELVETICA, 9, BOLD), Foreground(FgAccent),
		Background(BgPanel), Anchor(W), Pady(2),
	)
	Pack(descLbl1, Fill(FILL_X))
	descLbl2 := desc.Label(
		Txt("Assuming isotropic deformation (where the volumetric strain is three times the axial strain), the reference size of the specimen is updated based on the PRESENT axial displacement Store."),
		Font(HELVETICA, 8), Foreground(FgText),
		Background(BgPanel), Anchor(W), Pady(2),
	)
	Pack(descLbl2, Fill(FILL_X))

	// ─── フッタ (Update / Save / Close) ───
	footRow2 := body.Frame(Background(BgPanel))
	Pack(footRow2, Fill(FILL_X), Side(TOP), Pady(2))
	updateBtn := footRow2.Button(
		Txt("Update Params"), Font(HELVETICA, 9, BOLD),
		Background(BgBtn), Foreground(FgText), Width(14),
		State("disabled"),
		Command(func() { onUpdateSpecimen(memE, memT, capW, stages) }),
	)
	saveBtn := footRow2.Button(
		Txt("Save to file"), Font(HELVETICA, 9),
		Background(BgBtn), Foreground(FgText), Width(14),
		Command(func() {
			onUpdateSpecimen(memE, memT, capW, stages)
			Logger("[specimen] saved to " + configPath("specimen.json"))
		}),
	)
	closeBtn := footRow2.Button(
		Txt("Close"), Font(HELVETICA, 9, BOLD),
		Background(BgRed), Foreground("#ffffff"), Width(10),
		Command(func() { Destroy(top) }),
	)
	Pack(updateBtn, Side(LEFT), Padx(2))
	Pack(saveBtn, Side(LEFT), Padx(2))
	Pack(closeBtn, Side(RIGHT), Padx(2))
}

// copyStageToPresent は src の D/H/LDT1/LDT2 を Present（dst=読み込み専用）に
// コピーし、Present の A/V を D/H から再計算して更新する。
// 標準円柱公式: Area = π·D²/4, Volume = Area·H
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

// onUpdateSpecimen は 4 ステージ分の Entry 値を読み取り、Present/Initial/
// BeforeCons/AfterCons の D/H/LDT1/LDT2 を更新し、A/V を D/H から再計算
// して specimen.json に保存する。装置定数も一緒に保存。
func onUpdateSpecimen(memE, memT, capW *EntryWidget, stages [4]specEditorStage) {
	Store.Lock()
	Store.Specimen.MembraneE = entryGetFloat(memE)
	Store.Specimen.MembraneT = entryGetFloat(memT)
	Store.Specimen.CapWeight = entryGetFloat(capW)
	keys := []*SpecimenStage{
		&Store.Specimen.Present, &Store.Specimen.Initial,
		&Store.Specimen.BeforeCons, &Store.Specimen.AfterCons,
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
	snap := Store.Specimen
	Store.Unlock()
	_ = saveJSON("specimen.json", specimenFile{Specimen: snap})
	Logger("[specimen] updated")
}

// onBeforeConsolidation は C++ の OnBUTTONBeConsol (Dialog_Specimen.cpp) 相当。
// 現在の物理値 LVDT/LDT1/LDT2/LCDPT を読み、Present の H/V から等方変形を
// 仮定して BeforeCons の H/V/A/D/LDT を再計算し、BeforeCons を Present に
// コピーする。同時に CH01/CH02/CH03/CH09 の校正 c を 0 シフト。
//
// (AGENTS.md「#5 No Update Reference Specimen Size math」によれば本来は
//
//	LoadInput_And_Calc_AllStage 相当の幾何再計算が必要だが、本ハンドラは
//	ユーザー操作の明示ボタンなので実装している)
func onBeforeConsolidation(stages [4]specEditorStage) {
	Store.RLock()
	phys := Store.Phys
	Store.RUnlock()

	// ch1=LVDT, ch2=LDT1, ch3=LDT2, ch9=LCDPT
	lvdt := phys[1]
	ldt1 := phys[2]
	ldt2 := phys[3]
	lcdpt := phys[9]

	// Present の現在値（エントリから直接）
	pH := entryGetFloat(stages[0].height)
	pV := entryGetFloat(stages[0].volume)
	pLDT1 := entryGetFloat(stages[0].ldt1)
	pLDT2 := entryGetFloat(stages[0].ldt2)

	// 等方変形を仮定した BeforeCons 値
	beH := pH - lvdt
	beV := pV * (1 - 3*lvdt/pH)
	beA := beV / beH
	beD := math.Sqrt(4 * beA / math.Pi)
	beLDT1 := pLDT1 - ldt1
	beLDT2 := pLDT2 - ldt2

	// BeforeCons 列（編集可）に書き戻し
	entrySet(stages[2].height, beH)
	entrySet(stages[2].volume, beV)
	entrySet(stages[2].area, beA)
	entrySet(stages[2].diameter, beD)
	entrySet(stages[2].ldt1, beLDT1)
	entrySet(stages[2].ldt2, beLDT2)

	// BeforeCons → Present コピー
	copyStageToPresent(&stages[2], &stages[0])

	// 内部状態を更新 + 校正 c を 0 シフト
	Store.Lock()
	Store.Specimen.BeforeCons.Height = beH
	Store.Specimen.BeforeCons.Volume = beV
	Store.Specimen.BeforeCons.Area = beA
	Store.Specimen.BeforeCons.Diameter = beD
	Store.Specimen.BeforeCons.LDT1 = beLDT1
	Store.Specimen.BeforeCons.LDT2 = beLDT2
	Store.Specimen.Present = Store.Specimen.BeforeCons
	Store.Cal[1].C -= lvdt
	Store.Cal[9].C -= lcdpt
	Store.Cal[2].C -= ldt1
	Store.Cal[3].C -= ldt2
	snapSpec := Store.Specimen
	snapCal := Store.Cal
	Store.Unlock()

	_ = saveJSON("specimen.json", specimenFile{Specimen: snapSpec})
	_ = saveJSON("calibration.json", calibrationFile{Cal: snapCal})
	Logf("[specimen] Before Consolidation: H=%.4f V=%.4f A=%.4f D=%.4f (0-adjusted CH01/CH09/CH02/CH03)", beH, beV, beA, beD)
}

// onAfterConsolidation は C++ の OnBUTTONAfConsolidation 相当。
// BeforeCons と似ているが、LVDT と LCDPT の両方を体積/面積計算に使う
// (C++ 側では consolidation 中の体積変化は LCDPT の値を信頼する)。
func onAfterConsolidation(stages [4]specEditorStage) {
	Store.RLock()
	phys := Store.Phys
	Store.RUnlock()

	lvdt := phys[1]
	ldt1 := phys[2]
	ldt2 := phys[3]
	lcdpt := phys[9]

	pH := entryGetFloat(stages[0].height)
	pV := entryGetFloat(stages[0].volume)
	pA := entryGetFloat(stages[0].area)
	pD := entryGetFloat(stages[0].diameter)
	pLDT1 := entryGetFloat(stages[0].ldt1)
	pLDT2 := entryGetFloat(stages[0].ldt2)

	// LVDT と LCDPT を併用した AfterCons 値
	afH := pH - lvdt
	afV := pV - lcdpt
	afA := afV / afH
	afD := pD * math.Sqrt(afA/pA)
	afLDT1 := pLDT1 - ldt1
	afLDT2 := pLDT2 - ldt2

	// AfterCons 列（編集可）に書き戻し
	entrySet(stages[3].height, afH)
	entrySet(stages[3].volume, afV)
	entrySet(stages[3].area, afA)
	entrySet(stages[3].diameter, afD)
	entrySet(stages[3].ldt1, afLDT1)
	entrySet(stages[3].ldt2, afLDT2)

	// AfterCons → Present コピー
	copyStageToPresent(&stages[3], &stages[0])

	Store.Lock()
	Store.Specimen.AfterCons.Height = afH
	Store.Specimen.AfterCons.Volume = afV
	Store.Specimen.AfterCons.Area = afA
	Store.Specimen.AfterCons.Diameter = afD
	Store.Specimen.AfterCons.LDT1 = afLDT1
	Store.Specimen.AfterCons.LDT2 = afLDT2
	Store.Specimen.Present = Store.Specimen.AfterCons
	Store.Cal[1].C -= lvdt
	Store.Cal[9].C -= lcdpt
	Store.Cal[2].C -= ldt1
	Store.Cal[3].C -= ldt2
	snapSpec := Store.Specimen
	snapCal := Store.Cal
	Store.Unlock()

	_ = saveJSON("specimen.json", specimenFile{Specimen: snapSpec})
	_ = saveJSON("calibration.json", calibrationFile{Cal: snapCal})
	Logf("[specimen] After Consolidation: H=%.4f V=%.4f A=%.4f D=%.4f (0-adjusted CH01/CH09/CH02/CH03)", afH, afV, afA, afD)
}
