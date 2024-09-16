package commands

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tagoro9/fotingo/internal/commandruntime"
	internalai "github.com/tagoro9/fotingo/internal/commands/ai"
	fterrors "github.com/tagoro9/fotingo/internal/errors"
	"github.com/tagoro9/fotingo/internal/ui"
)

const aiSetupOperationID = "ai-setup"

type aiSetupFlags struct {
	agents []string
	all    bool
	scope  string
	dryRun bool
	force  bool
}

type aiSetupResult struct {
	Scope     internalai.Scope
	Providers []internalai.Provider
	Results   []internalai.InstallResult
}

var aiSetupCmdFlags = aiSetupFlags{}

var aiSetupSelectProvidersFn = func(title string, items []ui.MultiSelectItem, minimum int) ([]string, error) {
	return ui.SelectIDs(title, items, minimum)
}

var aiSetupGetwdFn = os.Getwd
var aiSetupUserHomeDirFn = os.UserHomeDir
var aiSetupGetenvFn = os.Getenv

func init() {
	Fotingo.AddCommand(aiCmd)
	aiCmd.AddCommand(aiSetupCmd)

	aiSetupCmd.Flags().StringSliceVar(
		&aiSetupCmdFlags.agents,
		"agent",
		nil,
		"Agent provider to configure (repeatable: cursor, codex, claude-code)",
	)
	aiSetupCmd.Flags().BoolVar(
		&aiSetupCmdFlags.all,
		"all",
		false,
		"Configure all supported providers",
	)
	aiSetupCmd.Flags().StringVar(
		&aiSetupCmdFlags.scope,
		"scope",
		string(internalai.ScopeProject),
		"Install scope (project or user)",
	)
	aiSetupCmd.Flags().BoolVar(
		&aiSetupCmdFlags.dryRun,
		"dry-run",
		false,
		"Preview target paths without writing files",
	)
	aiSetupCmd.Flags().BoolVar(
		&aiSetupCmdFlags.force,
		"force",
		false,
		"Overwrite existing generated skill files",
	)

	_ = aiSetupCmd.RegisterFlagCompletionFunc("agent", completeAISetupAgentFlag)
	_ = aiSetupCmd.RegisterFlagCompletionFunc("scope", completeAISetupScopeFlag)
}

var aiCmd = &cobra.Command{
	Use:   "ai",
	Short: "AI-related setup and utilities",
	Long:  "Commands for configuring AI tooling integrations with fotingo.",
}

var aiSetupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Install fotingo agent skills for supported AI providers",
	Long:  aiSetupLongDescription(),
	RunE: func(cmd *cobra.Command, args []string) error {
		if ShouldOutputJSON() {
			result, err := runAISetup()
			return outputAISetupJSON(result, err)
		}

		return runWithSharedShell(func(out commandruntime.LocalizedEmitter) error {
			result, err := runAISetup()
			if err != nil {
				return err
			}

			out.InfoRaw(
				commandruntime.LogEmojiConfigure,
				fmt.Sprintf(
					"AI setup scope=%s providers=%s",
					result.Scope,
					joinProviders(result.Providers),
				),
			)
			for _, item := range result.Results {
				line := fmt.Sprintf(
					"%s %s -> %s",
					item.Status,
					item.Target.Provider,
					item.Target.SkillPath,
				)
				if item.Reason != "" {
					line = fmt.Sprintf("%s (%s)", line, item.Reason)
				}
				out.InfoRaw(commandruntime.LogEmojiConfigure, line)
			}

			planned, created, updated, skipped := internalai.CountInstallResults(result.Results)
			out.SuccessRaw(
				aiSetupOperationID,
				commandruntime.LogEmojiSuccess,
				fmt.Sprintf(
					"AI setup complete (created=%d updated=%d skipped=%d planned=%d)",
					created,
					updated,
					skipped,
					planned,
				),
			)
			return nil
		})
	},
}

func aiSetupLongDescription() string {
	examples := internalai.DefaultCommandExamples()
	return fmt.Sprintf(
		"Install fotingo agent skills for cursor, codex, and claude-code.\n\n"+
			"Examples:\n"+
			"  # Select providers interactively in TTY mode\n"+
			"  fotingo ai setup\n\n"+
			"  # Install for selected providers at project scope\n"+
			"  fotingo ai setup --agent codex --agent cursor --scope project\n\n"+
			"  # Install all providers at user scope\n"+
			"  fotingo ai setup --all --scope user\n\n"+
			"  # Preview without writing\n"+
			"  fotingo ai setup --all --dry-run\n\n"+
			"Generated skills include shared workflow snippets:\n"+
			"  %s\n"+
			"  %s\n"+
			"  %s\n",
		examples.InspectJSON,
		examples.StartExistingIssue,
		examples.ReviewTemplateOverrides,
	)
}

