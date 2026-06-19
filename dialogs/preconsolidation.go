package dialogs

// ─── Pre-Consolidation ダイアログ ────────────────────────────────────────────
//
// C++ の IDD_Control_PreConsolidation 相当。PreCon アルゴリズムに渡す
// 3 つのパラメータ（Target q / q error at max motor speed / Max motor speed）
// を編集して precon.json に保存する。controlLoop（main 側）が毎 tick 読み
// に行く。

import (
	. "modernc.org/tk9.0"
)

// OpenPreConsolidation は Pre-Consolidation パラメータダイアログを開く。
func OpenPreConsolidation() {
	top, body := makeDialogShell("Control Parameters in Pre-Consolidation Process", 400, 240)

	grp := body.Frame(Background(BgPanel), Relief(SUNKEN), Borderwidth(1))
	Pack(grp, Fill(FILL_X), Side(TOP), Pady(2))
	lbl := grp.Label(
		Txt(" Settings of pre-consolidation"),
		Font(HELVETICA, 9, BOLD), Foreground(FgAccent),
		Background(BgPanel), Anchor(W), Pady(3),
	)
	Pack(lbl, Fill(FILL_X))

	Store.RLock()
	pc := Store.PreCon
	Store.RUnlock()

	_, tgt := mkRow(grp, "Target Deviator Stress, q (kPa)", 8)
	_, qErr := mkRow(grp, "q error at max motor speed (kPa)", 8)
	_, spd := mkRow(grp, "Max Motor Speed (rpm)", 8)
	entrySet(tgt, pc.TargetQ)
	entrySet(qErr, pc.QError)
	entrySet(spd, pc.MaxSpeed)

	foot := body.Frame(Background(BgPanel))
	Pack(foot, Fill(FILL_X), Side(TOP), Pady(4))
	updateBtn := foot.Button(
		Txt("Update"), Font(HELVETICA, 9, BOLD),
		Background(BgBtn), Foreground(FgText), Width(10),
		Command(func() {
			Store.Lock()
			Store.PreCon = PreConParams{
				TargetQ:  entryGetFloat(tgt),
				QError:   entryGetFloat(qErr),
				MaxSpeed: entryGetFloat(spd),
			}
			snap := Store.PreCon
			Store.Unlock()
			_ = saveJSON("precon.json", preConFile{PreCon: snap})
			Logger("[precon] updated")
		}),
	)
	closeBtn := foot.Button(
		Txt("Close"), Font(HELVETICA, 9, BOLD),
		Background(BgRed), Foreground("#ffffff"), Width(10),
		Command(func() { Destroy(top) }),
	)
	Pack(updateBtn, Side(LEFT), Padx(2))
	Pack(closeBtn, Side(RIGHT), Padx(2))
}
