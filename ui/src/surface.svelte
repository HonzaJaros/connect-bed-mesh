<!--
  The mesh as a 3D surface on a canvas. Drag to turn it, and set how much the
  heights are exaggerated: a bed is flat to a tenth of a millimetre over 25
  centimetres, so drawn to scale it would look perfectly flat.
-->
<script lang="ts">
  import { SliderField } from '@prusa/connect-ui/primitives';

  import type { Bounds, Grid } from './api';
  import { colourOf } from './colours';
  import { heldHost } from './host';

  let {
    grid,
    limit,
    bounds,
  }: { grid: Grid; limit: number; bounds?: Bounds | undefined } = $props();

  const t = heldHost().t;

  let canvas = $state<HTMLCanvasElement | undefined>();
  let width = $state(0);
  let yaw = $state(-0.6);
  let pitch = $state(1.0);
  // How tall the highest point stands: one step is a twentieth of the bed's
  // width.
  let exaggeration = $state(6);

  // The bed's own proportions when the printer printed its corners, square
  // otherwise.
  const aspect = $derived(
    bounds ? (bounds.max_y - bounds.min_y) / (bounds.max_x - bounds.min_x) : 1,
  );

  interface Point {
    x: number;
    y: number;
    z: number | null;
  }

  const points = $derived.by((): Point[][] => {
    const rows = grid.length;
    const cols = grid[0]?.length ?? 0;

    return grid.map((row, j) =>
      row.map((z, i) => ({
        x: cols > 1 ? i / (cols - 1) - 0.5 : 0,
        y: rows > 1 ? (j / (rows - 1) - 0.5) * aspect : 0,
        z: z === null ? null : (z / limit) * exaggeration * 0.05,
      })),
    );
  });

  const project = (x: number, y: number, z: number) => {
    const x1 = x * Math.cos(yaw) - y * Math.sin(yaw);
    const y1 = x * Math.sin(yaw) + y * Math.cos(yaw);

    return {
      sx: x1,
      sy: -(y1 * Math.cos(pitch) + z * Math.sin(pitch)),
      depth: y1 * Math.sin(pitch) - z * Math.cos(pitch),
    };
  };

  $effect(() => {
    if (!canvas || width === 0) return;
    const context = canvas.getContext('2d');
    if (!context) return;

    const height = Math.round(width * 0.75);
    const ratio = window.devicePixelRatio || 1;
    canvas.width = width * ratio;
    canvas.height = height * ratio;
    canvas.style.height = String(height) + 'px';
    context.setTransform(ratio, 0, 0, ratio, 0, 0);
    context.clearRect(0, 0, width, height);

    const scale = width * 0.62;
    const toScreen = (p: { sx: number; sy: number }) => [
      width / 2 + p.sx * scale,
      height / 2 + p.sy * scale,
    ];

    // The bed's outline at zero, under the surface.
    const corners = [
      [-0.5, -0.5 * aspect],
      [0.5, -0.5 * aspect],
      [0.5, 0.5 * aspect],
      [-0.5, 0.5 * aspect],
    ] as const;
    context.beginPath();
    corners.forEach(([x, y], n) => {
      const [px, py] = toScreen(project(x, y, 0));
      if (n === 0) context.moveTo(px ?? 0, py ?? 0);
      else context.lineTo(px ?? 0, py ?? 0);
    });
    context.closePath();
    context.strokeStyle = 'rgba(127, 127, 127, 0.6)';
    context.setLineDash([4, 4]);
    context.stroke();
    context.setLineDash([]);

    // Every quad whose four corners hold a value, far ones first.
    const quads: { depth: number; corners: number[][]; z: number }[] = [];
    for (let j = 0; j + 1 < points.length; j++) {
      for (let i = 0; i + 1 < (points[j]?.length ?? 0); i++) {
        const four = [
          points[j]?.[i],
          points[j]?.[i + 1],
          points[j + 1]?.[i + 1],
          points[j + 1]?.[i],
        ];
        if (four.some((p) => (p?.z ?? null) === null)) continue;

        const projected = four.map((p) =>
          project(p?.x ?? 0, p?.y ?? 0, p?.z ?? 0),
        );
        const heights = [
          grid[j]?.[i],
          grid[j]?.[i + 1],
          grid[j + 1]?.[i + 1],
          grid[j + 1]?.[i],
        ].map((z) => z ?? 0);

        quads.push({
          depth: projected.reduce((sum, p) => sum + p.depth, 0) / 4,
          corners: projected.map(toScreen),
          z: heights.reduce((sum, z) => sum + z, 0) / 4,
        });
      }
    }

    quads.sort((a, b) => b.depth - a.depth);
    context.lineWidth = 0.5;
    context.strokeStyle = 'rgba(0, 0, 0, 0.25)';
    for (const quad of quads) {
      context.beginPath();
      quad.corners.forEach(([px, py], n) => {
        if (n === 0) context.moveTo(px ?? 0, py ?? 0);
        else context.lineTo(px ?? 0, py ?? 0);
      });
      context.closePath();
      context.fillStyle = colourOf(quad.z, limit);
      context.fill();
      context.stroke();
    }

    // Which side is the front, at the middle of the front edge.
    const [fx, fy] = toScreen(project(0, -0.5 * aspect - 0.08, 0));
    context.fillStyle = 'rgba(127, 127, 127, 0.9)';
    context.font = '12px system-ui, sans-serif';
    context.textAlign = 'center';
    context.fillText(t('bedmesh.side.front'), fx ?? 0, fy ?? 0);
  });

  let dragging: { x: number; y: number } | undefined;

  const start = (event: PointerEvent) => {
    dragging = { x: event.clientX, y: event.clientY };
    (event.currentTarget as HTMLElement).setPointerCapture(event.pointerId);
  };

  const move = (event: PointerEvent) => {
    if (!dragging) return;
    yaw += (event.clientX - dragging.x) * 0.01;
    pitch = Math.min(
      Math.PI / 2,
      Math.max(0, pitch - (event.clientY - dragging.y) * 0.01),
    );
    dragging = { x: event.clientX, y: event.clientY };
  };

  const stop = () => (dragging = undefined);

  const setExaggeration = (next: number): void => {
    exaggeration = next;
  };
</script>

<div class="flex flex-col gap-2" bind:clientWidth={width}>
  <canvas
    bind:this={canvas}
    class="w-full cursor-grab touch-none active:cursor-grabbing"
    aria-label={t('bedmesh.surface.label')}
    onpointerdown={start}
    onpointermove={move}
    onpointerup={stop}
    onpointercancel={stop}
  ></canvas>
  <SliderField
    label={t('bedmesh.surface.exaggeration')}
    unit="×"
    min={1}
    max={16}
    step={1}
    value={exaggeration}
    onCommit={setExaggeration}
  />
  <p class="text-xs text-muted-foreground">{t('bedmesh.surface.hint')}</p>
</div>
