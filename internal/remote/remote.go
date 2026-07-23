// Package remote parses hosted Git remotes into their storage identity.
package remote

import (
	"fmt"
	"net/url"
	"strings"
)

// Identity is the transport-independent portion of a hosted Git remote.
type Identity struct {
	Host     string
	Segments []string
}

// Parse accepts SCP-style SSH remotes, ssh:// URLs, and https:// URLs.
func Parse(raw string) (Identity, error) {
	if raw == "" {
		return Identity{}, fmt.Errorf("remote is empty")
	}

	if strings.Contains(raw, `\`) {
		return Identity{}, fmt.Errorf("remote %q contains a backslash; use a hosted SCP, SSH, or HTTPS remote", raw)
	}

	if strings.Contains(raw, "://") {
		return parseURL(raw)
	}

	return parseSCP(raw)
}

func parseURL(raw string) (Identity, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return Identity{}, fmt.Errorf("invalid remote %q: %w", raw, err)
	}
	if !strings.EqualFold(u.Scheme, "ssh") && !strings.EqualFold(u.Scheme, "https") {
		return Identity{}, fmt.Errorf("unsupported remote scheme %q in %q; use SCP-style SSH, ssh://, or https://", u.Scheme, raw)
	}
	if u.RawQuery != "" || u.Fragment != "" {
		return Identity{}, fmt.Errorf("remote %q must not contain a query or fragment", raw)
	}
	if u.Host == "" {
		return Identity{}, fmt.Errorf("remote %q has no host", raw)
	}

	host := u.Hostname()
	if host == "" {
		return Identity{}, fmt.Errorf("remote %q has no host", raw)
	}

	return normalize(host, u.Path, raw)
}

func parseSCP(raw string) (Identity, error) {
	colon := strings.IndexByte(raw, ':')
	if colon < 1 || colon == len(raw)-1 {
		return Identity{}, fmt.Errorf("remote %q is not a hosted remote; use SCP-style SSH, ssh://, or https://", raw)
	}

	authority, path := raw[:colon], raw[colon+1:]
	if strings.ContainsAny(authority, `/`) || strings.Contains(authority, ":") {
		return Identity{}, fmt.Errorf("remote %q is not a valid SCP-style remote", raw)
	}

	host := authority
	if at := strings.LastIndexByte(authority, '@'); at >= 0 {
		if at == 0 {
			return Identity{}, fmt.Errorf("remote %q has an empty username", raw)
		}
		host = authority[at+1:]
	}

	return normalize(host, path, raw)
}

func normalize(host, path, raw string) (Identity, error) {
	if err := validateHost(host); err != nil {
		return Identity{}, fmt.Errorf("invalid remote %q: %w", raw, err)
	}

	path = strings.Trim(path, "/")
	path = strings.TrimSuffix(path, ".git")
	if path == "" {
		return Identity{}, fmt.Errorf("remote %q has no repository path", raw)
	}

	segments := strings.Split(path, "/")
	for _, segment := range segments {
		if segment == "" {
			return Identity{}, fmt.Errorf("remote %q contains an empty repository path segment", raw)
		}
		if segment == "." || segment == ".." {
			return Identity{}, fmt.Errorf("remote %q contains unsafe path segment %q", raw, segment)
		}
		if strings.IndexByte(segment, 0) >= 0 {
			return Identity{}, fmt.Errorf("remote %q contains a NUL byte", raw)
		}
	}

	return Identity{Host: host, Segments: segments}, nil
}

func validateHost(host string) error {
	if host == "" {
		return fmt.Errorf("host is empty")
	}
	if host == "." || host == ".." {
		return fmt.Errorf("host %q is unsafe", host)
	}
	if strings.ContainsAny(host, `/\`) || strings.IndexByte(host, 0) >= 0 {
		return fmt.Errorf("host %q is not a filesystem-safe host token", host)
	}
	return nil
}
