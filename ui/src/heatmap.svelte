<!--
  The mesh seen from above: one cell per point, the back of the bed at the
  top, the colour of each cell its height. A small grid prints its values in
  the cells; a large one (the XL's 36 by 36) shows a value on hover.
-->
<script lang="ts">
  import { formatNumber } from '@prusa/connect-ui/i18n';

  import type { Grid } from './api';
  import { colourOf, inkOn } from './colours';
  import { heldHost } from './host';

  let { grid, limit }: { grid: Grid; limit: number } = $props();

  const t = heldHost().t;
  const CELL = 40;

  const rows = $derived(grid.length);
  const cols = $derived(grid[0]?.length ?? 0);
  const labelled = $derived(cols <= 13);
  // Back row first: the bed as a person standing in front of it sees it.
  const drawn = $derived([...grid].reverse());

  const value = (z: number): string =>
    formatNumber(z, {
      minimumFractionDigits: 3,
      maximumFractionDigits: 3,
      signDisplay: 'exceptZero',
    });
</script>

<figure class="flex flex-col gap-1">
  <figcaption class="text-center text-xs text-muted-foreground">
    {t('bedmesh.side.back')}
  </figcaption>
  <svg
    class="w-full"
    viewBox={[0, 0, cols * CELL, rows * CELL].join(' ')}
    role="img"
    aria-label={t('bedmesh.heatmap.label', { cols, rows })}
  >
    {#each drawn as row, j (j)}
      {#each row as z, i (i)}
        <g>
          <title
            >{z === null
              ? t('bedmesh.heatmap.empty', { col: i, row: rows - 1 - j })
              : t('bedmesh.heatmap.point', {
                  col: i,
                  row: rows - 1 - j,
                  value: value(z),
                })}</title
          >
          <rect
            x={i * CELL}
            y={j * CELL}
            width={CELL}
            height={CELL}
            fill={z === null ? 'transparent' : colourOf(z, limit)}
            class={z === null ? 'stroke-border' : ''}
            stroke-width="0.5"
          />
          {#if labelled && z !== null}
            <text
              x={i * CELL + CELL / 2}
              y={j * CELL + CELL / 2}
              fill={inkOn(z, limit)}
              font-size="10"
              text-anchor="middle"
              dominant-baseline="central">{value(z)}</text
            >
          {/if}
        </g>
      {/each}
    {/each}
  </svg>
  <figcaption class="text-center text-xs text-muted-foreground">
    {t('bedmesh.side.front')}
  </figcaption>
</figure>
