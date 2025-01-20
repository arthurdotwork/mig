package commands

import (
	"context"

	"github.com/spf13/cobra"
)

type Runner interface {
	Run(ctx context.Context) error
}

func Migrate(ctx context.Context, r Runner) Command {
	return func(c *cobra.Command, args []string) error {
		if err := r.Run(ctx); err != nil {
			return err
		}

		return nil
	}
}
