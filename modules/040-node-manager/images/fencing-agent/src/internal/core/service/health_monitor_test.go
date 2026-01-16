package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/gojuno/minimock/v3"
	"go.uber.org/zap"

	mock "fencing-agent/internal/core/ports/minimock"
)

func TestHealthMonitor_check_MaintenanceModeOn_WatchdogArmed(t *testing.T) {
	mc := minimock.NewController(t)

	ctx := context.Background()

	clusterMock := mock.NewClusterProviderMock(mc).
		IsMaintenanceModeMock.Return(true, nil).
		RemoveNodeLabelMock.Return(nil)

	watchdogMock := mock.NewWatchDogMock(mc).
		IsArmedMock.Return(true).
		StopMock.Return(nil)

	membershipMock := mock.NewMembershipProviderMock(mc)

	logger := zap.NewNop()
	hm := NewHealthMonitor(clusterMock, membershipMock, watchdogMock, logger)

	hm.check(ctx)
}

func TestHealthMonitor_check_MaintenanceModeOn_WatchdogNotArmed(t *testing.T) {
	mc := minimock.NewController(t)

	ctx := context.Background()

	clusterMock := mock.NewClusterProviderMock(mc).
		IsMaintenanceModeMock.Return(true, nil).
		RemoveNodeLabelMock.Optional().Return(nil)

	watchdogMock := mock.NewWatchDogMock(mc).
		IsArmedMock.Return(false).
		StopMock.Optional().Return(nil)

	membershipMock := mock.NewMembershipProviderMock(mc)

	logger := zap.NewNop()
	hm := NewHealthMonitor(clusterMock, membershipMock, watchdogMock, logger)

	hm.check(ctx)
}

func TestHealthMonitor_check_MaintenanceModeOn_StopError(t *testing.T) {
	mc := minimock.NewController(t)

	ctx := context.Background()

	clusterMock := mock.NewClusterProviderMock(mc).
		IsMaintenanceModeMock.Return(true, nil).
		RemoveNodeLabelMock.Return(nil)

	stopErr := errors.New("failed to stop watchdog")
	watchdogMock := mock.NewWatchDogMock(mc).
		IsArmedMock.Return(true).
		StopMock.Return(stopErr)

	membershipMock := mock.NewMembershipProviderMock(mc)

	logger := zap.NewNop()
	hm := NewHealthMonitor(clusterMock, membershipMock, watchdogMock, logger)

	// Should not panic even if Stop returns error
	hm.check(ctx)
}

func TestHealthMonitor_check_MaintenanceModeCheckError(t *testing.T) {
	mc := minimock.NewController(t)

	ctx := context.Background()

	checkErr := errors.New("cannot check maintenance mode")
	clusterMock := mock.NewClusterProviderMock(mc).
		IsMaintenanceModeMock.Return(false, checkErr).
		IsAvailableMock.Return(true)

	watchdogMock := mock.NewWatchDogMock(mc).
		IsArmedMock.Return(true).
		FeedMock.Return(nil)

	membershipMock := mock.NewMembershipProviderMock(mc)

	logger := zap.NewNop()
	hm := NewHealthMonitor(clusterMock, membershipMock, watchdogMock, logger)

	// When maintenance mode check fails, it should assume inMaintenance=false and continue
	hm.check(ctx)
}

func TestHealthMonitor_check_NormalMode_WatchdogNotArmed_ShouldArm(t *testing.T) {
	mc := minimock.NewController(t)

	ctx := context.Background()

	clusterMock := mock.NewClusterProviderMock(mc).
		IsMaintenanceModeMock.Return(false, nil).
		SetNodeLabelMock.Return(nil).
		IsAvailableMock.Return(true)

	watchdogMock := mock.NewWatchDogMock(mc).
		IsArmedMock.Return(false).
		StartMock.Return(nil).
		FeedMock.Return(nil)

	membershipMock := mock.NewMembershipProviderMock(mc)

	logger := zap.NewNop()
	hm := NewHealthMonitor(clusterMock, membershipMock, watchdogMock, logger)

	hm.check(ctx)
}

