package dialogs

// ─── JSON永続化 ──────────────────────────────────────────────────────────────
//
// 設定ファイル（calibration.json / specimen.json / precon.json /
// stepctrl.json / envvars.json）は os.UserConfigDir()/DigitShowGo/ の下に
// すべてJSONで保存する。C++のDigitShowModbusがconfig.yaml + 各種JSONを
// 混在させていたのに対し、本Go版ではフォーマットをJSONに統一している
// (AGENTS.md「#1 JSON persistence」を参照)。

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
)

// configDir は設定ファイルの保存先ディレクトリを返す。
// os.UserConfigDir() が失敗した場合はカレントディレクトリにフォールバック。
func configDir() string {
	if d, err := os.UserConfigDir(); err == nil {
		return filepath.Join(d, "DigitShowGo")
	}
	return "."
}

// configPath は設定ファイル名（拡張子付き）を受け取り、フルパスを返す。
func configPath(name string) string { return filepath.Join(configDir(), name) }

// ConfigPath は configPath のエクスポート版。main 側からも使える。
func ConfigPath(name string) string { return configPath(name) }

// ensureConfigDir は保存先ディレクトリを冪等に作成する。
func ensureConfigDir() { _ = os.MkdirAll(configDir(), 0o755) }

// loadJSON はファイルを読んでJSONデコードする。存在しない場合は
// os.ReadFile のエラーをそのまま返す。
func loadJSON(name string, v any) error {
	b, err := os.ReadFile(configPath(name))
	if err != nil {
		return err
	}
	return json.Unmarshal(b, v)
}

// saveJSON はディレクトリをensureしたうえでJSONをIndent=2で書き出す。
func saveJSON(name string, v any) error {
	ensureConfigDir()
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(configPath(name), b, 0o644)
}

// SaveJSON は saveJSON のエクスポート版。main の saveEnvVarsIfMissing
// などからも使える。
func SaveJSON(name string, v any) error { return saveJSON(name, v) }

// ─── ファイルフォーマット用ラッパ ────────────────────────────────────────────
//
// on-diskのJSONは {"cal":[...]} / {"specimen":{...}} / {"precon":{...}} /
// {"env":{...}} のように1階層ラップされている。Goの構造体を直接 unmarshal
// すると匿名フィールドしか読めないので、名前付きフィールドで挟む。

type calibrationFile struct {
	Cal [16]CalCoeff `json:"cal"`
}
type specimenFile struct {
	Specimen SpecimenData `json:"specimen"`
}
type preConFile struct {
	PreCon PreConParams `json:"precon"`
}
type envVarsFile struct {
	Env EnvVars `json:"env"`
}

// EnvVarsFile は envVarsFile のエクスポート版。main 側から envvars.json
// を直接書き出す際（初回起動時の seed など）に使う。
type EnvVarsFile = envVarsFile

// ─── Step Control の on-disk フォーマット ───────────────────────────────────
//
// DigitShowModbus.h:46-47 の DSM_STEPCTRL_STEP_MAX=1024, ARGS_MAX=16 に対応し、
// トップレベルが {"type":"StepControl", "0000":{...}, "0001":{...}, ...} と
// 1024個の連番キーで各行を表現するJSONを出力する。Goの固定長配列で
// メモリに持つが、JSONでは疎な map に変換する。
const dsmStepCtrlMax = 1024

type stepCtrlFileEntry struct {
	Ctrl   int     `json:"ctrl"`
	Args00 float64 `json:"args00"`
	Args01 float64 `json:"args01"`
	Args02 float64 `json:"args02"`
	Args03 float64 `json:"args03"`
	Args04 float64 `json:"args04"`
	Args05 float64 `json:"args05"`
	Args06 float64 `json:"args06"`
	Args07 float64 `json:"args07"`
	Args08 float64 `json:"args08"`
	Args09 float64 `json:"args09"`
	Args10 float64 `json:"args10"`
	Args11 float64 `json:"args11"`
	Args12 float64 `json:"args12"`
	Args13 float64 `json:"args13"`
	Args14 float64 `json:"args14"`
	Args15 float64 `json:"args15"`
}

// args は stepCtrlFileEntry を [16]float64 に展開して返す。
func (e stepCtrlFileEntry) args() [16]float64 {
	return [16]float64{
		e.Args00, e.Args01, e.Args02, e.Args03,
		e.Args04, e.Args05, e.Args06, e.Args07,
		e.Args08, e.Args09, e.Args10, e.Args11,
		e.Args12, e.Args13, e.Args14, e.Args15,
	}
}

// makeStepCtrlFileEntry は [16]float64 の args を stepCtrlFileEntry に
// 展開して新しいエントリを作る。
func makeStepCtrlFileEntry(ctrl int, args [16]float64) stepCtrlFileEntry {
	return stepCtrlFileEntry{
		Ctrl:   ctrl,
		Args00: args[0], Args01: args[1], Args02: args[2], Args03: args[3],
		Args04: args[4], Args05: args[5], Args06: args[6], Args07: args[7],
		Args08: args[8], Args09: args[9], Args10: args[10], Args11: args[11],
		Args12: args[12], Args13: args[13], Args14: args[14], Args15: args[15],
	}
}

