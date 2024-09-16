package commands

import (
	"os"
	"strings"
	"sync"

	"github.com/spf13/cobra"
)

var completionArgsFn = func() []string {
	return os.Args
}

var (
	invocationCompletionMu sync.RWMutex
	invocationCompletion   bool
)

func setInvocationShellCompletion(value bool) {
	invocationCompletionMu.Lock()
	defer invocationCompletionMu.Unlock()
	invocationCompletion = value
}

func isInvocationShellCompletion() bool {
	invocationCompletionMu.RLock()
	defer invocationCompletionMu.RUnlock()
	return invocationCompletion
}

func isCompletionCommandName(name string) bool {
	switch strings.TrimSpace(name) {
	case "completion", "__complete", "__completeNoDesc":
		return true
	default:
		return false
	}
}

func IsShellCompletionRequest() bool {
	args := completionArgsFn()
	if len(args) == 0 {
		return false
	}

	for _, arg := range args[1:] {
		if isCompletionCommandName(arg) {
			return true
		}
	}

	return false
}

func isShellCompletionCommand(cmd *cobra.Command) bool {
	for current := cmd; current != nil; current = current.Parent() {
		if isCompletionCommandName(current.Name()) || isCompletionCommandName(current.CalledAs()) {
			return true
		}
	}

	return IsShellCompletionRequest()
}
