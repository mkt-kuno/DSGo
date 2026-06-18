# DigitShowGo

A Go/Tk port of `DigitShowBasicM` / `DigitShowModbus` for Modbus-RTU based
triaxial test control & monitoring.

## Build

```sh
go build -o dsgo.exe .
./dsgo.exe
```

The binary is `dsgo.exe` (Go module name: `dsgo`).

The minimum supported screen size is **1600x900**.

## Current Status (v0.1.0)

### Implemented

- **Main Dialog UI** matching DigitShowModbus layout (3 data groups at
  top, Plot | spdlog | Settings row at bottom)
- **Modbus RTU** FC04 polling loop, prefers `COM12` and falls back to
  any other detected port, then to simulation
- **16-ch raw / 16-ch physical / 32-ch parameter** display with raw
  value warning colours (yellow / red)
- **8-ch Voltage Out** display below Parameter
- **Live mini-plot** (Tk canvas strip, low rate)
- **spdlog latest** text area + Mode indicator + Save: Filename row
- **Current Settings / Basic Settings** with Apply buttons
- **Step Control** fields (Step No, Control No, Elapsed, Cyclic No)
- **Start/Stop Control and Start/Stop Saving** buttons
- **Menu** with `AD Input / DA Output / Specimen / Control / Other`
  cascades (matches DigitShowModbus)
- **Sub-dialogs** (each as a Toplevel window):
  - **Calibration Value** — per-channel `a*x^2 + b*x + c` with Zero
    button (offset = -current physical value, matching C++ `c = c - phy`)
  - **Voltage Output** — 8 DA channels with Output / Refresh
  - **Specimen Data** — 4 stages (Present / Initial / Before
    consolidation / After consolidation) with diameter, height, area,
    volume, LDT1, LDT2
  - **Pre-Consolidation** — Target q / q error at max speed / max RPM
  - **Step Control** — 16 Args[NN] entries, control help text
  - **Environmental Variables** — 16 entries, Update / Update All
  - **Version** — info dialog
- **JSON-backed persistence** under
  `os.UserConfigDir()/DigitShowGo/` for:
  - `calibration.json`
  - `specimen.json`
  - `precon.json`
  - `stepctrl.json`
  - `envvars.json`

### Open TODOs (large features, deferred)

1. **Plot (Chart + LMDB?)** — high-speed chart with persistent
   preview buffer (LMDB).  Currently a low-rate Tk canvas strip.
2. **SQLite** — replace JSON / TSV with sqlite-backed logging.
3. **WebServer** — remote control surface (mirrors DigitShowModbus's
   built-in web server).
4. **Real DA voltage output** — currently the Output button is a UI
   echo only; the board write path is not wired yet.
5. **TSV saving** — file format and saving-on-button logic.
6. **Step Control program execution** — parse and run a step list
   (currently a stub).
7. **Specimen size recomputation** — the Before/After Consolidation
   math (`LoadInput_And_Calc_AllStage` in the C++ side).
8. **Pre-Consolidation control loop** — actual motor / load control.
9. **YAML profile switching** — `config.yaml` style (DigitShowModbus).
10. **ShutdownBlockReason on Windows** — graceful close during a long
    test.

## Files

- `main.go` — main UI, app data, Modbus worker, ticker loop.
- `dialogs.go` — all sub-dialogs (Calibration, VoltageOut, Specimen,
  PreConsolidation, StepControl, EnvVar, Version).
- `go.mod` — Go module `dsgo`.
- `README.md` — this file.
- `AGENTS.md` — notes for AI agents / future maintainers.

## Difference from DigitShowBasicM / DigitShowModbus

The UI follows the latest DigitShowModbus (v4.8.x) screenshots
provided in `latex_doctor_by_dissertation/6th/img/digitshowmodbus/`,
not the older DigitShowBasicM simple version.  Sub-menus follow the
same names.  See `AGENTS.md` for documented intentional deviations.
