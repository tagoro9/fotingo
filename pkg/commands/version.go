package commands

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/tagoro9/fotingo/internal/commandruntime"
	"github.com/tagoro9/fotingo/internal/i18n"
)

func init() {
	Fotingo.AddCommand(versionCmd)
}

var versionCmd = &cobra.Command{
	Use:   i18n.T(i18n.VersionUse),
	Short: i18n.T(i18n.VersionShort),
	Long:  i18n.T(i18n.VersionLong),
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(localizer.T(i18n.VersionOutput, commandruntime.GetBuildInfo().Version))
	},
}
