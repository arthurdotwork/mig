package main

import (
	"context"
	"log/slog"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	slog.InfoContext(ctx, "starting mig...")
}
