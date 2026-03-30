<script lang="ts">
	import type { SnapshotPlayer, TickPlayer, WeaponInfo } from '$lib/types';
	import { shortWeapon } from '$lib/utils/format';

	let {
		players,
		tickPlayers,
		teamFilter,
		isTeamGame,
		gametype = ''
	}: {
		players: SnapshotPlayer[];
		tickPlayers: Record<number, TickPlayer>;
		teamFilter: number | null;
		isTeamGame: boolean;
		gametype?: string;
	} = $props();

	let filtered = $derived(
		teamFilter !== null ? (players ?? []).filter((p) => p.team === teamFilter) : (players ?? [])
	);

	let showCtf = $derived(gametype.toLowerCase().includes('ctf'));

	function hpBarColor(hp: number): string {
		if (hp > 0.6) return 'bg-hgreen';
		if (hp > 0.3) return 'bg-[#c8b030]';
		return 'bg-[#c83030]';
	}

	function shBarColor(_sh: number, hasOvershield: boolean): string {
		return hasOvershield ? 'bg-[#8840d0]' : 'bg-[#3a80d0]';
	}

	function teamColor(team: number): string {
		if (!isTeamGame) return 'text-text';
		return team === 0 ? 'text-hred' : 'text-hblue';
	}

	function selectedWeapon(tp: TickPlayer): WeaponInfo | null {
		if (!tp.weapons?.length) return null;
		return tp.weapons.find((w) => w.slot === tp.selected_weapon_slot) ?? tp.weapons[0] ?? null;
	}

	function weaponDisplay(w: WeaponInfo): string {
		const name = shortWeapon(w.tag);
		if (w.is_energy) {
			return w.charge != null ? `${name} ${Math.round((w.charge ?? 0) * 100)}%` : name;
		}
		return w.ammo_mag != null ? `${name} ${w.ammo_mag}/${w.ammo_pack ?? 0}` : name;
	}

	function accuracy(sp: SnapshotPlayer): string {
		if (!sp.shots_fired || sp.shots_fired === 0) return '-';
		return Math.round((sp.shots_hit / sp.shots_fired) * 100) + '%';
	}

	function respawnSec(tp: TickPlayer): string {
		if (tp.alive || tp.respawn_in_ticks == null) return '';
		return Math.ceil(tp.respawn_in_ticks / 30) + 's';
	}

	function fmtCoord(v: number): string {
		return v.toFixed(1);
	}

	function velocity(tp: TickPlayer): string {
		const mag = Math.sqrt(tp.vx * tp.vx + tp.vy * tp.vy + tp.vz * tp.vz);
		return mag.toFixed(1);
	}
</script>

