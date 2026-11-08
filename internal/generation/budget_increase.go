package generation

// IncreaseWindowBudget is an explicit local operator action. It preserves the
// window/ledger identities, settled charges, unknown reservations and replay
// markers. Startup never invokes it. expected rejects stale operator commands.
func (l *Ledger) IncreaseWindowBudget(expected, maximum int64) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if err := l.ready(); err != nil {
		return err
	}
	if expected != l.state.MaxWindowMicros || maximum <= expected || maximum > 1_000_000_000 {
		return ErrLedgerUnavailable
	}
	next := l.state
	next.MaxWindowMicros = maximum
	return l.persist(next)
}
