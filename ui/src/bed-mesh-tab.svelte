<!--
  The Bed Mesh tab of a printer's page.

  1. Run mesh bed leveling: Prusa Connect sends the printer G28 and G29.
  2. Paste what the printer printed for M420 V into the console box. The
     process reads the mesh out of it and the tab previews it at once.
  3. Save it into the printer's history, with a note.

  The mesh shows as a heatmap seen from above and as a 3D surface. Any saved
  mesh can be opened again, or chosen as the baseline: the tab then draws how
  far every point moved since.
-->
<script lang="ts">
  import {
    type FaultText,
    formatDateTime,
    formatNumber,
  } from '@prusa/connect-ui/i18n';
  import {
    Badge,
    Button,
    Card,
    CardContent,
    CardHeader,
    CardTitle,
    FaultAlert,
    Input,
    Skeleton,
    Textarea,
    toast,
  } from '@prusa/connect-ui/primitives';

  import {
    type Bounds,
    difference,
    type Grid,
    type MeshRecord,
    meshesOn,
    type Parsed,
    statsOf,
    type Summary,
  } from './api';
  import { colourOf, limitOf } from './colours';
  import { faultText } from './faults';
  import Heatmap from './heatmap.svelte';
  import { heldHost } from './host';
  import Surface from './surface.svelte';
  import { readOverUsb, usbAvailable } from './usb';

  let { deviceId }: { deviceId: string } = $props();

  /** The user manual, on the extension's repository. */
  const MANUAL_URL =
    'https://github.com/HonzaJaros/connect-bed-mesh/blob/main/docs/MANUAL.md';

  const host = heldHost();
  const t = host.t;
  const meshes = meshesOn(host);

  // What the person pasted, and what the process read out of it.
  let text = $state('');
  let note = $state('');
  let preview = $state<Parsed | undefined>();
  let pasteFault = $state<FaultText | undefined>();
  let saving = $state(false);

  // The printer's history, and the mesh open from it.
  let history = $state<Summary[] | undefined>();
  let historyFault = $state<FaultText | undefined>();
  let opened = $state<MeshRecord | undefined>();
  let baseline = $state<MeshRecord | undefined>();

  // Running mesh bed leveling asks once before it moves the printer.
  let confirming = $state(false);
  let levelling = $state(false);

  const reload = async () => {
    try {
      history = await meshes.list(deviceId, host.signal);
      historyFault = undefined;
    } catch (thrown) {
      if (!host.signal.aborted) historyFault = faultText(thrown);
    }
  };

  $effect(() => {
    void reload();
  });

  // A paste is read as soon as the person stops typing.
  let timer: ReturnType<typeof setTimeout> | undefined;
  $effect(() => {
    const pasted = text;
    clearTimeout(timer);
    if (pasted.trim() === '') {
      preview = undefined;
      pasteFault = undefined;

      return;
    }

    timer = setTimeout(() => {
      meshes
        .parse(pasted, host.signal)
        .then((parsed) => {
          if (pasted !== text) return;
          preview = parsed;
          pasteFault = undefined;
          opened = undefined;
        })
        .catch((thrown: unknown) => {
          if (pasted !== text || host.signal.aborted) return;
          preview = undefined;
          pasteFault = faultText(thrown);
        });
    }, 300);
  });

  const typeNote = (event: Event): void => {
    if (event.currentTarget instanceof HTMLInputElement) {
      note = event.currentTarget.value;
    }
  };

  const save = async () => {
    saving = true;
    try {
      const saved = await meshes.save(deviceId, text, note.trim(), host.signal);
      text = '';
      note = '';
      preview = undefined;
      opened = saved;
      toast.success(t('bedmesh.saved'));
      await reload();
    } catch (thrown) {
      if (!host.signal.aborted) pasteFault = faultText(thrown);
    } finally {
      saving = false;
    }
  };

  const open = async (summary: Summary, asBaseline = false) => {
    try {
      const record = await meshes.read(deviceId, summary.id, host.signal);
      if (asBaseline) baseline = record;
      else {
        opened = record;
        preview = undefined;
      }
    } catch (thrown) {
      if (!host.signal.aborted) toast.error(faultText(thrown).title);
    }
  };

  const remove = async (summary: Summary) => {
    try {
      await meshes.remove(deviceId, summary.id);
      if (opened?.id === summary.id) opened = undefined;
      if (baseline?.id === summary.id) baseline = undefined;
      await reload();
    } catch (thrown) {
      toast.error(faultText(thrown).title);
    }
  };

  // The browser's own serial port, when it has one: the printer's answer to
  // M420 V lands in the console box as if it had been pasted.
  const usb = usbAvailable();
  let reading = $state(false);

  const readUsb = async () => {
    reading = true;
    try {
      text = await readOverUsb('M420 V', (sofar) => (text = sofar));
    } catch (thrown) {
      // Closing the browser's port picker is not a failure.
      if (!(
        thrown instanceof DOMException && thrown.name === 'NotFoundError'
      )) {
        toast.error(t('bedmesh.usb.failed'));
      }
    } finally {
      reading = false;
    }
  };

  const level = async () => {
    confirming = false;
    levelling = true;
    try {
      await meshes.level(deviceId);
      toast.success(t('bedmesh.level.started'));
    } catch (thrown) {
      toast.error(faultText(thrown).title);
    } finally {
      levelling = false;
    }
  };

  // What is drawn: the preview of a paste, or a mesh opened from the history,
  // and minus the baseline when one is chosen.
  const shown = $derived.by(
    ():
      | { grid: Grid; bounds?: Bounds | undefined; compared: boolean }
      | undefined => {
      const current = preview ?? opened;
      if (!current) return undefined;

      const grid = current.z;
      if (baseline && baseline.id !== opened?.id) {
        const changed = difference(grid, baseline.z);
        if (changed)
          return { grid: changed, bounds: current.bounds, compared: true };
      }

      return { grid, bounds: current.bounds, compared: false };
    },
  );

  const stats = $derived(shown ? statsOf(shown.grid) : undefined);
  const limit = $derived(
    shown
      ? limitOf(shown.grid.flat().filter((z): z is number => z !== null))
      : 1,
  );
  const mismatched = $derived(
    baseline !== undefined &&
      shown !== undefined &&
      !shown.compared &&
      baseline.id !== opened?.id,
  );

  const mm = (value: number, signed = false): string =>
    t('bedmesh.unit.mm', {
      value: formatNumber(value, {
        minimumFractionDigits: 3,
        maximumFractionDigits: 3,
        signDisplay: signed ? 'exceptZero' : 'auto',
      }),
    });
