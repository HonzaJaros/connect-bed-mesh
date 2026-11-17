# Changelog

What changes for an operator of Bed Mesh. The top heading is the `version` in
`module.yaml`.

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
