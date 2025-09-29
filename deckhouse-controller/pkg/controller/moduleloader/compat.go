package moduleloader

import "context"

// restoreAbsentModulesFromOverrides is kept for test compatibility; it delegates to the actual implementation.
func (l *Loader) restoreAbsentModulesFromOverrides(ctx context.Context) error {
	return l.restoreModulesByOverrides(ctx)
}

// restoreAbsentModulesFromReleases is kept for test compatibility; it delegates to the actual implementation.
func (l *Loader) restoreAbsentModulesFromReleases(ctx context.Context) error {
	return l.restoreModulesByReleases(ctx)
}

