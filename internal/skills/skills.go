// Package skills provides embedded skill files for AI agent integration.
package skills

import _ "embed"

//go:embed SKILL.md
var SkillContent []byte
