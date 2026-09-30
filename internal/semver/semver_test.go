package semver_test

import (
	"testing"

	"github.com/gronitab/git-release/internal/semver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseTag(t *testing.T) {
	tests := []struct {
		input   string
		prefix  string
		want    semver.Version
		wantErr bool
	}{
		{
			input:  "v1.2.3",
			prefix: "v",
			want:   semver.Version{Major: 1, Minor: 2, Patch: 3, RC: 0, IsRC: false},
		},
		{
			input:  "v1.2.3-rc",
			prefix: "v",
			want:   semver.Version{Major: 1, Minor: 2, Patch: 3, RC: 1, IsRC: true},
		},
		{
			input:  "v1.2.3-rc.2",
			prefix: "v",
			want:   semver.Version{Major: 1, Minor: 2, Patch: 3, RC: 2, IsRC: true},
		},
		{
			input:  "v1.2.3-rc.10",
			prefix: "v",
			want:   semver.Version{Major: 1, Minor: 2, Patch: 3, RC: 10, IsRC: true},
		},
		{
			input:  "v0.0.1",
			prefix: "v",
			want:   semver.Version{Major: 0, Minor: 0, Patch: 1, RC: 0, IsRC: false},
		},
		{
			input:   "v1.2",
			prefix:  "v",
			wantErr: true,
		},
		{
			input:   "1.2.3",
			prefix:  "v",
			wantErr: true,
		},
		{
			input:   "vnope",
			prefix:  "v",
			wantErr: true,
		},
		{
			// custom prefix
			input:  "rel1.2.3",
			prefix: "rel",
			want:   semver.Version{Major: 1, Minor: 2, Patch: 3, RC: 0, IsRC: false},
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := semver.ParseTag(tt.input, tt.prefix)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestVersionString(t *testing.T) {
	tests := []struct {
		v      semver.Version
		prefix string
		want   string
	}{
		{semver.Version{Major: 1, Minor: 2, Patch: 3}, "v", "v1.2.3"},
		{semver.Version{Major: 1, Minor: 2, Patch: 3, RC: 1, IsRC: true}, "v", "v1.2.3-rc"},
		{semver.Version{Major: 1, Minor: 2, Patch: 3, RC: 2, IsRC: true}, "v", "v1.2.3-rc.2"},
		{semver.Version{Major: 1, Minor: 2, Patch: 3, RC: 5, IsRC: true}, "v", "v1.2.3-rc.5"},
		{semver.Version{Major: 2, Minor: 0, Patch: 0}, "v", "v2.0.0"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.v.String(tt.prefix))
		})
	}
}

func TestVersionComparison(t *testing.T) {
	mustParse := func(s string) semver.Version {
		v, err := semver.ParseTag(s, "v")
		require.NoError(t, err)
		return v
	}

	// Prod vs prod
	assert.True(t, mustParse("v1.2.3").GreaterThan(mustParse("v1.2.2")))
	assert.True(t, mustParse("v1.3.0").GreaterThan(mustParse("v1.2.9")))
	assert.True(t, mustParse("v2.0.0").GreaterThan(mustParse("v1.99.99")))
	assert.False(t, mustParse("v1.2.3").GreaterThan(mustParse("v1.2.3")))

	// RC is lower than the same prod version
	assert.True(t, mustParse("v1.2.3").GreaterThan(mustParse("v1.2.3-rc")))
	assert.True(t, mustParse("v1.2.3").GreaterThan(mustParse("v1.2.3-rc.5")))

	// RC ordering
	assert.True(t, mustParse("v1.2.3-rc.2").GreaterThan(mustParse("v1.2.3-rc")))
	assert.True(t, mustParse("v1.2.3-rc.3").GreaterThan(mustParse("v1.2.3-rc.2")))

	// Cross-version RC
	assert.True(t, mustParse("v1.3.0-rc").GreaterThan(mustParse("v1.2.3")))
	assert.True(t, mustParse("v1.3.0-rc").GreaterThan(mustParse("v1.2.3-rc.99")))
}

func TestVersionEquals(t *testing.T) {
	mustParse := func(s string) semver.Version {
		v, err := semver.ParseTag(s, "v")
		require.NoError(t, err)
		return v
	}

	assert.True(t, mustParse("v1.2.3").Equal(mustParse("v1.2.3")))
	assert.True(t, mustParse("v1.2.3-rc").Equal(mustParse("v1.2.3-rc")))
	assert.True(t, mustParse("v1.2.3-rc.2").Equal(mustParse("v1.2.3-rc.2")))
	assert.False(t, mustParse("v1.2.3").Equal(mustParse("v1.2.3-rc")))
	assert.False(t, mustParse("v1.2.3-rc").Equal(mustParse("v1.2.3-rc.2")))
}

func TestBaseVersion(t *testing.T) {
	mustParse := func(s string) semver.Version {
		v, err := semver.ParseTag(s, "v")
		require.NoError(t, err)
		return v
	}

	rc := mustParse("v1.2.3-rc.4")
	base := rc.Base()
	assert.Equal(t, mustParse("v1.2.3"), base)
	assert.False(t, base.IsRC)
}

func TestBumpVersion(t *testing.T) {
	mustParse := func(s string) semver.Version {
		v, err := semver.ParseTag(s, "v")
		require.NoError(t, err)
		return v
	}

	v := mustParse("v1.2.3")

	patch := v.BumpPatch()
	assert.Equal(t, mustParse("v1.2.4"), patch)

	minor := v.BumpMinor()
	assert.Equal(t, mustParse("v1.3.0"), minor)

	major := v.BumpMajor()
	assert.Equal(t, mustParse("v2.0.0"), major)
}

func TestNextRC(t *testing.T) {
	mustParse := func(s string) semver.Version {
		v, err := semver.ParseTag(s, "v")
		require.NoError(t, err)
		return v
	}

	v := mustParse("v1.2.3")

	first := v.FirstRC()
	assert.Equal(t, mustParse("v1.2.3-rc"), first)

	rc1 := mustParse("v1.2.3-rc")
	rc2 := rc1.NextRC()
	assert.Equal(t, mustParse("v1.2.3-rc.2"), rc2)

	rc2v := mustParse("v1.2.3-rc.2")
	rc3 := rc2v.NextRC()
	assert.Equal(t, mustParse("v1.2.3-rc.3"), rc3)
}

func TestSortVersions(t *testing.T) {
	mustParse := func(s string) semver.Version {
		v, err := semver.ParseTag(s, "v")
		require.NoError(t, err)
		return v
	}

	versions := []semver.Version{
		mustParse("v1.2.3-rc"),
		mustParse("v1.3.0"),
		mustParse("v1.2.3"),
		mustParse("v1.3.0-rc.2"),
		mustParse("v1.2.2"),
	}

	semver.SortDesc(versions)

	expected := []semver.Version{
		mustParse("v1.3.0"),
		mustParse("v1.3.0-rc.2"),
		mustParse("v1.2.3"),
		mustParse("v1.2.3-rc"),
		mustParse("v1.2.2"),
	}

	assert.Equal(t, expected, versions)
}
