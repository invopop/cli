// Package skills provides embedded skill files for AI agent integration.
package skills

import _ "embed"

// SkillContent contains the embedded AI skill definition.
//
//go:embed SKILL.md
var SkillContent []byte
