package pod

import (
	"testing"

	"github.com/opencharly/spec/spec"
)

// ufWith is a loaded project carrying exactly the given namespaces.
func ufWith(names ...string) *spec.UnifiedFile {
	uf := &spec.UnifiedFile{Namespaces: map[string]*spec.UnifiedFile{}}
	for _, n := range names {
		uf.Namespaces[n] = &spec.UnifiedFile{}
	}
	return uf
}

// TestSplitOnNamespace pins opencharly/spec#450's rule — DESIGN D-REF-1: a dotted name's first
// segment is a NAMESPACE if one with that name exists, otherwise a node of this project. The
// measured failure was the first row: `charly update kde-vnc-cdp.check-kde-vnc-cdp-pod` read the
// whole dotted address as ONE box, so the container lookup missed the name start had created.
func TestSplitOnNamespace(t *testing.T) {
	tests := []struct {
		name     string
		uf       *spec.UnifiedFile
		box      string
		inst     string
		wantBox  string
		wantInst string
	}{
		{
			name:     "a mounted namespace splits the address into the deploy key pair",
			uf:       ufWith("kde-vnc-cdp"),
			box:      "kde-vnc-cdp.check-kde-vnc-cdp-pod",
			wantBox:  "kde-vnc-cdp",
			wantInst: "check-kde-vnc-cdp-pod",
		},
		{
			name:    "a dotted NESTED path whose head is not a namespace stays whole",
			uf:      ufWith("kde-vnc-cdp"),
			box:     "parent.child",
			wantBox: "parent.child",
		},
		{
			name:    "a root deploy name is untouched",
			uf:      ufWith("kde-vnc-cdp"),
			box:     "check-pod",
			wantBox: "check-pod",
		},
		{
			name:     "an explicit -i instance already carries the split",
			uf:       ufWith("kde-vnc-cdp"),
			box:      "kde-vnc-cdp.check-kde-vnc-cdp-pod",
			inst:     "prod",
			wantBox:  "kde-vnc-cdp.check-kde-vnc-cdp-pod",
			wantInst: "prod",
		},
		{
			name:    "no project loaded: nothing is guessed",
			uf:      nil,
			box:     "kde-vnc-cdp.check-kde-vnc-cdp-pod",
			wantBox: "kde-vnc-cdp.check-kde-vnc-cdp-pod",
		},
		{
			name:    "a trailing separator is not an address",
			uf:      ufWith("kde-vnc-cdp"),
			box:     "kde-vnc-cdp.",
			wantBox: "kde-vnc-cdp.",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotBox, gotInst := splitOnNamespace(tc.uf, tc.box, tc.inst)
			if gotBox != tc.wantBox || gotInst != tc.wantInst {
				t.Fatalf("splitOnNamespace(%q, %q) = (%q, %q), want (%q, %q)",
					tc.box, tc.inst, gotBox, gotInst, tc.wantBox, tc.wantInst)
			}
		})
	}
}