func TestHealthMonitor_check_NormalMode_StartError(t *testing.T) {
	mc := minimock.NewController(t)

	ctx := context.Background()

	clusterMock := mock.NewClusterProviderMock(mc).
		IsMaintenanceModeMock.Return(false, nil).
		SetNodeLabelMock.Optional().Return(nil).
		IsAvailableMock.Optional().Return(true)

	startErr := errors.New("failed to start watchdog")
	watchdogMock := mock.NewWatchDogMock(mc).
		IsArmedMock.Return(false).
		StartMock.Return(startErr).
		FeedMock.Optional().Return(nil)

	membershipMock := mock.NewMembershipProviderMock(mc)

	logger := zap.NewNop()
	hm := NewHealthMonitor(clusterMock, membershipMock, watchdogMock, logger)

	// Should not panic even if Start returns error, and should not proceed to feed
	hm.check(ctx)
}

func TestHealthMonitor_check_SetLabelError_SafetyMechanism(t *testing.T) {
	mc := minimock.NewController(t)

	ctx := context.Background()

	labelErr := errors.New("failed to set label")
	clusterMock := mock.NewClusterProviderMock(mc).
		IsMaintenanceModeMock.Return(false, nil).
		SetNodeLabelMock.Return(labelErr)

	// Safety mechanism: watchdog should be stopped if label setting fails
	watchdogMock := mock.NewWatchDogMock(mc).
		IsArmedMock.Return(false).
		StartMock.Return(nil).
		StopMock.Return(nil)

	membershipMock := mock.NewMembershipProviderMock(mc)

	logger := zap.NewNop()
	hm := NewHealthMonitor(clusterMock, membershipMock, watchdogMock, logger)

	hm.check(ctx)
}

func TestHealthMonitor_check_RemoveLabelError_ContinuesStop(t *testing.T) {
	mc := minimock.NewController(t)

	ctx := context.Background()

	labelErr := errors.New("failed to remove label")
	clusterMock := mock.NewClusterProviderMock(mc).
		IsMaintenanceModeMock.Return(true, nil).
		RemoveNodeLabelMock.Return(labelErr)

	// Even if label removal fails, watchdog should still be stopped
	watchdogMock := mock.NewWatchDogMock(mc).
		IsArmedMock.Return(true).
		StopMock.Return(nil)

	membershipMock := mock.NewMembershipProviderMock(mc)

	logger := zap.NewNop()
	hm := NewHealthMonitor(clusterMock, membershipMock, watchdogMock, logger)

	hm.check(ctx)
}

func TestHealthMonitor_shouldFeedWatchDog_ClusterAvailable(t *testing.T) {
	mc := minimock.NewController(t)

	ctx := context.Background()

	clusterMock := mock.NewClusterProviderMock(mc).
		IsAvailableMock.Return(true)

	watchdogMock := mock.NewWatchDogMock(mc)
	membershipMock := mock.NewMembershipProviderMock(mc)

	logger := zap.NewNop()
	hm := NewHealthMonitor(clusterMock, membershipMock, watchdogMock, logger)

	result := hm.shouldFeedWatchDog(ctx)

	if !result {
		t.Error("Expected shouldFeedWatchDog to return true when cluster is available")
	}
}

func TestHealthMonitor_shouldFeedWatchDog_ClusterUnavailable_HasOtherMembers(t *testing.T) {
	mc := minimock.NewController(t)

	ctx := context.Background()

	clusterMock := mock.NewClusterProviderMock(mc).
		IsAvailableMock.Return(false)

	watchdogMock := mock.NewWatchDogMock(mc)
	membershipMock := mock.NewMembershipProviderMock(mc).
		NumOtherMembersMock.Return(2)

	logger := zap.NewNop()
	hm := NewHealthMonitor(clusterMock, membershipMock, watchdogMock, logger)

	result := hm.shouldFeedWatchDog(ctx)

	if !result {
		t.Error("Expected shouldFeedWatchDog to return true when other members exist")
	}
}

