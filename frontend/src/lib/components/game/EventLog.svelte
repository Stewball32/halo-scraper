<script lang="ts">
	import type { EventEntry, SnapshotPayload } from '$lib/types';
	import { eventColor, describeEvent } from '$lib/utils/format';

	let {
		events,
		snapshots,
		instanceFilter
	}: {
		events: EventEntry[];
		snapshots: Record<string, SnapshotPayload>;
		instanceFilter: string | null;
	} = $props();

	let filtered = $derived(
		instanceFilter ? events.filter((e) => e.instance === instanceFilter) : events
	);

	function playerNames(instance: string): Record<number, string> {
		const snap = snapshots[instance];
		if (!snap) return {};
		const names: Record<number, string> = {};
		for (const p of snap.players) {
			names[p.index] = p.name;
		}
		return names;
	}
</script>

<div class="flex h-full flex-col overflow-hidden">
	<div
		class="flex items-center gap-2 border-b border-border bg-card2 px-2 py-1 text-[10px] uppercase tracking-wide text-dim"
	>
		Events
		<span class="ml-auto">{filtered.length}</span>
	</div>
	<div class="flex-1 overflow-y-auto">
		{#each filtered as ev (ev.timestamp + ev.tick + ev.payload.event_type)}
			<div
				class="flex items-baseline gap-1 border-b border-[#141420] px-2 py-0.5 text-[10px] leading-relaxed hover:bg-[#161622]"
			>
				<span class="w-12 shrink-0 text-[9px] text-dim">{ev.tick}</span>
				{#if !instanceFilter}
					<span class="w-16 shrink-0 overflow-hidden text-ellipsis text-[9px] text-dim"
						>{ev.instance}</span
					>
				{/if}
				<span class="w-20 shrink-0 text-[9px] {eventColor(ev.payload.event_type)}"
					>{ev.payload.event_type}</span
				>
				<span class="min-w-0 flex-1 overflow-hidden text-ellipsis whitespace-nowrap text-text">
					{describeEvent(ev.payload as Record<string, unknown>, playerNames(ev.instance))}
				</span>
			</div>
		{/each}
		{#if filtered.length === 0}
			<div class="px-2 py-4 text-center text-[10px] text-dim">No events yet</div>
		{/if}
	</div>
</div>
