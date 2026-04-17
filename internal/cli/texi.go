package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

func newGenerateTexiCmd() *cobra.Command {
	var output string
	cmd := &cobra.Command{
		Use:    "generate-texi",
		Short:  "Generate Texinfo source for info pages",
		Hidden: true,
		Args:   cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			root := cmd.Root()
			texi := generateTexinfo(root)
			if output != "" {
				return os.WriteFile(output, []byte(texi), 0o644)
			}
			fmt.Fprint(cmd.OutOrStdout(), texi)
			return nil
		},
	}
	cmd.Flags().StringVar(&output, "output", "", "Output file (default: stdout)")
	return silenceSubcommand(cmd)
}

func generateTexinfo(root *cobra.Command) string {
	var b strings.Builder
	b.WriteString(`\input texinfo
@setfilename mw.info
@settitle Makework Manual
@dircategory Development
@direntry
* Makework: (mw). Git worktree manager.
@end direntry

@node Top
@top Makework

Makework is an opinionated git worktree manager.

@menu
`)
	for _, sub := range root.Commands() {
		if sub.Hidden || sub.Name() == "help" {
			continue
		}
		fmt.Fprintf(&b, "* %s::\t%s\n", sub.Name(), sub.Short)
	}
	b.WriteString("@end menu\n\n")

	for _, sub := range root.Commands() {
		if sub.Hidden || sub.Name() == "help" {
			continue
		}
		writeTexiCommand(&b, sub)
	}

	b.WriteString("@bye\n")
	return b.String()
}

func writeTexiCommand(b *strings.Builder, cmd *cobra.Command) {
	name := cmd.Name()
	fmt.Fprintf(b, "@node %s\n", name)
	fmt.Fprintf(b, "@section %s\n\n", name)
	fmt.Fprintf(b, "@code{mw %s}\n\n", cmd.UseLine())
	if cmd.Long != "" {
		b.WriteString(texiEscape(cmd.Long))
	} else {
		b.WriteString(texiEscape(cmd.Short))
	}
	b.WriteString("\n\n")

	if cmd.HasSubCommands() {
		for _, sub := range cmd.Commands() {
			if sub.Hidden {
				continue
			}
			fmt.Fprintf(b, "@subsection %s %s\n\n", name, sub.Name())
			fmt.Fprintf(b, "@code{mw %s %s}\n\n", name, sub.UseLine())
			desc := sub.Short
			if sub.Long != "" {
				desc = sub.Long
			}
			b.WriteString(texiEscape(desc))
			b.WriteString("\n\n")
		}
	}
}

func texiEscape(s string) string {
	s = strings.ReplaceAll(s, "@", "@@")
	s = strings.ReplaceAll(s, "{", "@{")
	s = strings.ReplaceAll(s, "}", "@}")
	return s
}
