# Bed Mesh for Prusa Connect

See your printer's **mesh bed leveling map** in Prusa Connect Local: a heatmap and a
3D surface on every printer's page, a history of every mesh you save, and a
point-by-point comparison between any two.

For printers with Prusa's **Buddy (xBuddy) firmware**: MK4 / MK4S, MK3.5, MK3.9,
Core One / Core One L, XL, MINI / MINI+.

**[User manual](https://github.com/HonzaJaros/connect-bed-mesh/blob/main/docs/MANUAL.md)**

## What it does

A **Bed Mesh** tab on every printer:

1. **Measure:** *Run mesh bed leveling* has Prusa Connect send the printer `G28` + `G29`.
2. **Read:** *Read over USB* asks the printer for `M420 V` straight from the browser
   (Chrome / Edge, Web Serial). Or paste the output of `M420 V` from any serial console.
3. **See:**
   - a heatmap seen from above, back of the bed at the top
   - a 3D surface you can turn, with adjustable height exaggeration
   - range, lowest, highest, mean and standard deviation
4. **Keep:** save the mesh with a note. The printer's model, firmware and bed temperature
   are recorded with it. Open any mesh from the history, or pick one as the baseline to see
   how far every point moved since.

English and Czech.

## Why the USB step

Buddy firmware keeps the measured mesh in its memory and prints it only on the USB serial
port. Over the Prusa Connect connection, the printer acknowledges a G-code but sends no
output back, and its telemetry carries no mesh. So the extension can start mesh bed
leveling remotely, but it cannot yet read the result remotely.

Feature request to Prusa: [prusa3d/Prusa-Firmware-Buddy#5626](https://github.com/prusa3d/Prusa-Firmware-Buddy/issues/5626).
When the firmware reports the mesh to Connect, this extension will read it automatically.

## What it asks for

| request | why |
|---|---|
| `core-device.get` | reads the printer as the person using the tab. They only see meshes of printers they may see, and a mesh records the printer's model, firmware and bed temperature |
| a data directory (`needs.data`) | the mesh history, one JSON file per mesh (1–15 KiB, depending on the grid). On a server that declares a record backend and grants no directory, the record store is used instead |

No egress: the extension makes no network connections. It starts no background work and
asks for no permissions of its own. The *Run mesh bed leveling* command is sent from the
browser as the signed-in person, so Prusa Connect's own permissions apply.

## Settings

| key | default | |
|---|---|---|
| `keep_history` | `50` | how many meshes one printer keeps; saving one more deletes the oldest |

## How it is built

- **Process** (Go, `main.go`, `routes.go`, `internal/`): reads the mesh out of the pasted
  text (`internal/mesh`, Marlin UBL's *Bed Topography Report* in its human and CSV forms),
  and keeps each printer's meshes in its data directory (`internal/store`, `internal/filerecords`).
- **Bundle** (Svelte, `ui/`): the printer tab, heatmap (SVG), 3D surface (canvas), Web
  Serial reader (`ui/src/usb.ts`).
- **API** (`openapi.json`, relative to `/api/v1/modules/com.github.aux.honzajaros.connect-bed-mesh/`):

| route | |
|---|---|
| `POST /parse` | read a mesh out of text, save nothing |
| `GET /printers/{device_id}/meshes` | a printer's meshes, newest first |
| `POST /printers/{device_id}/meshes` | read a mesh out of text and save it |
| `GET /printers/{device_id}/meshes/{mesh_id}` | one mesh with its heights |
| `DELETE /printers/{device_id}/meshes/{mesh_id}` | delete one mesh |

## Build

Needs Go (the version in `go.mod`), Node 22.12+ and pnpm 11 (`corepack enable`).

```bash
make core      # fetch the pinned Prusa Connect into .core/ (once)
make check     # Go vet and tests, the client is current, the UI's lint, types and build
make release   # release/: module.yaml, ui/, one binary per platform
make install   # install release/ into bin/try with Prusa Connect's own `module add`
```

After `make install`:

```bash
make -C .core build
```

```bash
cd bin/try && ../../.core/bin/server -config config.yaml
```

Open http://127.0.0.1:18480, finish the first run with the setup code the server logs, and
open a printer's **Bed Mesh** tab.

After changing `openapi.json`, run `make client` and commit what it writes.


## License

MIT. Built from Prusa Research's
[prusa-connect-extension-example](https://github.com/prusa3d/prusa-connect-extension-example),
MIT as well.
