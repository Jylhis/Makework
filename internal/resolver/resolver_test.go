package resolver

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jylhis/makework/internal/config"
)

func makeTarget(repo, branch, path string) CatalogTarget {
	return CatalogTarget{RepoName: repo, Branch: branch, Path: path}
}

func makeTargetWithProject(repo, branch, proj, path string) CatalogTarget {
	return CatalogTarget{RepoName: repo, Branch: branch, ProjectName: proj, Path: path}
}

func defaultCtx(now uint64) *ResolveContext { return &ResolveContext{Now: now} }

func defaultCfg() *config.ResolverConfig {
	c := config.DefaultResolver()
	return &c
}

// --- ParseQuery ---

func TestParseQueryExplicit(t *testing.T) {
	q, err := ParseQuery("backend@feature-auth")
	if err != nil {
		t.Fatal(err)
	}
	if q.Repo != "backend" || q.Branch != "feature-auth" {
		t.Errorf("got %+v", q)
	}
}

func TestParseQueryFuzzy(t *testing.T) {
	q, err := ParseQuery("auth")
	if err != nil {
		t.Fatal(err)
	}
	if q.Fuzzy != "auth" || q.IsExplicit() {
		t.Errorf("got %+v", q)
	}
}

func TestParseQueryEmptyLeft(t *testing.T) {
	_, err := ParseQuery("@main")
	if err == nil {
		t.Error("expected error for empty left of @")
	}
}

func TestParseQueryEmptyRight(t *testing.T) {
	_, err := ParseQuery("repo@")
	if err == nil {
		t.Error("expected error for empty right of @")
	}
}

func TestParseQueryTooShort(t *testing.T) {
	_, err := ParseQuery("a")
	if err != ErrQueryTooShort {
		t.Errorf("expected ErrQueryTooShort, got %v", err)
	}
}

func TestParseQueryWhitespace(t *testing.T) {
	_, err := ParseQuery("   ")
	if err != ErrQueryTooShort {
		t.Errorf("expected ErrQueryTooShort, got %v", err)
	}
}

func TestParseQueryMultipleAtSplitsOnFirst(t *testing.T) {
	q, err := ParseQuery("a@b@c")
	if err != nil {
		t.Fatal(err)
	}
	if q.Repo != "a" || q.Branch != "b@c" {
		t.Errorf("got %+v", q)
	}
}

// --- FuzzyScore ---

func TestFuzzyExactMatch(t *testing.T) {
	if FuzzyScore("backend", "backend") != 1.0 {
		t.Error("exact match should be 1.0")
	}
}

func TestFuzzyExactCaseInsensitive(t *testing.T) {
	if FuzzyScore("Backend", "backend") != 1.0 {
		t.Error("case-insensitive exact should be 1.0")
	}
}

func TestFuzzyPrefixBeatsNonPrefix(t *testing.T) {
	prefix := FuzzyScore("back", "backend")
	nonPrefix := FuzzyScore("end", "backend")
	if prefix <= nonPrefix {
		t.Errorf("prefix %f should beat non-prefix %f", prefix, nonPrefix)
	}
}

func TestFuzzySubstringBonus(t *testing.T) {
	sub := FuzzyScore("end", "backend")
	noMatch := FuzzyScore("xyz", "backend")
	if sub <= noMatch {
		t.Errorf("substring %f should beat no-match %f", sub, noMatch)
	}
}

func TestFuzzyEmpty(t *testing.T) {
	if FuzzyScore("", "backend") != 0 {
		t.Error("empty query should be 0")
	}
	if FuzzyScore("test", "") != 0 {
		t.Error("empty target should be 0")
	}
	if FuzzyScore("", "") != 0 {
		t.Error("both empty should be 0")
	}
}

// --- VisitsDB ---

func TestVisitsRecordNew(t *testing.T) {
	db := NewVisitsDB()
	db.RecordVisit("myrepo:main", 1000)
	if len(db.Entries) != 1 || db.Entries[0].Score != 1.0 || db.Entries[0].LastVisited != 1000 {
		t.Errorf("got %+v", db.Entries)
	}
}

func TestVisitsRecordIncrement(t *testing.T) {
	db := NewVisitsDB()
	db.RecordVisit("myrepo:main", 1000)
	db.RecordVisit("myrepo:main", 2000)
	if len(db.Entries) != 1 || db.Entries[0].Score != 2.0 || db.Entries[0].LastVisited != 2000 {
		t.Errorf("got %+v", db.Entries)
	}
}

