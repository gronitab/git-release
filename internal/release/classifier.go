// Package release implements the core release logic: tag classification,
// in-flight RC detection, and next-version computation.
package release

import (
	"errors"

	"github.com/gronitab/git-release/internal/semver"
)

// Scope represents the semver bump scope for a release or stage command.
type Scope int

const (
	ScopeNone  Scope = iota // continue whatever is in flight; default to patch
	ScopePatch              // explicit patch bump
	ScopeMinor              // explicit minor bump
	ScopeMajor              // explicit major bump
)

// Sentinel errors.
var (
	// ErrHigherScopeInFlight is returned when staging a lower scope while a
	// higher in-flight RC already exists.
	ErrHigherScopeInFlight = errors.New("a higher-scope RC is already in flight; clean it up before staging a lower scope")

	// ErrRCInFlight is returned when doing an explicit-scope release while an
	// RC is in flight, so the caller can prompt for confirmation.
	ErrRCInFlight = errors.New("an RC is in flight")
)

// State holds the classified release state derived from the full tag list.
type State struct {
	// LatestProd is the highest production (non-RC) tag, or the zero Version
	// if no production tags exist.
	LatestProd semver.Version

	// InFlightRC is the highest RC whose base semver is strictly greater than
	// LatestProd, or nil if no such RC exists.
	InFlightRC *semver.Version

	// AllVersions holds every parsed version, sorted descending.
	AllVersions []semver.Version
}

// Classify derives the release State from a flat list of parsed versions.
func Classify(versions []semver.Version) State {
	semver.SortDesc(versions)

	var state State
	state.AllVersions = versions

	// Find latest prod
	for _, v := range versions {
		v := v
		if !v.IsRC {
			state.LatestProd = v
			break
		}
	}

	// Find highest in-flight RC (base > latest prod)
	for _, v := range versions {
		v := v
		if !v.IsRC {
			continue
		}
		base := v.Base()
		if base.GreaterThan(state.LatestProd) {
			state.InFlightRC = &v
			break // sorted desc, so first match is highest
		}
	}

	return state
}

// NextStage computes what tag `git release stage [scope]` would create.
func NextStage(s State, scope Scope) (semver.Version, error) {
	switch scope {
	case ScopeNone:
		return nextStageDefault(s)
	case ScopePatch:
		return nextStageScoped(s, s.LatestProd.BumpPatch())
	case ScopeMinor:
		return nextStageScoped(s, s.LatestProd.BumpMinor())
	case ScopeMajor:
		return nextStageScoped(s, s.LatestProd.BumpMajor())
	default:
		return nextStageDefault(s)
	}
}

func nextStageDefault(s State) (semver.Version, error) {
	if s.InFlightRC != nil {
		return s.InFlightRC.NextRC(), nil
	}
	target := s.LatestProd.BumpPatch()
	return target.FirstRC(), nil
}

func nextStageScoped(s State, target semver.Version) (semver.Version, error) {
	if s.InFlightRC == nil {
		return target.FirstRC(), nil
	}

	inFlightBase := s.InFlightRC.Base()

	switch {
	case inFlightBase.GreaterThan(target):
		return semver.Version{}, ErrHigherScopeInFlight
	case inFlightBase.Equal(target):
		return s.InFlightRC.NextRC(), nil
	default:
		// in-flight base < target: old RC becomes stale, start fresh
		return target.FirstRC(), nil
	}
}

// NextRelease computes what tag `git release [scope]` would create.
// Returns ErrRCInFlight when an explicit scope is given and an RC is in
// flight, so the caller can show a confirmation prompt.
func NextRelease(s State, scope Scope) (semver.Version, error) {
	switch scope {
	case ScopeNone:
		if s.InFlightRC != nil {
			return s.InFlightRC.Base(), nil
		}
		return s.LatestProd.BumpPatch(), nil

	case ScopePatch:
		return releaseWithScope(s, s.LatestProd.BumpPatch())
	case ScopeMinor:
		return releaseWithScope(s, s.LatestProd.BumpMinor())
	case ScopeMajor:
		return releaseWithScope(s, s.LatestProd.BumpMajor())
	default:
		return NextRelease(s, ScopeNone)
	}
}

func releaseWithScope(s State, target semver.Version) (semver.Version, error) {
	if s.InFlightRC != nil {
		return semver.Version{}, ErrRCInFlight
	}
	return target, nil
}
