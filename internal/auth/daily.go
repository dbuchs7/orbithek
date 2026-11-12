package auth

import "fmt"

// DailyRoom returns the key of the space a user's daily notes go to, or
// empty when they have not chosen one (ADR-0205). Whether they may still
// create items there is the caller's concern.
func (a *Accounts) DailyRoom(id UserID) string {
	a.mu.Lock()
	defer a.mu.Unlock()
	if u, ok := a.users[id]; ok {
		return u.dailyRoom
	}
	return ""
}

// SetDailyRoom chooses the space a user's daily notes go to; empty clears
// the choice. Choosing the same space again changes nothing.
func (a *Accounts) SetDailyRoom(id UserID, space string) error {
	if space != "" && !favouriteKeyRE.MatchString(space) {
		return fmt.Errorf("%w: space key %q", ErrInvalid, space)
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	u, ok := a.users[id]
	if !ok {
		return ErrNotFound
	}
	if u.dailyRoom == space {
		return nil
	}
	return a.record(event{Kind: dailyRoomSet, User: id, Room: space})
}
