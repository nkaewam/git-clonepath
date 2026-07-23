package remote

import (
	"reflect"
	"strings"
	"testing"
)

func TestParseSupportedRemotes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		raw  string
		want Identity
	}{
		{
			name: "SCP with username",
			raw:  "git@github.com:nkaewam/git-clonepath.git",
			want: Identity{Host: "github.com", Segments: []string{"nkaewam", "git-clonepath"}},
		},
		{
			name: "SCP alias preserves case",
			raw:  "WorkGit:Team/Repo.git",
			want: Identity{Host: "WorkGit", Segments: []string{"Team", "Repo"}},
		},
		{
			name: "SSH URL drops user and port",
			raw:  "ssh://deploy@Git.Example.com:2222/Org/Repo.git",
			want: Identity{Host: "Git.Example.com", Segments: []string{"Org", "Repo"}},
		},
		{
			name: "HTTPS drops credentials and port",
			raw:  "https://user:token@github.example:8443/acme/widgets.git/",
			want: Identity{Host: "github.example", Segments: []string{"acme", "widgets"}},
		},
		{
			name: "URL scheme is case insensitive",
			raw:  "HTTPS://Git.Example.com/Org/Repo.git",
			want: Identity{Host: "Git.Example.com", Segments: []string{"Org", "Repo"}},
		},
		{
			name: "one suffix only",
			raw:  "https://example.com/team/repo.git.git",
			want: Identity{Host: "example.com", Segments: []string{"team", "repo.git"}},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := Parse(tt.raw)
			if err != nil {
				t.Fatalf("Parse(%q): %v", tt.raw, err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("Parse(%q) = %#v, want %#v", tt.raw, got, tt.want)
			}
		})
	}
}

func TestEquivalentTransportsHaveSameIdentity(t *testing.T) {
	t.Parallel()

	a, err := Parse("git@example.com:group/repo.git")
	if err != nil {
		t.Fatal(err)
	}
	b, err := Parse("ssh://other@example.com:2222/group/repo.git")
	if err != nil {
		t.Fatal(err)
	}
	c, err := Parse("https://example.com/group/repo.git")
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(a, b) || !reflect.DeepEqual(b, c) {
		t.Fatalf("identities differ: %#v %#v %#v", a, b, c)
	}
}

func TestParseRejectsUnsafeAndUnsupportedRemotes(t *testing.T) {
	t.Parallel()

	tests := []string{
		"",
		"repo",
		"./repo",
		"/tmp/repo",
		`C:\repo`,
		"file:///tmp/repo",
		"git://example.com/team/repo.git",
		"https://example.com",
		"https://example.com/.git",
		"https://example.com/team//repo.git",
		"https://example.com/team/../repo.git",
		"https://example.com/team/./repo.git",
		"https://example.com/team/%2e%2e/repo.git",
		"https://example.com/team/%2F/repo.git",
		"https://example.com/team/repo.git?token=x",
		"git@:team/repo.git",
		"../host:team/repo.git",
	}

	for _, raw := range tests {
		raw := raw
		t.Run(strings.ReplaceAll(raw, "/", "_"), func(t *testing.T) {
			t.Parallel()
			if got, err := Parse(raw); err == nil {
				t.Fatalf("Parse(%q) unexpectedly succeeded: %#v", raw, got)
			}
		})
	}
}