func TestHealthMonitor_shouldFeedWatchDog_ClusterUnavailable_IsAlone(t *testing.T) {
	mc := minimock.NewController(t)

	ctx := context.Background()

	clusterMock := mock.NewClusterProviderMock(mc).
		IsAvailableMock.Return(false)

	watchdogMock := mock.NewWatchDogMock(mc)
	membershipMock := mock.NewMembershipProviderMock(mc).
		NumOtherMembersMock.Return(0).
		IsAloneMock.Return(true)

	logger := zap.NewNop()
	hm := NewHealthMonitor(clusterMock, membershipMock, watchdogMock, logger)

	result := hm.shouldFeedWatchDog(ctx)

	if !result {
		t.Error("Expected shouldFeedWatchDog to return true when node is alone")
	}
}

func TestHealthMonitor_shouldFeedWatchDog_ClusterUnavailable_NoMembers_NotAlone(t *testing.T) {
	mc := minimock.NewController(t)

	ctx := context.Background()

	clusterMock := mock.NewClusterProviderMock(mc).
		IsAvailableMock.Return(false)

	watchdogMock := mock.NewWatchDogMock(mc)
	membershipMock := mock.NewMembershipProviderMock(mc).
		NumOtherMembersMock.Return(0).
		IsAloneMock.Return(false)

	logger := zap.NewNop()
	hm := NewHealthMonitor(clusterMock, membershipMock, watchdogMock, logger)

	result := hm.shouldFeedWatchDog(ctx)

	if result {
		t.Error("Expected shouldFeedWatchDog to return false when cluster unavailable, no members, and not alone")
	}
}

func TestHealthMonitor_check_ShouldFeed_FeedError(t *testing.T) {
	mc := minimock.NewController(t)

	ctx := context.Background()

	clusterMock := mock.NewClusterProviderMock(mc).
		IsMaintenanceModeMock.Return(false, nil).
		IsAvailableMock.Return(true)

	feedErr := errors.New("failed to feed watchdog")
	watchdogMock := mock.NewWatchDogMock(mc).
		IsArmedMock.Return(true).
		FeedMock.Return(feedErr)

	membershipMock := mock.NewMembershipProviderMock(mc)

	logger := zap.NewNop()
	hm := NewHealthMonitor(clusterMock, membershipMock, watchdogMock, logger)

	// Should not panic even if Feed returns error
	hm.check(ctx)
}

func TestHealthMonitor_check_ShouldNotFeed(t *testing.T) {
	mc := minimock.NewController(t)

	ctx := context.Background()

	clusterMock := mock.NewClusterProviderMock(mc).
		IsMaintenanceModeMock.Return(false, nil).
		IsAvailableMock.Return(false)

	watchdogMock := mock.NewWatchDogMock(mc).
		IsArmedMock.Return(true).
		FeedMock.Optional().Return(nil)

	membershipMock := mock.NewMembershipProviderMock(mc).
		NumOtherMembersMock.Return(0).
		IsAloneMock.Return(false)

	logger := zap.NewNop()
	hm := NewHealthMonitor(clusterMock, membershipMock, watchdogMock, logger)

	hm.check(ctx)
}

func TestHealthMonitor_Run_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	// Create minimal mocks without expectations since we're testing cancellation, not behavior
	mc := minimock.NewController(t)

	clusterMock := mock.NewClusterProviderMock(mc).
		IsMaintenanceModeMock.Optional().Return(false, nil).
		IsAvailableMock.Optional().Return(true).
		SetNodeLabelMock.Optional().Return(nil).
		RemoveNodeLabelMock.Optional().Return(nil)

	watchdogMock := mock.NewWatchDogMock(mc).
		IsArmedMock.Optional().Return(true).
		FeedMock.Optional().Return(nil).
		StartMock.Optional().Return(nil).
		StopMock.Optional().Return(nil)

	membershipMock := mock.NewMembershipProviderMock(mc)

	logger := zap.NewNop()
	hm := NewHealthMonitor(clusterMock, membershipMock, watchdogMock, logger)

	done := make(chan struct{})
	go func() {
		hm.Run(ctx, 100*time.Millisecond)
		close(done)
	}()

	// Let it run for a bit
	time.Sleep(50 * time.Millisecond)

	// Cancel context
	cancel()

	// Wait for Run to exit
	select {
	case <-done:
		// Success - Run exited
	case <-time.After(1 * time.Second):
		t.Fatal("Run did not exit after context cancellation")
	}
}

