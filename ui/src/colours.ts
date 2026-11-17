// The colour scale of a mesh: blue below zero, a pale neutral at zero, red
// above. The scale is symmetric around zero, so a colour means the same height
// on both sides, and a flat bed reads pale.

const STOPS: readonly (readonly [number, readonly [number, number, number]])[] =
  [
    [0, [33, 102, 172]],
    [0.25, [103, 169, 207]],
    [0.5, [240, 240, 240]],
    [0.75, [239, 138, 98]],
    [1, [178, 24, 43]],
  ];

/** The colour of a height, on a scale running from -limit to +limit. */
export const colourOf = (z: number, limit: number): string => {
  const t = limit > 0 ? Math.min(1, Math.max(0, (z / limit + 1) / 2)) : 0.5;

  for (let k = 1; k < STOPS.length; k++) {
    const upper = STOPS[k];
    const lower = STOPS[k - 1];
    if (!upper || !lower || t > upper[0]) continue;

    const f = (t - lower[0]) / (upper[0] - lower[0]);
    const [r = 0, g = 0, b = 0] = lower[1].map((c, n) =>
      Math.round(c + ((upper[1][n] ?? c) - c) * f),
    );

    return `rgb(${String(r)} ${String(g)} ${String(b)})`;
  }

  return 'rgb(178 24 43)';
};

/** Dark text on a pale cell, light text on a deep one. */
export const inkOn = (z: number, limit: number): string =>
  limit > 0 && Math.abs(z / limit) > 0.55 ? '#ffffff' : '#1a1a1a';

/** The limit a grid is drawn to: the farthest point from zero, never zero. */
export const limitOf = (values: readonly number[]): number =>
  Math.max(0.001, ...values.map(Math.abs));
