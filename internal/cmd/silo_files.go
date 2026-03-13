package cmd

import (
	"fmt"
	"io"
	"os"

	"github.com/invopop/client.go/invopop"
	iohelper "github.com/invopop/invocli/internal/io"
	"github.com/spf13/cobra"
)

func siloFilesCmd(getClient func(cmd *cobra.Command) *invopop.Client) *cobra.Command {
	files := &cobra.Command{
		Use:   "files",
		Short: "Manage silo entry files",
	}

	files.AddCommand(siloFilesCreateCmd(getClient))
	files.AddCommand(siloFilesDownloadCmd(getClient))

	return files
}

func siloFilesCreateCmd(getClient func(cmd *cobra.Command) *invopop.Client) *cobra.Command {
	var (
		file     string
		name     string
		category string
		mime     string
	)
	cmd := &cobra.Command{
		Use:   "create <entry-id>",
		Short: "Upload a file to a silo entry",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if file == "" {
				return fmt.Errorf("--file/-f is required for file upload")
			}
			data, err := os.ReadFile(file)
			if err != nil {
				return fmt.Errorf("reading file %s: %w", file, err)
			}
			if name == "" {
				name = file
			}
			c := getClient(cmd)
			f, err := c.Silo().Files().Create(cmd.Context(), &invopop.CreateSiloFile{
				EntryID:  args[0],
				Name:     name,
				Category: category,
				MIME:     mime,
				Data:     data,
			})
			if err != nil {
				return err
			}
			return iohelper.PrintJSON(f)
		},
	}
	cmd.Flags().StringVarP(&file, "file", "f", "", "Path to file to upload (required)")
	cmd.Flags().StringVar(&name, "name", "", "File name (defaults to file path)")
	cmd.Flags().StringVar(&category, "category", "", "File category (format, request, response, etc.)")
	cmd.Flags().StringVar(&mime, "mime", "", "MIME type (auto-detected if not provided)")
	return cmd
}

func siloFilesDownloadCmd(getClient func(cmd *cobra.Command) *invopop.Client) *cobra.Command {
	return &cobra.Command{
		Use:   "download <entry-id> <file-id>",
		Short: "Download a file from a silo entry",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			c := getClient(cmd)
			rc, err := c.Silo().Files().Download(cmd.Context(), args[0], args[1])
			if err != nil {
				return err
			}
			defer rc.Close()
			_, err = io.Copy(os.Stdout, rc)
			return err
		},
	}
}
