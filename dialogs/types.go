package dialogs

import (
	"fmt"
	"sync"
	"time"
)

// ─── Channel tables ───────────────────────────────────────────────────────────
// Mirrors the MFC DigitShowBasicM reference.  Kept here (rather than in main)
// so the dialogs that show "00:LoadCell(N)" headers don't have to import
// the main package for a few lookup tables.
var (
	RawChNames = [16]string{
		"LoadCell", "LVDT", "LDT1", "LDT2",
		"none", "none", "none", "none",
		"HCDPT", "LCDPT", "none", "none",
		"none", "none", "none", "none",
	}

	PhysChNames = [16]string{
		"Load", "ExtDisp", "LDT1Disp", "LDT2Disp",
		"none", "none", "none", "none",
		"EffCellP", "VolChange", "none", "none",
		"none", "none", "none", "none",
	}

	PhysUnits = [16]string{
		"N", "mm", "mm", "mm",
		"--", "--", "--", "--",
		"kPa", "mm3", "--", "--",
		"--", "--", "--", "--",
	}

	VoltChNames = [8]string{
		"Motor ON/OFF", "Motor UP/DOWN", "Motor Speed", "EP Cell Pressure",
		"EP Axis Pressure", "Torsional ON/OFF", "Torsional CW/CCW", "Torsional Speed",
	}

	VoltUnits = [8]string{
		"on/off", "on/off", "rpm", "kPa",
		"kPa", "on/off", "dir", "rpm",
	}
)

// ─── Theme (MFC "Digital" dark teal) ───────────────────────────────────────────
// Exposed as package vars so dialogs can refer to them by short name
// (bgMain, fgAccent, ...) without dragging the tk9.0 import into this header.
var (
	BgMain   = "#0d3b3b"
	BgPanel  = "#0d3b3b"
	BgCell   = "#0a2f2f"
	BgBtn    = "#0a2f2f"
	BgRed    = "#ef5350"
	FgText   = "#ffffff"
	FgLabel  = "#cccccc"
	FgDim    = "#888888"
	FgAccent = "#4fc3f7"
	FgWarn   = "#fff176"
)

// ─── Shared types ─────────────────────────────────────────────────────────────

// CalCoeff is the per-channel quadratic y = a*x^2 + b*x + c.
//
// The same struct is reused for both AI (quadratic) and AO (linear) channels
// to keep the on-disk JSON schema simple.  For AO, only A and B are used
// (matches the C++ DigitShowModbus.h:250-253 AO struct which has no C
// term); C is always 0 there.
//
// JSON tags are lowercase to match the on-disk format written by
// DigitShowModbus's nlohmann::json serialiser.
type CalCoeff struct {
	A float64 `json:"a"`
	B float64 `json:"b"`
	C float64 `json:"c"`
}

// SpecimenStage: Initial / Present / Before / After consolidation.
// JSON tags match Dialog_Specimen.cpp:261-287.
type SpecimenStage struct {
	Diameter float64 `json:"diameter"`
	Height   float64 `json:"height"`
	Area     float64 `json:"area"`
	Volume   float64 `json:"volume"`
	LDT1     float64 `json:"ldt_1"`
	LDT2     float64 `json:"ldt_2"`
}

type SpecimenData struct {
	MembraneE  float64       `json:"membrane_youngs_modulus"`
	MembraneT  float64       `json:"membrane_thickness"`
	CapWeight  float64       `json:"cap_weight"`
	Present    SpecimenStage `json:"present"`
	Initial    SpecimenStage `json:"initial"`
	BeforeCons SpecimenStage `json:"before"`
	AfterCons  SpecimenStage `json:"after"`
}

// PreConsolidation control parameters.  JSON keys match the DigitShowModbus
// Context::Control::PreConsolication context names.
type PreConParams struct {
	TargetQ  float64 `json:"target"`
	QError   float64 `json:"error"`
	MaxSpeed float64 `json:"motor_speed"`
}

// Step Control mirrors DigitShowModbus.h:46-47 with STEP_MAX=1024, ARGS_MAX=16.
type StepCtrl struct {
	CurrentStepNo int               // runtime: which row the motor loop is executing
	EditStepNo    int               // editable: which row the dialog is editing
	ControlNo     [1024]int         // per-row control number (1024 rows)
	Args          [1024][16]float64 // per-row args (1024 rows × 16 args)
	CyclicNo      int               // runtime: cycle counter of the active cyclic step
}

// EnvVars: read from / written to os.Environ on Apply.  See AGENTS.md #7
// for why the C++ side maps each entry to a real env-var name; we don't
// push to os.Setenv in this port.
type EnvVars struct {
	Values [16]float64 `json:"values"`
	Names  [16]string  `json:"names"`
}

