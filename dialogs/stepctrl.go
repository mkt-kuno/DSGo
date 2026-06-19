package dialogs

// ─── Step Control ダイアログ ────────────────────────────────────────────────
//
// C++ の IDD_StepCtrl 相当。1024 行 × (Control No + 16 args) のステップ
// プログラムを編集する。
//   - 上段: 現在の CurrentStepNo / Current Control No. を表示し、
//     "ChangeNo" チェック ON のときだけ "<-" "->" で +/-1 できる
//     (C++ 側と同じく CurrentStepNo を直接動かす)
//   - 中段: 編集対象行 EditStepNo の Control No. + Args[0..15] を編集
//   - 下段: 16 args の各意味を説明するヘルプテキスト
//   - "Read from file" / "Save to file" で 1024 行テーブル全体を
//     stepctrl.json にダンプ

import (
	"fmt"

	. "modernc.org/tk9.0"
)

// OpenStepCtrl は Step Control ダイアログを開く。
func OpenStepCtrl() {
	top, body := makeDialogShell("Step Control", 900, 480)

	// ─── 上段: タイトルバー ───
	topRow := body.Frame(Background(BgPanel), Relief(SUNKEN), Borderwidth(1))
	Pack(topRow, Fill(FILL_X), Side(TOP), Pady(2))
	lbl := topRow.Label(
		Txt(" Step Control"),
		Font(HELVETICA, 9, BOLD), Foreground(FgAccent),
		Background(BgPanel), Anchor(W), Pady(3),
	)
	Pack(lbl, Fill(FILL_X))

	// ─── 上段: Current Step / Control No. + <-> + ChangeNo チェック ───
	ctrlRow := body.Frame(Background(BgPanel))
	Pack(ctrlRow, Fill(FILL_X), Side(TOP), Pady(1))
	lbl = ctrlRow.Label(
		Txt("Current Step No."), Font(HELVETICA, 9),
		Foreground(FgText), Background(BgPanel), Width(16), Anchor(W),
	)
	Pack(lbl, Side(LEFT), Padx(2))
	stepEntry := ctrlRow.Entry(
		Font("Courier", 9), Background(BgCell), Foreground(FgText),
		Width(8), Relief(SUNKEN), Borderwidth(1),
		State("disabled"),
	)
	Pack(stepEntry, Side(LEFT), Padx(4))
	lbl = ctrlRow.Label(
		Txt("Current Control No."), Font(HELVETICA, 9),
		Foreground(FgText), Background(BgPanel), Width(18), Anchor(W),
	)
	Pack(lbl, Side(LEFT), Padx(2))
	ctrlEntry := ctrlRow.Entry(
		Font("Courier", 9), Background(BgCell), Foreground(FgText),
		Width(8), Relief(SUNKEN), Borderwidth(1),
		State("disabled"),
	)
	Pack(ctrlEntry, Side(LEFT), Padx(4))

	// "<-" "->" は ChangeNo チェックが ON のときだけ有効
	var decBtn, incBtn *ButtonWidget
	changeEnabled := false
	changeChk := ctrlRow.Checkbutton(
		Txt("ChangeNo"), Font(HELVETICA, 9),
		Background(BgPanel), Foreground(FgText),
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

	// refreshCurrentDisplay は CurrentStepNo を読み直して表示更新する
	// 内部クロージャ。Update/Read で CurrentStepNo が変わったときに呼ぶ。
	refreshCurrentDisplay := func() {
		Store.RLock()
		cur := Store.StepCtrl.CurrentStepNo
		var curCtrl int
		if cur >= 0 && cur < dsmStepCtrlMax {
			curCtrl = Store.StepCtrl.ControlNo[cur]
		}
		Store.RUnlock()
		entrySetRO(stepEntry, cur)
		entrySetRO(ctrlEntry, curCtrl)
	}

	decBtn = ctrlRow.Button(
		Txt("<-"), Font(HELVETICA, 9),
		Background(BgBtn), Foreground(FgText), Width(4),
		State("disabled"),
		Command(func() {
			Store.Lock()
			n := Store.StepCtrl.CurrentStepNo - 1
			if n < 0 {
				n = 0
			}
			Store.StepCtrl.CurrentStepNo = n
			Store.StepCtrl.CyclicNo = 0
			Store.Unlock()
			refreshCurrentDisplay()
			Logf("[step] CurrentStepNo -> %d (CyclicNo reset)", n)
		}),
	)
	incBtn = ctrlRow.Button(
		Txt("->"), Font(HELVETICA, 9),
		Background(BgBtn), Foreground(FgText), Width(4),
		State("disabled"),
		Command(func() {
			Store.Lock()
			n := Store.StepCtrl.CurrentStepNo + 1
			if n >= dsmStepCtrlMax {
				n = dsmStepCtrlMax - 1
			}
			Store.StepCtrl.CurrentStepNo = n
			Store.StepCtrl.CyclicNo = 0
			Store.Unlock()
			refreshCurrentDisplay()
			Logf("[step] CurrentStepNo -> %d (CyclicNo reset)", n)
		}),
	)
	Pack(decBtn, Side(LEFT), Padx(2))
	Pack(incBtn, Side(LEFT), Padx(2))

	// ─── 中段: 編集対象行 (EditStepNo) の Control No. + 16 args ───
	editRow := body.Frame(Background(BgPanel), Relief(SUNKEN), Borderwidth(1))
	Pack(editRow, Fill(FILL_X), Side(TOP), Pady(2))
	lbl = editRow.Label(
		Txt(" Control Arguments"),
		Font(HELVETICA, 9, BOLD), Foreground(FgAccent),
		Background(BgPanel), Anchor(W), Pady(3),
	)
	Pack(lbl, Fill(FILL_X))

	idxRow := editRow.Frame(Background(BgPanel))
	Pack(idxRow, Fill(FILL_X), Side(TOP), Pady(1))
	lbl = idxRow.Label(
		Txt("Step No."), Font(HELVETICA, 9),
		Foreground(FgText), Background(BgPanel), Width(8), Anchor(W),
	)
	Pack(lbl, Side(LEFT), Padx(2))
	editStep := idxRow.Entry(
		Font("Courier", 9), Background(BgCell), Foreground(FgText),
		Width(6), Relief(SUNKEN), Borderwidth(1),
	)
	Pack(editStep, Side(LEFT), Padx(2))
	lbl = idxRow.Label(
		Txt("Control No."), Font(HELVETICA, 9),
		Foreground(FgText), Background(BgPanel), Width(10), Anchor(W),
	)
	Pack(lbl, Side(LEFT), Padx(2))
	editCtrl := idxRow.Entry(
		Font("Courier", 9), Background(BgCell), Foreground(FgText),
		Width(6), Relief(SUNKEN), Borderwidth(1),
	)
	Pack(editCtrl, Side(LEFT), Padx(2))

	var args [16]*EntryWidget
	loadBtn := idxRow.Button(
		Txt("Load"), Font(HELVETICA, 9),
		Background(BgBtn), Foreground(FgText), Width(8),
		Command(func() {
			Store.Lock()
			n := clampStepNo(entryGetInt(editStep))
			Store.StepCtrl.EditStepNo = n
			ctrl := Store.StepCtrl.ControlNo[n]
			argVals := Store.StepCtrl.Args[n]
			Store.Unlock()
			entrySet(editStep, n)
			entrySet(editCtrl, ctrl)
			for i := 0; i < 16; i++ {
				entrySet(args[i], argVals[i])
			}
			Logf("[step] loaded step %d (ctrl=%d) into editor", n, ctrl)
		}),
	)
	updArgs := idxRow.Button(
		Txt("Update"), Font(HELVETICA, 9),
		Background(BgBtn), Foreground(FgText), Width(8),
		Command(func() {
			Store.Lock()
			n := clampStepNo(entryGetInt(editStep))
			Store.StepCtrl.EditStepNo = n
			Store.StepCtrl.ControlNo[n] = entryGetInt(editCtrl)
			for i := 0; i < 16; i++ {
				Store.StepCtrl.Args[n][i] = entryGetFloat(args[i])
			}
			snap := Store.StepCtrl
			Store.Unlock()
			_ = saveStepCtrlJSON(&snap)
			Logf("[step] updated step %d", n)
		}),
	)
	Pack(loadBtn, Side(LEFT), Padx(2))
	Pack(updArgs, Side(LEFT), Padx(2))

	// 16 args の Entry 行
	argRow := editRow.Frame(Background(BgPanel))
	Pack(argRow, Fill(FILL_X), Side(TOP), Pady(1))
	Store.RLock()
	editRowIdx := clampStepNo(Store.StepCtrl.EditStepNo)
	argVals := Store.StepCtrl.Args[editRowIdx]
	Store.RUnlock()
	for i := 0; i < 16; i++ {
		lbl := argRow.Label(
			Txt(fmt.Sprintf("Args[%02d]", i)), Font(HELVETICA, 7),
			Foreground(FgDim), Background(BgPanel), Width(6), Anchor(W),
		)
		Pack(lbl, Side(LEFT), Padx(1))
		args[i] = argRow.Entry(
			Font("Courier", 8), Background(BgCell), Foreground(FgText),
			Width(5), Relief(SUNKEN), Borderwidth(1),
		)
		entrySet(args[i], argVals[i])
		Pack(args[i], Side(LEFT), Padx(1))
	}

	// ─── 下段: ヘルプテキスト ───
	desc := body.Frame(Background(BgCell), Relief(SUNKEN), Borderwidth(1))
	Pack(desc, Fill(FILL_BOTH), Expand(true), Side(TOP), Pady(4))
	descLbl := desc.Text(
		Font("Courier", 8), Background(BgCell), Foreground(FgText),
		Height(8), Wrap("word"), State("disabled"),
	)
	Pack(descLbl, Fill(FILL_BOTH), Expand(true))
	appendTextWidget(descLbl, stepCtrlHelp)

	// ─── フッタ: Read / Save / Close ───
	foot := body.Frame(Background(BgPanel))
	Pack(foot, Fill(FILL_X), Side(TOP), Pady(2))
	readBtn := foot.Button(
		Txt("Read from file"), Font(HELVETICA, 9),
		Background(BgBtn), Foreground(FgText), Width(14),
		Command(func() {
			sc, err := loadStepCtrlJSON()
			if err != nil {
				Logger("[step] read failed: " + err.Error())
				return
			}
			Store.Lock()
			Store.StepCtrl = sc
			// 新しく読んだ EditStepNo / CurrentStepNo に応じて
			// 表示側（編集欄 + 読み込み専用 Current 欄）も全部更新
			editIdx := clampStepNo(Store.StepCtrl.EditStepNo)
			ctrl := Store.StepCtrl.ControlNo[editIdx]
			rowArgs := Store.StepCtrl.Args[editIdx]
			cur := Store.StepCtrl.CurrentStepNo
			var curCtrl int
			if cur >= 0 && cur < dsmStepCtrlMax {
				curCtrl = Store.StepCtrl.ControlNo[cur]
			}
			Store.Unlock()
			entrySet(editStep, editIdx)
			entrySet(editCtrl, ctrl)
			for i := 0; i < 16; i++ {
				entrySet(args[i], rowArgs[i])
			}
			entrySetRO(stepEntry, cur)
			entrySetRO(ctrlEntry, curCtrl)
			Logger("[step] read 1024-row table from " + configPath("stepctrl.json"))
		}),
	)
	writeBtn := foot.Button(
		Txt("Save to file"), Font(HELVETICA, 9),
		Background(BgBtn), Foreground(FgText), Width(14),
		Command(func() {
			// ダイアログ上で未保存の編集をまずメモリに反映してから 1024 行
			// 全体を書き出す。Load した直後の状態に上書き保存する形。
			Store.Lock()
			n := clampStepNo(Store.StepCtrl.EditStepNo)
			Store.StepCtrl.ControlNo[n] = entryGetInt(editCtrl)
			for i := 0; i < 16; i++ {
				Store.StepCtrl.Args[n][i] = entryGetFloat(args[i])
			}
			snap := Store.StepCtrl
			Store.Unlock()
			_ = saveStepCtrlJSON(&snap)
			Logger("[step] saved 1024-row table to " + configPath("stepctrl.json"))
		}),
	)
	closeBtn := foot.Button(
		Txt("Close"), Font(HELVETICA, 9, BOLD),
		Background(BgRed), Foreground("#ffffff"), Width(10),
		Command(func() { Destroy(top) }),
	)
	Pack(readBtn, Side(LEFT), Padx(2))
	Pack(writeBtn, Side(LEFT), Padx(2))
	Pack(closeBtn, Side(RIGHT), Padx(2))

	// ─── 初期表示: CurrentStepNo と編集対象行の Control No. を反映 ───
	Store.RLock()
	curStep := Store.StepCtrl.CurrentStepNo
	var curCtrl int
	if curStep >= 0 && curStep < dsmStepCtrlMax {
		curCtrl = Store.StepCtrl.ControlNo[curStep]
	}
	// editRowIdx は上で計算済み（args初期化時にRLock中だった）ので再RLock
	// せず同じ値を使う
	editCtrlStart := Store.StepCtrl.ControlNo[editRowIdx]
	Store.RUnlock()
	entrySetRO(stepEntry, curStep)
	entrySetRO(ctrlEntry, curCtrl)
	entrySet(editStep, editRowIdx)
	entrySet(editCtrl, editCtrlStart)
}

// stepCtrlHelp はダイアログ下段のヘルプテキスト。
// Control No. 0..5 の意味と各 Args[*] の役割を C++ のコメントを翻訳しつつ
// 列挙する。物理単位は冒頭で一括して宣言。
const stepCtrlHelp = `Unit: Stress (kPa), Stress_rate (kPa/min), Motor_Speed (RPM), Strain (%), Time (min)

  0: Stop ([0] 0:do_nothing/1:motor_off, [1] 0:do_nothing/1:motor_up, [2] 0:do_nothing/1:motor_speed_zero, [3] 0:do_nothing/1:ep_cell_zero, [4] 0:do_nothing/1:ep_axis_zero)

  1: Monotonic Axial Loading ( [0] 0:compression/1:extension, [1] motor_speed, [2] eff_rad_stress*, [3] enable_axial_strain_limiter?(Disable:0/Enable:1), [4] axial_strain_limit, [5] enable_q_limiter?(Disable:0/Enable:1), [6] q_limit )

  2: Cyclic Axial Loading Between Specified STRESS Limits ( [0] 0:compression/1:extension, [1] motor_speed, [2] q_lower_limit, [3] q_upper_limit, [4] cycle_number, [5] eff_rad_stress* )

  3: Cyclic Axial Loading Between Specified STRAIN Limits ( [0] 0:compression/1:extension, [1] motor_speed, [2] axial_strain_lower_limit, [3] axial_strain_upper_limit, [4] cycle_number, [5] eff_rad_stress* )

  4: Creep ( [0] q, [1] q_error_at_max_motor_speed, [2] max_motor_speed, [3] duration_time, [4] eff_rad_stress* )

  5: Linear Stress Path Loading ( [0] ini_eff_axial_stress, [1] ini_eff_rad_stress, [2] end_eff_axial_stress, [3] end_eff_rad_stress, [4] cell_pressure_rate, [5] eff_axial_stress_error_at_max_motor_speed, [6] max_motor_speed )

  * Effective radial stress is controlled only when a POSITIVE value is entered.
`
