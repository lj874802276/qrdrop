package service

import (
	"log"
	"time"

	"github.com/yourname/qrdrop/server/config"
	"github.com/yourname/qrdrop/server/storage"
)

// cleanupInterval is how often expired inboxes are swept.
const cleanupInterval = time.Minute

// CleanupLoop keeps an eye on inboxes until stop is closed. It never deletes
// received files or session records — those are the user's property and must
// only be removed by an explicit manual action.
func CleanupLoop(store *storage.Store, cfg *config.Config, stop <-chan struct{}) {
	ticker := time.NewTicker(cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			Cleanup(store, cfg)
		}
	}
}

// Cleanup performs one diagnostic sweep. It reports how many inboxes are closed
// or expired for observability but intentionally leaves their files and rows
// untouched so the history remains traceable.
func Cleanup(store *storage.Store, cfg *config.Config) {
	tokens, err := store.ExpiredSessions(time.Now())
	if err != nil {
		log.Printf("cleanup: list inboxes: %v", err)
		return
	}
	if len(tokens) > 0 {
		log.Printf("cleanup: %d inbox(es) closed/expired — files retained for history", len(tokens))
	}
}
