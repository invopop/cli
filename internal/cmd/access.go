package cmd

import (
	"errors"

	"github.com/invopop/cli/internal/auth"
	iohelper "github.com/invopop/cli/internal/io"
	"github.com/invopop/client.go/invopop"
	"github.com/spf13/cobra"
)

func accessCmd(getClient func() (*invopop.Client, error), getSession func() (*auth.Session, error), apiToken func() string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "access",
		Short: "See the workspaces you can reach",
	}
	cmd.AddCommand(accessWorkspaceCmd(getClient))
	cmd.AddCommand(accessWorkspacesCmd(getSession, apiToken))
	return cmd
}

func accessWorkspaceCmd(getClient func() (*invopop.Client, error)) *cobra.Command {
	return &cobra.Command{
		Use:   "workspace",
		Short: "Show details of the workspace in use",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			c, err := getClient()
			if err != nil {
				return err
			}
			ws, err := c.Access().Workspace().Fetch(cmd.Context())
			if err != nil {
				return err
			}
			return iohelper.PrintJSON(ws)
		},
	}
}

// workspaceListing is one workspace in `access workspaces`, flattened out of
// its org so it can be read or filtered at a glance.
type workspaceListing struct {
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

func accessWorkspacesCmd(getSession func() (*auth.Session, error), apiToken func() string) *cobra.Command {
	return &cobra.Command{
		Use:   "workspaces",
		Short: "List the workspaces you belong to",
		Long: `List the workspaces you belong to as JSON.

Each has a ref, the name to pass to --workspace or ` + "`invopop auth workspace`" + `,
and says whether it is the default and whether commands here would use it.
Needs a sign-in with ` + "`invopop auth login`" + `: an API token belongs to a single
workspace and cannot list others.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if apiToken() != "" {
				return errors.New("listing workspaces needs a sign-in, but an API token is set, which belongs to a single workspace: use `invopop access workspace` to see it")
			}
			s, err := getSession()
			if err != nil {
				return err
			}
			c, err := s.Load()
			if err != nil {
				return err
			}
			orgs, err := s.Orgs(cmd.Context(), c)
			if err != nil {
				return err
			}
			inUse := c.WorkspaceID
			if s.Workspace != "" {
				inUse = ""
				ws, err := s.ResolveWorkspace(cmd.Context(), c, s.Workspace)
				if err != nil {
					cmd.PrintErrf("Warning: %s names no workspace below: %v\n", workspaceSource(cmd), err)
				} else {
					inUse = ws.ID
				}
			}
			return iohelper.PrintJSON(listWorkspaces(orgs, c.WorkspaceID, inUse))
		},
	}
}

func listWorkspaces(orgs []*auth.Org, defaultID, inUseID string) []*workspaceListing {
	list := make([]*workspaceListing, 0)
	for _, o := range orgs {
		for _, w := range o.Workspaces {
			list = append(list, &workspaceListing{
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
