package observation

// StreamState is the tail of a stream's content chain, as the collect usecase
// reads it before deciding what (if anything) a fresh observation emits.
type StreamState struct {
	Exists          bool // the stream has at least one event
	LastType        EventType
	LastContentHash *Digest // most recent snapshot's content hash (skips Vanished); nil if never snapshotted
}

// Transition is the decision for one observation cycle.
type Transition struct {
	Type EventType
	Emit bool // false => content unchanged or a non-event (e.g. 404 on a stream never seen)
	// PrevContentHash links the new event into the per-stream content chain:
	// the last actual snapshot's hash (nil at stream start). Vanished carries
	// the pre-vanish snapshot forward so the content chain stays continuous.
	PrevContentHash *Digest
}

// Decide maps (current stream tail, freshly observed content hash) to an event.
// observed == nil means the resource was absent this cycle (404/removed).
//
// This is the Single Emission invariant (immutable-design-guard #8): an
// unchanged content hash emits nothing.
func Decide(s StreamState, observed *Digest) Transition {
	if observed == nil {
		// Resource is gone.
		if !s.Exists || s.LastType == ResourceVanished {
			return Transition{Type: Unknown, Emit: false}
		}
		return Transition{Type: ResourceVanished, Emit: true, PrevContentHash: s.LastContentHash}
	}

	// Resource is present.
	switch {
	case !s.Exists:
		return Transition{Type: ResourceObserved, Emit: true, PrevContentHash: nil}
	case s.LastType == ResourceVanished:
		// Reappeared after a vanish — a transition regardless of content equality.
		return Transition{Type: ResourceRestored, Emit: true, PrevContentHash: s.LastContentHash}
	case s.LastContentHash != nil && *s.LastContentHash == *observed:
		return Transition{Type: Unknown, Emit: false}
	default:
		return Transition{Type: ResourceChanged, Emit: true, PrevContentHash: s.LastContentHash}
	}
}