<div class="overflow-x-auto">
	<table class="w-full border-collapse text-[10px]">
		<thead>
			<tr class="bg-card2 text-[9px] uppercase tracking-wide text-dim">
				<th class="whitespace-nowrap px-1 py-0.5 text-left">#</th>
				<th class="px-0.5 py-0.5"></th>
				<th class="whitespace-nowrap px-1 py-0.5 text-left">Name</th>
				<th class="whitespace-nowrap px-1 py-0.5 text-left">HP / SH</th>
				<th class="whitespace-nowrap px-1 py-0.5 text-left">K/D/A</th>
				{#if showCtf}
					<th class="whitespace-nowrap px-1 py-0.5 text-left">CTF</th>
				{/if}
				<th class="whitespace-nowrap px-1 py-0.5 text-left">TK</th>
				<th class="whitespace-nowrap px-1 py-0.5 text-left">Sui</th>
				<th class="whitespace-nowrap px-1 py-0.5 text-left">Str</th>
				<th class="whitespace-nowrap px-1 py-0.5 text-left">MK</th>
				<th class="whitespace-nowrap px-1 py-0.5 text-left">Acc</th>
				<th class="whitespace-nowrap px-1 py-0.5 text-left">Weapon</th>
				<th class="whitespace-nowrap px-1 py-0.5 text-left">Inv</th>
				<th class="whitespace-nowrap px-1 py-0.5 text-left">Nd</th>
				<th class="whitespace-nowrap px-1 py-0.5 text-left">Resp</th>
				<th class="whitespace-nowrap px-1 py-0.5 text-left">Pos</th>
				<th class="whitespace-nowrap px-1 py-0.5 text-left">Vel</th>
				<th class="whitespace-nowrap px-1 py-0.5 text-left">Zm</th>
				<th class="whitespace-nowrap px-1 py-0.5 text-left">Flags</th>
			</tr>
		</thead>
		<tbody>
			{#each filtered as sp (sp.index)}
				{@const tp = tickPlayers[sp.index]}
				{@const alive = tp?.alive ?? false}
				{@const dead = tp && !tp.alive}
				<tr class="border-b border-[#1c1c2a] {dead ? 'opacity-50' : ''} {!tp ? 'opacity-10' : ''}">
					<!-- Index -->
					<td class="whitespace-nowrap px-1 py-0.5 text-dim">{sp.index}</td>
					<!-- Alive indicator -->
					<td class="px-0.5 py-0.5">
						{#if tp}
							<span
								class="inline-block h-1.5 w-1.5 rounded-full {alive
									? 'bg-hgreen'
									: 'bg-hred opacity-50'}"
							></span>
						{/if}
					</td>
					<!-- Name -->
					<td class="whitespace-nowrap px-1 py-0.5">
						<span class="{teamColor(sp.team)} {dead ? 'line-through' : ''}">{sp.name}</span>
					</td>
					<!-- HP / SH -->
					<td class="whitespace-nowrap px-1 py-0.5">
						{#if tp}
							<div class="flex flex-col gap-px" style="min-width: 80px;">
								<div class="flex items-center gap-1">
									<div class="h-1 flex-1 overflow-hidden rounded-sm bg-[#252535]">
										<div
											class="h-full rounded-sm transition-[width] duration-100 {hpBarColor(tp.health)}"
											style="width: {Math.round(tp.health * 100)}%"
										></div>
									</div>
									<span class="w-5 text-right text-[9px] text-dim">{Math.round(tp.health * 100)}</span>
								</div>
								<div class="flex items-center gap-1">
									<div class="h-1 flex-1 overflow-hidden rounded-sm bg-[#252535]">
										<div
											class="h-full rounded-sm transition-[width] duration-100 {shBarColor(tp.shields, tp.has_overshield)}"
											style="width: {Math.min(Math.round(tp.shields * 100), 100)}%"
										></div>
									</div>
									<span class="w-5 text-right text-[9px] text-dim">{Math.round(tp.shields * 100)}</span>
								</div>
							</div>
						{/if}
					</td>
					<!-- K/D/A -->
					<td class="whitespace-nowrap px-1 py-0.5">
						<span class="text-hgreen">{sp.kills}</span>
						<span class="text-dim">/</span>
						<span class="text-hred">{sp.deaths}</span>
						<span class="text-dim">/</span>
						<span class="text-dim">{sp.assists}</span>
					</td>
					<!-- CTF -->
					{#if showCtf}
						<td class="whitespace-nowrap px-1 py-0.5 text-hcyan">{sp.ctf_score}</td>
					{/if}
					<!-- Team Kills -->
					<td class="whitespace-nowrap px-1 py-0.5 text-horange">{sp.team_kills}</td>
					<!-- Suicides -->
					<td class="whitespace-nowrap px-1 py-0.5 text-dim">{sp.suicides}</td>
					<!-- Kill Streak -->
					<td class="whitespace-nowrap px-1 py-0.5">
						{#if sp.kill_streak > 1}
							<span class="text-hgold">{sp.kill_streak}</span>
						{:else}
							<span class="text-dim">-</span>
						{/if}
					</td>
					<!-- Multikill -->
					<td class="whitespace-nowrap px-1 py-0.5">
						{#if sp.multikill > 1}
							<span class="text-hgold">{sp.multikill}x</span>
						{:else}
							<span class="text-dim">-</span>
						{/if}
					</td>
					<!-- Accuracy -->
					<td class="whitespace-nowrap px-1 py-0.5 text-dim">{accuracy(sp)}</td>
					<!-- Selected Weapon -->
					<td class="whitespace-nowrap px-1 py-0.5">
						{#if tp}
							{@const sw = selectedWeapon(tp)}
							{#if sw}
								<span class="text-hgold">{weaponDisplay(sw)}</span>
							{/if}
						{/if}
					</td>
					<!-- Weapon Inventory (all slots) -->
					<td class="whitespace-nowrap px-1 py-0.5">
						{#if tp?.weapons?.length}
							{#each tp.weapons as w}
								<span class="{w.slot === tp.selected_weapon_slot ? 'text-hgold' : 'text-dim'} mr-0.5 text-[9px]">
									{shortWeapon(w.tag)}
								</span>
							{/each}
						{/if}
					</td>
					<!-- Grenades -->
					<td class="whitespace-nowrap px-1 py-0.5">
						{#if tp}
							<span class="text-[#c0a840]">{tp.frags}</span>
							<span class="text-dim">/</span>
							<span class="text-[#60a8e0]">{tp.plasmas}</span>
						{/if}
					</td>
					<!-- Respawn -->
					<td class="whitespace-nowrap px-1 py-0.5">
						{#if tp && !tp.alive}
							<span class="text-hred">{respawnSec(tp)}</span>
						{/if}
					</td>
					<!-- Position -->
					<td class="whitespace-nowrap px-1 py-0.5 text-[9px] text-dim">
						{#if tp}
							{fmtCoord(tp.x)} {fmtCoord(tp.y)} {fmtCoord(tp.z)}
						{/if}
					</td>
					<!-- Velocity -->
					<td class="whitespace-nowrap px-1 py-0.5 text-[9px] text-dim">
						{#if tp}
							{velocity(tp)}
						{/if}
					</td>
					<!-- Zoom -->
					<td class="whitespace-nowrap px-1 py-0.5">
						{#if tp && tp.zoom_level > 0}
							<span class="text-hcyan">{tp.zoom_level}x</span>
						{:else}
							<span class="text-dim">-</span>
						{/if}
					</td>
					<!-- Action Flags -->
					<td class="whitespace-nowrap px-1 py-0.5">
						{#if tp}
							<div class="flex flex-wrap gap-px text-[8px]">
								{#if tp.is_crouching}<span class="text-hcyan">CR</span>{/if}
								{#if tp.is_jumping}<span class="text-hgreen">JU</span>{/if}
								{#if tp.is_firing}<span class="text-hyellow">FI</span>{/if}
								{#if tp.is_shooting}<span class="text-hred">SH</span>{/if}
								{#if tp.is_throwing_grenade}<span class="text-[#a0b080]">GR</span>{/if}
								{#if tp.is_meleeing}<span class="text-[#e060c0]">ME</span>{/if}
								{#if tp.has_camo}<span class="text-hpurple">CAM</span>{/if}
								{#if tp.has_overshield}<span class="text-[#8840d0]">OS</span>{/if}
								{#if tp.is_flashlight_on}<span class="text-hyellow">FL</span>{/if}
								{#if tp.zoom_level > 0}<span class="text-hcyan">ZM</span>{/if}
							</div>
						{/if}
					</td>
				</tr>
			{/each}
		</tbody>
	</table>
</div>