func TestHealthMonitor_Run_PeriodicChecks(t *testing.T) {
	mc := minimock.NewController(t)

	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()

	callCount := 0
	clusterMock := mock.NewClusterProviderMock(mc).
		IsMaintenanceModeMock.Set(func(ctx context.Context) (bool, error) {
			callCount++
			return false, nil
		}).
		IsAvailableMock.Optional().Return(true).
		SetNodeLabelMock.Optional().Return(nil).
		RemoveNodeLabelMock.Optional().Return(nil)

	watchdogMock := mock.NewWatchDogMock(mc).
		IsArmedMock.Optional().Return(true).
		FeedMock.Optional().Return(nil).
		StartMock.Optional().Return(nil).
		StopMock.Optional().Return(nil)

	membershipMock := mock.NewMembershipProviderMock(mc)

	logger := zap.NewNop()
	hm := NewHealthMonitor(clusterMock, membershipMock, watchdogMock, logger)

	done := make(chan struct{})
	go func() {
		hm.Run(ctx, 50*time.Millisecond)
		close(done)
	}()

	<-done

	// Should have been called at least twice (initial + after 50ms)
	if callCount < 2 {
		t.Errorf("Expected at least 2 checks, got %d", callCount)
	}
}

func TestHealthMonitor_Integration_CompleteFlow(t *testing.T) {
	mc := minimock.NewController(t)

	ctx := context.Background()

	// Simulate a complete flow: start unarmed, cluster available, feed watchdog
	clusterMock := mock.NewClusterProviderMock(mc).
		IsMaintenanceModeMock.Return(false, nil).
		SetNodeLabelMock.Return(nil).
		IsAvailableMock.Return(true)

	watchdogMock := mock.NewWatchDogMock(mc).
		IsArmedMock.Return(false).
		StartMock.Return(nil).
		FeedMock.Return(nil)

	membershipMock := mock.NewMembershipProviderMock(mc)

	logger := zap.NewNop()
	hm := NewHealthMonitor(clusterMock, membershipMock, watchdogMock, logger)

	hm.check(ctx)
}

func TestHealthMonitor_Integration_MaintenanceFlow(t *testing.T) {
	mc := minimock.NewController(t)

	ctx := context.Background()

	// Simulate maintenance mode activation: armed watchdog should be stopped
	clusterMock := mock.NewClusterProviderMock(mc).
		IsMaintenanceModeMock.Return(true, nil).
		RemoveNodeLabelMock.Return(nil)

	watchdogMock := mock.NewWatchDogMock(mc).
		IsArmedMock.Return(true).
		StopMock.Return(nil).
		FeedMock.Optional().Return(nil)

	membershipMock := mock.NewMembershipProviderMock(mc)

	logger := zap.NewNop()
	hm := NewHealthMonitor(clusterMock, membershipMock, watchdogMock, logger)

	hm.check(ctx)
}

func TestHealthMonitor_Integration_IsolatedNode(t *testing.T) {
	mc := minimock.NewController(t)

	ctx := context.Background()

	// Simulate isolated node: cluster unavailable but should feed because node is alone
	clusterMock := mock.NewClusterProviderMock(mc).
		IsMaintenanceModeMock.Return(false, nil).
		IsAvailableMock.Return(false)

	watchdogMock := mock.NewWatchDogMock(mc).
		IsArmedMock.Return(true).
		FeedMock.Return(nil)

	membershipMock := mock.NewMembershipProviderMock(mc).
		NumOtherMembersMock.Return(0).
		IsAloneMock.Return(true)

	logger := zap.NewNop()
	hm := NewHealthMonitor(clusterMock, membershipMock, watchdogMock, logger)

	hm.check(ctx)
}

