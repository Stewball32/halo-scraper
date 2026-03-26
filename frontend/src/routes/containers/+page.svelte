<script lang="ts">
  import { onMount } from "svelte";
  import {
    fetchContainers,
    createContainer,
    startContainer,
    stopContainer,
    removeContainer,
    getContainerStatus,
  } from "$lib/stores/api";
  import StatusBadge from "$lib/components/shared/StatusBadge.svelte";
  import ConfirmDialog from "$lib/components/shared/ConfirmDialog.svelte";
  import GamepadControls from "$lib/components/containers/GamepadControls.svelte";
  import type { ContainerInfo } from "$lib/types";

  let containers = $state<ContainerInfo[]>([]);
  let statuses = $state<Record<string, string>>({});
  let newName = $state("");
  let creating = $state(false);
  let error = $state("");
  let actionLoading = $state<Record<string, boolean>>({});
  let expandedStream = $state<Record<string, boolean>>({});
  let confirmRemove = $state<string | null>(null);

  async function refresh() {
    try {
      containers = await fetchContainers();
      error = "";
      // Fetch statuses in parallel
      const entries = await Promise.all(
        containers.map(async (c) => {
          try {
            const s = await getContainerStatus(c.name);
            return [c.name, s] as const;
          } catch {
            return [c.name, "unknown"] as const;
          }
        }),
      );
      statuses = Object.fromEntries(entries);
    } catch (e) {
      error = e instanceof Error ? e.message : "Failed to fetch containers";
    }
  }

  async function doCreate() {
    const name = newName.trim();
    if (!name) return;
    creating = true;
    error = "";
    try {
      await createContainer(name);
      newName = "";
      await refresh();
    } catch (e) {
      error = e instanceof Error ? e.message : "Create failed";
    } finally {
      creating = false;
    }
  }

  async function doStart(name: string) {
    actionLoading[name] = true;
    try {
      await startContainer(name);
      await refresh();
    } catch (e) {
      error = e instanceof Error ? e.message : "Start failed";
    } finally {
      actionLoading[name] = false;
    }
  }

  async function doStop(name: string) {
    actionLoading[name] = true;
    try {
      await stopContainer(name);
      await refresh();
    } catch (e) {
      error = e instanceof Error ? e.message : "Stop failed";
    } finally {
      actionLoading[name] = false;
    }
  }

  async function doRemove(name: string) {
    confirmRemove = null;
    actionLoading[name] = true;
    try {
      await removeContainer(name);
      await refresh();
    } catch (e) {
      error = e instanceof Error ? e.message : "Remove failed";
    } finally {
      actionLoading[name] = false;
    }
  }

  function browserUrl(c: ContainerInfo): string {
    const host =
      typeof window !== "undefined" ? window.location.hostname : "localhost";
    return `http://${host}:${c.ports.browser_web}`;
  }

  onMount(() => {
    refresh();
    const i = setInterval(refresh, 10000);
    return () => clearInterval(i);
  });
</script>

<svelte:head>
  <title>Halo Scraper — Containers</title>
</svelte:head>

<ConfirmDialog
  open={confirmRemove !== null}
  title="Remove Container"
  message="This will stop and permanently remove the container pair. Are you sure?"
  confirmLabel="Remove"
  onconfirm={() => confirmRemove && doRemove(confirmRemove)}
  oncancel={() => (confirmRemove = null)}
/>

