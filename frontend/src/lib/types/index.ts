// WebSocket envelope — matches internal/scraper/types.go:Envelope
export interface Envelope {
	type: 'snapshot' | 'tick' | 'event';
	instance: string;
	tick: number;
	payload: unknown;
}

// --- Snapshot types (game.go:SnapshotPayload) ---

export type GameState = 'menu' | 'pregame' | 'in_game' | 'postgame';

export interface SnapshotPayload {
	game_state: GameState;
	map: string;
	gametype: string;
	is_team_game: boolean;
	score_limit: number;
	time_limit_ticks: number;
	team_scores: TeamScore[];
	players: SnapshotPlayer[];
	power_item_spawns: PowerItemSpawn[];
}

export interface SnapshotPlayer {
	index: number;
	name: string;
	team: number;
	kills: number;
	deaths: number;
	assists: number;
	ctf_score: number;
	team_kills: number;
	suicides: number;
	kill_streak: number;
	multikill: number;
	shots_fired: number;
	shots_hit: number;
	is_local?: boolean | null;
	local_index?: number | null;
}

export interface TeamScore {
	team: number;
	score: number;
}

export interface PowerItemSpawn {
	spawn_id: number;
	tag: string;
	spawn_interval_ticks: number;
	x: number;
	y: number;
	z: number;
}

// --- Tick types (game.go:TickPayload) ---

export interface TickPayload {
	players: TickPlayer[];
	power_items: PowerItemStatus[];
}

export interface TickPlayer {
	index: number;
	alive: boolean;
	respawn_in_ticks: number | null;
	x: number;
	y: number;
	z: number;
	vx: number;
	vy: number;
	vz: number;
	aim_x: number;
	aim_y: number;
	aim_z: number;
	zoom_level: number;
	crouchscale: number;
	health: number;
	shields: number;
	has_camo: boolean;
	has_overshield: boolean;
	frags: number;
	plasmas: number;
	selected_weapon_slot: number;
	is_crouching: boolean;
	is_jumping: boolean;
	is_firing: boolean;
	is_shooting: boolean;
	is_flashlight_on: boolean;
	is_throwing_grenade: boolean;
	is_meleeing: boolean;
	is_pressing_action: boolean;
	is_holding_action: boolean;
	weapons: WeaponInfo[];
}

export interface WeaponInfo {
	slot: number;
	object_id: number;
	tag: string;
	ammo_pack: number | null;
	ammo_mag: number | null;
	charge?: number | null;
	is_energy: boolean;
}

export interface PowerItemStatus {
	spawn_id: number;
	status: 'held' | 'world' | 'respawning';
	held_by: number | null;
	world_pos: { x: number; y: number; z: number } | null;
	respawn_in_ticks: number | null;
}

// --- Event types ---

export interface EventPayload {
	event_type: string;
	player?: number;
	killer?: number;
	killed?: number;
	weapon?: string;
	damage_type?: string;
	kills?: number;
	deaths?: number;
	assists?: number;
	kill_streak?: number;
	multikill?: number;
	count?: number;
	vehicle?: string;
	item?: string;
	team?: number;
	[key: string]: unknown;
}

// --- REST API types ---

export interface HostStatus {
	state: 'online' | 'connecting' | 'offline';
	since: string;
	error: string;
	xbox_name?: string;
}

export interface StatusResponse {
	uptime_s: number;
	ws_clients: number;
	hosts: Record<string, HostStatus>;
}

// Container types — matches internal/podman/state.go

export interface ContainerPorts {
	xemu_http: number;
	xemu_https: number;
	xemu_ws: number;
	browser_web: number;
	browser_vnc: number;
}

export interface ContainerInfo {
	name: string;
	index: number;
	ports: ContainerPorts;
	created: string;
}

// UI event log entry
export interface EventEntry {
	instance: string;
	tick: number;
	payload: EventPayload;
	timestamp: number;
}
