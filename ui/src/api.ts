// The service layer: this module's own routes, reached through the client the
// host handed over, and the one core command the tab sends.
//
// `bind` comes from `bedmesh-client`, which `make client` generates from
// openapi.json. A call carries the person's session, so the process reads every
// printer as them.
import { sendCommand } from '@prusa/connect-ui/data';
import type { ModuleHost } from '@prusa/connect-ui/slots';
import { bind, type components } from 'bedmesh-client';

export type Parsed = components['schemas']['Parsed'];
export type Summary = components['schemas']['Summary'];
export type MeshRecord = components['schemas']['Record'];
export type Stats = components['schemas']['Stats'];
export type Bounds = components['schemas']['Bounds'];
/** Heights in millimetres, front row first. A null holds no value. */
export type Grid = (number | null)[][];

/** The verb Prusa Connect sends a Buddy printer as G28 then G29. */
export const MESH_BED_LEVELING = 'core.command.mesh_bed_leveling';

export const meshesOn = (host: ModuleHost) => {
  const routes = bind(host.client);

  return {
    parse: (text: string, signal?: AbortSignal): Promise<Parsed> =>
      routes.parseMesh({ body: { text } }, signal),
    list: async (deviceId: string, signal?: AbortSignal): Promise<Summary[]> =>
      (await routes.listMeshes({ path: { device_id: deviceId } }, signal))
        .meshes,
    read: (
      deviceId: string,
      meshId: string,
      signal?: AbortSignal,
    ): Promise<MeshRecord> =>
      routes.readMesh(
        { path: { device_id: deviceId, mesh_id: meshId } },
        signal,
      ),
    save: (
      deviceId: string,
      text: string,
      note: string,
      signal?: AbortSignal,
    ): Promise<MeshRecord> =>
      routes.saveMesh(
        { path: { device_id: deviceId }, body: { text, note } },
        signal,
      ),
    remove: (deviceId: string, meshId: string): Promise<void> =>
      routes.deleteMesh({ path: { device_id: deviceId, mesh_id: meshId } }),
    level: (deviceId: string) =>
      sendCommand(host.client, deviceId, MESH_BED_LEVELING),
  };
};

export type Meshes = ReturnType<typeof meshesOn>;

/** One grid minus another, point by point: what changed between two meshes. */
export const difference = (current: Grid, baseline: Grid): Grid | undefined => {
  if (
    current.length !== baseline.length ||
    current.some((row, j) => row.length !== baseline[j]?.length)
  ) {
    return undefined;
  }

  return current.map((row, j) =>
    row.map((z, i) => {
      const base = baseline[j]?.[i];

      return z === null || base === null || base === undefined
        ? null
        : Math.round((z - base) * 10000) / 10000;
    }),
  );
};

/** The statistics of a grid, as the process computes them. */
export const statsOf = (grid: Grid): Stats => {
  const values = grid.flat().filter((z): z is number => z !== null);
  const missing = grid.flat().length - values.length;

  if (values.length === 0) {
    return {
      min: 0,
      max: 0,
      range: 0,
      mean: 0,
      std_dev: 0,
      points: 0,
      missing,
    };
  }

  const min = Math.min(...values);
  const max = Math.max(...values);
  const mean = values.reduce((sum, z) => sum + z, 0) / values.length;
  const variance =
    values.reduce((sum, z) => sum + (z - mean) ** 2, 0) / values.length;

  return {
    min,
    max,
    range: max - min,
    mean,
    std_dev: Math.sqrt(variance),
    points: values.length,
    missing,
  };
};
