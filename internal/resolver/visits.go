package resolver

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// VisitEntry is one frecency-tracked target (repo:branch key).
type VisitEntry struct {
	Key         string  `json:"key"`
	Score       float64 `json:"score"`
	LastVisited uint64  `json:"last_visited"`
}

// VisitsDB is the on-disk frecency store written to
// $XDG_STATE_HOME/makework/visits.json.
type VisitsDB struct {
	Entries []VisitEntry `json:"entries"`
	MaxAge  float64      `json:"max_age"`
}

// NewVisitsDB returns a VisitsDB with the default MaxAge cap.
func NewVisitsDB() VisitsDB {
	return VisitsDB{MaxAge: 10000.0}
}

// LoadVisits reads the visits file at path, returning a fresh DB when
// the file is absent.
func LoadVisits(path string) (VisitsDB, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return NewVisitsDB(), nil
	}
	if err != nil {
		return VisitsDB{}, err
	}
	var db VisitsDB
	if err := json.Unmarshal(data, &db); err != nil {
		return VisitsDB{}, fmt.Errorf("visits.json parse error: %w", err)
	}
	if db.MaxAge == 0 {
		db.MaxAge = 10000.0
	}
	return db, nil
}

// Save atomically writes the visits DB via tmp+rename.
func (db *VisitsDB) Save(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(db, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// RecordVisit increments the score for key and updates its timestamp.
func (db *VisitsDB) RecordVisit(key string, now uint64) {
	for i, e := range db.Entries {
		if e.Key == key {
			db.Entries[i].Score += 1.0
			db.Entries[i].LastVisited = now
			db.Compact()
			return
		}
	}
	db.Entries = append(db.Entries, VisitEntry{Key: key, Score: 1.0, LastVisited: now})
	db.Compact()
}

// Compact rescales scores when their sum exceeds MaxAge and drops
// entries that fall below 1.0 after rescaling. Mirrors the z-frecency
// "aging" trick.
func (db *VisitsDB) Compact() {
	var total float64
	for _, e := range db.Entries {
		total += e.Score
	}
	if total <= db.MaxAge {
		return
	}
	target := db.MaxAge * 0.9
	k := total / target
	for i := range db.Entries {
		db.Entries[i].Score /= k
	}
	filtered := db.Entries[:0]
	for _, e := range db.Entries {
		if e.Score >= 1.0 {
			filtered = append(filtered, e)
		}
	}
	db.Entries = filtered
}

// FrecencyScore is the time-decayed score for key relative to now.
func (db *VisitsDB) FrecencyScore(key string, now uint64) float64 {
	for _, e := range db.Entries {
		if e.Key == key {
			elapsed := float64(now - e.LastVisited)
			timeWeight := 1.0 / (1.0 + elapsed/3600.0)
			return e.Score * timeWeight
		}
	}
	return 0
}

// FrecencyScoreWithSiblings is FrecencyScore for key plus half-weight
// credit for any sibling entry under the same repoPrefix.
func (db *VisitsDB) FrecencyScoreWithSiblings(key, repoPrefix string, now uint64) float64 {
	var score float64
	for _, e := range db.Entries {
		elapsed := float64(now - e.LastVisited)
		timeWeight := 1.0 / (1.0 + elapsed/3600.0)
		if e.Key == key {
			score += e.Score * timeWeight
		} else if strings.HasPrefix(e.Key, repoPrefix) {
			score += e.Score * timeWeight * 0.5
		}
	}
	return score
}
