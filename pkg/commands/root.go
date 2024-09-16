package commands

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/tagoro9/fotingo/internal/commandruntime"
	"github.com/tagoro9/fotingo/internal/config"
	fterrors "github.com/tagoro9/fotingo/internal/errors"
	"github.com/tagoro9/fotingo/internal/i18n"
	"github.com/tagoro9/fotingo/internal/telemetry"
)

// GlobalFlags holds the global flags available to all commands
type GlobalFlags struct {
	Branch  string
	Yes     bool
	JSON    bool
	Quiet   bool
	Verbose bool
	Debug   bool
	NoColor bool
}

// Global holds the current global flags
var Global = GlobalFlags{}
var localizer = i18n.New(i18n.DefaultLocale)

var Fotingo = &cobra.Command{
	Use:           "fotingo",
	Short:         i18n.T(i18n.RootShort),
	Long:          i18n.T(i18n.RootLong),
	SilenceUsage:  true,
	SilenceErrors: true,
}

var fotingoConfig = config.NewConfig()

func init() {
	localizer = i18n.NewFromConfig(fotingoConfig)
	i18n.SetGlobalLocale(localizer.Locale())

	Fotingo.Use = localizer.T(i18n.RootUse)
	Fotingo.Short = localizer.T(i18n.RootShort)
	Fotingo.Long = localizer.T(i18n.RootLong)

	// Global flags available to all commands
	Fotingo.PersistentFlags().StringVarP(&Global.Branch, "branch", "b", "", localizer.T(i18n.RootFlagBranch))
	Fotingo.PersistentFlags().BoolVarP(&Global.Yes, "yes", "y", false, localizer.T(i18n.RootFlagYes))
	Fotingo.PersistentFlags().BoolVar(&Global.JSON, "json", false, localizer.T(i18n.RootFlagJSON))
	Fotingo.PersistentFlags().BoolVar(&Global.Quiet, "quiet", false, localizer.T(i18n.RootFlagQuiet))
	Fotingo.PersistentFlags().BoolVarP(&Global.Verbose, "verbose", "v", false, localizer.T(i18n.RootFlagVerbose))
	Fotingo.PersistentFlags().BoolVar(&Global.Debug, "debug", false, localizer.T(i18n.RootFlagDebug))
	Fotingo.PersistentFlags().BoolVar(&Global.NoColor, "no-color", false, localizer.T(i18n.RootFlagNoColor))
	Fotingo.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		setInvocationShellCompletion(isShellCompletionCommand(cmd))
		startTelemetryForCommand(cmd)
		return prepareStartupAnnouncements(cmd)
	}
}

// Execute runs the root command and handles exit codes.
// This is the main entry point for the CLI and should be called from main().
func Execute() {
	exitCode := ExecuteWithExitCode()
	if exitCode != fterrors.ExitSuccess {
		os.Exit(exitCode)
	}
}

// ExecuteWithExitCode runs the root command and returns the exit code.
// This is useful for testing where os.Exit should not be called.
func ExecuteWithExitCode() (exitCode int) {
	start := time.Now()
	setInvocationShellCompletion(false)
	defer setInvocationShellCompletion(false)
	configureTelemetryRuntime()
	defer telemetry.Shutdown()
	resetCommandStateForExecute(Fotingo)
	defer resetCommandStateForExecute(Fotingo)

	defer func() {
		if recovered := recover(); recovered != nil {
			finishTelemetryForCrash(currentTelemetryCommand(), start, recovered)
			crashErr := fmt.Errorf("internal crash: %v", recovered)
			if Global.JSON {
				OutputJSONError(crashErr)
			} else {
				fmt.Fprintln(os.Stderr, crashErr)
			}
			exitCode = fterrors.ExitGeneral
			if commandruntime.ShouldPrintDoneFooter(
				ShouldSuppressOutput(),
				isInvocationShellCompletion(),
				IsShellCompletionRequest(),
			) {
				commandruntime.PrintDoneFooter(os.Stdout, start)
			}
		}
	}()

	if err := Fotingo.Execute(); err != nil {
		exitCode = fterrors.GetExitCode(err)
		finishTelemetryForError(currentTelemetryCommand(), start, err, exitCode)

		if Global.JSON {
			OutputJSONError(err)
		} else {
			fmt.Fprintln(os.Stderr, err)
		}

		if commandruntime.ShouldPrintDoneFooter(
			ShouldSuppressOutput(),
			isInvocationShellCompletion(),
			IsShellCompletionRequest(),
		) {
			commandruntime.PrintDoneFooter(os.Stdout, start)
		}
		return exitCode
	}

	finishTelemetryForSuccess(currentTelemetryCommand(), start)
	if commandruntime.ShouldPrintDoneFooter(
		ShouldSuppressOutput(),
		isInvocationShellCompletion(),
		IsShellCompletionRequest(),
	) {
		commandruntime.PrintDoneFooter(os.Stdout, start)
	}
	return fterrors.ExitSuccess
}

func resetCommandStateForExecute(cmd *cobra.Command) {
	if cmd == nil {
		return
	}
	resetFlagSetToDefaults(cmd.Flags())
	resetFlagSetToDefaults(cmd.PersistentFlags())
	for _, subCmd := range cmd.Commands() {
		resetCommandStateForExecute(subCmd)
	}
}

func resetFlagSetToDefaults(flagSet *pflag.FlagSet) {
	if flagSet == nil {
		return
	}
	flagSet.VisitAll(func(flag *pflag.Flag) {
		if sliceValue, ok := flag.Value.(pflag.SliceValue); ok {
			_ = sliceValue.Replace(parseSliceFlagDefault(flag.DefValue))
			flag.Changed = false
			return
		}

		if setErr := flag.Value.Set(flag.DefValue); setErr != nil {
			return
		}
		flag.Changed = false
	})
}

func parseSliceFlagDefault(defValue string) []string {
	trimmed := strings.TrimSpace(defValue)
	if trimmed == "" || trimmed == "[]" {
		return []string{}
	}

	trimmed = strings.TrimPrefix(trimmed, "[")
	trimmed = strings.TrimSuffix(trimmed, "]")
	if trimmed == "" {
		return []string{}
	}

	parts := strings.Split(trimmed, ",")
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		value := strings.TrimSpace(part)
		if value == "" {
			continue
		}
		values = append(values, value)
	}
	return values
}
