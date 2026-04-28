// Package resolver implements weighted fuzzy resolution of project names
// using fuzzy matching, frecency (visit history), activity, and cwd context.
package resolver

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/agnivade/levenshtein"
	"github.com/jylhis/makework/internal/catalog"
	"github.com/jylhis/makework/internal/config"
)

// --- Types ---

type Target struct {
	Path         string
	RepoName     string
	Branch       string
	ProjectName  string
	Score        float64
	MatchReason  MatchReason
	SignalScores SignalScores
}

type SignalScores struct {
	Fuzzy    float64
	Frecency float64
	Activity float64
	Context  float64
}

type MatchReason int

const (
	MatchExactName MatchReason = iota
	MatchFuzzyName
	MatchBranchMatch
	MatchRecentActivity
	MatchFrecency
)

type CatalogTarget struct {
	RepoName    string
	Branch      string
	ProjectName string
	Path        string
}

type ResolveContext struct {
	Cwd string
	Now uint64
}

func DefaultContext() ResolveContext {
	return ResolveContext{
		Now: uint64(time.Now().Unix()),
	}
}

type Index struct {
	Targets []CatalogTarget
	Visits  VisitsDB
}

// --- Errors ---

var (
	ErrQueryTooShort = errors.New("query too short (minimum 2 characters)")
	ErrEmptyCatalog  = errors.New("no projects registered; run 'mw repo sync' or 'mw repo add' first")
)

type ErrNoMatches struct{ Query string }

func (e ErrNoMatches) Error() string { return fmt.Sprintf("no projects match '%s'", e.Query) }

type ErrInvalidAtSyntax struct{ Detail string }

func (e ErrInvalidAtSyntax) Error() string { return fmt.Sprintf("invalid @-syntax in '%s'", e.Detail) }

// --- Query parsing ---

type ParsedQuery struct {
	Repo   string // non-empty for Explicit
	Branch string // non-empty for Explicit
	Fuzzy  string // non-empty for fuzzy
}

func (q ParsedQuery) IsExplicit() bool { return q.Repo != "" }

func ParseQuery(query string) (ParsedQuery, error) {
	query = strings.TrimSpace(query)
	if utf8.RuneCountInString(query) < 2 {
		return ParsedQuery{}, ErrQueryTooShort
	}
	if i := strings.Index(query, "@"); i >= 0 {
		repo := query[:i]
		branch := query[i+1:]
		if repo == "" {
			return ParsedQuery{}, ErrInvalidAtSyntax{Detail: "missing project name in '" + query + "'"}
		}
		if branch == "" {
			return ParsedQuery{}, ErrInvalidAtSyntax{Detail: "missing branch name in '" + query + "'"}
		}
		return ParsedQuery{Repo: repo, Branch: branch}, nil
	}
	return ParsedQuery{Fuzzy: query}, nil
}

// --- Visits DB ---

type VisitEntry struct {
	Key         string  `json:"key"`
	Score       float64 `json:"score"`
	LastVisited uint64  `json:"last_visited"`
}

type VisitsDB struct {
	Entries []VisitEntry `json:"entries"`
	MaxAge  float64      `json:"max_age"`
}

func NewVisitsDB() VisitsDB {
	return VisitsDB{MaxAge: 10000.0}
}

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

// --- Scoring ---

// NormalizedLevenshtein computes 1 - lev(a,b)/max(runeCount(a),runeCount(b)).
// Uses rune count (not byte count) for the divisor to match strsim 0.11.
func NormalizedLevenshtein(a, b string) float64 {
	ra := utf8.RuneCountInString(a)
	rb := utf8.RuneCountInString(b)
	if ra == 0 && rb == 0 {
		return 1.0
	}
	maxLen := ra
	if rb > maxLen {
		maxLen = rb
	}
	dist := levenshtein.ComputeDistance(a, b)
	return 1.0 - float64(dist)/float64(maxLen)
}

func FuzzyScore(query, targetName string) float64 {
	if targetName == "" || query == "" {
		return 0
	}
	qLow := strings.ToLower(query)
	tLow := strings.ToLower(targetName)
	if qLow == tLow {
		return 1.0
	}
	lev := NormalizedLevenshtein(qLow, tLow)
	substringBonus := 0.0
	if strings.Contains(tLow, qLow) {
		substringBonus = 0.2
	}
	prefixMul := 1.0
	if strings.HasPrefix(tLow, qLow) {
		prefixMul = 1.3
	}
	raw := (lev + substringBonus) * prefixMul
	return math.Min(raw, 0.95)
}

