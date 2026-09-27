// plugin-pod's OWN self-contained CUE schema — the SINGLE SOURCE for this
// plugin's declaration surface. There is NO schema-less plugin: every plugin
// ships a non-empty, self-contained schema, served over Describe (the SDK splices
// `base ++ plugin` at the load gate), and this one DOCUMENTS the plugin's command
// surface.
//
// SELF-CONTAINED: it references NO base def, so it compiles STANDALONE — the exact
// property `cue exp gengotypes` needs to generate Go params, AND the property that
// lets the SDK compile it serve-side.
#PodPlugin: {
	// The independent top-level command words this plugin serves (no shared parent
	// — each command word is its own top-level charly command). A command's args
	// are pass-through CLI tokens (there is no typed plugin_input), so these words
	// ARE this plugin's authored declaration surface.
	commands: ["start", "stop", "restart", "logs", "remove", "shell", "service", "volume", "cp", "config", "update"]

	// What the plugin does, in one line (the public-docs surface).
	contract: string & !=""
}
