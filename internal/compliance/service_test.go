package compliance_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"switch/internal/compliance"
	"switch/internal/orchestrator/domain"
)

func TestService_ScreenClear(t *testing.T) {
	svc := compliance.New()
	result, err := svc.Screen(context.Background(), &domain.Payment{
		SourceBIC:      "BANKUS33",
		DestinationBIC: "BANKDEFF",
	})
	require.NoError(t, err)
	require.True(t, result.Cleared)
}

func TestService_ScreenMissingBIC(t *testing.T) {
	svc := compliance.New()
	_, err := svc.Screen(context.Background(), &domain.Payment{
		SourceBIC: "BANKUS33",
	})
	require.Error(t, err)
}
