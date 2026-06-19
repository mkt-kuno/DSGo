package dialogs

// ─── Calibration Value ダイアログ ───────────────────────────────────────────
//
// C++ の IDD_Calibration_Factor 相当。16チャンネル分の二次校正
// y = a*x^2 + b*x + c を表形式で編集する。各行に「Zero」ボタンを
// 置いて、現在の物理値 c = c - phy だけオフセットさせる
// (C++ OnBUTTONZero と同じセマンティクス)。

import (
	"fmt"

	. "modernc.org/tk9.0"
)

// OpenCalibration は Calibration Value ダイアログを開く。
func OpenCalibration() {
	top, body := makeDialogShell("Calibration Value", 700, 620)

	// ヘッダ行
	hdr := body.Frame(Background(BgPanel))
	Pack(hdr, Fill(FILL_X), Side(TOP), Pady(2))
	hdrTitles := []string{"x : Raw Value", "y : Physical Value", "a*x^2", "b*x", "c", "y"}
	hdrWidths := []int{22, 22, 8, 8, 8, 8}
	for i, t := range hdrTitles {
		lbl := hdr.Label(
			Txt(t), Font(HELVETICA, 9, BOLD),
			Foreground(FgAccent), Background(BgPanel),
			Width(hdrWidths[i]), Anchor(W),
		)
		Pack(lbl, Side(LEFT), Padx(2))
	}

	Store.RLock()
	cal := Store.Cal
	phys := Store.Phys
	Store.RUnlock()

	var (
		aEntries [16]*EntryWidget
		bEntries [16]*EntryWidget
		cEntries [16]*EntryWidget
		yEntries [16]*EntryWidget
	)

	// 16チャンネル分の編集行
	for i := 0; i < 16; i++ {
		row := body.Frame(Background(BgPanel))
		Pack(row, Fill(FILL_X), Side(TOP), Pady(0))

		// 行ラベル: "00:LoadCell(i16)"  "->"  "00:Load(N)"
		lbl := row.Label(
			Txt(fmt.Sprintf("%02d:%s(i16)", i, RawChNames[i])),
			Font(HELVETICA, 8), Foreground(FgLabel),
			Background(BgPanel), Width(22), Anchor(W),
		)
		Pack(lbl, Side(LEFT), Padx(2))
		lbl = row.Label(
			Txt("-->"), Font(HELVETICA, 8), Foreground(FgDim),
			Background(BgPanel), Width(3), Anchor(W),
		)
		Pack(lbl, Side(LEFT), Padx(2))
		lbl = row.Label(
			Txt(fmt.Sprintf("%02d:%s(%s)", i, PhysChNames[i], PhysUnits[i])),
			Font(HELVETICA, 8), Foreground(FgLabel),
			Background(BgPanel), Width(22), Anchor(W),
		)
		Pack(lbl, Side(LEFT), Padx(2))

		aEntries[i] = row.Entry(Font("Courier", 9), Background(BgCell), Foreground(FgText),
			Width(8), Relief(SUNKEN), Borderwidth(1))
		bEntries[i] = row.Entry(Font("Courier", 9), Background(BgCell), Foreground(FgText),
			Width(8), Relief(SUNKEN), Borderwidth(1))
		cEntries[i] = row.Entry(Font("Courier", 9), Background(BgCell), Foreground(FgText),
			Width(8), Relief(SUNKEN), Borderwidth(1))
		yEntries[i] = row.Entry(Font("Courier", 9), Background(BgCell), Foreground(FgText),
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
			Background(BgBtn), Foreground(FgText),
			Width(9), Command(func() { onZeroCalibration(idx, aEntries, bEntries, cEntries, yEntries) }),
		)
		Pack(zb, Side(LEFT), Padx(2))
	}

	// フッタ
	foot := body.Frame(Background(BgPanel))
	Pack(foot, Fill(FILL_X), Side(TOP), Pady(4))
	updateBtn := foot.Button(
		Txt("Update Variants"), Font(HELVETICA, 9, BOLD),
		Background(BgBtn), Foreground(FgText),
		Width(14), Command(func() { onUpdateCalibration(aEntries, bEntries, cEntries) }),
	)
	loadBtn := foot.Button(
		Txt("Load from a file"), Font(HELVETICA, 9),
		Background(BgBtn), Foreground(FgText),
		Width(14), Command(func() { onLoadCalibration(aEntries, bEntries, cEntries) }),
	)
	saveBtn := foot.Button(
		Txt("Save to a file"), Font(HELVETICA, 9),
		Background(BgBtn), Foreground(FgText),
		Width(14), Command(func() { onSaveCalibration(aEntries, bEntries, cEntries) }),
	)
	closeBtn := foot.Button(
		Txt("Close"), Font(HELVETICA, 9, BOLD),
		Background(BgRed), Foreground("#ffffff"),
		Width(10), Command(func() { Destroy(top) }),
	)
	Pack(updateBtn, Side(LEFT), Padx(2))
	Pack(loadBtn, Side(LEFT), Padx(2))
	Pack(saveBtn, Side(LEFT), Padx(2))
	Pack(closeBtn, Side(RIGHT), Padx(2))
}

