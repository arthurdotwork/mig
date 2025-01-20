package mig

import (
	"context"

	"github.com/arthurdotwork/mig/internal/runner"
)

type Config struct {
	MigrationsDir string
}

type Mig struct {
	config Config
	runner *runner.Runner
}

func New(config Config) (*Mig, error) {
	return &Mig{config: config, runner: runner.New()}, nil
}

func (m *Mig) Run(ctx context.Context) error {
	if err := m.runner.Run(ctx); err != nil {
		return err
	}

	return nil
}
