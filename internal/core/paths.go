package core

// Paths in this file are slash-separated, not filepath-separated, because
// every one of them ends up inside a git config value: git wants forward
// slashes in includeIf patterns and include.path on every platform, Windows
// included. Input arriving from the filesystem side of ghu is normalised on
// the way in, so a filepath-joined home does not produce mixed separators.
//
// Nothing here performs I/O, which keeps the trickiest logic in ghu --
// directory matching and include ordering -- testable with plain strings.

import (
	"path"
	"path/filepath"
	"sort"
	"strings"
)

// Expand turns a leading ~ into home and cleans the result.
func Expand(p, home string) string {
	p, home = slash(p), slash(home)
	switch {
	case p == "~":
		return path.Clean(home)
	case strings.HasPrefix(p, "~/"):
		return path.Join(home, p[2:])
	default:
		return path.Clean(p)
	}
}

// Tildify is the inverse of Expand, for display only.
func Tildify(p, home string) string {
	home = path.Clean(slash(home))
	p = path.Clean(slash(p))
	if p == home {
		return "~"
	}
	if strings.HasPrefix(p, home+"/") {
		return "~/" + p[len(home)+1:]
	}
	return p
}

// Ordered returns the profiles sorted shortest-directory-first. Git applies
// includeIf entries in file order with the last match winning, so emitting
// shallow trees before deep ones makes the deepest match win, which is what
// makes git's resolution agree with Match.
func Ordered(profiles []Profile, home string) []Profile {
	out := make([]Profile, len(profiles))
	copy(out, profiles)
	sort.SliceStable(out, func(i, j int) bool {
		di := depth(Expand(out[i].Dir, home))
		dj := depth(Expand(out[j].Dir, home))
		if di != dj {
			return di < dj
		}
		return out[i].Name < out[j].Name
	})
	return out
}

// Match returns the profile governing dir, preferring the deepest tree that
// contains it.
func Match(profiles []Profile, dir, home string) (Profile, bool) {
	target := path.Clean(Expand(dir, home))

	var best Profile
	var bestDepth = -1
	var found bool

	for _, p := range profiles {
		root := path.Clean(Expand(p.Dir, home))
		if !within(target, root) {
			continue
		}
		if d := depth(root); d > bestDepth {
			best, bestDepth, found = p, d, true
		}
	}
	return best, found
}

// slash normalises a path to the forward-slash form these helpers work in. It
// is a no-op wherever the separator already is /.
func slash(p string) string { return filepath.ToSlash(p) }

// within reports whether target is root or sits underneath it.
//
// The separator is appended before comparing, so ~/codex does not match a
// profile rooted at ~/code. Comparison is case-insensitive, matching the
// gitdir/i patterns ghu writes.
func within(target, root string) bool {
	if strings.EqualFold(target, root) {
		return true
	}
	prefix := root
	if !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}
	return strings.HasPrefix(strings.ToLower(target), strings.ToLower(prefix))
}

func depth(p string) int {
	p = strings.Trim(path.Clean(p), "/")
	if p == "" {
		return 0
	}
	return strings.Count(p, "/") + 1
}
