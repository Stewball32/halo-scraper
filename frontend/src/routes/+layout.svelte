<script lang="ts">
	import '../app.css';
	import { page } from '$app/stores';
	import { base } from '$app/paths';
	import { onMount } from 'svelte';
	import { connect, disconnect, getConnected } from '$lib/stores/websocket.svelte';

	let { children } = $props();

	let isOverlay = $derived($page.url.pathname.startsWith(`${base}/overlays`));

	onMount(() => {
		connect();
		return () => disconnect();
	});

	const nav = [
		{ href: `${base}/`, label: 'Dashboard', icon: 'M3 12l2-2m0 0l7-7 7 7M5 10v10a1 1 0 001 1h3m10-11l2 2m-2-2v10a1 1 0 01-1 1h-3m-6 0a1 1 0 001-1v-4a1 1 0 011-1h2a1 1 0 011 1v4a1 1 0 001 1m-6 0h6' },
		{ href: `${base}/containers`, label: 'Containers', icon: 'M20 7l-8-4-8 4m16 0l-8 4m8-4v10l-8 4m0-10L4 7m8 4v10M4 7v10l8 4' },
		{ href: `${base}/game`, label: 'Game', icon: 'M14.752 11.168l-3.197-2.132A1 1 0 0010 9.87v4.263a1 1 0 001.555.832l3.197-2.132a1 1 0 000-1.664z M21 12a9 9 0 11-18 0 9 9 0 0118 0z' }
	];

	function isActive(href: string, path: string): boolean {
		if (href === `${base}/`) return path === `${base}/` || path === `${base}`;
		return path.startsWith(href);
	}
</script>

{#if isOverlay}
	{@render children()}
{:else}
<div class="flex h-full flex-col md:flex-row">
	<!-- Desktop sidebar -->
	<nav
		class="hidden w-14 flex-shrink-0 flex-col items-center gap-1 border-r border-border bg-card py-3 md:flex"
	>
		<span class="mb-3 text-[10px] font-bold uppercase tracking-widest text-hgold">HS</span>
		{#each nav as item}
			<a
				href={item.href}
				class="group relative flex h-10 w-10 items-center justify-center rounded transition-colors {isActive(
					item.href,
					$page.url.pathname
				)
					? 'bg-card2 text-hgold'
					: 'text-dim hover:bg-card2 hover:text-text'}"
				title={item.label}
			>
				<svg class="h-5 w-5" fill="none" stroke="currentColor" stroke-width="1.5" viewBox="0 0 24 24">
					<path stroke-linecap="round" stroke-linejoin="round" d={item.icon} />
				</svg>
				<span
					class="pointer-events-none absolute left-12 z-10 hidden whitespace-nowrap rounded bg-card2 px-2 py-1 text-[10px] text-text shadow-lg group-hover:block"
				>
					{item.label}
				</span>
			</a>
		{/each}
		<div class="mt-auto flex flex-col items-center gap-1">
			<div
				class="h-2 w-2 rounded-full {getConnected() ? 'bg-hgreen' : 'bg-hred'}"
				title={getConnected() ? 'WebSocket connected' : 'WebSocket disconnected'}
			></div>
			<span class="text-[8px] text-dim">{getConnected() ? 'WS' : '--'}</span>
		</div>
	</nav>

	<!-- Main content -->
	<main class="flex-1 overflow-hidden pb-14 md:pb-0">
		{@render children()}
	</main>

	<!-- Mobile bottom nav -->
	<nav
		class="fixed inset-x-0 bottom-0 z-40 flex items-center justify-around border-t border-border bg-card py-1 md:hidden"
	>
		{#each nav as item}
			<a
				href={item.href}
				class="flex flex-col items-center gap-0.5 px-3 py-1 {isActive(
					item.href,
					$page.url.pathname
				)
					? 'text-hgold'
					: 'text-dim'}"
			>
				<svg class="h-5 w-5" fill="none" stroke="currentColor" stroke-width="1.5" viewBox="0 0 24 24">
					<path stroke-linecap="round" stroke-linejoin="round" d={item.icon} />
				</svg>
				<span class="text-[9px]">{item.label}</span>
			</a>
		{/each}
		<div
			class="h-2 w-2 self-center rounded-full {getConnected() ? 'bg-hgreen' : 'bg-hred'}"
		></div>
	</nav>
</div>
{/if}
