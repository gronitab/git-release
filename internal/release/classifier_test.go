package release_test

import (
	"testing"

	"github.com/gronitab/git-release/internal/release"
	"github.com/gronitab/git-release/internal/semver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func mustParse(t *testing.T, s string) semver.Version {
	t.Helper()
	v, err := semver.ParseTag(s, "v")
	require.NoError(t, err)
	return v
}

func mustParseAll(t *testing.T, tags ...string) []semver.Version {
	t.Helper()
	var vs []semver.Version
	for _, tag := range tags {
		vs = append(vs, mustParse(t, tag))
	}
	return vs
}

func TestClassify_Empty(t *testing.T) {
	s := release.Classify(nil)
	assert.True(t, s.LatestProd.Equal(semver.Version{}))
	assert.Nil(t, s.InFlightRC)
}

func TestClassify_ProdOnly(t *testing.T) {
	tags := mustParseAll(t, "v1.2.2", "v1.1.0", "v1.0.0")
	s := release.Classify(tags)

	assert.Equal(t, mustParse(t, "v1.2.2"), s.LatestProd)
	assert.Nil(t, s.InFlightRC)
}

func TestClassify_InFlightRC(t *testing.T) {
	// RC whose base > latest prod → in-flight
	tags := mustParseAll(t, "v1.2.2", "v1.3.0-rc.2", "v1.3.0-rc")
	s := release.Classify(tags)

	assert.Equal(t, mustParse(t, "v1.2.2"), s.LatestProd)
	require.NotNil(t, s.InFlightRC)
	assert.Equal(t, mustParse(t, "v1.3.0-rc.2"), *s.InFlightRC)
}

func TestClassify_RCSupersededByProd(t *testing.T) {
	// RC whose base == latest prod → NOT in-flight
	tags := mustParseAll(t, "v1.3.0", "v1.3.0-rc.2", "v1.2.2")
	s := release.Classify(tags)

	assert.Equal(t, mustParse(t, "v1.3.0"), s.LatestProd)
	assert.Nil(t, s.InFlightRC)
}

func TestClassify_MultipleRCsPicksHighest(t *testing.T) {
	// Multiple RCs for the same target → in-flight is the highest one
	tags := mustParseAll(t, "v1.2.2", "v1.2.3-rc", "v1.2.3-rc.2", "v1.2.3-rc.3")
	s := release.Classify(tags)

	require.NotNil(t, s.InFlightRC)
	assert.Equal(t, mustParse(t, "v1.2.3-rc.3"), *s.InFlightRC)
}

func TestClassify_NoProd_RCIsInflight(t *testing.T) {
	// No prod tag at all — any RC is in-flight
	tags := mustParseAll(t, "v0.1.0-rc")
	s := release.Classify(tags)

	assert.True(t, s.LatestProd.Equal(semver.Version{}))
	require.NotNil(t, s.InFlightRC)
	assert.Equal(t, mustParse(t, "v0.1.0-rc"), *s.InFlightRC)
}

// --- NextStage ---

func TestNextStage_NoRC_NoProd_DefaultsToPatch(t *testing.T) {
	// From nothing, default scope is patch of 0.0.0 → 0.0.1-rc
	s := release.State{LatestProd: semver.Version{}}
	next, err := release.NextStage(s, release.ScopeNone)
	require.NoError(t, err)
	assert.Equal(t, mustParse(t, "v0.0.1-rc"), next)
}

func TestNextStage_NoRC_CreatesPatchRC(t *testing.T) {
	s := release.State{LatestProd: mustParse(t, "v1.2.2")}
	next, err := release.NextStage(s, release.ScopeNone)
	require.NoError(t, err)
	assert.Equal(t, mustParse(t, "v1.2.3-rc"), next)
}

