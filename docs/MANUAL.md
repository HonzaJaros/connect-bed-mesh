# Bed Mesh for Prusa Connect: user manual

Bed Mesh adds a **Bed Mesh** tab to every printer in Prusa Connect Local. It shows
your printer's mesh bed leveling map as a heatmap and as a 3D surface, keeps a
history of every mesh you save, and compares any two of them.

It works with printers running Prusa's **Buddy (xBuddy) firmware**, which are
the printers Prusa Connect Local supports: MK4 / MK4S, MK3.5 / MK3.5S, MK3.9 /
MK3.9S, Core One / Core One L, XL and MINI / MINI+.

- [Why there is a USB step](#why-there-is-a-usb-step)
- [Install](#install)
- [Quick start](#quick-start)
- [Step 1: Measure the bed](#step-1-measure-the-bed)
- [Step 2: Get the mesh](#step-2-get-the-mesh)
- [Step 3: Save it](#step-3-save-it)
- [Reading the map](#reading-the-map)
- [History and comparing](#history-and-comparing)
- [Settings](#settings)
- [Troubleshooting](#troubleshooting)

## Why there is a USB step

The printer keeps the measured mesh in its memory. Today the only way to read
it is to ask over the printer's USB serial port with `M420 V`. Prusa Connect
can tell the printer to *measure* the bed, but the printer does not send the
result back to Connect.

We have asked Prusa to add this:
[prusa3d/Prusa-Firmware-Buddy#5626](https://github.com/prusa3d/Prusa-Firmware-Buddy/issues/5626).
When the firmware and Prusa Connect support it, the extension will read the
mesh automatically and the USB step will go away.

## Install

1. In Prusa Connect Local, open **Extensions**, find **Bed Mesh** and choose
   **Install**.
2. Read the install dialog. Bed Mesh asks for:
   - **Reading a printer** (`core-device.get`): it reads each printer as the
     person using the tab. That way you only see meshes of printers you may
     see, and a saved mesh records the printer's model, firmware and bed
     temperature.
   - **A data folder** (`needs.data`): where the mesh history is kept, one
     small file per mesh.
   - It makes **no network connections** and starts no background work.
3. Open any printer. The **Bed Mesh** tab appears after the built-in tabs.

## Quick start

1. Heat the bed to the temperature you print at.
2. On the printer's **Bed Mesh** tab, choose **Run mesh bed leveling** and
   confirm. The printer homes and probes the bed.
3. Plug the printer into your computer with a USB cable. Choose **Read over
   USB**, then pick the printer's port.
4. Check the preview, write a note (for example "satin sheet, 60 °C"), and
   choose **Save to history**.

## Step 1: Measure the bed

**Run mesh bed leveling** asks Prusa Connect to send the printer `G28` (home)
and `G29` (mesh bed leveling). It is the same command Prusa Connect's own
printer menu uses.

Before you run it:

- **Clear the bed.** The nozzle will touch down on many points.
- **Heat the bed** to your printing temperature and give it a few minutes. A
  bed changes shape as it heats, so a cold mesh does not show the bed you print
  on. The bed temperature is saved with the mesh, so you can compare like with
  like later.
- **Clean the nozzle.** Filament on the nozzle tip spoils the probing on
  printers that probe with the nozzle (MK4, Core One, XL).

You can also skip this step and read the mesh the printer measured at the start
of its last print. That mesh stays in the printer's memory until it is switched
off.

> The button is refused when the printer is printing or busy, or when your
> account may not operate the printer.

## Step 2: Get the mesh

### Option A: Read over USB (Chrome or Edge)

1. Connect the printer to the computer you are using with a USB cable. Use
   the printer's USB port on the electronics box, not the USB-A slot for flash
   drives.
2. On the **Bed Mesh** tab, choose **Read over USB**.
3. The browser asks which port to use. Pick the printer. It usually appears as
   "Original Prusa…", "USB Serial Device" (Windows), `cu.usbmodem…` (macOS) or
   `ttyACM0` (Linux).
4. The page sends `M420 V`, waits for the report, and closes the port. It sends
   nothing else.

The button is there only when **all** of these are true:

- the browser is **Chrome, Edge or Opera on a computer**. Firefox, Safari and
  phones have no Web Serial.
- Prusa Connect is opened over **HTTPS**, or on `localhost`. Browsers allow
  serial ports only on secure pages. A Connect Local opened as
  `http://192.168.x.x` is not secure, so the tab offers pasting instead. To
  get the button, open Connect over HTTPS, or open it on the computer that runs
  it (`http://localhost:…`).
- no other program holds the port. Close PrusaSlicer's firmware flasher,
  Pronterface, OctoPrint or a terminal first.

On Linux, your user may need to be in the `dialout` group to open the port.

### Option B: Paste from a serial console

Any serial console works. Connect at **115200 baud**, send `M420 V`, copy
everything the printer printed, and paste it into the box on the tab.

**Pronterface** (Windows, macOS, Linux):

1. Pick the printer's port, set 115200, choose **Connect**.
2. Type `M420 V` in the box at the bottom right and press Enter.
3. Select the output in the log, copy it, and paste it into the tab.

**macOS / Linux terminal**:

```bash
screen /dev/cu.usbmodem* 115200
```

Type `M420 V` and press Enter. (You will not see what you type.) Copy the
output. Leave `screen` with `Ctrl+A`, then `K`, then `Y`.

**Windows**: use PuTTY with *Connection type: Serial*, the printer's COM port
and speed 115200.

### What the printer prints

The paste box understands the printer's report as it comes out. Console
prefixes such as `Recv:`, `echo:` and timestamps are fine:

```text
Bed Topography Report:

    (  2,206)                                   (242,206)
         0      1      2  …     20
20 | +0.012  -0.004  +0.020 …
   |
 …
 0 | -0.081  -0.060  -0.042 …
         0      1      2  …     20
    (  2, -4)                                   (242, -4)
echo:Bed Leveling ON
```

`M420 V T1` (the CSV version of the same report) works too.

As soon as you paste, the tab shows a **preview**. Nothing is stored until you
save.

To try the extension without a printer or a cable, paste
[sample-m420v-mk4.txt](sample-m420v-mk4.txt): a made-up 21 × 21 report in the
exact format an MK4 prints.

## Step 3: Save it

Write a note if you like (the sheet, the temperature, what you changed), and
choose **Save to history**. The mesh is stored in Prusa Connect, on your
server, with:

- the time,
- the printer's name, model and firmware,
- the bed temperature Prusa Connect last read from the printer.

## Reading the map

**Heatmap.** The bed seen from above, with the **back of the bed at the top**,
just as you see it standing in front of the printer. Each cell is one probed
point:

- **red** is above zero (higher than average, the nozzle is closer),
- **blue** is below zero (lower than average),
- **pale** is close to zero.

The colour scale is symmetric around zero and runs to the point farthest from
zero, shown under the map. On small grids each cell shows its value; on large
grids (the XL's 36 × 36) hover over a cell to see it. An empty cell is a point
the printer did not measure.

**3D surface.** Drag to turn the bed. **Height exaggeration** makes the hills
taller: a bed that is flat to a tenth of a millimetre would look perfectly
flat at true scale.

**Numbers.**

| | what it means |
|---|---|
| **Range** | highest point minus lowest point. The single number to watch. |
| Lowest / Highest | the extreme points, relative to the printer's zero |
| Mean | the average height |
| Std. dev. | how far points typically are from the mean: a low number is an even bed |

Mesh bed leveling compensates for the shape it measures, so a bed does not have
to be perfectly flat to print well. A range that **grows over time**, or a
sudden step in one corner, usually means something to look at: a loose or
damaged sheet, debris under the sheet, a bed heater that pulled the bed, or a
loose screw.

## History and comparing

The **History** lists every saved mesh of this printer, newest first.

- **Open** shows a saved mesh.
- **Compare against** picks a mesh as the **baseline**. The map then shows
  how far **each point moved** since the baseline: red rose, blue sank. Use it
  to see what a new sheet, a bed adjustment or a warm-up actually changed.
  Choose **Stop comparing** to go back. Two meshes are compared only when
  their grids match.
- **Delete** removes one mesh.

Each printer keeps its newest **50** meshes. Saving one more drops the oldest.
An operator can change the number (see [Settings](#settings)).

## Settings

An operator sets these in the extension's configuration:

| key | default | what it does |
|---|---|---|
| `keep_history` | `50` | how many meshes one printer keeps |

## Troubleshooting

**"No bed mesh was found in the pasted text."**
Paste everything from the line `Bed Topography Report` to the last row of
numbers. If the printer answered `Invalid mesh.`, it holds no mesh: run
**Run mesh bed leveling** first (or start a print), then ask again.

**There is no "Read over USB" button.**
See the conditions under [Option A](#option-a-read-over-usb-chrome-or-edge).
Pasting always works.

**"The printer could not be read over USB."**
Check the cable (some USB cables carry power only), and close any other
program that has the port open. Unplug and plug the cable back in, then try
again.

**The browser lists no port for the printer.**
Try another cable or another USB port on the computer. On Windows, make sure
the printer appears in Device Manager under *Ports (COM & LPT)*.

**Run mesh bed leveling is refused.**
The printer must be idle and connected to Prusa Connect, and your account must
be allowed to operate it.

**The map looks upside down.**
The back of the bed is at the top of the heatmap, and "Front" is marked on the
3D view. The rows come from the printer's own report, which labels every row
with its position.

## Privacy

Meshes are stored only in your own Prusa Connect, in the extension's own
records. The extension makes no connection outside your server. Reading over
USB happens in your browser, between the page and the printer.
