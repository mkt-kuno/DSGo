package dialogs

// ─── Voltage Output ダイアログ ───────────────────────────────────────────────
//
// C++ の IDD_DA_Voltage 相当。8チャンネル分の手動電圧値を編集して
// 「Output」ボタンで DA ボードへ FC16 書き込みを発行する。
//
// 書き込み要求は即座に serial.Port に触れず、QueueAOOutWrite 経由で
// modbusWorker の次のループまで遅延される。これによりUIスレッドが
// シリアルポートの読み書きループと競合しない。
// (AGENTS.md「#13 AO writes go through a one-slot worker queue」を参照)

import (
	"fmt"

	. "modernc.org/tk9.0"
)

// OpenVoltageOut は Voltage Output ダイアログを開く。
// 同じダイアログを2つ開いたとき Entry ウィジェットが共有されないよう、
// ローカル変数 voltEntries に保持する。
func OpenVoltageOut() {
	top, body := makeDialogShell("Voltage Output on DA Board", 380, 400)

	// ヘッダ行
	head := body.Frame(Background(BgPanel))
	Pack(head, Fill(FILL_X), Side(TOP))
	lbl := head.Label(
		Txt(""), Font(HELVETICA, 9), Background(BgPanel), Width(20), Anchor(W),
	)
	Pack(lbl, Side(LEFT), Padx(2))
	lbl = head.Label(
		Txt("Voltage (V)"), Font(HELVETICA, 9, BOLD),
		Foreground(FgAccent), Background(BgPanel), Width(12), Anchor(E),
	)
	Pack(lbl, Side(LEFT), Padx(2))

	// ローカルの voltEntries（ダイアログインスタンスごとに独立）
	var voltEntries [8]*EntryWidget

	// 右側ボタン列
	right := body.Frame(Background(BgPanel))
	Pack(right, Side(RIGHT), Fill(FILL_Y), Padx(4))
	refBtn := right.Button(
		Txt("Refresh"), Font(HELVETICA, 9),
		Background(BgBtn), Foreground(FgText), Width(8),
		Command(func() { onRefreshVoltage(voltEntries) }),
	)
	outBtn := right.Button(
		Txt("Output"), Font(HELVETICA, 9),
		Background(BgBtn), Foreground(FgText), Width(8),
		Command(func() { onOutputVoltage(voltEntries) }),
	)
	closeBtn := right.Button(
		Txt("Close"), Font(HELVETICA, 9, BOLD),
		Background(BgRed), Foreground("#ffffff"), Width(8),
		Command(func() { Destroy(top) }),
	)
	Pack(refBtn, Side(TOP), Pady(2))
	Pack(outBtn, Side(TOP), Pady(2))
	Pack(closeBtn, Side(TOP), Pady(20))

	// 左側: 8チャンネル分の編集行
	left := body.Frame(Background(BgCell), Relief(SUNKEN), Borderwidth(1))
	Pack(left, Side(LEFT), Fill(FILL_BOTH), Expand(true), Padx(2))

	Store.RLock()
	volts := Store.Volts
	Store.RUnlock()

	for i := 0; i < 8; i++ {
		row := left.Frame(Background(BgCell))
		Pack(row, Fill(FILL_X), Side(TOP), Pady(1))
		lbl := row.Label(
			Txt(fmt.Sprintf("%02d:%s", i, VoltChNames[i])),
			Font(HELVETICA, 8), Foreground(FgLabel),
			Background(BgCell), Width(22), Anchor(W),
		)
		Pack(lbl, Side(LEFT), Padx(2))
		voltEntries[i] = row.Entry(
			Font("Courier", 9), Background(BgPanel), Foreground(FgText),
			Width(8), Relief(SUNKEN), Borderwidth(1), Justify(RIGHT),
		)
		entrySet(voltEntries[i], volts[i])
		Pack(voltEntries[i], Side(LEFT), Padx(4))
	}
}

// onOutputVoltage は 8 チャンネル分の電圧を Entry から読み取り、
// 各チャンネルに AOutCal を適用したうえでクランプ→DAC count に変換し
// キューに積む。C++ の AioUpdateOut (DigitShowModbusDoc.cpp:310-323) と
// 同じ数学:
//
//	raw    = clamp(A*V + B, 0, 10)        // V
//	reg[i] = uint16(raw * 1000)           // 0..10000 DAC counts
func onOutputVoltage(e [8]*EntryWidget) {
	var (
		registers [8]uint16
		voltsSnap [8]float64
	)
	Store.Lock()
	for i := 0; i < 8; i++ {
		Store.Volts[i] = entryGetFloat(e[i])
		voltsSnap[i] = Store.Volts[i]
		cal := Store.AOutCal[i]
		raw := cal.A*Store.Volts[i] + cal.B
		if raw < 0 {
			raw = 0
		}
		if raw > 10 {
			raw = 10
		}
		registers[i] = uint16(raw * 1000)
	}
	Store.Unlock()

	QueueAOOutWrite(registers)
	Logf("[dac] voltage out queued: V=[%.3f %.3f %.3f %.3f %.3f %.3f %.3f %.3f] reg=%v",
		voltsSnap[0], voltsSnap[1], voltsSnap[2], voltsSnap[3],
		voltsSnap[4], voltsSnap[5], voltsSnap[6], voltsSnap[7],
		registers)
}

// onRefreshVoltage は Store.Volts の最新値を Entry に再表示する。
func onRefreshVoltage(e [8]*EntryWidget) {
	Store.RLock()
	volts := Store.Volts
	Store.RUnlock()
	for i := 0; i < 8; i++ {
		entrySet(e[i], volts[i])
	}
	Logger("[dac] refreshed")
}
