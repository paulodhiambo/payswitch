package saga_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"switch/internal/orchestrator/domain"
	"switch/internal/orchestrator/saga"
)

type mockStep struct {
	name        string
	executeErr  error
	compensated bool
	paid        *domain.Payment
}

func (m *mockStep) Name() string { return m.name }

func (m *mockStep) Execute(ctx context.Context, p *domain.Payment) error {
	m.paid = p
	return m.executeErr
}

func (m *mockStep) Compensate(ctx context.Context, p *domain.Payment) error {
	m.compensated = true
	return nil
}

func TestSaga_Run_Success(t *testing.T) {
	s := saga.New(
		&mockStep{name: "step1"},
		&mockStep{name: "step2"},
	)
	err := s.Run(context.Background(), &domain.Payment{})
	require.NoError(t, err)
}

func TestSaga_CompensatesOnFailure(t *testing.T) {
	failing := &mockStep{name: "credit_destination", executeErr: errors.New("bank timeout")}
	reserve := &mockStep{name: "reserve_source"}

	s := saga.New(reserve, failing)
	err := s.Run(context.Background(), &domain.Payment{})

	require.Error(t, err)
	require.True(t, reserve.compensated, "reserve step should have been compensated")
}

func TestSaga_NoCompensationOnFirstStepFailure(t *testing.T) {
	failing := &mockStep{name: "first", executeErr: errors.New("fail")}

	s := saga.New(failing)
	err := s.Run(context.Background(), &domain.Payment{})

	require.Error(t, err)
	require.False(t, failing.compensated, "first step should not be compensated")
}

func TestSaga_AllStepsCompensatedInReverseOrder(t *testing.T) {
	first := &mockStep{name: "first"}
	second := &mockStep{name: "second"}
	failing := &mockStep{name: "third", executeErr: errors.New("fail")}

	s := saga.New(first, second, failing)
	err := s.Run(context.Background(), &domain.Payment{})

	require.Error(t, err)
	require.True(t, first.compensated)
	require.True(t, second.compensated)
}

func TestSaga_CompensatePayment_RunsAllCompensations(t *testing.T) {
	first := &mockStep{name: "first"}
	second := &mockStep{name: "second"}

	s := saga.New(first, second)
	p := &domain.Payment{ID: "pay-1", Status: domain.StatusCommitted}
	err := s.CompensatePayment(context.Background(), p)
	require.NoError(t, err)
	require.True(t, first.compensated)
	require.True(t, second.compensated)
}

func TestSaga_CompensatePayment_OnlyCompletedSteps(t *testing.T) {
	first := &mockStep{name: "first"}
	second := &mockStep{name: "second"}

	s := saga.New(first, second)
	p := &domain.Payment{ID: "pay-1", Status: domain.StatusValidated}
	err := s.CompensatePayment(context.Background(), p)
	require.NoError(t, err)
	require.True(t, first.compensated)
	require.False(t, second.compensated, "second step should not have been compensated")
}
