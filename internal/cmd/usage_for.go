package cmd

import "strings"

// usageFor narrows a group's usage text to one verb, so `<group> <verb>
// --help` answers with that verb's own flags instead of the whole group — an
// agent that asks about one command should not pay for a hundred lines about
// the others. An entry is a line indented exactly two spaces that starts with
// prefix and then the verb; the deeper-indented lines after it belong to it.
// A verb with no entry gets the whole text.
func usageFor(text, prefix, verb string) string {
	var out []string
	keep := false
	for _, line := range strings.Split(text, "\n") {
		body := strings.TrimLeft(line, " ")
		indent := len(line) - len(body)
		switch {
		case body == "":
			keep = false
		case indent == 2:
			keep = false
			if rest, ok := strings.CutPrefix(body, prefix); ok {
				name := rest
				if i := strings.IndexAny(rest, " |"); i >= 0 {
					name = rest[:i]
				}
				keep = name == verb
			}
		case indent < 2:
			keep = false
		}
		if keep {
			out = append(out, line)
		}
	}
	if len(out) == 0 {
		return text
	}
	return "Usage:\n" + strings.Join(out, "\n") + "\n"
}

// wantsHelp reports whether any argument asks for help.
func wantsHelp(args []string) bool {
	for _, arg := range args {
		if arg == "-h" || arg == "--help" {
			return true
		}
	}
	return false
}
