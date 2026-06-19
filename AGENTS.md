# AGENTS.md - Notes for AI Agents and Future Maintainers

## Project Overview

`dsgo.exe` is a Go/Tk port of `DigitShowBasicM` (simple version) and
`DigitShowModbus` (high-end version) for Modbus-RTU based triaxial test
control & monitoring.  It is a desktop GUI for Windows, written in Go with
the `modernc.org/tk9.0` (Tk 9.0) bindings.

The reference C++ MFC implementations live at:
- `C:\Users\mkt-kuno\Desktop\DigitShowBasicM` (simple)
- `C:\Users\mkt-kuno\Desktop\DigitShowModbus` (high-end)
- `C:\Users\mkt-kuno\Desktop\DigitShowDoc` (LaTeX manual + screenshots)
- `C:\Users\mkt-kuno\Desktop\latex_doctor_by_dissertation\6th\img\digitshowmodbus`
  (newest screenshots)

## Build & Run

```sh
go build -o dsgo.exe .
./dsgo.exe
```

The binary is named **`dsgo.exe`** (NOT `helloworld.exe`).  The Go module
name is `dsgo`.

## File Layout

- `main.go` - main UI, app data structure, Modbus worker, ticker loop
- `dialogs.go` - all DigitShowModbus dialogs (Calibration, VoltageOut, ...)
- `go.mod` - Go module (`module dsgo`)
- `README.md` - end-user docs and current status
- `AGENTS.md` - this file (notes for agents)

## Key tk9.0 Idioms Used

- `Toplevel(...)` creates a new top-level window
- `Destroy(top)` closes a top-level window (it is a top-level **function**,
  not a method on `*ToplevelWidget`)
- `WmGeometry(top.Window, "WxH")` - `ToplevelWidget` embeds `*Window`, so
  pass the embedded field, not the wrapper
- `eval.EvalErr("tcl code")` runs raw Tcl and returns the result
- `AppendLog(msg)` queues a log line; the main-thread ticker flushes it
  into the spdlog-style text widget
- For per-row column-uniform widths, use `Grid + GridColumnConfigure(..., Weight(1), Uniform(tag))`

## Deliberate Deviations from DigitShowModbus / DigitShowBasicM

The user explicitly allowed not copying inefficient C++ logic verbatim. The
following are documented intentional deviations:

1. **JSON persistence** instead of `nlohmann::json` / YAML.
   The C++ reference mixes `config.yaml` for env-style values and
   `*.json` for calibration.  We use only JSON (`calibration.json`,
   `specimen.json`, `precon.json`, `stepctrl.json`, `envvars.json`)
   under `os.UserConfigDir()/DigitShowGo/`.

2. **Live mini-plot is a Tk canvas strip** rather than the
   `ChartCtrl` library.  See TODOs in README.md for the
   `Plot (Chart + LMDB?)` upgrade.

3. **Single Modbus FC04 polling loop** at 100 ms, with
   `reconnectRequested atomic.Bool` for user-triggered reconnects.
   The C++ version's `ShdController` and `SnapshotBuffer` machinery is
   not ported - the Go worker simply overwrites `appData.raw/phys/params`.

4. **`computePhys` ignores raw zero-point** unless the calibration
   `Zero` button is pressed.  The C++ side calls `OnBUTTONZero` from
   the same dialog; we expose the same button with identical math
   (`c = c - phy`).

5. **No "Update Reference Specimen Size" math**.  The Specimen dialog
   shows the Before/After Consolidation buttons but their handlers are
   `no-op` stubs (logged only).  The C++ side calls
   `LoadInput_And_Calc_AllStage` to recompute volumes/areas from
   strains - that geometric recomputation is not ported.

6. **Step Control dialog** exposes the layout (current step, control,
   16 `Args[NN]` entries, help text) but the "<-" / "->" step increment
   buttons are `no-op` stubs.  The C++ side uses these to seek into a
   multi-step control program; we don't have one yet.

