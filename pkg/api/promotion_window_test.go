package api

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	kargoapi "github.com/akuity/kargo/api/v1alpha1"
)

func generatePromotionWindow(name, kind, schedule string) *kargoapi.CognitePromotionWindow {
	return &kargoapi.CognitePromotionWindow{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "test",
		},
		Spec: kargoapi.PromotionWindowSpec{
			Kind:     kind,
			Schedule: schedule,
			Duration: &metav1.Duration{Duration: time.Hour},
			LabelSelector: metav1.LabelSelector{
				MatchLabels: map[string]string{
					"env": "staging",
				},
			},
		},
	}
}

func TestCheckPromotionWindows(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, kargoapi.SchemeBuilder.AddToScheme(scheme))
	janPastMidnight := time.Date(2024, 1, 1, 0, 30, 0, 0, time.UTC) // Jan 1, 2024 00:30 UTC
	defaultStage := metav1.ObjectMeta{
		Name:      "test-stage",
		Namespace: "test",
		Labels: map[string]string{
			"env": "staging",
		},
	}

	tests := []struct {
		name       string
		now        time.Time
		client     client.Client
		assertions func(t *testing.T, allow bool, err error)
	}{
		{
			name:   "allow promotions by default",
			now:    janPastMidnight, // Jan 1, 2024 00:30 UTC
			client: fake.NewClientBuilder().WithScheme(scheme).Build(),
			assertions: func(t *testing.T, allow bool, err error) {
				require.NoError(t, err)
				require.True(t, allow)
			},
		},
		{
			name: "allow promotion with active allow window",
			now:  janPastMidnight, // Jan 1, 2024 00:30 UTC
			client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(
				generatePromotionWindow("allow-window", "allow", "0 0 * * *"),
			).Build(),
			assertions: func(t *testing.T, allow bool, err error) {
				require.NoError(t, err)
				require.True(t, allow)
			},
		},
		{
			name: "allow promotion when at least one allow window is active",
			now:  janPastMidnight, // Jan 1, 2024 00:30 UTC
			client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(
				generatePromotionWindow("allow-window-1", "allow", "0 3 * * *"),
				generatePromotionWindow("allow-window-2", "allow", "0 0 * * *"),
			).Build(),
			assertions: func(t *testing.T, allow bool, err error) {
				require.NoError(t, err)
				require.True(t, allow)
			},
		},
		{
			name: "disallow promotion on inactive allow window",
			now:  time.Date(2024, 1, 1, 1, 30, 0, 0, time.UTC), // Jan 1, 2024 01:30 UTC
			client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(
				generatePromotionWindow("allow-window", "allow", "0 0 * * *"),
			).Build(),
			assertions: func(t *testing.T, allow bool, err error) {
				require.NoError(t, err)
				require.False(t, allow)
			},
		},
		{
			name: "disallow promotion on inactive allow windows",
			now:  time.Date(2024, 1, 1, 1, 30, 0, 0, time.UTC), // Jan 1, 2024 01:30 UTC
			client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(
				generatePromotionWindow("allow-window-1", "allow", "0 0 * * *"),
				generatePromotionWindow("allow-window-2", "allow", "0 0 * * *"),
			).Build(),
			assertions: func(t *testing.T, allow bool, err error) {
				require.NoError(t, err)
				require.False(t, allow)
			},
		},
		{
			name: "deny promotion on active deny window",
			now:  janPastMidnight, // Jan 1, 2024 00:30 UTC
			client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(
				generatePromotionWindow("deny-window", "deny", "0 0 * * *"),
			).Build(),
			assertions: func(t *testing.T, allow bool, err error) {
				require.NoError(t, err)
				require.False(t, allow)
			},
		},
		{
			name: "allow promotion on inactive deny window",
			now:  janPastMidnight, // Jan 1, 2024 00:30 UTC
			client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(
				generatePromotionWindow("deny-window", "deny", "0 3 * * *"),
			).Build(),
			assertions: func(t *testing.T, allow bool, err error) {
				require.NoError(t, err)
				require.True(t, allow)
			},
		},
		{
			name: "disallow promotion with deny window active and allow window inactive",
			now:  time.Date(2024, 1, 1, 0, 30, 0, 0, time.UTC), // Jan 1, 2024 00:30 UTC
			client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(
				generatePromotionWindow("deny-window", "deny", "0 0 * * *"),
				generatePromotionWindow("allow-window", "allow", "0 2 * * *"),
			).Build(),
			assertions: func(t *testing.T, allow bool, err error) {
				require.NoError(t, err)
				require.False(t, allow)
			},
		},
		{
			name: "disallow promotion with both deny window and allow window active",
			now:  janPastMidnight, // Jan 1, 2024 00:30 UTC
			client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(
				generatePromotionWindow("deny-window", "deny", "0 0 * * *"),
				generatePromotionWindow("allow-window", "allow", "0 0 * * *"),
			).Build(),
			assertions: func(t *testing.T, allow bool, err error) {
				require.NoError(t, err)
				require.False(t, allow)
			},
		},
		{
			name: "disallow promotion with both allow window and deny window active",
			now:  janPastMidnight, // Jan 1, 2024 00:30 UTC
			client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(
				generatePromotionWindow("allow-window", "allow", "0 0 * * *"),
				generatePromotionWindow("deny-window", "deny", "0 0 * * *"),
			).Build(),
			assertions: func(t *testing.T, allow bool, err error) {
				require.NoError(t, err)
				require.False(t, allow)
			},
		},
		{
			name: "allow promotion with allow window active and deny window inactive",
			now:  janPastMidnight, // Jan 1, 2024 00:30 UTC
			client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(
				generatePromotionWindow("allow-window", "allow", "0 0 * * *"),
				generatePromotionWindow("deny-window", "deny", "0 2 * * *"),
			).Build(),
			assertions: func(t *testing.T, allow bool, err error) {
				require.NoError(t, err)
				require.True(t, allow)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			active, err := CheckPromotionWindows(
				context.Background(),
				tt.now,
				tt.client,
				defaultStage,
			)

			tt.assertions(t, active, err)
		})
	}
}
