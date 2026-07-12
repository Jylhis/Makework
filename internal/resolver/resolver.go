// Package resolver implements weighted fuzzy resolution of project names
// using fuzzy matching, frecency (visit history), activity, and cwd context.
package resolver

import (
	"cmp"
	"errors"
	"fmt"
	"math"
	"path/filepath"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/agnivade/levenshtein"
	"github.com/jylhis/makework/internal/catalog"
	"github.com/jylhis/makework/internal/config"
	"github.com/jylhis/makework/internal/query"
)

// --- Types ---

type Target struct {
	Path         string
	RepoName     string
	Branch       string
	ProjectName  string
	Score        float64
	SignalScores SignalScores
}

type SignalScores struct {
	Fuzzy    float64
	Frecency float64
	Activity float64
	Context  float64
}

type CatalogTarget struct {
	RepoName    string
	Branch      string
	ProjectName string
	Path        string
	// Activity is a unit-less recency-of-commits signal for this
	// repo+branch, populated by BuildTargets via query.LogWorktree.
	Activity float64
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

func scoreTarget(t *CatalogTarget, query string, visits *VisitsDB, cfg *config.ResolverConfig, ctx *ResolveContext) (float64, SignalScores) {
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

	activity := t.Activity

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

	return total, signals
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
		score, signals := scoreTarget(t, query, &index.Visits, cfg, ctx)
		if score <= 0 {
			continue
		}
		results = append(results, Target{
			Path:         t.Path,
			RepoName:     t.RepoName,
			Branch:       t.Branch,
			ProjectName:  t.ProjectName,
			Score:        score,
			SignalScores: signals,
		})
	}

	slices.SortFunc(results, func(a, b Target) int {
		return cmp.Compare(b.Score, a.Score)
	})

	if len(results) == 0 {
		return nil, ErrNoMatches{Query: query}
	}
	return results, nil
}

// DefaultDisambiguationThreshold is the relative gap below which the top
// two results are considered too close to auto-pick, prompting the user to
// disambiguate. Expressed as a fraction of the top score.
const DefaultDisambiguationThreshold = 0.10

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

// BuildTargets creates CatalogTarget entries from a Catalog and
// annotates each with a recent-activity score derived from one
// `git rev-list --count --since=30.days.ago` call per repo against the
// default branch when activity scoring is enabled. Errors are swallowed
// (activity stays 0) so a flaky repo can't break resolution.
func BuildTargets(cat *catalog.Catalog, cfg *config.ResolverConfig) []CatalogTarget {
	activity := make(map[string]float64, len(cat.Repos))
	if cfg != nil && cfg.WeightActivity > 0 {
		for repoName, r := range cat.Repos {
			commitCount, err := query.RecentCommitCount(r.Path, r.MainBranch, "30.days.ago")
			if err != nil {
				continue
			}
			activity[repoName] = math.Log1p(float64(commitCount))
		}
	}

	var targets []CatalogTarget
	for repoName, r := range cat.Repos {
		act := activity[repoName]
		targets = append(targets, CatalogTarget{
			RepoName: repoName,
			Branch:   r.MainBranch,
			Path:     r.Path,
			Activity: act,
		})
		for projName, proj := range r.Projects {
			if projName != repoName {
				targets = append(targets, CatalogTarget{
					RepoName:    repoName,
					Branch:      r.MainBranch,
					ProjectName: projName,
					Path:        r.Path,
					Activity:    act,
				})
			}
			for subName, sub := range proj.Subprojects {
				targets = append(targets, CatalogTarget{
					RepoName:    repoName,
					Branch:      r.MainBranch,
					ProjectName: subName,
					Path:        filepath.Join(r.Path, sub.SubprojectPath),
					Activity:    act,
				})
			}
		}
	}
	return targets
}
