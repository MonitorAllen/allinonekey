package util

import "sync"

var (
	activeMasterKeys = make(map[uint]string)
	mu               sync.RWMutex
)

func SetActiveKey(userID uint, mk string) {
	mu.Lock()
	defer mu.Unlock()
	activeMasterKeys[userID] = mk
}

func GetActiveKey(userID uint) (string, bool) {
	mu.RLock()
	defer mu.RUnlock()
	mk, ok := activeMasterKeys[userID]
	return mk, ok
}

func RemoveActiveKey(userID uint) {
	mu.Lock()
	defer mu.Unlock()
	delete(activeMasterKeys, userID)
}

func ActiveKeySnapshot() map[uint]string {
	mu.RLock()
	defer mu.RUnlock()

	snapshot := make(map[uint]string, len(activeMasterKeys))
	for userID, mk := range activeMasterKeys {
		snapshot[userID] = mk
	}
	return snapshot
}