// onUpdateCalibration は全16チャンネルの a/b/c を Entry から読み取り
// Store.Cal に書き戻し、calibration.json に保存する。
func onUpdateCalibration(a, b, c [16]*EntryWidget) {
	Store.Lock()
	for i := 0; i < 16; i++ {
		Store.Cal[i] = CalCoeff{
			A: entryGetFloat(a[i]),
			B: entryGetFloat(b[i]),
			C: entryGetFloat(c[i]),
		}
	}
	cal := Store.Cal
	Store.Unlock()
	_ = saveJSON("calibration.json", calibrationFile{Cal: cal})
	Logger("[calib] variants updated")
}

// onSaveCalibration は更新→保存の順に実行し、保存パスをログに流す。
func onSaveCalibration(a, b, c [16]*EntryWidget) {
	onUpdateCalibration(a, b, c)
	Logger("[calib] saved to " + configPath("calibration.json"))
}

// onLoadCalibration は calibration.json を読み込んで Data と Entry を更新する。
func onLoadCalibration(a, b, c [16]*EntryWidget) {
	var cf calibrationFile
	if err := loadJSON("calibration.json", &cf); err != nil {
		Logger("[calib] load failed: " + err.Error())
		return
	}
	Store.Lock()
	Store.Cal = cf.Cal
	Store.Unlock()
	for i := 0; i < 16; i++ {
		entrySet(a[i], cf.Cal[i].A)
		entrySet(b[i], cf.Cal[i].B)
		entrySet(c[i], cf.Cal[i].C)
	}
	Logger("[calib] loaded " + configPath("calibration.json"))
}

// onZeroCalibration は C++ の `c = c - phy` 相当のオフセット処理。
// まず a/b/c を現在のEntry値で確定させ、その直下の生値 r に対して
// 現在の校正で物理値 phy を計算し、c から phy を引いて y=0 に揃える。
// （Specimen ダイアログの Before/After Consolidation も同じ校正シフトを
//
//	4チャンネルに対して行うので、こちらは1チャンネル版）
func onZeroCalibration(idx int, a, b, c, y [16]*EntryWidget) {
	onUpdateCalibration(a, b, c)
	Store.RLock()
	r := Store.Raw[idx]
	cal := Store.Cal[idx]
	Store.RUnlock()
	x := float64(r)
	phy := cal.A*x*x + cal.B*x + cal.C
	newC := cal.C - phy
	Store.Lock()
	Store.Cal[idx].C = newC
	Store.Unlock()
	entrySet(c[idx], newC)
	entrySet(y[idx], 0)
	Logf("[calib] zero CH%02d: c <- %.6f", idx, newC)
}