<div class="flex h-full flex-col gap-4 overflow-y-auto p-4">
  <h1 class="text-sm font-bold uppercase tracking-wider text-hgold">
    Containers
  </h1>

  <!-- Create form -->
  <div class="flex flex-wrap items-center gap-2">
    <input
      type="text"
      bind:value={newName}
      placeholder="Container name..."
      class="rounded border border-border bg-card px-3 py-1.5 text-xs text-text placeholder:text-dim focus:border-hgold focus:outline-none"
      onkeydown={(e) => e.key === "Enter" && doCreate()}
    />
    <button
      class="rounded border border-hgreen/30 bg-hgreen/10 px-3 py-1.5 text-xs text-hgreen transition-colors hover:bg-hgreen/20 disabled:opacity-50"
      disabled={creating || !newName.trim()}
      onclick={doCreate}
    >
      {creating ? "Creating..." : "Create"}
    </button>
  </div>

  {#if error}
    <div
      class="rounded border border-hred/30 bg-hred/10 px-3 py-2 text-xs text-hred"
    >
      {error}
    </div>
  {/if}

  <!-- Container list -->
  <div class="grid grid-cols-1 gap-3 lg:grid-cols-2">
    {#each containers.toSorted( (a, b) => a.name.localeCompare(b.name), ) as c (c.name)}
      {@const st = statuses[c.name] ?? "unknown"}
      {@const loading = actionLoading[c.name] ?? false}
      {@const isRunning = st === "running"}
      <div class="flex flex-col gap-2 rounded border border-border bg-card p-3">
        <!-- Header -->
        <div class="flex items-center gap-2">
          <span class="text-xs font-bold text-text">{c.name}</span>
          <StatusBadge state={st} />
          {#if loading}
            <span class="text-[10px] text-dim">working...</span>
          {/if}
        </div>

        <!-- Ports -->
        <div class="flex flex-wrap gap-x-4 gap-y-1 text-[10px] text-dim">
          <span>HTTP: <span class="text-text">{c.ports.xemu_http}</span></span>
          <span>HTTPS: <span class="text-text">{c.ports.xemu_https}</span></span
          >
          <span>WS: <span class="text-text">{c.ports.xemu_ws}</span></span>
          <span
            >Browser: <span class="text-text">{c.ports.browser_web}</span></span
          >
          <span>VNC: <span class="text-text">{c.ports.browser_vnc}</span></span>
        </div>

        <div class="text-[10px] text-dim">
          Created: {new Date(c.created).toLocaleString()}
        </div>

        <!-- Actions -->
        <div class="flex flex-wrap items-center gap-2">
          {#if !isRunning}
            <button
              class="rounded border border-hgreen/30 bg-hgreen/10 px-2 py-1 text-[11px] text-hgreen transition-colors hover:bg-hgreen/20 disabled:opacity-50"
              disabled={loading}
              onclick={() => doStart(c.name)}
            >
              Start
            </button>
          {:else}
            <button
              class="rounded border border-hyellow/30 bg-hyellow/10 px-2 py-1 text-[11px] text-hyellow transition-colors hover:bg-hyellow/20 disabled:opacity-50"
              disabled={loading}
              onclick={() => doStop(c.name)}
            >
              Stop
            </button>
          {/if}
          <button
            class="rounded border border-hred/30 bg-hred/10 px-2 py-1 text-[11px] text-hred transition-colors hover:bg-hred/20 disabled:opacity-50"
            disabled={loading}
            onclick={() => (confirmRemove = c.name)}
          >
            Remove
          </button>

          {#if isRunning}
            <a
              href={browserUrl(c)}
              target="_blank"
              rel="noopener"
              class="ml-auto text-[11px] text-hblue hover:underline"
            >
              Open Stream
            </a>
            <button
              class="text-[11px] text-dim hover:text-text"
              onclick={() => (expandedStream[c.name] = !expandedStream[c.name])}
            >
              {expandedStream[c.name] ? "Hide" : "Embed"}
            </button>
          {/if}
        </div>

        <!-- Stream embed -->
        {#if isRunning && expandedStream[c.name]}
          <div class="mt-1 overflow-hidden rounded border border-border">
            <iframe
              src={browserUrl(c)}
              title="{c.name} stream"
              class="h-100 w-full bg-black md:h-125"
              allowfullscreen
            ></iframe>
          </div>
          <GamepadControls name={c.name} vncPort={c.ports.browser_web} />
        {/if}
      </div>
    {/each}

    {#if containers.length === 0 && !error}
      <div class="text-xs text-dim">No containers. Create one above.</div>
    {/if}
  </div>
</div>
