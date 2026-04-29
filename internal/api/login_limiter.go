package api

import (
	"sync"
	"time"
)

const (
	maxLoginFailures = 5
	loginLockout     = 10 * time.Minute
)

type loginAttempt struct {
	Failures  int
	LockedTil time.Time
}

var loginAttempts = struct {
	sync.Mutex
	items map[string]loginAttempt
}{items: map[string]loginAttempt{}}

func loginAttemptKey(username string, ip string) string {
	return username + "|" + ip
}

func isLoginLocked(username string, ip string) (bool, time.Time) {
	loginAttempts.Lock()
	defer loginAttempts.Unlock()
	item := loginAttempts.items[loginAttemptKey(username, ip)]
	if item.LockedTil.IsZero() || time.Now().After(item.LockedTil) {
		return false, time.Time{}
	}
	return true, item.LockedTil
}

func recordLoginFailure(username string, ip string) (bool, time.Time) {
	loginAttempts.Lock()
	defer loginAttempts.Unlock()
	key := loginAttemptKey(username, ip)
	item := loginAttempts.items[key]
	item.Failures++
	if item.Failures >= maxLoginFailures {
		item.LockedTil = time.Now().Add(loginLockout)
	}
	loginAttempts.items[key] = item
	return !item.LockedTil.IsZero() && time.Now().Before(item.LockedTil), item.LockedTil
}

func resetLoginFailures(username string, ip string) {
	loginAttempts.Lock()
	defer loginAttempts.Unlock()
	delete(loginAttempts.items, loginAttemptKey(username, ip))
}