7. **Environmental Variables dialog** allows reading/editing/Update
   but does NOT actually push the values to `os.Setenv`.  The C++
   side maps each entry to a real env-var name (e.g.
   `DSM_A02_Motor_Speed_A`) and updates `getenv` on Apply.

8. **No shutdown-block-reason on Windows**.  The MFC app uses
   `ShutdownBlockReasonCreate` while a control is running to prevent
   Windows killing it during a long test.

9. **Specimen dialog header** uses 9-character column titles
   ("Present", "Initial", "Before consol.", "After consol.") instead
   of the MFC's 4-column wide header; the columns themselves are the
   same 4 stages.

10. **No YAML config**.  DigitShowModbus supports loading a
    `config.yaml` to switch control profiles; the Go port uses the
    Basic Settings panel (ControlType/SamplingTime combobox) only.

11. **Linear AO calibration reuses `CalCoeff` quadratic type** instead
    of introducing a new `CalCoeffLin`.  The C++ side
    (`DigitShowModbus.h:250-253`) has two separate structs - AI is
    quadratic (`a*x^2 + b*x + c`) and AO is linear (`a*x + b`).
    The Go port stores both in the same `CalCoeff{A, B, C}` struct
    to keep the on-disk JSON schema uniform, and the AO write path
    only reads `A` and `B` (the `C` field is always 0 for AO).  See
    the `aOutCal` docstring on `AppData` in `main.go`.

12. **FC16 (Write Multiple Registers) used for AO output** instead
    of FC06 (Write Single Register).  The C++ reference writes all
    8 AO channels in a single FC16 transaction at base address
    `0x0000` (`DigitShowModbusDoc.cpp:254, 271`).  The Go port
    matches that: `writeModbusAOs(port, modbusAOBaseAddr, regs)`
    with `modbusAOBaseAddr = 0x0000`.  The base is a const so
    firmwares that expect a different mapping (e.g. `0x0100 + ch*2`)
    can be supported by changing one line.  Register scaling
    (`int16(raw*1000)`, clamp `[0, 10]V`) matches
    `DigitShowModbusDoc.cpp:317-321` (`AioUpdateOut`).

13. **AO writes go through a one-slot worker queue** rather than
    the UI thread touching `serial.Port` directly.  The
    `modbusWorker` owns the port; AO write requests from the
    `Voltage Output` dialog are funnelled through `queueAOOutWrite`
    and serviced between FC04 reads.  This keeps the serial port
    access single-threaded and means write latency is bounded by
    one worker tick (100 ms) rather than racing the read loop.

## TODOs in Priority Order

See `README.md` for the end-user facing list.  In rough order of
priority for further development:

1. **TSV saving** - file format and saving-on-button logic
2. **Step Control program execution** - parse and run a step list
3. **Specimen size recomputation** - the Before/After Consolidation math
4. **Pre-Consolidation control loop** - actual motor/load control
   (will populate `appData.aOutCal` with the C++ defaults for the
   natural-unit channels and start issuing motor / pressure / torsional
   FC16 writes from the worker on every tick)
5. **Plot (Chart + LMDB?)** - high-rate capture and rewind
6. **SQLite logging** - replace TSV with sqlite
7. **WebServer** - remote control surface
8. **AO calibration editor dialog** - currently `aOutCal` is hard-coded
   to identity (A=1, B=0) in `main()`.  Add a UI for editing the
   per-channel AO coefficients similar to the AI Calibration dialog.

## Cross-Thread Notes

- All `tk9.0` calls that touch the GUI MUST run on the main goroutine
  (the one started by `App.Wait()`).
- The Modbus worker runs on its own goroutine and pushes data via
  `appData.mu` (RWMutex).
- Log messages from the worker go through `appendLog` -> buffered
  channel -> flushed in the main-thread `updateUI` ticker.
- `requestReconnect` uses `atomic.Bool` to avoid race with the worker.
