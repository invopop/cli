package keys

import (
	"encoding/json"
	"fmt"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/invopop/gobl/dsig"
	"github.com/invopop/invocli/internal/cobrax"
)

type command struct {
}

// New prepares a new command line parser for keys.
func New() *cobra.Command {
	c := new(command)

	cmd := &cobra.Command{
		Use:   "keys",
		Short: "Key management",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cobrax.Usage(cmd, "choose a key operation")
		},
	}
	cmd.AddCommand(&cobra.Command{
		Use:   "generate",
		Short: "Generate an ECDSA key pair",
		RunE:  c.generate,
	})

	return cmd
}

func (c *command) generate(cmd *cobra.Command, args []string) error {
	key := dsig.NewES256Key()

	color.HiRed("Private Key:")
	data, err := json.Marshal(key)
	if err != nil {
		return err
	}
	fmt.Println(string(data))

	color.HiGreen("Public Key:")
	data, err = json.Marshal(key.Public())
	if err != nil {
		return err
	}
	fmt.Println(string(data))

	return nil
}