// AppData is the shared application state.  Both the modbus worker (in main)
// and every dialog read/write it under a single RWMutex.  Main owns the
// instance and registers it with Setup(); both sides reach it through Data.
type AppData struct {
	mu sync.RWMutex

	// Live data
	Raw    [16]int16
	Phys   [16]float64
	Params [32]float64
	Volts  [8]float64

	// Connection state
	PortStr string
	SimMode bool

	// Control state
	ControlOn   bool
	SavingOn    bool
	SaveFile    string
	SaveElapsed time.Duration
	ControlType string
	SampleTime  string
	StepNo      int
	ControlNo   int
	CyclicNo    int

	// Motor output (computed by controlLoop; would be pushed to the DA
	// board via FC16 in a production build).
	MotorSpeed float64
	MotorDir   float64

	// Persisted configuration
	// aOutCal is the per-channel D/A (AO) calibration.  Eight channels
	// matching DigitShowModbus.h:44 (DSM_AO_CH_MAX = 8) and the order of
	// the `Volts` array above: 0 Motor On/Off, 1 Motor Up/Down, 2 Motor
	// Speed, 3 EP Cell, 4 EP Axis, 5 Torsional On/Off, 6 Torsional
	// CW/CCW, 7 Torsional Speed.
	//
	// The C++ AO struct (DigitShowModbus.h:250-253) is linear
	// `out = A*V + B` (no C term).  We store it in the same CalCoeff
	// struct as the AI quadratic for JSON simplicity, but the write
	// path reads only A and B - C is always 0 here.
	AOutCal  [8]CalCoeff
	Cal      [16]CalCoeff
	Specimen SpecimenData
	PreCon   PreConParams
	StepCtrl StepCtrl
	EnvVars  EnvVars
}

// Lock/Unlock/RLock/RUnlock expose the embedded mutex as exported methods
// so other packages (main) can use appData as a black box.  Internally
// we still use mu directly to avoid the method-call overhead in hot paths.
func (d *AppData) Lock()    { d.mu.Lock() }
func (d *AppData) Unlock()  { d.mu.Unlock() }
func (d *AppData) RLock()   { d.mu.RLock() }
func (d *AppData) RUnlock() { d.mu.RUnlock() }

// ─── Package state ────────────────────────────────────────────────────────────

// Store は main から渡される AppData ポインタ。tk9.0 には Data / State /
// App / Log などの識別子が多数あるため、衝突しない Store / Logger を選んだ。
var Store *AppData

// Logger は main の appendLog コールバック。循環importを避けるため、
// ダイアログ側からはこの関数ポインタ経由でログを出す。
var Logger func(msg string)

// AO write queue: the modbusWorker (in main) and the voltage-out dialog
// (here) both need to touch this.  The dialog enqueues, the worker consumes.
var aoWriteState = struct {
	sync.Mutex
	pending [8]uint16
	dirty   bool
}{}

// QueueAOOutWrite enqueues the given 8 AO register values for the modbus
// worker to write at the next loop iteration.  A new request overwrites a
// pending one (the worker always sees the most recent values).
func QueueAOOutWrite(regs [8]uint16) {
	aoWriteState.Lock()
	aoWriteState.pending = regs
	aoWriteState.dirty = true
	aoWriteState.Unlock()
}

// ConsumeAOOutWrite returns the most recently queued AO register values
// (and true) if a write is pending, or a zero array (and false) otherwise.
func ConsumeAOOutWrite() (regs [8]uint16, ok bool) {
	aoWriteState.Lock()
	defer aoWriteState.Unlock()
	if !aoWriteState.dirty {
		return [8]uint16{}, false
	}
	aoWriteState.dirty = false
	return aoWriteState.pending, true
}

// ─── Setup ────────────────────────────────────────────────────────────────────

// Setup は main() の起動時に1回だけ呼ばれる。共有 AppData ポインタとログ
// コールバックを登録し、以降の Open* 関数 / QueueAOOutWrite はこれを
// 経由して動く。
func Setup(d *AppData, log func(msg string)) {
	if d == nil {
		panic("dialogs.Setup: nil AppData")
	}
	if log == nil {
		log = func(string) {}
	}
	Store = d
	Logger = log
}

// Logf はダイアログ側で多用する fmt.Sprintf + Logger のショートカット。
// fmt import をこのファイルに閉じ込めておくためのラッパー。
func Logf(format string, args ...any) {
	if Logger == nil {
		return
	}
	Logger(fmt.Sprintf(format, args...))
}
