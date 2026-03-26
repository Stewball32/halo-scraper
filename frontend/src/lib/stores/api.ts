import type { StatusResponse, ContainerInfo } from '$lib/types';

function baseUrl(): string {
	if (typeof window === 'undefined') return '';
	// In dev, Vite proxy handles /api; in prod, same origin
	return '';
}

export async function fetchStatus(): Promise<StatusResponse> {
	const r = await fetch(`${baseUrl()}/api/status`);
	if (!r.ok) throw new Error(`status ${r.status}`);
	return r.json();
}

export async function fetchLogs(): Promise<string[]> {
	const r = await fetch(`${baseUrl()}/api/logs`);
	if (!r.ok) throw new Error(`status ${r.status}`);
	return r.json();
}

export async function reconnectHost(name: string): Promise<void> {
	await fetch(`${baseUrl()}/api/hosts/${encodeURIComponent(name)}/reconnect`, {
		method: 'POST'
	});
}

export async function fetchContainers(): Promise<ContainerInfo[]> {
	const r = await fetch(`${baseUrl()}/api/containers`);
	if (!r.ok) throw new Error(`status ${r.status}`);
	return r.json();
}

export async function createContainer(name: string): Promise<ContainerInfo> {
	const r = await fetch(`${baseUrl()}/api/containers`, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ name })
	});
	if (!r.ok) {
		const text = await r.text();
		throw new Error(text || `status ${r.status}`);
	}
	return r.json();
}

export async function getContainerStatus(name: string): Promise<string> {
	const r = await fetch(`${baseUrl()}/api/containers/${encodeURIComponent(name)}`);
	if (!r.ok) throw new Error(`status ${r.status}`);
	const data = await r.json();
	return data.status;
}

export async function startContainer(name: string): Promise<void> {
	const r = await fetch(`${baseUrl()}/api/containers/${encodeURIComponent(name)}/start`, {
		method: 'POST'
	});
	if (!r.ok) {
		const text = await r.text();
		throw new Error(text || `status ${r.status}`);
	}
}

export async function stopContainer(name: string): Promise<void> {
	const r = await fetch(`${baseUrl()}/api/containers/${encodeURIComponent(name)}/stop`, {
		method: 'POST'
	});
	if (!r.ok) {
		const text = await r.text();
		throw new Error(text || `status ${r.status}`);
	}
}

export async function removeContainer(name: string): Promise<void> {
	const r = await fetch(`${baseUrl()}/api/containers/${encodeURIComponent(name)}`, {
		method: 'DELETE'
	});
	if (!r.ok) {
		const text = await r.text();
		throw new Error(text || `status ${r.status}`);
	}
}
