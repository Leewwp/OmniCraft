package main

import "testing"

func TestResolveProfileDefaultsToRelease(t *testing.T) {
	profile, err := resolveProfile("")
	if err != nil {
		t.Fatalf("resolve default profile: %v", err)
	}
	if profile.Name != "release" || !profile.RunReleaseChecks || !profile.RunExpiryChecks {
		t.Fatalf("default profile = %#v, want strict release checks", profile)
	}
	if profile.CrossRefs.IncludeArchive || profile.CrossRefs.OnlyArchive {
		t.Fatalf("release profile must exclude only archive cross-references: %#v", profile.CrossRefs)
	}
}

func TestResolveProfileArchive(t *testing.T) {
	profile, err := resolveProfile("archive")
	if err != nil {
		t.Fatalf("resolve archive profile: %v", err)
	}
	if profile.Name != "archive" || profile.RunReleaseChecks || profile.RunExpiryChecks {
		t.Fatalf("archive profile = %#v, want archive cross-references only", profile)
	}
	if !profile.CrossRefs.OnlyArchive || profile.CrossRefs.IncludeArchive {
		t.Fatalf("archive cross-reference options = %#v", profile.CrossRefs)
	}
}

func TestResolveProfileAll(t *testing.T) {
	profile, err := resolveProfile("all")
	if err != nil {
		t.Fatalf("resolve all profile: %v", err)
	}
	if profile.Name != "all" || !profile.RunReleaseChecks || !profile.RunExpiryChecks {
		t.Fatalf("all profile = %#v, want release and expiry checks", profile)
	}
	if !profile.CrossRefs.IncludeArchive || profile.CrossRefs.OnlyArchive {
		t.Fatalf("all cross-reference options = %#v", profile.CrossRefs)
	}
}

func TestResolveProfileRejectsUnknownProfile(t *testing.T) {
	if _, err := resolveProfile("quiet"); err == nil {
		t.Fatal("unknown profile must be rejected")
	}
}