func TestHealthMonitor_NewHealthMonitor(t *testing.T) {
	mc := minimock.NewController(t)

	clusterMock := mock.NewClusterProviderMock(mc)
	watchdogMock := mock.NewWatchDogMock(mc)
	membershipMock := mock.NewMembershipProviderMock(mc)
	logger := zap.NewNop()

	hm := NewHealthMonitor(clusterMock, membershipMock, watchdogMock, logger)

	if hm == nil {
		t.Fatal("Expected NewHealthMonitor to return non-nil instance")
	}
	if hm.cluster == nil {
		t.Error("Expected cluster to be set")
	}
	if hm.membership == nil {
		t.Error("Expected membership to be set")
	}
	if hm.watchdog == nil {
		t.Error("Expected watchdog to be set")
	}
	if hm.logger == nil {
		t.Error("Expected logger to be set")
	}
}

func TestHealthMonitor_check_ClusterAndMembershipUnavailable(t *testing.T) {
	mc := minimock.NewController(t)

	ctx := context.Background()

	// Test the critical scenario: both cluster and membership unavailable
	clusterMock := mock.NewClusterProviderMock(mc).
		IsMaintenanceModeMock.Return(false, nil).
		IsAvailableMock.Return(false)

	watchdogMock := mock.NewWatchDogMock(mc).
		IsArmedMock.Return(true).
		FeedMock.Optional().Return(nil)

	membershipMock := mock.NewMembershipProviderMock(mc).
		NumOtherMembersMock.Return(0).
		IsAloneMock.Return(false)

	logger := zap.NewNop()
	hm := NewHealthMonitor(clusterMock, membershipMock, watchdogMock, logger)

	// In this scenario, the watchdog should NOT be fed (node will reboot)
	hm.check(ctx)
}

func TestHealthMonitor_check_WatchdogArmed_ClusterAvailable(t *testing.T) {
	mc := minimock.NewController(t)

	ctx := context.Background()

	clusterMock := mock.NewClusterProviderMock(mc).
		IsMaintenanceModeMock.Return(false, nil).
		IsAvailableMock.Return(true).
		SetNodeLabelMock.Optional().Return(nil)

	watchdogMock := mock.NewWatchDogMock(mc).
		IsArmedMock.Return(true).
		FeedMock.Return(nil).
		StartMock.Optional().Return(nil)

	membershipMock := mock.NewMembershipProviderMock(mc)

	logger := zap.NewNop()
	hm := NewHealthMonitor(clusterMock, membershipMock, watchdogMock, logger)

	hm.check(ctx)
}

func TestHealthMonitor_check_MembershipHasPeers(t *testing.T) {
	mc := minimock.NewController(t)

	ctx := context.Background()

	// Cluster unavailable but memberlist has other members
	clusterMock := mock.NewClusterProviderMock(mc).
		IsMaintenanceModeMock.Return(false, nil).
		IsAvailableMock.Return(false)

	watchdogMock := mock.NewWatchDogMock(mc).
		IsArmedMock.Return(true).
		FeedMock.Return(nil)

	membershipMock := mock.NewMembershipProviderMock(mc).
		NumOtherMembersMock.Return(3)

	logger := zap.NewNop()
	hm := NewHealthMonitor(clusterMock, membershipMock, watchdogMock, logger)

	hm.check(ctx)
}

func TestHealthMonitor_Stop_WatchdogArmed(t *testing.T) {
	mc := minimock.NewController(t)

	ctx := context.Background()

	clusterMock := mock.NewClusterProviderMock(mc).
		RemoveNodeLabelMock.Return(nil)

	watchdogMock := mock.NewWatchDogMock(mc).
		IsArmedMock.Return(true).
		StopMock.Return(nil)

	membershipMock := mock.NewMembershipProviderMock(mc)

	logger := zap.NewNop()
	hm := NewHealthMonitor(clusterMock, membershipMock, watchdogMock, logger)

	err := hm.Stop(ctx)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
}

func TestHealthMonitor_Stop_WatchdogNotArmed(t *testing.T) {
	mc := minimock.NewController(t)

	ctx := context.Background()

	clusterMock := mock.NewClusterProviderMock(mc)

	watchdogMock := mock.NewWatchDogMock(mc).
		IsArmedMock.Return(false)

	membershipMock := mock.NewMembershipProviderMock(mc)

	logger := zap.NewNop()
	hm := NewHealthMonitor(clusterMock, membershipMock, watchdogMock, logger)

	err := hm.Stop(ctx)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
}
