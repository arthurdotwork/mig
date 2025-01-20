package runner

import (
	"context"
	"fmt"
)

type Runner struct {
}

func New() *Runner {
	return &Runner{}
}

func (r *Runner) Run(ctx context.Context) error {
	fmt.Println("migrating...")
	return nil
}