func scoreTarget(t *CatalogTarget, query string, visits *VisitsDB, cfg *config.ResolverConfig, ctx *ResolveContext) (float64, MatchReason, SignalScores) {
	bestFuzzy := FuzzyScore(query, t.RepoName)
	if t.ProjectName != "" {
		if s := FuzzyScore(query, t.ProjectName); s > bestFuzzy {
			bestFuzzy = s
		}
	}
	if t.Branch != "" {
		if s := FuzzyScore(query, t.Branch); s > bestFuzzy {
			bestFuzzy = s
		}
	}
	if base := filepath.Base(t.Path); base != "" {
		if s := FuzzyScore(query, base); s > bestFuzzy {
			bestFuzzy = s
		}
	}

	visitKey := t.RepoName + ":" + t.Branch
	repoPrefix := t.RepoName + ":"
	frecency := visits.FrecencyScoreWithSiblings(visitKey, repoPrefix, ctx.Now)

	activity := 0.0

	contextScore := 0.0
	if ctx.Cwd != "" {
		if strings.HasPrefix(ctx.Cwd, t.Path) || strings.HasPrefix(t.Path, ctx.Cwd) {
			contextScore = 0.5
		}
	}

	signals := SignalScores{
		Fuzzy:    bestFuzzy,
		Frecency: frecency,
		Activity: activity,
		Context:  contextScore,
	}

	total := bestFuzzy*cfg.WeightFuzzy +
		frecency*cfg.WeightFrecency +
		activity*cfg.WeightActivity +
		contextScore*cfg.WeightContext

	reason := MatchFuzzyName
	best := bestFuzzy * cfg.WeightFuzzy
	if fW := frecency * cfg.WeightFrecency; fW > best {
		best = fW
		reason = MatchFrecency
	}
	if aW := activity * cfg.WeightActivity; aW > best {
		reason = MatchRecentActivity
	}

	return total, reason, signals
}

// --- Main resolve ---

func Resolve(query string, index *Index, cfg *config.ResolverConfig, ctx *ResolveContext) ([]Target, error) {
	if len(index.Targets) == 0 {
		return nil, ErrEmptyCatalog
	}
	if utf8.RuneCountInString(strings.TrimSpace(query)) < 2 {
		return nil, ErrQueryTooShort
	}

	var results []Target
	for i := range index.Targets {
		t := &index.Targets[i]
		score, reason, signals := scoreTarget(t, query, &index.Visits, cfg, ctx)
		if score <= 0 {
			continue
		}
		results = append(results, Target{
			Path:         t.Path,
			RepoName:     t.RepoName,
			Branch:       t.Branch,
			ProjectName:  t.ProjectName,
			Score:        score,
			MatchReason:  reason,
			SignalScores: signals,
		})
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	if len(results) == 0 {
		return nil, ErrNoMatches{Query: query}
	}
	return results, nil
}

func NeedsDisambiguation(results []Target, threshold float64) bool {
	if len(results) < 2 {
		return false
	}
	top := results[0].Score
	if top == 0 {
		return false
	}
	second := results[1].Score
	return (top-second)/top < threshold
}

// BuildTargets creates CatalogTarget entries from a Catalog.
func BuildTargets(cat *catalog.Catalog) []CatalogTarget {
	var targets []CatalogTarget
	for repoName, r := range cat.Repos {
		targets = append(targets, CatalogTarget{
			RepoName: repoName,
			Branch:   r.MainBranch,
			Path:     r.Path,
		})
		for projName, proj := range r.Projects {
			if projName != repoName {
				targets = append(targets, CatalogTarget{
					RepoName:    repoName,
					Branch:      r.MainBranch,
					ProjectName: projName,
					Path:        r.Path,
				})
			}
			for subName, sub := range proj.Subprojects {
				targets = append(targets, CatalogTarget{
					RepoName:    repoName,
					Branch:      r.MainBranch,
					ProjectName: subName,
					Path:        filepath.Join(r.Path, sub.SubprojectPath),
				})
			}
		}
	}
	return targets
}
