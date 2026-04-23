export function hpBarColor(hp: number): string {
	if (hp > 0.6) return 'bg-hgreen';
	if (hp > 0.3) return 'bg-[#c8b030]';
	return 'bg-[#c83030]';
}

export function shBarColor(_sh: number, hasOvershield: boolean): string {
	return hasOvershield ? 'bg-[#8840d0]' : 'bg-[#3a80d0]';
}
