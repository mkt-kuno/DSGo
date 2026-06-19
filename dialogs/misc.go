package dialogs

// ─── 雑多なダイアログ/メニューアクション ─────────────────────────────────────
//
// - Version         : アプリ情報
// - WebServerInfo   : 将来実装予定のWebサーバー情報（現状はプレースホルダ）
// - OpenAppDataFolder / OpenTempFolder : エクスプローラで該当フォルダを開く

import (
	"os"
	"os/exec"

	. "modernc.org/tk9.0"
)

// OpenVersion はバージョン情報ダイアログを開く。
func OpenVersion() {
	top, body := makeDialogShell("DigitShowGo", 480, 380)

	row := body.Frame(Background(BgPanel))
	Pack(row, Fill(FILL_X), Side(TOP), Pady(4))
	lbl := row.Label(
		Txt("DigitShowGo Information"),
		Font(HELVETICA, 11, BOLD), Foreground(FgAccent),
		Background(BgPanel), Anchor(W),
	)
	Pack(lbl, Side(LEFT), Padx(8))
	okBtn := row.Button(
		Txt("OK"), Font(HELVETICA, 9, BOLD),
		Background(BgBtn), Foreground(FgText), Width(8),
		Command(func() { Destroy(top) }),
	)
	Pack(okBtn, Side(RIGHT), Padx(8))

	txt := body.Text(
		Font("Courier", 9), Background(BgCell), Foreground(FgText),
		Height(18), Wrap("word"), State("disabled"),
	)
	Pack(txt, Fill(FILL_BOTH), Expand(true), Pady(4))
	appendTextWidget(txt, versionInfo())
}

// versionInfo はバージョン情報ダイアログ本文。
// 実装済み機能と Open TODOs を C++ 側のコメントを要約しつつ列挙する。
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

// OpenWebServerInfo は WebServer 情報ダイアログを開く（現状プレースホルダ）。
func OpenWebServerInfo() {
	top, body := makeDialogShell("Web Server Info", 460, 220)

	hdr := body.Frame(Background(BgPanel))
	Pack(hdr, Fill(FILL_X), Side(TOP), Pady(4))
	lbl := hdr.Label(
		Txt("Web Server Info"),
		Font(HELVETICA, 11, BOLD), Foreground(FgAccent),
		Background(BgPanel), Anchor(W),
	)
	Pack(lbl, Side(LEFT), Padx(8))
	okBtn := hdr.Button(
		Txt("OK"), Font(HELVETICA, 9, BOLD),
		Background(BgBtn), Foreground(FgText), Width(8),
		Command(func() { Destroy(top) }),
	)
	Pack(okBtn, Side(RIGHT), Padx(8))

	txt := body.Text(
		Font("Courier", 9), Background(BgCell), Foreground(FgText),
		Height(10), Wrap("word"), State("disabled"),
	)
	Pack(txt, Fill(FILL_BOTH), Expand(true), Pady(4))
	appendTextWidget(txt, "Web Server Info not implemented")
}

// OpenAppDataFolder は設定保存先ディレクトリをエクスプローラで開く。
// ディレクトリが存在しなければ作成する。
func OpenAppDataFolder() {
	dir := configDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		Logger("[folder] mkdir failed: " + err.Error())
		return
	}
	if err := exec.Command("explorer", dir).Start(); err != nil {
		Logger("[folder] open appdata failed: " + err.Error())
		return
	}
	Logger("[folder] opened " + dir)
}

// OpenTempFolder は OS の一時ディレクトリをエクスプローラで開く。
func OpenTempFolder() {
	dir := os.TempDir()
	if err := exec.Command("explorer", dir).Start(); err != nil {
		Logger("[folder] open temp failed: " + err.Error())
		return
	}
	Logger("[folder] opened " + dir)
}
