package saga

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"switch/internal/orchestrator/domain"
	"switch/pkg/telemetry"
)

var tracer = otel.Tracer("saga")

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
	ctx, span := tracer.Start(ctx, "saga.Run",
		trace.WithAttributes(telemetry.SpanAttrs(p.ID, p.EndToEndID, string(p.Status))...),
	)
	defer span.End()

	completed := make([]Step, 0, len(s.steps))
	for _, step := range s.steps {
		if err := s.execStep(ctx, step, p); err != nil {
			span.SetStatus(codes.Error, err.Error())
			span.RecordError(err)
			s.rollback(ctx, p, completed)
			return err
		}
		completed = append(completed, step)
	}
	return nil
}

func (s *Saga) execStep(ctx context.Context, step Step, p *domain.Payment) error {
	ctx, span := tracer.Start(ctx, "saga.step."+step.Name(),
		trace.WithAttributes(attribute.String("step", step.Name())),
	)
	defer span.End()
	return step.Execute(ctx, p)
}

func (s *Saga) CompensatePayment(ctx context.Context, p *domain.Payment) error {
	for i := len(s.steps) - 1; i >= 0; i-- {
		if err := s.steps[i].Compensate(ctx, p); err != nil {
			return err
		}
	}
	return nil
}

func (s *Saga) rollback(ctx context.Context, p *domain.Payment, completed []Step) {
	for i := len(completed) - 1; i >= 0; i-- {
		_ = completed[i].Compensate(ctx, p)
	}
}
