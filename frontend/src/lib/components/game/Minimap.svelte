<script lang="ts">
	import type { SnapshotPlayer, TickPlayer, PowerItemStatus, PowerItemSpawn } from '$lib/types';

	let {
		players,
		tickPlayers,
		powerItems,
		powerSpawns,
		isTeamGame
	}: {
		players: SnapshotPlayer[];
		tickPlayers: Record<number, TickPlayer>;
		powerItems: PowerItemStatus[];
		powerSpawns: PowerItemSpawn[];
		isTeamGame: boolean;
	} = $props();

	// FFA color palette for non-team games
	const ffaColors = [
		'#e05050', '#4d9fff', '#40cc60', '#e0c040',
		'#e07840', '#40d8c8', '#a060e0', '#f0c840',
		'#ff70b0', '#70ff70', '#7080ff', '#ff9060',
		'#c080ff', '#80ffd0', '#ffa0a0', '#a0d0ff'
	];

	function playerColor(sp: SnapshotPlayer): string {
		if (isTeamGame) return sp.team === 0 ? '#e05050' : '#4d9fff';
		return ffaColors[sp.index % ffaColors.length];
	}

	function piStatusColor(status: string): string {
		if (status === 'world') return '#40cc60';
		if (status === 'held') return '#e0c040';
		return '#e05050';
	}

	// Compute bounding box from all known positions (spawns anchor the bounds to prevent jitter)
	let bounds = $derived.by(() => {
		const pts: { x: number; y: number }[] = [];

		for (const tp of Object.values(tickPlayers)) {
			if (tp.alive) pts.push({ x: tp.x, y: tp.y });
		}
		for (const pi of powerItems) {
			if (pi.world_pos) pts.push({ x: pi.world_pos.x, y: pi.world_pos.y });
		}
		for (const sp of powerSpawns) {
			pts.push({ x: sp.x, y: sp.y });
		}

		if (pts.length === 0) return null;

		let minX = Infinity, maxX = -Infinity, minY = Infinity, maxY = -Infinity;
		for (const p of pts) {
			minX = Math.min(minX, p.x);
			maxX = Math.max(maxX, p.x);
			minY = Math.min(minY, p.y);
			maxY = Math.max(maxY, p.y);
		}

		const w = Math.max(maxX - minX, 10);
		const h = Math.max(maxY - minY, 10);
		const pad = Math.max(w, h) * 0.12;

		return {
			x: minX - pad,
			y: -(maxY + pad), // negate Y for SVG (Halo +Y up → SVG +Y down)
			w: w + 2 * pad,
			h: h + 2 * pad
		};
	});

	let dotR = $derived(bounds ? Math.max(bounds.w, bounds.h) * 0.018 : 1);
	let aimLen = $derived(bounds ? Math.max(bounds.w, bounds.h) * 0.04 : 2);
	let fontSize = $derived(bounds ? Math.max(bounds.w, bounds.h) * 0.025 : 1);
	let piSize = $derived(bounds ? Math.max(bounds.w, bounds.h) * 0.012 : 0.5);
</script>

{#if bounds}
	<svg
		class="h-full w-full rounded border border-border bg-[#0a0a14]"
		viewBox="{bounds.x} {bounds.y} {bounds.w} {bounds.h}"
		preserveAspectRatio="xMidYMid meet"
	>
		<!-- Power item spawns -->
		{#each powerItems as pi}
			{@const spawn = powerSpawns.find((s) => s.spawn_id === pi.spawn_id)}
			{@const pos = pi.status === 'world' && pi.world_pos ? pi.world_pos : spawn}
			{#if pos}
				<rect
					x={pos.x - piSize}
					y={-pos.y - piSize}
					width={piSize * 2}
					height={piSize * 2}
					fill={piStatusColor(pi.status)}
					opacity={pi.status === 'respawning' ? 0.3 : 0.8}
					rx={piSize * 0.3}
				/>
			{/if}
		{/each}

		<!-- Players -->
		{#each players as sp (sp.index)}
			{@const tp = tickPlayers[sp.index]}
			{#if tp}
				{@const color = playerColor(sp)}
				<g>
					{#if tp.alive}
						<!-- Aim direction line -->
						{@const mag = Math.sqrt(tp.aim_x * tp.aim_x + tp.aim_y * tp.aim_y)}
						{#if mag > 0.001}
							<line
								x1={tp.x}
								y1={-tp.y}
								x2={tp.x + (tp.aim_x / mag) * aimLen}
								y2={-tp.y - (tp.aim_y / mag) * aimLen}
								stroke={color}
								stroke-width={dotR * 0.4}
								opacity="0.6"
							/>
						{/if}
						<!-- Player dot -->
						<circle
							cx={tp.x}
							cy={-tp.y}
							r={dotR}
							fill={color}
							stroke="#000"
							stroke-width={dotR * 0.15}
						/>
						{#if tp.has_camo}
							<circle
								cx={tp.x}
								cy={-tp.y}
								r={dotR * 1.5}
								fill="none"
								stroke="#a060e0"
								stroke-width={dotR * 0.2}
								stroke-dasharray="{dotR * 0.5} {dotR * 0.5}"
								opacity="0.6"
							/>
						{/if}
						{#if tp.has_overshield}
							<circle
								cx={tp.x}
								cy={-tp.y}
								r={dotR * 1.4}
								fill="none"
								stroke="#8840d0"
								stroke-width={dotR * 0.25}
								opacity="0.5"
							/>
						{/if}
					{:else}
						<!-- Dead player — hollow circle -->
						<circle
							cx={tp.x}
							cy={-tp.y}
							r={dotR * 0.7}
							fill="none"
							stroke={color}
							stroke-width={dotR * 0.2}
							opacity="0.3"
						/>
					{/if}
					<!-- Player name label -->
					<text
						x={tp.x + dotR * 1.8}
						y={-tp.y + fontSize * 0.35}
						fill={tp.alive ? color : '#666680'}
						font-size={fontSize}
						opacity={tp.alive ? 0.9 : 0.4}
					>
						{sp.name}
					</text>
				</g>
			{/if}
		{/each}
	</svg>
{:else}
	<div class="flex h-full items-center justify-center rounded border border-border bg-[#0a0a14] text-[10px] text-dim">
		No position data
	</div>
{/if}
