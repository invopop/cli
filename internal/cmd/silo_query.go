package cmd

import (
	"context"
	"fmt"

	iohelper "github.com/invopop/cli/internal/io"
	"github.com/invopop/client.go/invopop"
	"github.com/spf13/cobra"
)

const queryConsistencyNote = `Queries run against silo's read model, which is eventually consistent: an
entry created a moment ago may not appear until the result's watermark passes
its creation time.`

func siloEntriesQueryCmd(getClient func() (*invopop.Client, error)) *cobra.Command {
	var (
		req invopop.QuerySiloEntries
		all bool
	)
	cmd := &cobra.Command{
		Use:   "query <schema>",
		Short: "Search silo entries of one document type with a filter",
		Long: `Search silo entries of one GOBL document type, such as bill/invoice, using a
filter expression. Combine terms with AND, and use OR only between values of a
single field. The fields available for each schema are listed in the API
reference.

` + queryConsistencyNote,
		Example: `  invopop silo entries query bill/invoice --filter 'state = "sent" AND issue_date > "2026-01-01"'
  invopop silo entries query bill/invoice --order-by 'issue_date desc' --fields id,code,state --all`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if all && req.Cursor != "" {
				return fmt.Errorf("--all and --cursor cannot be used together")
			}
			c, err := getClient()
			if err != nil {
				return err
			}
			req.Schema = args[0]
			var col *invopop.SiloQueryEntryCollection
			if all {
				col, err = queryAllEntries(cmd.Context(), c, &req)
			} else {
				col, err = c.Silo().Entries().Query(cmd.Context(), &req)
			}
			if err != nil {
				return err
			}
			return iohelper.PrintJSON(col)
		},
	}
	cmd.Flags().StringVar(&req.Filter, "filter", "", "Filter expression")
	cmd.Flags().StringVar(&req.OrderBy, "order-by", "", "Sort order, as a field optionally followed by asc or desc")
	cmd.Flags().Int32Var(&req.Limit, "limit", 0, "Maximum number of results per page, up to 100")
	cmd.Flags().StringVar(&req.Cursor, "cursor", "", "Pagination cursor from a previous next_cursor")
	cmd.Flags().StringVar(&req.Fields, "fields", "", "Comma-separated fields to populate on each entry")
	cmd.Flags().BoolVar(&req.Count, "count", false, "Include the number of matching entries")
	cmd.Flags().BoolVar(&all, "all", false, "Follow next_cursor and return every matching entry")
	return cmd
}

// queryAllEntries follows each page's next cursor, gathering every entry into
// the last page's collection so the freshness and advice it reports are the
// most recent.
func queryAllEntries(ctx context.Context, c *invopop.Client, req *invopop.QuerySiloEntries) (*invopop.SiloQueryEntryCollection, error) {
	page := *req
	var (
		list  []*invopop.SiloEntry
		first *invopop.SiloQueryEntryCollection
		seen  = make(map[string]bool)
	)
	for {
		col, err := c.Silo().Entries().Query(ctx, &page)
		if err != nil {
			return nil, err
		}
		if first == nil {
			first = col
		}
		list = append(list, col.List...)
		if col.NextCursor == "" {
			col.List = list
			col.Cursor = ""
			// The count costs an extra query, so it was only asked for once.
			col.Count, col.CountCapped, col.CountUnavailable = first.Count, first.CountCapped, first.CountUnavailable
			return col, nil
		}
		if seen[col.NextCursor] {
			return nil, fmt.Errorf("query returned cursor %q twice", col.NextCursor)
		}
		seen[col.NextCursor] = true
		page.Cursor = col.NextCursor
		page.Count = false
	}
}

func siloEntriesCountCmd(getClient func() (*invopop.Client, error)) *cobra.Command {
	var filter string
	cmd := &cobra.Command{
		Use:   "count <schema>",
		Short: "Count silo entries of one document type matching a filter",
		Long: `Count silo entries of one GOBL document type, such as bill/invoice, that match
a filter expression, without returning them.

` + queryConsistencyNote,
		Example: `  invopop silo entries count bill/invoice --filter 'state = "sent"'`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := getClient()
			if err != nil {
				return err
			}
			res, err := c.Silo().Entries().Count(cmd.Context(), &invopop.CountSiloEntries{
				Schema: args[0],
				Filter: filter,
			})
			if err != nil {
				return err
			}
			return iohelper.PrintJSON(res)
		},
	}
	cmd.Flags().StringVar(&filter, "filter", "", "Filter expression")
	return cmd
}
