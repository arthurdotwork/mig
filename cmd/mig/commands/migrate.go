package commands

import (
	"context"

	"github.com/spf13/cobra"
)

func Migrate(ctx context.Context) Command {
	return func(c *cobra.Command, args []string) error {
		c.Println("migrating...")
		return nil
	}
}