func runAISetup() (aiSetupResult, error) {
	scope, err := internalai.ParseScope(aiSetupCmdFlags.scope)
	if err != nil {
		return aiSetupResult{}, fterrors.ConfigError(err.Error())
	}

	providers, err := resolveAISetupProviders()
	if err != nil {
		return aiSetupResult{}, err
	}

	cwd, err := aiSetupGetwdFn()
	if err != nil {
		return aiSetupResult{}, fterrors.ConfigErrorf("failed to resolve current directory: %v", err)
	}
	projectRoot := internalai.FindProjectRoot(cwd)

	userHome, err := aiSetupUserHomeDirFn()
	if err != nil && scope == internalai.ScopeUser {
		return aiSetupResult{}, fterrors.ConfigErrorf("failed to resolve user home: %v", err)
	}

	codexHome := strings.TrimSpace(aiSetupGetenvFn("CODEX_HOME"))
	targets, err := internalai.PlanInstallTargets(
		providers,
		scope,
		projectRoot,
		userHome,
		codexHome,
	)
	if err != nil {
		return aiSetupResult{}, fterrors.ConfigError(err.Error())
	}

	results, err := internalai.ApplyInstallPlan(
		targets,
		internalai.InstallOptions{
			DryRun: aiSetupCmdFlags.dryRun,
			Force:  aiSetupCmdFlags.force,
		},
		internalai.RenderSkill,
	)
	if err != nil {
		return aiSetupResult{}, fterrors.ConfigError(err.Error())
	}

	return aiSetupResult{
		Scope:     scope,
		Providers: providers,
		Results:   results,
	}, nil
}

func resolveAISetupProviders() ([]internalai.Provider, error) {
	providers, err := internalai.ParseProviders(aiSetupCmdFlags.agents, aiSetupCmdFlags.all)
	if err != nil {
		return nil, fterrors.ConfigError(err.Error())
	}

	if len(providers) > 0 {
		return providers, nil
	}

	if ShouldOutputJSON() || !isInteractiveTerminalFn() {
		return nil, fterrors.ConfigError(
			"no providers selected; pass --agent <provider> or --all",
		)
	}

	items := make([]ui.MultiSelectItem, 0, len(internalai.SupportedProviders()))
	for _, provider := range internalai.SupportedProviders() {
		items = append(items, ui.MultiSelectItem{
			ID:     string(provider),
			Label:  internalai.ProviderLabel(provider),
			Detail: fmt.Sprintf("Install fotingo skill for %s", provider),
			Icon:   string(commandruntime.LogEmojiConfigure),
		})
	}

	selection, err := aiSetupSelectProvidersFn("Select AI providers", items, 1)
	if err != nil {
		return nil, err
	}

	providers, err = internalai.ParseProviders(selection, false)
	if err != nil {
		return nil, fterrors.ConfigError(err.Error())
	}
	if len(providers) == 0 {
		return nil, fterrors.ConfigError("no providers selected")
	}
	return providers, nil
}

func completeAISetupAgentFlag(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	values := internalai.SupportedProviderValues()
	filtered := make([]string, 0, len(values))
	for _, value := range values {
		if strings.HasPrefix(value, toComplete) {
			filtered = append(filtered, value)
		}
	}
	return filtered, cobra.ShellCompDirectiveNoFileComp
}

func completeAISetupScopeFlag(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	values := internalai.ScopeValues()
	filtered := make([]string, 0, len(values))
	for _, value := range values {
		if strings.HasPrefix(value, toComplete) {
			filtered = append(filtered, value)
		}
	}
	return filtered, cobra.ShellCompDirectiveNoFileComp
}

func outputAISetupJSON(result aiSetupResult, err error) error {
	output := AISetupOutput{
		Success: err == nil,
	}
	if err != nil {
		output.Error = err.Error()
		OutputJSON(output)
		return err
	}

	output.Scope = string(result.Scope)
	output.DryRun = aiSetupCmdFlags.dryRun
	output.Force = aiSetupCmdFlags.force

	output.Providers = make([]string, 0, len(result.Providers))
	for _, provider := range result.Providers {
		output.Providers = append(output.Providers, string(provider))
	}

	output.Results = make([]AISetupResult, 0, len(result.Results))
	for _, item := range result.Results {
		output.Results = append(output.Results, AISetupResult{
			Provider: string(item.Target.Provider),
			Scope:    string(item.Target.Scope),
			Root:     item.Target.ProviderRoot,
			Path:     item.Target.SkillPath,
			Status:   string(item.Status),
			Reason:   item.Reason,
		})
	}

	OutputJSON(output)
	return nil
}

func joinProviders(providers []internalai.Provider) string {
	names := make([]string, 0, len(providers))
	for _, provider := range providers {
		names = append(names, string(provider))
	}
	return strings.Join(names, ", ")
}
