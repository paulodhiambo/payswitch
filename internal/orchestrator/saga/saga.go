package saga

import (
	"context"

	"switch/internal/orchestrator/domain"
)

type Step interface {
	Name() string
	Execute(ctx context.Context, p *domain.Payment) error
	Compensate(ctx context.Context, p *domain.Payment) error
}

type Saga struct {
	steps []Step
}

func New(steps ...Step) *Saga {
	return &Saga{steps: steps}
}

func (s *Saga) Run(ctx context.Context, p *domain.Payment) error {
	completed := make([]Step, 0, len(s.steps))
	for _, step := range s.steps {
		if err := step.Execute(ctx, p); err != nil {
			s.rollback(ctx, p, completed)
			return err
		}
		completed = append(completed, step)
	}
	return nil
}

func (s *Saga) rollback(ctx context.Context, p *domain.Payment, completed []Step) {
	for i := len(completed) - 1; i >= 0; i-- {
		_ = completed[i].Compensate(ctx, p)
	}
}
