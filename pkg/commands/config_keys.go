package commands

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"
)

type configValueType string

const (
	configValueTypeString   configValueType = "string"
	configValueTypeBool     configValueType = "bool"
	configValueTypeDuration configValueType = "duration"
)

type configKeySpec struct {
	Key         string
	ValueType   configValueType
	Description string
	Sensitive   bool
	Allowed     []string
	Normalize   func(string) (string, error)
}

var configKeySpecs = []configKeySpec{
	{Key: "cache.path", ValueType: configValueTypeString, Description: "Override cache database path"},
	{Key: "git.remote", ValueType: configValueTypeString, Description: "Default Git remote name"},
	{Key: "git.branchTemplate", ValueType: configValueTypeString, Description: "Branch naming template"},
	{Key: "github.cache.labelsTTL", ValueType: configValueTypeDuration, Description: "TTL for GitHub labels cache"},
	{
		Key:         "github.cache.collaboratorsTTL",
		ValueType:   configValueTypeDuration,
		Description: "TTL for GitHub collaborators cache",
	},
	{
		Key:         "github.cache.orgMembersTTL",
		ValueType:   configValueTypeDuration,
		Description: "TTL for GitHub organization members cache",
	},
	{
		Key:         "github.cache.teamsTTL",
		ValueType:   configValueTypeDuration,
		Description: "TTL for GitHub teams cache",
	},
	{
		Key:         "github.cache.userProfilesTTL",
		ValueType:   configValueTypeDuration,
		Description: "TTL for GitHub user profile cache",
	},
	{Key: "github.releaseTemplate", ValueType: configValueTypeString, Description: "Release body template"},
	{Key: "github.token", ValueType: configValueTypeString, Description: "GitHub token (OAuth or PAT)", Sensitive: true},
	{
		Key:         "jira.cache.issueTypesTTL",
		ValueType:   configValueTypeDuration,
		Description: "TTL for Jira issue types cache",
	},
	{
		Key:         "jira.root",
		ValueType:   configValueTypeString,
		Description: "Jira site root URL",
		Normalize:   normalizeJiraRootConfigValue,
	},
	{Key: "jira.user.login", ValueType: configValueTypeString, Description: "Jira username/email"},
	{Key: "jira.user.token", ValueType: configValueTypeString, Description: "Jira API token", Sensitive: true},
	{Key: "locale", ValueType: configValueTypeString, Description: "CLI locale", Allowed: []string{"en"}},
	{Key: "telemetry.enabled", ValueType: configValueTypeBool, Description: "Enable anonymous product telemetry"},
}

var configKeySpecByName = func() map[string]configKeySpec {
	index := make(map[string]configKeySpec, len(configKeySpecs))
	for _, spec := range configKeySpecs {
		index[spec.Key] = spec
	}
	return index
}()

func allConfigKeySpecs() []configKeySpec {
	specs := slices.Clone(configKeySpecs)
	slices.SortFunc(specs, func(a, b configKeySpec) int {
		return strings.Compare(a.Key, b.Key)
	})
	return specs
}

func visibleConfigKeySpecs() []configKeySpec { return allConfigKeySpecs() }

func configKeyNames() []string {
	specs := visibleConfigKeySpecs()
	keys := make([]string, 0, len(specs))
	for _, spec := range specs {
		keys = append(keys, spec.Key)
	}
	return keys
}

func lookupConfigKeySpec(key string) (configKeySpec, bool) {
	spec, ok := configKeySpecByName[strings.TrimSpace(key)]
	return spec, ok
}

func parseConfigKeyValue(spec configKeySpec, input string) (any, error) {
	trimmed := strings.TrimSpace(input)
	if spec.Normalize != nil {
		normalized, err := spec.Normalize(trimmed)
		if err != nil {
			return nil, err
		}
		trimmed = normalized
	}

	if len(spec.Allowed) > 0 {
		for _, allowed := range spec.Allowed {
			if strings.EqualFold(trimmed, allowed) {
				return allowed, nil
			}
		}
		return nil, fmt.Errorf("value %q must be one of %s", trimmed, strings.Join(spec.Allowed, ", "))
	}

	switch spec.ValueType {
	case configValueTypeString:
		return trimmed, nil
	case configValueTypeBool:
		value, err := strconv.ParseBool(trimmed)
		if err != nil {
			return nil, fmt.Errorf("value %q is not a valid boolean", trimmed)
		}
		return value, nil
	case configValueTypeDuration:
		value, err := time.ParseDuration(trimmed)
		if err != nil {
			return nil, fmt.Errorf("value %q is not a valid duration", trimmed)
		}
		return value.String(), nil
	default:
		return nil, fmt.Errorf("unsupported value type %s", spec.ValueType)
	}
}

func configValueAsString(spec configKeySpec, value any) string {
	if spec.Sensitive {
		if isConfigValueEmpty(value) {
			return ""
		}

		return "<redacted>"
	}

	if spec.ValueType == configValueTypeString {
		switch typed := value.(type) {
		case nil:
			return ""
		case string:
			return typed
		default:
			return ""
		}
	}

	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		return typed
	case bool:
		return strconv.FormatBool(typed)
	default:
		return fmt.Sprintf("%v", typed)
	}
}

func isConfigValueEmpty(value any) bool {
	switch typed := value.(type) {
	case nil:
		return true
	case string:
		return strings.TrimSpace(typed) == ""
	default:
		return strings.TrimSpace(fmt.Sprintf("%v", typed)) == ""
	}
}
