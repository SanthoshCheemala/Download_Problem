package agent

import "sort"

// Blacklist records a peer id as known-faulty.
func (a *Agent) Blacklist(id int) {
	a.blacklistMu.Lock()
	defer a.blacklistMu.Unlock()
	a.blacklist[id] = struct{}{}
}

// IsBlacklisted reports whether a peer id has been blacklisted.
func (a *Agent) IsBlacklisted(id int) bool {
	a.blacklistMu.RLock()
	defer a.blacklistMu.RUnlock()
	_, ok := a.blacklist[id]
	return ok
}

// BlacklistSize returns how many peers this agent has blacklisted.
func (a *Agent) BlacklistSize() int {
	a.blacklistMu.RLock()
	defer a.blacklistMu.RUnlock()
	return len(a.blacklist)
}

// BlacklistedIDs returns a sorted snapshot of all blacklisted peer ids.
func (a *Agent) BlacklistedIDs() []int {
	a.blacklistMu.RLock()
	defer a.blacklistMu.RUnlock()
	ids := make([]int, 0, len(a.blacklist))
	for id := range a.blacklist {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	return ids
}
