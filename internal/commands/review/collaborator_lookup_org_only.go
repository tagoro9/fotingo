//go:build fotingo_org_only_participants

package review

// collaboratorFallbackEnabled reports whether repository collaborator lookups
// may supplement organization-scoped participant matching in this build.
func collaboratorFallbackEnabled() bool {
	return false
}
