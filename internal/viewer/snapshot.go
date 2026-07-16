package viewer

import (
	"github.com/nkenji09/scholia/internal/index"
	"github.com/nkenji09/scholia/internal/store"
)

// loadIndexed reloads .scholia fresh for each request so the viewer always
// reflects the current on-disk state, including edits made concurrently via
// the CLI or the config PUT endpoint (§3.9: the index is disposable and
// rebuilt on read; a small project tree makes per-request rebuild cheap
// enough to skip caching for this unit).
func loadIndexed(s *store.Store) (store.Snapshot, *index.Index, error) {
	snap, err := s.LoadAll()
	if err != nil {
		return store.Snapshot{}, nil, err
	}
	return snap, index.Build(&snap), nil
}

func containsStr(list []string, want string) bool {
	for _, v := range list {
		if v == want {
			return true
		}
	}
	return false
}