func TestNextStage_ExistingRC_Increments(t *testing.T) {
	rc := mustParse(t, "v1.2.3-rc")
	s := release.State{LatestProd: mustParse(t, "v1.2.2"), InFlightRC: &rc}
	next, err := release.NextStage(s, release.ScopeNone)
	require.NoError(t, err)
	assert.Equal(t, mustParse(t, "v1.2.3-rc.2"), next)
}

func TestNextStage_ExplicitMinor_NoRC(t *testing.T) {
	s := release.State{LatestProd: mustParse(t, "v1.2.2")}
	next, err := release.NextStage(s, release.ScopeMinor)
	require.NoError(t, err)
	assert.Equal(t, mustParse(t, "v1.3.0-rc"), next)
}

func TestNextStage_ExplicitMinor_StalePatchRC(t *testing.T) {
	// Stale patch RC exists; minor target is higher → create new minor RC
	rc := mustParse(t, "v1.2.3-rc")
	s := release.State{LatestProd: mustParse(t, "v1.2.2"), InFlightRC: &rc}
	next, err := release.NextStage(s, release.ScopeMinor)
	require.NoError(t, err)
	assert.Equal(t, mustParse(t, "v1.3.0-rc"), next)
}

func TestNextStage_ExplicitMinor_MatchingRC_Increments(t *testing.T) {
	rc := mustParse(t, "v1.3.0-rc")
	s := release.State{LatestProd: mustParse(t, "v1.2.2"), InFlightRC: &rc}
	next, err := release.NextStage(s, release.ScopeMinor)
	require.NoError(t, err)
	assert.Equal(t, mustParse(t, "v1.3.0-rc.2"), next)
}

func TestNextStage_ExplicitMinor_HigherRCInFlight_Errors(t *testing.T) {
	rc := mustParse(t, "v2.0.0-rc")
	s := release.State{LatestProd: mustParse(t, "v1.2.2"), InFlightRC: &rc}
	_, err := release.NextStage(s, release.ScopeMinor)
	assert.ErrorIs(t, err, release.ErrHigherScopeInFlight)
}

func TestNextStage_ExplicitMajor_NoRC(t *testing.T) {
	s := release.State{LatestProd: mustParse(t, "v1.2.2")}
	next, err := release.NextStage(s, release.ScopeMajor)
	require.NoError(t, err)
	assert.Equal(t, mustParse(t, "v2.0.0-rc"), next)
}

// --- NextRelease ---

func TestNextRelease_NoRC_BumpsPatch(t *testing.T) {
	s := release.State{LatestProd: mustParse(t, "v1.2.2")}
	next, err := release.NextRelease(s, release.ScopeNone)
	require.NoError(t, err)
	assert.Equal(t, mustParse(t, "v1.2.3"), next)
}

func TestNextRelease_InFlightRC_Promotes(t *testing.T) {
	rc := mustParse(t, "v1.3.0-rc.2")
	s := release.State{LatestProd: mustParse(t, "v1.2.2"), InFlightRC: &rc}
	next, err := release.NextRelease(s, release.ScopeNone)
	require.NoError(t, err)
	assert.Equal(t, mustParse(t, "v1.3.0"), next)
}

func TestNextRelease_ExplicitScope_NoRC(t *testing.T) {
	s := release.State{LatestProd: mustParse(t, "v1.2.2")}
	next, err := release.NextRelease(s, release.ScopeMinor)
	require.NoError(t, err)
	assert.Equal(t, mustParse(t, "v1.3.0"), next)
}

func TestNextRelease_ExplicitScope_BypassesRC(t *testing.T) {
	// With explicit scope and in-flight RC → returns ErrRCInFlight so caller
	// can prompt for confirmation.
	rc := mustParse(t, "v1.3.0-rc.2")
	s := release.State{LatestProd: mustParse(t, "v1.2.2"), InFlightRC: &rc}
	_, err := release.NextRelease(s, release.ScopeMajor)
	assert.ErrorIs(t, err, release.ErrRCInFlight)
}
