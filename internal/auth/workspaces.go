package auth

import (
	"errors"
	"fmt"
	"strings"
)

// SandboxPrefix marks a slug as naming a sandbox workspace. A live workspace
// and its sandbox copy can share a slug, so a bare slug only ever names a
// live one.
const SandboxPrefix = "sandbox/"

// ErrUnknownWorkspace is returned when nothing matches a workspace reference.
var ErrUnknownWorkspace = errors.New("unknown workspace")

// Ref is how the workspace is named on the command line: its slug, with
// SandboxPrefix for a sandbox workspace, or its ID when the slug is not
// known, as for a default stored by an earlier version.
func (w *Workspace) Ref() string {
	if w.Slug == "" {
		return w.ID
	}
	if w.Sandbox {
		return SandboxPrefix + w.Slug
	}
	return w.Slug
}

// FindWorkspace picks the workspace ref names from list. A ref is an ID, the
// slug of a live workspace, or SandboxPrefix followed by the slug of a
// sandbox workspace. Slugs match without regard to case.
func FindWorkspace(list []*Workspace, ref string) (*Workspace, error) {
	ref = strings.TrimSpace(ref)
	for _, w := range list {
		if w.ID == ref {
			return w, nil
		}
	}
	slug, sandbox := ref, false
	if len(ref) > len(SandboxPrefix) && strings.EqualFold(ref[:len(SandboxPrefix)], SandboxPrefix) {
		slug, sandbox = ref[len(SandboxPrefix):], true
	}
	var found, other []*Workspace
	for _, w := range list {
		if !strings.EqualFold(w.Slug, slug) {
			continue
		}
		if w.Sandbox == sandbox {
			found = append(found, w)
		} else {
			other = append(other, w)
		}
	}
	switch len(found) {
	case 1:
		return found[0], nil
	case 0:
		if len(other) > 0 && !sandbox {
			return nil, fmt.Errorf("%w %q: no live workspace uses that slug, did you mean %q?", ErrUnknownWorkspace, ref, other[0].Ref())
		}
		return nil, fmt.Errorf("%w %q among those you belong to: list them with `invopop access workspaces`", ErrUnknownWorkspace, ref)
	}
	// Nothing stops two orgs using the same slug.
	msg := fmt.Sprintf("several workspaces match %q, pick one by ID:", ref)
	for _, w := range found {
		msg += fmt.Sprintf("\n  %s  %s", w.ID, w.Name)
	}
	return nil, errors.New(msg)
}

// knownList gives the workspaces c remembers in the form FindWorkspace takes.
func (c *Credentials) knownList() []*Workspace {
	list := make([]*Workspace, 0, len(c.Workspaces))
	for id, k := range c.Workspaces {
		list = append(list, &Workspace{ID: id, Name: k.Name, Slug: k.Slug, Sandbox: k.Sandbox})
	}
	return list
}

// WorkspaceListing is one workspace in a flat listing of those the user
// belongs to, as `invopop access workspaces` prints them.
type WorkspaceListing struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Slug    string `json:"slug"`
	Ref     string `json:"ref"`
	Sandbox bool   `json:"sandbox"`
	OrgID   string `json:"org_id"`
	OrgName string `json:"org_name"`
	Default bool   `json:"default"`
	InUse   bool   `json:"in_use"`
}

// ListWorkspaces flattens orgs into one row per workspace, marking the
// default and the one in use.
func ListWorkspaces(orgs []*Org, defaultID, inUseID string) []*WorkspaceListing {
	list := make([]*WorkspaceListing, 0)
	for _, o := range orgs {
		for _, w := range o.Workspaces {
			list = append(list, &WorkspaceListing{
				ID:      w.ID,
				Name:    w.Name,
				Slug:    w.Slug,
				Ref:     w.Ref(),
				Sandbox: w.Sandbox,
				OrgID:   o.ID,
				OrgName: o.Name,
				Default: w.ID == defaultID,
				InUse:   w.ID == inUseID,
			})
		}
	}
	return list
}
