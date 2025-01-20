package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/arthurdotwork/mig/cmd/mig/commands"
	"github.com/arthurdotwork/mig/internal/runner"
	"github.com/spf13/cobra"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sig
		cancel()
	}()

	r := runner.New()

	rootCmd := &cobra.Command{
		Use:   "mig",
		Short: "mig is a tool for managing database migrations",
	}

	rootCmd.AddCommand(&cobra.Command{
		Use:   "migrate",
		Short: "run the migrations",
		RunE:  commands.Migrate(ctx, r),
	})

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
