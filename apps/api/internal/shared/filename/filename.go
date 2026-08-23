// Package filename holds pure file-name utilities shared across modules —
// moved out of projects/application/evidence_service.go's package-private
// sanitizeFileName so platform/application.TenantService.UploadLogo can use
// the exact same sanitization without projects and platform importing each
// other's application layers (.claude/rules/backend-modular-monolith.md).
package filename

import "regexp"

var unsafeChars = regexp.MustCompile(`[^a-zA-Z0-9._-]+`)

// Sanitize replaces every character outside [a-zA-Z0-9._-] with "-", falling
// back to "file" if that leaves nothing.
func Sanitize(name string) string {
	cleaned := unsafeChars.ReplaceAllString(name, "-")
	if cleaned == "" {
		return "file"
	}
	return cleaned
}