func TestFrecencyRecentBeatsOld(t *testing.T) {
	db := VisitsDB{
		Entries: []VisitEntry{
			{Key: "recent:main", Score: 5.0, LastVisited: 9000},
			{Key: "old:main", Score: 5.0, LastVisited: 1000},
		},
		MaxAge: 10000,
	}
	recent := db.FrecencyScore("recent:main", 10000)
	old := db.FrecencyScore("old:main", 10000)
	if recent <= old {
		t.Errorf("recent %f should beat old %f", recent, old)
	}
}

func TestFrecencyHighScoreBeatsLow(t *testing.T) {
	db := VisitsDB{
		Entries: []VisitEntry{
			{Key: "high:main", Score: 50, LastVisited: 5000},
			{Key: "low:main", Score: 2, LastVisited: 5000},
		},
		MaxAge: 10000,
	}
	high := db.FrecencyScore("high:main", 10000)
	low := db.FrecencyScore("low:main", 10000)
	if high <= low {
		t.Errorf("high %f should beat low %f", high, low)
	}
}

func TestFrecencyMissingKey(t *testing.T) {
	db := NewVisitsDB()
	if db.FrecencyScore("nope:main", 10000) != 0 {
		t.Error("missing key should be 0")
	}
}

func TestSiblingContributesHalf(t *testing.T) {
	db := VisitsDB{
		Entries: []VisitEntry{{Key: "repo:feature-a", Score: 10, LastVisited: 9000}},
		MaxAge:  10000,
	}
	exact := db.FrecencyScoreWithSiblings("repo:feature-a", "repo:", 10000)
	sibling := db.FrecencyScoreWithSiblings("repo:main", "repo:", 10000)
	if exact <= sibling {
		t.Errorf("exact %f should beat sibling %f", exact, sibling)
	}
	if sibling <= 0 {
		t.Error("sibling should contribute > 0")
	}
}

func TestCompactionDividesScores(t *testing.T) {
	db := VisitsDB{
		Entries: []VisitEntry{
			{Key: "a:main", Score: 6000},
			{Key: "b:main", Score: 5000},
		},
		MaxAge: 10000,
	}
	db.Compact()
	var total float64
	for _, e := range db.Entries {
		total += e.Score
	}
	target := 10000.0 * 0.9
	if abs(total-target) > 1.0 {
		t.Errorf("total %f should be ~%f", total, target)
	}
}

func TestCompactionPrunesBelowOne(t *testing.T) {
	db := VisitsDB{
		Entries: []VisitEntry{
			{Key: "big:main", Score: 10000},
			{Key: "tiny:main", Score: 0.5},
		},
		MaxAge: 10000,
	}
	db.Compact()
	for _, e := range db.Entries {
		if e.Key == "tiny:main" {
			t.Error("tiny entry should be pruned")
		}
	}
}

func TestCompactionNoopUnderThreshold(t *testing.T) {
	db := VisitsDB{
		Entries: []VisitEntry{{Key: "a:main", Score: 5}},
		MaxAge:  10000,
	}
	db.Compact()
	if db.Entries[0].Score != 5 {
		t.Error("should not compact under max_age")
	}
}

func TestVisitsRoundTripJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "visits.json")

	db := NewVisitsDB()
	db.RecordVisit("repo:main", 1000)
	db.RecordVisit("repo:feature", 2000)
	if err := db.Save(path); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadVisits(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Entries) != 2 || loaded.Entries[0].Key != "repo:main" {
		t.Errorf("loaded = %+v", loaded.Entries)
	}
}

func TestVisitsLoadMissing(t *testing.T) {
	db, err := LoadVisits("/nonexistent/visits.json")
	if err != nil {
		t.Fatal(err)
	}
	if len(db.Entries) != 0 {
		t.Error("expected empty")
	}
}

func TestVisitsLoadCorrupted(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "visits.json")
	os.WriteFile(path, []byte("not valid json {{{"), 0o644)
	_, err := LoadVisits(path)
	if err == nil {
		t.Error("expected parse error")
	}
}

// --- Resolve ---

func TestResolveExactMatchFirst(t *testing.T) {
	index := &Index{
		Targets: []CatalogTarget{
			makeTarget("backend", "main", "/repos/backend"),
			makeTarget("backend-api", "main", "/repos/backend-api"),
			makeTarget("frontend", "main", "/repos/frontend"),
		},
		Visits: NewVisitsDB(),
	}
	results, err := Resolve("backend", index, defaultCfg(), defaultCtx(10000))
	if err != nil {
		t.Fatal(err)
	}
	if results[0].RepoName != "backend" {
		t.Errorf("first = %s", results[0].RepoName)
	}
	if results[0].Score <= results[1].Score {
		t.Error("exact should beat partial")
	}
}

