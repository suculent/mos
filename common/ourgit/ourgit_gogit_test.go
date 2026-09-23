// Copyright (c) 2014-2017 Cesanta Software Limited
// All rights reserved

package ourgit

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// The go-git backend is the only first-party consumer of go-git's API, and it
// has had no test coverage -- which made bumping go-git across 15 minor
// versions a leap of faith. This exercises the calls mos actually makes during
// a build (Clone, Checkout, hash and ref lookups) against a repository the
// test builds itself, so it needs no network.
func TestGoGitAgainstLocalRepo(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git CLI not available")
	}

	src := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = src
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@example.com",
			"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@example.com")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}

	run("init", "-q", "-b", "master")
	if err := os.WriteFile(filepath.Join(src, "mos.yml"), []byte("author: test\n"), 0644); err != nil {
		t.Fatal(err)
	}
	run("add", "mos.yml")
	run("commit", "-q", "-m", "initial")
	run("tag", "1.0")
	run("branch", "release")

	g := NewOurGitGoGit(nil)

	dst := filepath.Join(t.TempDir(), "clone")
	if err := g.Clone(src, dst, CloneOptions{Ref: "master"}); err != nil {
		t.Fatalf("Clone: %v", err)
	}

	hash, err := g.GetCurrentHash(dst)
	if err != nil {
		t.Fatalf("GetCurrentHash: %v", err)
	}
	if len(hash) != fullHashLen {
		t.Errorf("GetCurrentHash returned %q, want a %d-char hash", hash, fullHashLen)
	}

	// Local branches only: a clone materialises just the branch it checked
	// out, so "release" exists here purely as a remote-tracking ref and is
	// correctly invisible to this call.
	ok, err := g.DoesBranchExist(dst, "master")
	if err != nil || !ok {
		t.Errorf("DoesBranchExist(master) = %v, %v; want true, nil", ok, err)
	}

	ok, err = g.DoesBranchExist(dst, "release")
	if err != nil || ok {
		t.Errorf("DoesBranchExist(release) = %v, %v; want false, nil", ok, err)
	}

	ok, err = g.DoesBranchExist(dst, "no-such-branch")
	if err != nil || ok {
		t.Errorf("DoesBranchExist(no-such-branch) = %v, %v; want false, nil", ok, err)
	}

	ok, err = g.DoesTagExist(dst, "1.0")
	if err != nil || !ok {
		t.Errorf("DoesTagExist(1.0) = %v, %v; want true, nil", ok, err)
	}

	if err := g.Checkout(dst, "1.0", RefTypeTag); err != nil {
		t.Fatalf("Checkout(tag 1.0): %v", err)
	}

	hashAtTag, err := g.GetCurrentHash(dst)
	if err != nil {
		t.Fatalf("GetCurrentHash after checkout: %v", err)
	}
	if !HashesEqual(hash, hashAtTag) {
		t.Errorf("tag 1.0 is at %q, want the initial commit %q", hashAtTag, hash)
	}

	clean, err := g.IsClean(dst, "1.0", nil)
	if err != nil {
		t.Fatalf("IsClean: %v", err)
	}
	if !clean {
		t.Error("IsClean = false on a fresh clone, want true")
	}

	origin, err := g.GetOriginURL(dst)
	if err != nil {
		t.Fatalf("GetOriginURL: %v", err)
	}
	if origin != src {
		t.Errorf("GetOriginURL = %q, want %q", origin, src)
	}
}