type stepCtrlFile struct {
	Type    string                       `json:"type"`
	Entries map[string]stepCtrlFileEntry `json:"-"`
}

// MarshalJSON は {"type":..., "0000":{...}, ...} のトップレベルオブジェクトに
// 展開するカスタムマーシャラ。
func (f *stepCtrlFile) MarshalJSON() ([]byte, error) {
	m := make(map[string]any, len(f.Entries)+1)
	m["type"] = f.Type
	for k, v := range f.Entries {
		m[k] = v
	}
	return json.Marshal(m)
}

// UnmarshalJSON はトップレベルオブジェクトを type と Entries に分解する
// カスタムアンマーシャラ。キーが "0000".."1023" の範囲外なら無視。
func (f *stepCtrlFile) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	f.Entries = make(map[string]stepCtrlFileEntry, len(raw))
	for k, v := range raw {
		if k == "type" {
			if err := json.Unmarshal(v, &f.Type); err != nil {
				return err
			}
			continue
		}
		var e stepCtrlFileEntry
		if err := json.Unmarshal(v, &e); err != nil {
			return err
		}
		f.Entries[k] = e
	}
	return nil
}

// stepCtrlToFile はメモリ上の StepCtrl を stepCtrlFile 形式に変換する。
// 1024 行すべてを "0000".."1023" キーで map に出力する。
func stepCtrlToFile(sc *StepCtrl) *stepCtrlFile {
	f := &stepCtrlFile{
		Type:    "StepControl",
		Entries: make(map[string]stepCtrlFileEntry, dsmStepCtrlMax),
	}
	for i := 0; i < dsmStepCtrlMax; i++ {
		key := fmt.Sprintf("%04d", i)
		f.Entries[key] = makeStepCtrlFileEntry(sc.ControlNo[i], sc.Args[i])
	}
	return f
}

// fileToStepCtrl は stepCtrlFile をメモリ上の StepCtrl に変換する。
// 存在しない行はゼロ値のまま（ControlNo=0, Args=[0]*16）。
func fileToStepCtrl(f *stepCtrlFile) StepCtrl {
	var sc StepCtrl
	for k, e := range f.Entries {
		idx, err := strconv.Atoi(k)
		if err != nil || idx < 0 || idx >= dsmStepCtrlMax {
			continue
		}
		sc.ControlNo[idx] = e.Ctrl
		sc.Args[idx] = e.args()
	}
	return sc
}

// saveStepCtrlJSON は stepctrl.json に書き出す。
func saveStepCtrlJSON(sc *StepCtrl) error {
	return saveJSON("stepctrl.json", stepCtrlToFile(sc))
}

// loadStepCtrlJSON は stepctrl.json を読み込む。ファイルが無ければ
// os.ReadFile のエラーがそのまま返る。
func loadStepCtrlJSON() (StepCtrl, error) {
	var f stepCtrlFile
	if err := loadJSON("stepctrl.json", &f); err != nil {
		return StepCtrl{}, err
	}
	return fileToStepCtrl(&f), nil
}

// clampStepNo は [0, dsmStepCtrlMax) の範囲に丸める。
func clampStepNo(n int) int {
	if n < 0 {
		return 0
	}
	if n >= dsmStepCtrlMax {
		return dsmStepCtrlMax - 1
	}
	return n
}

// LoadAllConfigs は起動時に5種類のJSONを一括で読み込み、Store に流し込む。
// 個別のファイルが存在しなくてもエラーにはせず、ログだけ残して続行する。
// （最初の起動時は全部無いので、これは正常系）
func LoadAllConfigs() {
	if Store == nil {
		// Setup が呼ばれていないときは何もしない（テスト時の便宜）
		return
	}

	var cf calibrationFile
	if err := loadJSON("calibration.json", &cf); err == nil {
		Store.Lock()
		Store.Cal = cf.Cal
		Store.Unlock()
		Logger("[config] calibration.json loaded")
	}
	var sf specimenFile
	if err := loadJSON("specimen.json", &sf); err == nil {
		Store.Lock()
		Store.Specimen = sf.Specimen
		Store.Unlock()
		Logger("[config] specimen.json loaded")
	}
	var pf preConFile
	if err := loadJSON("precon.json", &pf); err == nil {
		Store.Lock()
		Store.PreCon = pf.PreCon
		Store.Unlock()
		Logger("[config] precon.json loaded")
	}
	if sc, err := loadStepCtrlJSON(); err == nil {
		Store.Lock()
		Store.StepCtrl = sc
		Store.Unlock()
		Logger("[config] stepctrl.json loaded")
	}
	var ef envVarsFile
	if err := loadJSON("envvars.json", &ef); err == nil {
		Store.Lock()
		Store.EnvVars = ef.Env
		Store.Unlock()
		Logger("[config] envvars.json loaded")
	}
}