</script>

<div class="flex flex-col gap-4">
  <!-- 1. Measure -->
  <Card>
    <CardHeader>
      <CardTitle>{t('bedmesh.level.title')}</CardTitle>
    </CardHeader>
    <CardContent class="flex flex-col gap-3">
      <p class="text-sm text-muted-foreground">{t('bedmesh.level.body')}</p>
      <div class="flex flex-wrap items-center gap-2">
        {#if confirming}
          <span class="text-sm">{t('bedmesh.level.confirm')}</span>
          <Button variant="destructive" onclick={level}>
            {t('bedmesh.level.yes')}
          </Button>
          <Button variant="ghost" onclick={() => (confirming = false)}>
            {t('bedmesh.cancel')}
          </Button>
        {:else}
          <Button
            variant="secondary"
            disabled={levelling}
            onclick={() => (confirming = true)}
          >
            {t('bedmesh.level.run')}
          </Button>
        {/if}
      </div>
    </CardContent>
  </Card>

  <!-- 2. Paste -->
  <Card>
    <CardHeader>
      <CardTitle>{t('bedmesh.paste.title')}</CardTitle>
    </CardHeader>
    <CardContent class="flex flex-col gap-3">
      <p class="text-sm text-muted-foreground">{t('bedmesh.paste.body')}</p>
      <p class="text-xs text-muted-foreground">
        {t('bedmesh.paste.why')}
        <a
          class="text-link underline-offset-4 hover:underline"
          href={MANUAL_URL}
          target="_blank"
          rel="noopener noreferrer">{t('bedmesh.manual')}</a
        >
      </p>
      <div class="flex flex-wrap items-center gap-2">
        {#if usb}
          <Button variant="secondary" disabled={reading} onclick={readUsb}>
            {reading ? t('bedmesh.usb.reading') : t('bedmesh.usb.read')}
          </Button>
          <span class="text-xs text-muted-foreground"
            >{t('bedmesh.usb.hint')}</span
          >
        {:else}
          <span class="text-xs text-muted-foreground"
            >{t('bedmesh.usb.unavailable')}</span
          >
        {/if}
      </div>
      <Textarea
        class="min-h-40 font-mono text-xs"
        spellcheck={false}
        placeholder={t('bedmesh.paste.placeholder')}
        aria-label={t('bedmesh.paste.title')}
        bind:value={text}
      />
      {#if pasteFault}
        <FaultAlert fault={pasteFault} />
      {/if}
      <div class="flex flex-wrap items-center gap-2">
        <Input
          class="max-w-md flex-1"
          maxlength={500}
          placeholder={t('bedmesh.paste.note')}
          aria-label={t('bedmesh.paste.note')}
          value={note}
          oninput={typeNote}
        />
        <Button disabled={!preview || saving} onclick={save}>
          {t('bedmesh.paste.save')}
        </Button>
      </div>
    </CardContent>
  </Card>

  <!-- The mesh -->
  {#if shown && stats}
    <Card>
      <CardHeader>
        <CardTitle>
          {#if shown.compared && baseline}
            {t('bedmesh.view.compared', {
              when: formatDateTime(baseline.saved_at),
            })}
          {:else if preview}
            {t('bedmesh.view.preview')}
          {:else if opened}
            {t('bedmesh.view.saved', { when: formatDateTime(opened.saved_at) })}
          {/if}
        </CardTitle>
      </CardHeader>
      <CardContent class="flex flex-col gap-4">
        <div class="flex flex-wrap gap-2">
          <Badge variant="secondary">
            {t('bedmesh.stats.range', { value: mm(stats.range) })}
          </Badge>
          <Badge variant="outline">
            {t('bedmesh.stats.min', { value: mm(stats.min, true) })}
          </Badge>
          <Badge variant="outline">
            {t('bedmesh.stats.max', { value: mm(stats.max, true) })}
          </Badge>
          <Badge variant="outline">
            {t('bedmesh.stats.mean', { value: mm(stats.mean, true) })}
          </Badge>
          <Badge variant="outline">
            {t('bedmesh.stats.std_dev', { value: mm(stats.std_dev) })}
          </Badge>
          <Badge variant="outline">
            {t('bedmesh.stats.points', {
              cols: shown.grid[0]?.length ?? 0,
              rows: shown.grid.length,
              missing: stats.missing,
            })}
          </Badge>
          {#if baseline}
            <Button
              size="sm"
              variant="ghost"
              onclick={() => (baseline = undefined)}
            >
              {t('bedmesh.view.clear_baseline')}
            </Button>
          {/if}
        </div>
        {#if mismatched}
          <p class="text-sm text-muted-foreground">
            {t('bedmesh.view.mismatched')}
          </p>
        {/if}
        <div class="grid gap-6 lg:grid-cols-2">
          <Heatmap grid={shown.grid} {limit} />
          <Surface grid={shown.grid} {limit} bounds={shown.bounds} />
        </div>
        <div class="flex items-center gap-2 text-xs text-muted-foreground">
          <span>{mm(-limit, true)}</span>
          <div
            class="rounded h-2 flex-1"
            style:background={`linear-gradient(to right, ${colourOf(-limit, limit)}, ${colourOf(0, limit)}, ${colourOf(limit, limit)})`}
          ></div>
          <span>{mm(limit, true)}</span>
        </div>
      </CardContent>
    </Card>
  {/if}

  <!-- 3. History -->
  <Card>
    <CardHeader>
      <CardTitle>{t('bedmesh.history.title')}</CardTitle>
    </CardHeader>
    <CardContent>
      {#if historyFault}
        <FaultAlert fault={historyFault} />
      {:else if history === undefined}
        <Skeleton class="h-16 w-full" />
      {:else if history.length === 0}
        <p class="text-sm text-muted-foreground">
          {t('bedmesh.history.empty')}
        </p>
      {:else}
        <ul class="flex flex-col divide-y divide-border">
          {#each history as summary (summary.id)}
            <li
              class="flex flex-wrap items-center gap-x-4 gap-y-1 py-2 text-sm"
            >
              <span class="min-w-40 font-medium">
                {formatDateTime(summary.saved_at)}
              </span>
              <span class="tabular-nums">
                {t('bedmesh.stats.range', { value: mm(summary.stats.range) })}
              </span>
              {#if summary.printer.bed_temp !== undefined}
                <span class="text-muted-foreground">
                  {t('bedmesh.history.bed', {
                    value: formatNumber(summary.printer.bed_temp, {
                      maximumFractionDigits: 0,
                    }),
                  })}
                </span>
              {/if}
              {#if summary.note}
                <span class="flex-1 truncate text-muted-foreground">
                  {summary.note}
                </span>
              {:else}
                <span class="flex-1"></span>
              {/if}
              <span class="flex gap-1">
                <Button size="sm" variant="ghost" onclick={() => open(summary)}>
                  {t('bedmesh.history.open')}
                </Button>
                <Button
                  size="sm"
                  variant="ghost"
                  disabled={baseline?.id === summary.id}
                  onclick={() => open(summary, true)}
                >
                  {t('bedmesh.history.baseline')}
                </Button>
                <Button
                  size="sm"
                  variant="ghost"
                  onclick={() => remove(summary)}
                >
                  {t('bedmesh.history.delete')}
                </Button>
              </span>
            </li>
          {/each}
        </ul>
      {/if}
    </CardContent>
  </Card>
</div>
