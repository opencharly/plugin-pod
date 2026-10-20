package pod

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
)

// TestDeployEntryDiscardNote_NamesWhatIsLostAndHowToKeepIt — `remove` deletes the deploy entry, and
// that entry is where the published port lives. Before this note the deletion was silent, which is how
// a remove+re-add came back on a random port instead of the configured one (charly#989). The message is
// the whole fix, so its contract is pinned here: it must name the box, say the configuration goes, and
// name the flag that keeps it.
func TestDeployEntryDiscardNote_NamesWhatIsLostAndHowToKeepIt(t *testing.T) {
	got := deployEntryDiscardNote("strata", "sc117")

	for _, want := range []string{
		"strata/sc117",   // which deployment
		"published port", // the thing that actually bit (#989)
		"devices", "env", // the rest of what lives in the entry
		"--keep-deploy",             // how to keep it instead
		"charly config",             // how to recreate it
		"whether or not it existed", // honest: a nil clean does not prove an entry was there
	} {
		if !strings.Contains(got, want) {
			t.Errorf("the note must mention %q; got: %s", want, got)
		}
	}
}

// TestDeployEntryDiscardNote_BaseBoxHasNoSlash — an uninstanced deployment must not render "box/".
func TestDeployEntryDiscardNote_BaseBoxHasNoSlash(t *testing.T) {
	got := deployEntryDiscardNote("strata", "")
	if strings.Contains(got, "strata/") {
		t.Errorf("an empty instance must not render a dangling slash: %s", got)
	}
	if !strings.Contains(got, "for strata ") {
		t.Errorf("the base box name must still appear: %s", got)
	}
}

// TestCleanDeployEntryAndReport_SuccessPrintsTheNote — the CALL SITE is the changed behaviour. If the
// reporting is removed from it, this fails: before the fix a successful clean printed nothing at all.
func TestCleanDeployEntryAndReport_SuccessPrintsTheNote(t *testing.T) {
	var out bytes.Buffer
	cleanDeployEntryAndReport(context.Background(), "strata", "sc117",
		func(context.Context, string, string) error { return nil }, &out)

	got := out.String()
	if !strings.Contains(got, "--keep-deploy") || !strings.Contains(got, "published port") {
		t.Fatalf("a successful clean must report what went with the entry; got: %q", got)
	}
	if strings.Contains(got, "Warning") {
		t.Errorf("a success must not read as a warning; got: %q", got)
	}
}

// TestCleanDeployEntryAndReport_FailureNeverClaimsRemoval — R1: the note says the entry is gone, so it
// must NOT be printed when the clean failed. Without this split the operator would be told a removal
// happened that did not.
func TestCleanDeployEntryAndReport_FailureNeverClaimsRemoval(t *testing.T) {
	var out bytes.Buffer
	cleanDeployEntryAndReport(context.Background(), "strata", "sc117",
		func(context.Context, string, string) error { return errors.New("loader unavailable") }, &out)

	got := out.String()
	if !strings.Contains(got, "Warning: cleaning deploy entry") {
		t.Errorf("a failed clean must report the error; got: %q", got)
	}
	if strings.Contains(got, "--keep-deploy") {
		t.Errorf("a failed clean must NOT print the note claiming the entry is gone; got: %q", got)
	}
}
