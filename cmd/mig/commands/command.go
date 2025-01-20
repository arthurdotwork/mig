package commands

import "github.com/spf13/cobra"

type Command func(c *cobra.Command, args []string) error
