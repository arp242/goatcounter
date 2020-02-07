// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the EUPL
// v1.2, which can be found in the LICENSE file or at http://eupl12.zgo.at

package main

import (
	"fmt"
	"os"
	"strings"
	"unicode/utf8"
)

const usageHelp = `
Show help; use "help commands" to dispay detailed help for a command, or "help
all" to display everything.
`

func help() {
	if len(os.Args) == 2 {
		die(0, usage[""], "")
	}
	if os.Args[2] == "all" {
		fmt.Fprint(stdout, strings.TrimSpace(usage[""]), "\n\n")
		for _, h := range []string{"help", "version", "migrate", "serve", "create", "saas", "reindex"} {
			head := fmt.Sprintf("─── Help for %q ", h)
			fmt.Fprintf(stdout, "%s%s\n\n", head, strings.Repeat("─", 80-utf8.RuneCountInString(head)))
			fmt.Fprint(stdout, strings.TrimSpace(usage[h]), "\n\n")
		}
		os.Exit(0)
	}

	t, ok := usage[os.Args[2]]
	if !ok {
		die(1, usage["help"], "no help topic for %q", os.Args[2])
	}
	die(0, t, "")
}
