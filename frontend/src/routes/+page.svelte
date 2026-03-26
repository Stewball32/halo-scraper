<script lang="ts">
	import { onMount } from 'svelte';
	import { fetchStatus, fetchLogs, reconnectHost } from '$lib/stores/api';
	import { fmtDuration, sinceStr } from '$lib/utils/format';
	import StatusBadge from '$lib/components/shared/StatusBadge.svelte';
	import type { StatusResponse } from '$lib/types';

	let status = $state<StatusResponse | null>(null);
	let logs = $state<string[]>([]);
	let uptimeOffset = $state(0);
	let statusFetchedAt = $state(Date.now());

	async function refreshStatus() {
		try {
			status = await fetchStatus();
			statusFetchedAt = Date.now();
		} catch {
			// silent
		}
	}

	async function refreshLogs() {
		try {
			logs = await fetchLogs();
		} catch {
			// silent
		}
	}

	async function doReconnect(name: string) {
		await reconnectHost(name);
		await refreshStatus();
	}

	onMount(() => {
		refreshStatus();
		refreshLogs();
		const si = setInterval(refreshStatus, 5000);
		const li = setInterval(refreshLogs, 3000);
		const ti = setInterval(() => {
			uptimeOffset = Math.round((Date.now() - statusFetchedAt) / 1000);
		}, 1000);
		return () => {
			clearInterval(si);
			clearInterval(li);
			clearInterval(ti);
		};
	});

	function logClass(line: string): string {
		const l = line.toLowerCase();
		if (l.includes('failed') || l.includes('error') || l.includes('fatal')) return 'text-hred';
		if (l.includes('ready') || l.includes('online')) return 'text-hgreen';
		if (l.includes('config:') || l.includes('shutting')) return 'text-dim';
		return 'text-text';
	}

	let hostEntries = $derived(
		status ? Object.entries(status.hosts).sort(([a], [b]) => a.localeCompare(b)) : []
	);
</script>

<svelte:head>
	<title>Xemu Cartographer — Dashboard</title>
</svelte:head>

<div class="flex h-full flex-col gap-4 overflow-y-auto p-4">
	<!-- Server Stats -->
	<div class="flex flex-wrap items-center gap-x-6 gap-y-2 text-xs">
		<span class="text-dim">
			Uptime:
			<span class="text-text">{status ? fmtDuration(status.uptime_s + uptimeOffset) : '—'}</span>
		</span>
		<span class="text-dim">
			WS Clients:
			<span class="text-text">{status?.ws_clients ?? '—'}</span>
		</span>
	</div>

	<!-- Host Status Cards -->
	<section>
		<h2 class="mb-2 border-b border-border pb-1 text-[10px] uppercase tracking-wider text-dim">
			Host Status
		</h2>
		<div class="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-3">
			{#each hostEntries as [name, host]}
				<div class="flex flex-col gap-1.5 rounded border border-border bg-card p-3">
					<div class="text-xs font-bold text-text">{name}</div>
					<div class="flex items-center gap-2">
						<StatusBadge state={host.state} />
					</div>
					{#if host.error}
						<div class="wrap-break-word text-[10px] text-hred">{host.error}</div>
					{/if}
					<div class="text-[10px] text-dim">{host.state} for {sinceStr(host.since)}</div>
					<button
						class="self-start rounded border border-border bg-card2 px-2.5 py-1 text-[11px] text-text transition-colors hover:bg-border disabled:cursor-default disabled:text-dim"
						disabled={host.state === 'connecting'}
						onclick={() => doReconnect(name)}
					>
						Reconnect
					</button>
				</div>
			{/each}
			{#if hostEntries.length === 0}
				<div class="text-xs text-dim">No hosts registered</div>
			{/if}
		</div>
	</section>

	<!-- Logs -->
	<section class="flex min-h-0 flex-1 flex-col">
		<h2 class="mb-2 border-b border-border pb-1 text-[10px] uppercase tracking-wider text-dim">
			Scraper Log
		</h2>
		<div
			class="flex flex-1 flex-col-reverse overflow-y-auto rounded border border-border bg-card p-2 text-[11px] leading-relaxed"
			style="max-height: 400px;"
		>
			{#each [...logs].reverse() as line}
				<div class="whitespace-pre-wrap break-all {logClass(line)}">{line}</div>
			{/each}
			{#if logs.length === 0}
				<div class="text-dim">No logs yet</div>
			{/if}
		</div>
	</section>
</div>
