package dialogs

// ─── ダイアログ共通UIヘルパー ────────────────────────────────────────────────
//
// makeDialogShell / mkRow / entrySet / entryGet / appendTextWidget など、
// すべてのダイアログで繰り返し使われるTkウィジェット生成と値読み書きを
// ここに集約する。新しいダイアログを追加するときはまずはこのファイル
// を見て流用できるか確認すること。

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	. "modernc.org/tk9.0"
	"modernc.org/tk9.0/extensions/eval"
)

// makeDialogShell はタイトルとサイズを指定してToplevelウィンドウを生成し、
// そのトップレベルウィジェット（Destroy(top)用）と本体フレームを返す。
func makeDialogShell(title string, w, h int) (top *ToplevelWidget, body *FrameWidget) {
	top = Toplevel(Background(BgMain))
	top.WmTitle(title)
	WmGeometry(top.Window, fmt.Sprintf("%dx%d", w, h))
	body = top.Frame(Background(BgMain), Padx(8), Pady(8))
	Pack(body, Fill(FILL_BOTH), Expand(true))
	return top, body
}

// mkRow は "<label> <Entry>" の1行分のフレームを生成して返す。
// C++のIDD_*ダイアログで頻出する「IDC_STATIC_xxx ラベル + IDC_EDIT_xxx 入力欄」
// の対をそのまま再現する。
func mkRow(parent *FrameWidget, label string, width int) (row *FrameWidget, e *EntryWidget) {
	row = parent.Frame(Background(BgPanel))
	Pack(row, Fill(FILL_X), Side(TOP), Pady(1))
	lbl := row.Label(
		Txt(label), Font(HELVETICA, 9),
		Foreground(FgText), Background(BgPanel), Width(22), Anchor(W),
	)
	Pack(lbl, Side(LEFT), Padx(2))
	e = row.Entry(
		Font("Courier", 9), Background(BgCell), Foreground(FgText),
		Width(width), Relief(SUNKEN), Borderwidth(1),
	)
	Pack(e, Side(LEFT), Padx(4))
	return row, e
}

// entrySet はEntryに値を書き込む（編集可のウィジェット用）。
// tk9.0のTextvariableは片方向なので、go側で直接delete+insertする。
func entrySet(e *EntryWidget, v any) {
	eval.EvalErr(fmt.Sprintf("%s delete 0 end; %s insert 0 {%v}", e, e, fmt.Sprint(v)))
}

// entrySetRO は State("disabled") なEntryに値を書き込む。一時的にnormalに
// 戻して書き、再びdisabledにする。specimenダイアログのPresent欄のように
// 表示専用だが自動計算結果を反映したいセル向け。
func entrySetRO(e *EntryWidget, v any) {
	s := fmt.Sprint(v)
	eval.EvalErr(fmt.Sprintf("%s configure -state normal", e))
	eval.EvalErr(fmt.Sprintf("%s delete 0 end; %s insert 0 {%s}", e, e, s))
	eval.EvalErr(fmt.Sprintf("%s configure -state disabled", e))
}

// entryGet はEntryから文字列を取り出す。
func entryGet(e *EntryWidget) string {
	return eval.EvalErr(fmt.Sprintf("%s get", e))
}

// entryGetFloat はEntryから浮動小数点を読む。空欄や非数値は0.0扱い。
func entryGetFloat(e *EntryWidget) float64 {
	v, _ := strconv.ParseFloat(strings.TrimSpace(entryGet(e)), 64)
	return v
}

// entryGetInt はEntryから整数を読む。空欄や非整数は0扱い。
func entryGetInt(e *EntryWidget) int {
	v, _ := strconv.Atoi(strings.TrimSpace(entryGet(e)))
	return v
}

// appendTextWidget は読み込み専用のTextWidgetにテキストブロックを追記する。
// Version / WebServer / StepCtrl help など、起動時またはボタン押下時に
// 静的に流し込む用途を想定。毎回タイムスタンプを先頭に付ける。
func appendTextWidget(t *TextWidget, s string) {
	ts := time.Now().Format("2006-01-02 15:04:05.0")
	body := fmt.Sprintf("[%s]\n%s\n", ts, s)
	eval.EvalErr(fmt.Sprintf("%s configure -state normal", t))
	eval.EvalErr(fmt.Sprintf("%s insert end {%s}", t, body))
	eval.EvalErr(fmt.Sprintf("%s see end", t))
	eval.EvalErr(fmt.Sprintf("%s configure -state disabled", t))
}

// anchorForHeader は Specimen ダイアログのヘッダセル用。中央寄せに見せる
// ためのユーティリティ（先頭列だけは左寄せ）。
func anchorForHeader(i int) any {
	if i == 0 {
		return W
	}
	return CENTER
}
