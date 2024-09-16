package commands

import (
	"os"

	"github.com/spf13/cobra"
	"github.com/tagoro9/fotingo/internal/i18n"
)

func init() {
	Fotingo.AddCommand(completionCmd)
}

var completionCmd = &cobra.Command{
	Use:                   i18n.T(i18n.CompletionUse),
	Short:                 i18n.T(i18n.CompletionShort),
	Long:                  i18n.T(i18n.CompletionLong),
	DisableFlagsInUseLine: true,
	ValidArgs:             []string{"bash", "zsh", "fish", "powershell"},
	Args:                  cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
	RunE: func(cmd *cobra.Command, args []string) error {
		switch args[0] {
		case "bash":
			return cmd.Root().GenBashCompletion(os.Stdout)
		case "zsh":
			return cmd.Root().GenZshCompletion(os.Stdout)
		case "fish":
			return cmd.Root().GenFishCompletion(os.Stdout, true)
		case "powershell":
			return cmd.Root().GenPowerShellCompletionWithDesc(os.Stdout)
		}
		return nil
	},
}
