package pod

import (
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
		"strata/sc117",              // which deployment
		"published port",            // the thing that actually bit (#989)
		"devices", "env",            // the rest of what lives in the entry
		"--keep-deploy",             // how to keep it instead
		"charly config",             // how to recreate it
	} {
		if !strings.Contains(got, want) {
			t.Errorf("the note must mention %q; got: %s", want, got)
		}
	}
}

// TestDeployEntryDiscardNote_BaseBoxHasNoSlash — an uninstanced deployment must not render "box/".
func TestDeployEntryDiscardNote_BaseBoxHasNoSlash(t *testing.T) {
	if got := deployEntryDiscardNote("strata", ""); strings.Contains(got, "strata/ ") || strings.Contains(got, "strata/:") {
		t.Errorf("an empty instance must not render a dangling slash: %s", got)
	} else if !strings.Contains(got, "strata:") {
		t.Errorf("the base box name must still appear: %s", got)
	}
}