func TestResolveFrecencyBoost(t *testing.T) {
	visits := NewVisitsDB()
	visits.Entries = append(visits.Entries, VisitEntry{
		Key: "frontend:main", Score: 50, LastVisited: 9900,
	})
	index := &Index{
		Targets: []CatalogTarget{
			makeTarget("backend", "main", "/repos/backend"),
			makeTarget("frontend", "main", "/repos/frontend"),
		},
		Visits: visits,
	}
	results, err := Resolve("end", index, defaultCfg(), defaultCtx(10000))
	if err != nil {
		t.Fatal(err)
	}
	if results[0].RepoName != "frontend" {
		t.Errorf("frecency should boost frontend, got %s", results[0].RepoName)
	}
}

func TestResolveEmptyCatalog(t *testing.T) {
	index := &Index{Visits: NewVisitsDB()}
	_, err := Resolve("test", index, defaultCfg(), defaultCtx(10000))
	if err != ErrEmptyCatalog {
		t.Errorf("expected ErrEmptyCatalog, got %v", err)
	}
}

func TestResolveNoMatches(t *testing.T) {
	index := &Index{
		Targets: []CatalogTarget{makeTarget("backend", "main", "/repos/backend")},
		Visits:  NewVisitsDB(),
	}
	_, err := Resolve("zzzzzzzzz", index, defaultCfg(), defaultCtx(10000))
	if _, ok := err.(ErrNoMatches); !ok {
		t.Errorf("expected ErrNoMatches, got %v", err)
	}
}

func TestResolveQueryTooShort(t *testing.T) {
	index := &Index{
		Targets: []CatalogTarget{makeTarget("backend", "main", "/repos/backend")},
		Visits:  NewVisitsDB(),
	}
	_, err := Resolve("a", index, defaultCfg(), defaultCtx(10000))
	if err != ErrQueryTooShort {
		t.Errorf("expected ErrQueryTooShort, got %v", err)
	}
}

func TestResolveColdStartFuzzy(t *testing.T) {
	index := &Index{
		Targets: []CatalogTarget{
			makeTarget("auth-service", "main", "/repos/auth-service"),
			makeTarget("billing-service", "main", "/repos/billing-service"),
		},
		Visits: NewVisitsDB(),
	}
	results, err := Resolve("auth", index, defaultCfg(), defaultCtx(10000))
	if err != nil {
		t.Fatal(err)
	}
	if results[0].RepoName != "auth-service" {
		t.Errorf("got %s", results[0].RepoName)
	}
}

func TestResolveProjectNameMatches(t *testing.T) {
	index := &Index{
		Targets: []CatalogTarget{
			makeTargetWithProject("monorepo", "main", "auth-module", "/repos/monorepo"),
		},
		Visits: NewVisitsDB(),
	}
	results, err := Resolve("auth", index, defaultCfg(), defaultCtx(10000))
	if err != nil {
		t.Fatal(err)
	}
	if len(results) == 0 {
		t.Error("should match on project name")
	}
}

// --- Disambiguation ---

func TestDisambiguationTriggered(t *testing.T) {
	results := []Target{
		{Score: 0.50}, {Score: 0.48},
	}
	if !NeedsDisambiguation(results, 0.10) {
		t.Error("expected disambiguation")
	}
}

func TestDisambiguationNotTriggered(t *testing.T) {
	results := []Target{
		{Score: 0.90}, {Score: 0.30},
	}
	if NeedsDisambiguation(results, 0.10) {
		t.Error("should not need disambiguation")
	}
}

func TestDisambiguationSingle(t *testing.T) {
	results := []Target{{Score: 0.90}}
	if NeedsDisambiguation(results, 0.10) {
		t.Error("single result should not need disambiguation")
	}
}

func TestDisambiguationZeroScore(t *testing.T) {
	results := []Target{{Score: 0}, {Score: 0}}
	if NeedsDisambiguation(results, 0.10) {
		t.Error("zero scores should not trigger disambiguation")
	}
}

// --- ResolverConfig defaults ---

func TestResolverConfigWeightsSumToOne(t *testing.T) {
	c := config.DefaultResolver()
	total := c.WeightFuzzy + c.WeightFrecency + c.WeightActivity + c.WeightContext
	if abs(total-1.0) > 0.001 {
		t.Errorf("weights sum = %f, want 1.0", total)
	}
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
