// Package skills lists and installs the skills Heracles ships per ADR 0019,
// following the directory conventions skills.sh-compatible providers use so
// the same bundled skills work whether installed by Heracles or by hand.
package skills

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	bundled "github.com/davidtobonm/heracles/skills"
)

// Names lists the skills Heracles ships, in installation order.
var Names = []string{"grill-with-docs", "to-prd-for-heracles", "to-issues-for-heracles"}

// Skill is one shipped skill's files, keyed by path relative to the skill's
// own directory (e.g. "SKILL.md").
type Skill struct {
	Name        string
	Description string
	Files       map[string][]byte
}

// Bundled returns the skills Heracles ships, in Names order.
func Bundled() ([]Skill, error) {
	result := make([]Skill, 0, len(Names))
	for _, name := range Names {
		files := map[string][]byte{}
		err := fs.WalkDir(bundled.FS, name, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() {
				return nil
			}
			contents, err := bundled.FS.ReadFile(path)
			if err != nil {
				return err
			}
			files[strings.TrimPrefix(path, name+"/")] = contents
			return nil
		})
		if err != nil {
			return nil, err
		}
		description, err := frontmatterDescription(files["SKILL.md"])
		if err != nil {
			return nil, fmt.Errorf("skill %s: %w", name, err)
		}
		result = append(result, Skill{Name: name, Description: description, Files: files})
	}
	return result, nil
}

// frontmatterDescription extracts the "description:" line from a SKILL.md's
// YAML frontmatter.
func frontmatterDescription(skillMD []byte) (string, error) {
	for _, line := range strings.Split(string(skillMD), "\n") {
		if rest, ok := strings.CutPrefix(line, "description:"); ok {
			return strings.TrimSpace(rest), nil
		}
	}
	return "", fmt.Errorf("SKILL.md missing description frontmatter")
}

// ProjectDir returns the project-scoped skill directory for provider under
// root, following the .{provider}/skills convention shared with skills.sh.
func ProjectDir(provider, root string) string {
	return filepath.Join(root, "."+provider, "skills")
}

// GlobalDir returns the user-global skill directory for provider under home,
// following the .{provider}/skills convention shared with skills.sh.
func GlobalDir(provider, home string) string {
	return filepath.Join(home, "."+provider, "skills")
}

// InstallResult reports one skill's outcome from Install.
type InstallResult struct {
	Skill     string
	Installed bool
	Skipped   bool
}

// Install writes each skill's files under dir/<skill-name>/, the
// skills.sh-compatible layout. If dir/<skill-name>/SKILL.md already exists
// and force is false, the skill is left untouched and reported as Skipped,
// so locally modified skills are never silently overwritten.
func Install(dir string, skills []Skill, force bool) ([]InstallResult, error) {
	results := make([]InstallResult, 0, len(skills))
	for _, skill := range skills {
		target := filepath.Join(dir, skill.Name)
		skillPath := filepath.Join(target, "SKILL.md")
		if !force {
			if _, err := os.Stat(skillPath); err == nil {
				results = append(results, InstallResult{Skill: skill.Name, Skipped: true})
				continue
			}
		}
		for relativePath, contents := range skill.Files {
			full := filepath.Join(target, relativePath)
			if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
				return nil, err
			}
			if err := os.WriteFile(full, contents, 0o644); err != nil {
				return nil, err
			}
		}
		results = append(results, InstallResult{Skill: skill.Name, Installed: true})
	}
	return results, nil
}
