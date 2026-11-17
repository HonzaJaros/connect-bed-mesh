# Changelog

What changes for an operator of Bed Mesh. The top heading is the `version` in
`module.yaml`.

## 0.1.5

**Bed Mesh** shows the mesh bed leveling map of your printer (MK4/MK4S, MK3.5,
MK3.9, Core One, XL, MINI) on its page in Prusa Connect Local: a heatmap seen
from above, a 3D surface you can turn, the range from the lowest to the highest
point, and a history of every mesh you save, with a point-by-point comparison
between any two.

**How to use it.** Open a printer and choose the **Bed Mesh** tab.

1. *Measure:* heat the bed, make sure it is clear, and choose **Run mesh bed
   leveling**. Prusa Connect sends the printer G28 and G29.
2. *Read the mesh:* plug the printer into your computer with a USB cable and
   choose **Read over USB** (Chrome or Edge, with Connect opened over HTTPS or
   on localhost). Or send `M420 V` from any serial console (Pronterface, or a
   terminal at 115200 baud) and paste what the printer printed.
3. *Save:* add a note, such as the sheet or what you changed, and choose
   **Save to history**. Open any saved mesh, or compare against it to see how
   far every point moved.

**Why the USB step.** The printer keeps the measured mesh in its memory and, for
now, prints it only on its USB port. Over Prusa Connect it acknowledges G-code
but sends no output back, and its telemetry carries no mesh. We have asked
Prusa to send the mesh to Connect:
[Prusa-Firmware-Buddy#5626](https://github.com/prusa3d/Prusa-Firmware-Buddy/issues/5626).
When the firmware does, Bed Mesh will read it automatically and the USB step
goes away.

Full manual: https://github.com/HonzaJaros/connect-bed-mesh/blob/main/docs/MANUAL.md

New in 0.1.5: the same extension as 0.1.4, built from a repository that no
longer carries a key pair committed there by mistake (it was never used to
sign anything). Since 0.1.4 the tab explains the USB step and links the manual.

## 0.1.4

**Bed Mesh** shows the mesh bed leveling map of your printer (MK4/MK4S, MK3.5,
MK3.9, Core One, XL, MINI) on its page in Prusa Connect Local: a heatmap seen
from above, a 3D surface you can turn, the range from the lowest to the highest
point, and a history of every mesh you save, with a point-by-point comparison
between any two.

**How to use it.** Open a printer and choose the **Bed Mesh** tab.

1. *Measure:* heat the bed, make sure it is clear, and choose **Run mesh bed
   leveling**. Prusa Connect sends the printer G28 and G29.
2. *Read the mesh:* plug the printer into your computer with a USB cable and
   choose **Read over USB** (Chrome or Edge, with Connect opened over HTTPS or
   on localhost). Or send `M420 V` from any serial console (Pronterface, or a
   terminal at 115200 baud) and paste what the printer printed.
3. *Save:* add a note, such as the sheet or what you changed, and choose
   **Save to history**. Open any saved mesh, or compare against it to see how
   far every point moved.

**Why the USB step.** The printer keeps the measured mesh in its memory and, for
now, prints it only on its USB port. Over Prusa Connect it acknowledges G-code
but sends no output back, and its telemetry carries no mesh. We have asked
Prusa to send the mesh to Connect:
[Prusa-Firmware-Buddy#5626](https://github.com/prusa3d/Prusa-Firmware-Buddy/issues/5626).
When the firmware does, Bed Mesh will read it automatically and the USB step
goes away.

Full manual: https://github.com/HonzaJaros/connect-bed-mesh/blob/main/docs/MANUAL.md

New in 0.1.4: the tab explains the USB step and links the manual.

## 0.1.3

- The same extension as 0.1.2. The marketplace upload of 0.1.2 failed: the
  release workflow did not pass the signing key to Prusa's builder.

## 0.1.2

- The extension id is now `com.github.aux.honzajaros.connect-bed-mesh`, from
  the repository `github.com/honzajaros/connect-bed-mesh`: the marketplace
  reserves ids that contain "prusa".
- Otherwise the same extension as 0.1.1. The marketplace build of 0.1.1 failed before
  publishing: the client package did not pin its pnpm version, so the builder
  used a newer pnpm that refused the lockfile.

## 0.1.1

- The mesh history is kept in the extension's own data directory. 0.1.0 used
  the server's record store, which a server set up by the first-run wizard
  does not serve, so the history answered "not available".

## 0.1.0

- A **Bed Mesh** tab on every printer with Buddy firmware.
- *Run mesh bed leveling* sends the printer G28 and G29 through Prusa Connect.
- *Read over USB* reads `M420 V` straight from the printer in Chrome and Edge.
  The output of `M420 V` can also be pasted from any serial console.
- A heatmap and a 3D surface of the mesh, with its range, lowest and highest
  points, mean and standard deviation.
- A history per printer (50 meshes by default, setting `keep_history`), with
  notes, the printer's model, firmware and bed temperature, and a comparison
  against any saved mesh.
- English and Czech.
