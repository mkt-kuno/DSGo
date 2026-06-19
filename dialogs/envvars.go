package dialogs

// ─── Environmental Variables ダイアログ ──────────────────────────────────────
//
// C++ の IDD_Env 相当。最大 16 個の名前付き double 値を編集して
// envvars.json に保存する。各エントリは本来 C++ 側で DSM_A02_Motor_Speed_A
// などの環境変数名にマッピングされ os.Setenv されるが、本 Go ポートでは
// メモリ上 / JSON 上に保持するだけで os.Environ には反映しない。
// (AGENTS.md「#7 Environmental Variables dialog allows reading/editing/Update
//  but does NOT actually push the values to os.Setenv.」を参照)

import (
	"fmt"

	. "modernc.org/tk9.0"
)

// OpenEnvVar は Environmental Variables ダイアログを開く。
func OpenEnvVar() {
	top, body := makeDialogShell("Environmental Variables", 540, 540)

	// 警告文 + Accept Risks チェック
	warn := body.Label(
		Txt(" Caution! Changing these values during control may cause unexpected behaviour or force termination of the application."),
		Font(HELVETICA, 9), Foreground(FgWarn),
		Background(BgCell), Anchor(W), Pady(4),
	)
	Pack(warn, Fill(FILL_X), Side(TOP))
	acceptChk := body.Checkbutton(
		Txt("Accept Risks"), Font(HELVETICA, 9),
		Background(BgPanel), Foreground(FgText),
	)
	Pack(acceptChk, Side(TOP), Anchor(E), Pady(2))

	// ヘッダ行
	hdr := body.Frame(Background(BgPanel))
	Pack(hdr, Fill(FILL_X), Side(TOP), Pady(2))
	for _, h := range []string{"Name", "Current", "Value", ""} {
		lbl := hdr.Label(
			Txt(h), Font(HELVETICA, 9, BOLD),
			Foreground(FgAccent), Background(BgPanel), Width(20), Anchor(W),
		)
		Pack(lbl, Side(LEFT), Padx(2))
	}

	Store.RLock()
	env := Store.EnvVars
	Store.RUnlock()

	var entries [16]*EntryWidget
	for i := 0; i < 16; i++ {
		row := body.Frame(Background(BgPanel))
		Pack(row, Fill(FILL_X), Side(TOP), Pady(0))
		// 名前（読み込み専用）
		lbl := row.Label(
			Txt(env.Names[i]), Font(HELVETICA, 8),
			Foreground(FgLabel), Background(BgPanel), Width(30), Anchor(W),
		)
		Pack(lbl, Side(LEFT), Padx(2))
		// 現在値（メモリ上の env.Values を表示）
		lbl = row.Label(
			Txt(fmt.Sprintf("%g", env.Values[i])), Font(HELVETICA, 8),
			Foreground(FgDim), Background(BgPanel), Width(12), Anchor(W),
		)
		Pack(lbl, Side(LEFT), Padx(2))
		// 編集用 Entry
		entries[i] = row.Entry(
			Font("Courier", 9), Background(BgCell), Foreground(FgText),
			Width(10), Relief(SUNKEN), Borderwidth(1),
		)
		entrySet(entries[i], env.Values[i])
		Pack(entries[i], Side(LEFT), Padx(2))
		idx := i
		upBtn := row.Button(
			Txt("Update"), Font(HELVETICA, 8),
			Background(BgBtn), Foreground(FgText), Width(7),
			Command(func() {
				Store.Lock()
				Store.EnvVars.Values[idx] = entryGetFloat(entries[idx])
				snap := Store.EnvVars
				Store.Unlock()
				_ = saveJSON("envvars.json", envVarsFile{Env: snap})
				Logf("[env] %s = %g", snap.Names[idx], snap.Values[idx])
			}),
		)
		Pack(upBtn, Side(LEFT), Padx(2))
	}

	// フッタ: Update All / Close
	foot := body.Frame(Background(BgPanel))
	Pack(foot, Fill(FILL_X), Side(TOP), Pady(4))
	upAllBtn := foot.Button(
		Txt("Update All"), Font(HELVETICA, 9, BOLD),
		Background(BgBtn), Foreground(FgText), Width(12),
		Command(func() {
			Store.Lock()
			for i := 0; i < 16; i++ {
				Store.EnvVars.Values[i] = entryGetFloat(entries[i])
			}
			snap := Store.EnvVars
			Store.Unlock()
			_ = saveJSON("envvars.json", envVarsFile{Env: snap})
			Logger("[env] all values saved to " + configPath("envvars.json"))
		}),
	)
	closeBtn := foot.Button(
		Txt("Close"), Font(HELVETICA, 9, BOLD),
		Background(BgRed), Foreground("#ffffff"), Width(10),
		Command(func() { Destroy(top) }),
	)
	Pack(upAllBtn, Side(LEFT), Padx(2))
	Pack(closeBtn, Side(RIGHT), Padx(2))
}
