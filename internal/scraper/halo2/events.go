package halo2

import (
	"encoding/json"

	"xemu-cartographer/internal/scraper"
)

// DetectEvents combines stat-diff detection with the Halo 2 game event buffer
// to produce events. The event buffer provides kill events with weapon info;
// stat-diff detection catches deaths, score changes, and streaks.
//
// NOTE: The event buffer is currently NON-FUNCTIONAL — GVAEventCount always
// reads 0 despite kills occurring. All kill/death detection relies on the
// stat-diff path below. Weapon attribution is unavailable until the event
// buffer offsets are verified or an alternative source is found.
func (r *Reader) DetectEvents(tick uint32, instance string, snap scraper.SnapshotPayload, result scraper.TickResult, state *scraper.TickState) []scraper.Envelope {
	var events []scraper.Envelope
	emit := func(payload any) {
		b, _ := json.Marshal(payload)
		events = append(events, scraper.Envelope{
			Type:     "event",
			Instance: instance,
			Tick:     tick,
			Payload:  b,
		})
	}

	// Build player name lookup.
	playerNames := make(map[int]string, len(snap.Players))
	snapshotByIdx := make(map[int]scraper.SnapshotPlayer, len(snap.Players))
	for _, p := range snap.Players {
		playerNames[p.Index] = p.Name
		snapshotByIdx[p.Index] = p
	}

	// -------------------------------------------------------------------
	// Event buffer: kill events with weapon attribution
	// -------------------------------------------------------------------
	bufferEvents := r.readNewEvents()
	for _, evt := range bufferEvents {
		switch evt.evtType {
		case 1: // kill
			weapon := DamageReportingNames[uint8(evt.weapon)]
			if weapon == "" {
				weapon = "unknown"
			}

			killerIdx := int(evt.source)
			victimIdx := int(evt.effected)

			emit(map[string]any{
				"event_type": scraper.EventKill,
				"killer":     killerIdx,
				"victim":     victimIdx,
				"weapon":     weapon,
			})

			// Check for team kill.
			killerSnap, kOk := snapshotByIdx[killerIdx]
			victimSnap, vOk := snapshotByIdx[victimIdx]
			if kOk && vOk && snap.IsTeamGame && killerSnap.Team == victimSnap.Team && killerIdx != victimIdx {
				emit(map[string]any{
					"event_type": scraper.EventTeamKill,
					"killer":     killerIdx,
					"victim":     victimIdx,
				})
			}
		}
	}

	// -------------------------------------------------------------------
	// Stat-diff detection for events the buffer doesn't cover
	// -------------------------------------------------------------------
	for _, ip := range result.InternalPlayers {
		idx := ip.Index

		// --- death (death counter increased) ---
		prevDeaths := state.PrevDeaths[idx]
		if ip.Deaths > prevDeaths {
			emit(map[string]any{
				"event_type": scraper.EventDeath,
				"player":     idx,
			})
		}

		// --- score update ---
		prevKills := state.PrevKills[idx]
		if ip.Kills > prevKills {
			emit(map[string]any{
				"event_type":  scraper.EventScore,
				"player":      idx,
				"kills":       ip.Kills,
				"deaths":      ip.Deaths,
				"assists":     ip.Assists,
				"kill_streak": ip.KillStreak,
			})

			// --- kill streak ---
			prevKS := state.PrevKillStreak[idx]
			if ip.KillStreak > prevKS && ip.KillStreak > 1 {
				emit(map[string]any{
					"event_type": scraper.EventKillStreak,
					"player":     idx,
					"count":      ip.KillStreak,
				})
			}
		}

		// --- suicide (suicide counter increased) ---
		prevSuicides := state.PrevSuicides[idx]
		if ip.Suicides > prevSuicides {
			emit(map[string]any{
				"event_type": scraper.EventDeath,
				"player":     idx,
				"suicide":    true,
			})
		}
	}

	// -------------------------------------------------------------------
	// Update tick state
	// -------------------------------------------------------------------
	for _, tp := range result.Payload.Players {
		state.PrevAlive[tp.Index] = tp.Alive
		state.PrevHealth[tp.Index] = tp.Health
		state.PrevShields[tp.Index] = tp.Shields
	}

	for _, ip := range result.InternalPlayers {
		idx := ip.Index
		state.PrevKills[idx] = ip.Kills
		state.PrevDeaths[idx] = ip.Deaths
		state.PrevAssists[idx] = ip.Assists
		state.PrevTeamKills[idx] = ip.TeamKills
		state.PrevSuicides[idx] = ip.Suicides
		state.PrevKillStreak[idx] = ip.KillStreak
		state.PrevMultikill[idx] = ip.Multikill
		state.PrevParentObject[idx] = ip.ParentObject
	}

	return events
}
