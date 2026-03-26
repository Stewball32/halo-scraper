import type { Envelope, SnapshotPayload, TickPayload, EventEntry, EventPayload } from '$lib/types';

const MAX_EVENTS = 200;
const RECONNECT_BASE = 1000;
const RECONNECT_MAX = 30000;

let ws: WebSocket | null = null;
let reconnectDelay = RECONNECT_BASE;
let reconnectTimer: ReturnType<typeof setTimeout> | null = null;
let intentionalClose = false;

// Reactive state
let connected = $state(false);
let snapshots = $state<Record<string, SnapshotPayload>>({});
let ticks = $state<Record<string, TickPayload>>({});
let events = $state<EventEntry[]>([]);
let messageRate = $state(0);

let msgCount = 0;
let rateInterval: ReturnType<typeof setInterval> | null = null;

function getWsUrl(): string {
	const loc = window.location;
	const proto = loc.protocol === 'https:' ? 'wss:' : 'ws:';
	return `${proto}//${loc.host}/ws`;
}

function onMessage(e: MessageEvent) {
	msgCount++;
	try {
		const msg: Envelope = JSON.parse(e.data);
		const { type, instance, tick, payload } = msg;

		if (type === 'snapshot') {
			snapshots[instance] = payload as SnapshotPayload;
		} else if (type === 'tick') {
			ticks[instance] = payload as TickPayload;
		} else if (type === 'event') {
			const entry: EventEntry = {
				instance,
				tick,
				payload: payload as EventPayload,
				timestamp: Date.now()
			};
			events = [entry, ...events].slice(0, MAX_EVENTS);

			// Update snapshot player scores from score events
			const ep = payload as EventPayload;
			if (ep.event_type === 'score' && ep.player != null) {
				const snap = snapshots[instance];
				if (snap) {
					const p = snap.players.find((pl) => pl.index === ep.player);
					if (p) {
						if (ep.kills != null) p.kills = ep.kills as number;
						if (ep.deaths != null) p.deaths = ep.deaths as number;
						if (ep.assists != null) p.assists = ep.assists as number;
						if (ep.kill_streak != null) p.kill_streak = ep.kill_streak as number;
						if (ep.multikill != null) p.multikill = ep.multikill as number;
					}
				}
			}
		}
	} catch {
		// ignore parse errors
	}
}

function doConnect() {
	if (ws) return;
	try {
		ws = new WebSocket(getWsUrl());
	} catch {
		scheduleReconnect();
		return;
	}

	ws.onopen = () => {
		connected = true;
		reconnectDelay = RECONNECT_BASE;
	};

	ws.onmessage = onMessage;

	ws.onclose = () => {
		connected = false;
		ws = null;
		if (!intentionalClose) {
			scheduleReconnect();
		}
	};

	ws.onerror = () => {
		// onclose will fire after this
	};
}

function scheduleReconnect() {
	if (reconnectTimer) return;
	reconnectTimer = setTimeout(() => {
		reconnectTimer = null;
		doConnect();
		reconnectDelay = Math.min(reconnectDelay * 2, RECONNECT_MAX);
	}, reconnectDelay);
}

export function connect() {
	intentionalClose = false;
	if (!rateInterval) {
		rateInterval = setInterval(() => {
			messageRate = msgCount;
			msgCount = 0;
		}, 1000);
	}
	doConnect();
}

export function disconnect() {
	intentionalClose = true;
	if (reconnectTimer) {
		clearTimeout(reconnectTimer);
		reconnectTimer = null;
	}
	if (rateInterval) {
		clearInterval(rateInterval);
		rateInterval = null;
	}
	if (ws) {
		ws.close();
		ws = null;
	}
	connected = false;
}

export function getConnected() {
	return connected;
}

export function getSnapshots() {
	return snapshots;
}

export function getTicks() {
	return ticks;
}

export function getEvents() {
	return events;
}

export function getMessageRate() {
	return messageRate;
}
