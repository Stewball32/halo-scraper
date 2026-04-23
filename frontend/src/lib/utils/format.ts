export function fmtDuration(seconds: number): string {
	const h = Math.floor(seconds / 3600);
	const m = Math.floor((seconds % 3600) / 60);
	const s = Math.floor(seconds % 60);
	if (h > 0) return `${h}h ${m}m`;
	if (m > 0) return `${m}m ${s}s`;
	return `${s}s`;
}

export function sinceStr(isoStr: string): string {
	const secs = Math.round((Date.now() - new Date(isoStr).getTime()) / 1000);
	return fmtDuration(Math.max(0, secs));
}

const WEAPON_SHORT: Record<string, string> = {
	// Halo CE
	'assault rifle': 'AR',
	pistol: 'Pistol',
	'sniper rifle': 'Sniper',
	'rocket launcher': 'Rockets',
	shotgun: 'Shotgun',
	'plasma pistol': 'P.Pistol',
	'plasma rifle': 'P.Rifle',
	needler: 'Needler',
	flamethrower: 'Flamer',
	'fuel rod gun': 'Fuel Rod',
	'energy sword': 'Sword',
	// Halo 2
	magnum: 'Magnum',
	smg: 'SMG',
	'battle rifle': 'BR',
	carbine: 'Carbine',
	'beam rifle': 'Beam',
	'fuel rod': 'Fuel Rod',
	'brute shot': 'Brute Shot',
	'brute plasma rifle': 'Brute P.R.',
	turret: 'Turret',
	'turret plasma': 'P.Turret',
	banshee: 'Banshee',
	ghost: 'Ghost',
	mongoose: 'Mongoose',
	scorpion: 'Scorpion',
	spectre: 'Spectre',
	warthog: 'Warthog',
	wraith: 'Wraith',
	tank: 'Tank',
	'sentinel beam': 'S.Beam',
	melee: 'Melee',
	guardians: 'Guardians',
	'fall damage': 'Fall',
	'frag grenade': 'Frag',
	'plasma grenade': 'Plasma',
	'flag melee': 'Flag',
	'bomb melee': 'Bomb',
	'oddball melee': 'Oddball'
};

export function shortWeapon(tag: string): string {
	if (!tag) return '';
	const lower = tag.toLowerCase();
	return WEAPON_SHORT[lower] ?? tag;
}

export function hpColor(hp: number): string {
	if (hp > 0.6) return 'bg-hgreen';
	if (hp > 0.3) return 'bg-hyellow';
	return 'bg-hred';
}

export function eventColor(eventType: string): string {
	const map: Record<string, string> = {
		kill: 'text-hred',
		death: 'text-horange',
		spawn: 'text-hgreen',
		damage: 'text-hyellow',
		melee: 'text-[#e060c0]',
		team_kill: 'text-[#e09040]',
		score: 'text-dim',
		item_picked_up: 'text-hblue',
		item_dropped: 'text-[#7090e0]',
		item_spawned: 'text-[#5070c0]',
		item_depleted: 'text-[#4060a0]',
		grenade_thrown: 'text-[#a0b080]',
		powerup_picked_up: 'text-hpurple',
		powerup_expired: 'text-[#8040c0]',
		vehicle_entered: 'text-hcyan',
		vehicle_exited: 'text-[#30a8a0]',
		player_quit: 'text-dim',
		multikill: 'text-hgold',
		kill_streak: 'text-hgold',
		game_start: 'text-hgold',
		game_end: 'text-hgold'
	};
	return map[eventType] ?? 'text-dim';
}

export function describeEvent(ev: Record<string, unknown>, playerNames: Record<number, string>): string {
	const name = (idx: unknown) => {
		if (idx == null) return '?';
		return playerNames[idx as number] ?? `Player ${idx}`;
	};
	const t = ev.event_type as string;

	switch (t) {
		case 'kill':
			return `${name(ev.killer)} killed ${name(ev.killed)}${ev.weapon ? ` (${shortWeapon(ev.weapon as string)})` : ''}`;
		case 'death':
			return `${name(ev.player)} died`;
		case 'spawn':
			return `${name(ev.player)} spawned`;
		case 'damage':
			return `${name(ev.dealer)} hit ${name(ev.target)}`;
		case 'melee':
			return `${name(ev.dealer)} melee → ${name(ev.target)}`;
		case 'team_kill':
			return `${name(ev.killer)} TK'd ${name(ev.killed)}`;
		case 'score':
			return `${name(ev.player)} K:${ev.kills ?? '?'} D:${ev.deaths ?? '?'} A:${ev.assists ?? '?'}`;
		case 'multikill':
			return `${name(ev.player)} ${ev.count}x MULTIKILL`;
		case 'kill_streak':
			return `${name(ev.player)} ${ev.count} KILL STREAK`;
		case 'vehicle_entered':
			return `${name(ev.player)} entered ${ev.vehicle ?? 'vehicle'}`;
		case 'vehicle_exited':
			return `${name(ev.player)} exited ${ev.vehicle ?? 'vehicle'}`;
		case 'grenade_thrown':
			return `${name(ev.player)} threw grenade`;
		case 'powerup_picked_up':
			return `${name(ev.player)} grabbed ${ev.item ?? 'powerup'}`;
		case 'powerup_expired':
			return `${name(ev.player)} lost ${ev.item ?? 'powerup'}`;
		case 'item_picked_up':
			return `${name(ev.player)} picked up ${shortWeapon((ev.item as string) ?? '')}`;
		case 'item_dropped':
			return `${name(ev.player)} dropped ${shortWeapon((ev.item as string) ?? '')}`;
		case 'item_spawned':
			return `${shortWeapon((ev.item as string) ?? '')} spawned`;
		case 'item_depleted':
			return `${shortWeapon((ev.item as string) ?? '')} depleted`;
		case 'player_quit':
			return `${name(ev.player)} quit`;
		case 'game_start':
			return 'Game started';
		case 'game_end':
			return 'Game ended';
		default:
			return t;
	}
}
