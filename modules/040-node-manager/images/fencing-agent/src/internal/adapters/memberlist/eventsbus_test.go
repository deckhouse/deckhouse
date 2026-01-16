package memberlist

import (
	"context"
	"fencing-agent/internal/core/domain"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestNewEventsBus(t *testing.T) {
	bus := NewEventsBus()

	if bus == nil {
		t.Fatal("Expected NewEventsBus to return non-nil instance")
	}

	if bus.subscribers == nil {
		t.Error("Expected subscribers slice to be initialized")
	}

	if len(bus.subscribers) != 0 {
		t.Errorf("Expected 0 initial subscribers, got %d", len(bus.subscribers))
	}

	if bus.mu == nil {
		t.Error("Expected mutex to be initialized")
	}
}

func TestEventsBus_Subscribe_CreatesChannel(t *testing.T) {
	bus := NewEventsBus()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch := bus.Subscribe(ctx)

	if ch == nil {
		t.Fatal("Expected Subscribe to return non-nil channel")
	}

	// Check that subscriber was added
	bus.mu.RLock()
	subscribersCount := len(bus.subscribers)
	bus.mu.RUnlock()

	if subscribersCount != 1 {
		t.Errorf("Expected 1 subscriber, got %d", subscribersCount)
	}
}

func TestEventsBus_Subscribe_MultipleSubscribers(t *testing.T) {
	bus := NewEventsBus()
	ctx := context.Background()

	ctx1, cancel1 := context.WithCancel(ctx)
	defer cancel1()
	ctx2, cancel2 := context.WithCancel(ctx)
	defer cancel2()
	ctx3, cancel3 := context.WithCancel(ctx)
	defer cancel3()

	ch1 := bus.Subscribe(ctx1)
	ch2 := bus.Subscribe(ctx2)
	ch3 := bus.Subscribe(ctx3)

	if ch1 == nil || ch2 == nil || ch3 == nil {
		t.Fatal("Expected all channels to be non-nil")
	}

	bus.mu.RLock()
	subscribersCount := len(bus.subscribers)
	bus.mu.RUnlock()

	if subscribersCount != 3 {
		t.Errorf("Expected 3 subscribers, got %d", subscribersCount)
	}
}

func TestEventsBus_Publish_SingleSubscriber(t *testing.T) {
	bus := NewEventsBus()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch := bus.Subscribe(ctx)

	testEvent := domain.Event{
		Node:      domain.Node{Name: "test-node"},
		EventType: domain.EventTypeJoin,
		Timestamp: time.Now().Unix(),
	}

	// Start receiver before publishing (unbuffered channel requirement)
	received := make(chan domain.Event, 1)
	go func() {
		event := <-ch
		received <- event
	}()

	// Small delay to ensure goroutine is listening
	time.Sleep(10 * time.Millisecond)

	// Publish event
	bus.Publish(testEvent)

	// Wait for event
	select {
	case receivedEvent := <-received:
		if receivedEvent.Node.Name != testEvent.Node.Name {
			t.Errorf("Expected node name '%s', got '%s'", testEvent.Node.Name, receivedEvent.Node.Name)
		}
		if receivedEvent.EventType != testEvent.EventType {
			t.Errorf("Expected event type %d, got %d", testEvent.EventType, receivedEvent.EventType)
		}
		if receivedEvent.Timestamp != testEvent.Timestamp {
			t.Errorf("Expected timestamp %d, got %d", testEvent.Timestamp, receivedEvent.Timestamp)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("Timeout waiting for event")
	}
}

func TestEventsBus_Publish_MultipleSubscribers(t *testing.T) {
	bus := NewEventsBus()
	ctx := context.Background()

	ctx1, cancel1 := context.WithCancel(ctx)
	defer cancel1()
	ctx2, cancel2 := context.WithCancel(ctx)
	defer cancel2()
	ctx3, cancel3 := context.WithCancel(ctx)
	defer cancel3()

	ch1 := bus.Subscribe(ctx1)
	ch2 := bus.Subscribe(ctx2)
	ch3 := bus.Subscribe(ctx3)

	testEvent := domain.Event{
		Node:      domain.Node{Name: "broadcast-node"},
		EventType: domain.EventTypeLeave,
		Timestamp: time.Now().Unix(),
	}

	// Start receivers before publishing
	var wg sync.WaitGroup
	wg.Add(3)
	results := make([]domain.Event, 3)

	startReceiver := func(ch <-chan domain.Event, idx int) {
		event := <-ch
		results[idx] = event
		wg.Done()
	}

	go startReceiver(ch1, 0)
	go startReceiver(ch2, 1)
	go startReceiver(ch3, 2)

	// Small delay to ensure all receivers are ready
	time.Sleep(20 * time.Millisecond)

	// Publish event
	bus.Publish(testEvent)

	// Wait with timeout
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Verify all received the event
		for i, event := range results {
			if event.Node.Name != testEvent.Node.Name {
				t.Errorf("Subscriber %d: expected node name '%s', got '%s'", i, testEvent.Node.Name, event.Node.Name)
			}
		}
	case <-time.After(200 * time.Millisecond):
		t.Error("Timeout waiting for events to be delivered")
	}
}

func TestEventsBus_Publish_NoSubscribers(t *testing.T) {
	bus := NewEventsBus()

	testEvent := domain.Event{
		Node:      domain.Node{Name: "lonely-event"},
		EventType: domain.EventTypeJoin,
		Timestamp: time.Now().Unix(),
	}

	// Should not panic when no subscribers
	bus.Publish(testEvent)
}

func TestEventsBus_Publish_DropsBehavior(t *testing.T) {
	bus := NewEventsBus()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch := bus.Subscribe(ctx)

	// Don't read from channel - it's unbuffered
	// Publish should not block due to select/default
	for i := 0; i < 10; i++ {
		event := domain.Event{
			Node:      domain.Node{Name: "dropped-event"},
			EventType: domain.EventTypeJoin,
			Timestamp: int64(i),
		}
		bus.Publish(event)
	}

	// If we got here, Publish didn't block - good!
	// Channel should be empty or have at most one pending send

	// Try to read - might get nothing due to timing
	select {
	case <-ch:
		// Might receive one
	case <-time.After(10 * time.Millisecond):
		// Or timeout - both are ok
	}
}

func TestEventsBus_Subscribe_ContextCancellation(t *testing.T) {
	bus := NewEventsBus()
	ctx, cancel := context.WithCancel(context.Background())

	ch := bus.Subscribe(ctx)

	// Verify subscriber exists
	bus.mu.RLock()
	initialCount := len(bus.subscribers)
	bus.mu.RUnlock()

	if initialCount != 1 {
		t.Fatalf("Expected 1 subscriber, got %d", initialCount)
	}

	// Cancel context
	cancel()

	// Wait for unsubscribe to complete
	time.Sleep(50 * time.Millisecond)

	// Verify subscriber was removed
	bus.mu.RLock()
	finalCount := len(bus.subscribers)
	bus.mu.RUnlock()

	if finalCount != 0 {
		t.Errorf("Expected 0 subscribers after cancellation, got %d", finalCount)
	}

	// Verify channel is closed
	select {
	case _, ok := <-ch:
		if ok {
			t.Error("Expected channel to be closed")
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("Timeout waiting for channel to close")
	}
}

func TestEventsBus_Subscribe_MultipleContextCancellations(t *testing.T) {
	bus := NewEventsBus()

	// Create 3 subscribers
	ctx1, cancel1 := context.WithCancel(context.Background())
	ctx2, cancel2 := context.WithCancel(context.Background())
	ctx3, cancel3 := context.WithCancel(context.Background())

	ch1 := bus.Subscribe(ctx1)
	ch2 := bus.Subscribe(ctx2)
	ch3 := bus.Subscribe(ctx3)

	// Verify 3 subscribers
	bus.mu.RLock()
	if len(bus.subscribers) != 3 {
		t.Fatalf("Expected 3 subscribers, got %d", len(bus.subscribers))
	}
	bus.mu.RUnlock()

	// Cancel first subscriber
	cancel1()
	time.Sleep(50 * time.Millisecond)

	bus.mu.RLock()
	if len(bus.subscribers) != 2 {
		t.Errorf("After cancel1: expected 2 subscribers, got %d", len(bus.subscribers))
	}
	bus.mu.RUnlock()

	// Cancel second subscriber
	cancel2()
	time.Sleep(50 * time.Millisecond)

	bus.mu.RLock()
	if len(bus.subscribers) != 1 {
		t.Errorf("After cancel2: expected 1 subscriber, got %d", len(bus.subscribers))
	}
	bus.mu.RUnlock()

	// Cancel third subscriber
	cancel3()
	time.Sleep(50 * time.Millisecond)

	bus.mu.RLock()
	if len(bus.subscribers) != 0 {
		t.Errorf("After cancel3: expected 0 subscribers, got %d", len(bus.subscribers))
	}
	bus.mu.RUnlock()

	// Verify all channels are closed
	for i, ch := range []<-chan domain.Event{ch1, ch2, ch3} {
		select {
		case _, ok := <-ch:
			if ok {
				t.Errorf("Channel %d: expected to be closed", i+1)
			}
		case <-time.After(100 * time.Millisecond):
			t.Errorf("Channel %d: timeout waiting for close", i+1)
		}
	}
}

func TestEventsBus_ConcurrentSubscribe(t *testing.T) {
	bus := NewEventsBus()

	const numSubscribers = 20

	var wg sync.WaitGroup
	wg.Add(numSubscribers)

	channels := make([]<-chan domain.Event, numSubscribers)
	cancels := make([]context.CancelFunc, numSubscribers)

	// Launch concurrent subscribers
	for i := 0; i < numSubscribers; i++ {
		go func(idx int) {
			defer wg.Done()
			ctx, cancel := context.WithCancel(context.Background())
			cancels[idx] = cancel
			channels[idx] = bus.Subscribe(ctx)
		}(i)
	}

	wg.Wait()

	// Verify all subscribers registered
	bus.mu.RLock()
	subscribersCount := len(bus.subscribers)
	bus.mu.RUnlock()

	if subscribersCount != numSubscribers {
		t.Errorf("Expected %d subscribers, got %d", numSubscribers, subscribersCount)
	}

	// Cleanup
	for _, cancel := range cancels {
		cancel()
	}
	time.Sleep(100 * time.Millisecond)

	bus.mu.RLock()
	finalCount := len(bus.subscribers)
	bus.mu.RUnlock()

	if finalCount != 0 {
		t.Errorf("Expected 0 subscribers after cleanup, got %d", finalCount)
	}
}

func TestEventsBus_ConcurrentPublish(t *testing.T) {
	bus := NewEventsBus()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch := bus.Subscribe(ctx)

	const numPublishers = 5
	const eventsPerPublisher = 10

	// Use atomic counter for thread-safe counting
	var receivedCount int32

	// Start receiver
	var receiverWg sync.WaitGroup
	receiverWg.Add(1)
	go func() {
		defer receiverWg.Done()
		for {
			select {
			case <-ch:
				atomic.AddInt32(&receivedCount, 1)
			case <-ctx.Done():
				return
			}
		}
	}()

	// Small delay to ensure receiver is ready
	time.Sleep(20 * time.Millisecond)

	var wg sync.WaitGroup
	wg.Add(numPublishers)

	// Launch concurrent publishers
	for i := 0; i < numPublishers; i++ {
		go func(publisherID int) {
			defer wg.Done()
			for j := 0; j < eventsPerPublisher; j++ {
				event := domain.Event{
					Node:      domain.Node{Name: "node"},
					EventType: domain.EventTypeJoin,
					Timestamp: int64(publisherID*1000 + j),
				}
				bus.Publish(event)
				time.Sleep(time.Millisecond) // Small delay to avoid dropping
			}
		}(i)
	}

	wg.Wait()

	// Give time for all events to be received
	time.Sleep(100 * time.Millisecond)

	// Cancel context to stop receiver
	cancel()
	receiverWg.Wait()

	// Get final count
	count := atomic.LoadInt32(&receivedCount)

	// Should have received all or most events
	// Due to unbuffered channels and timing, might not get all
	if count < int32(numPublishers*eventsPerPublisher/2) {
		t.Errorf("Expected at least %d events, received %d", numPublishers*eventsPerPublisher/2, count)
	}
}

func TestEventsBus_Integration_PubSubLifecycle(t *testing.T) {
	bus := NewEventsBus()

	// Phase 1: Subscribe
	ctx1, cancel1 := context.WithCancel(context.Background())
	ch1 := bus.Subscribe(ctx1)

	// Phase 2: Setup receiver and publish
	received1 := make(chan domain.Event, 1)
	go func() {
		for {
			select {
			case e := <-ch1:
				received1 <- e
			case <-ctx1.Done():
				return
			}
		}
	}()

	time.Sleep(10 * time.Millisecond)

	event1 := domain.Event{
		Node:      domain.Node{Name: "node1"},
		EventType: domain.EventTypeJoin,
		Timestamp: 1,
	}
	bus.Publish(event1)

	select {
	case e := <-received1:
		if e.Node.Name != "node1" {
			t.Errorf("Expected 'node1', got '%s'", e.Node.Name)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("Timeout waiting for event1")
	}

	// Phase 3: Add second subscriber
	ctx2, cancel2 := context.WithCancel(context.Background())
	ch2 := bus.Subscribe(ctx2)

	received2 := make(chan domain.Event, 1)
	go func() {
		for {
			select {
			case e := <-ch2:
				received2 <- e
			case <-ctx2.Done():
				return
			}
		}
	}()

	time.Sleep(10 * time.Millisecond)

	// Phase 4: Publish to both
	event2 := domain.Event{
		Node:      domain.Node{Name: "node2"},
		EventType: domain.EventTypeLeave,
		Timestamp: 2,
	}
	bus.Publish(event2)

	// Both should receive
	select {
	case e := <-received1:
		if e.Node.Name != "node2" {
			t.Errorf("ch1: Expected 'node2', got '%s'", e.Node.Name)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("ch1: Timeout waiting for event2")
	}

	select {
	case e := <-received2:
		if e.Node.Name != "node2" {
			t.Errorf("ch2: Expected 'node2', got '%s'", e.Node.Name)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("ch2: Timeout waiting for event2")
	}

	// Phase 5: Cancel first subscriber
	cancel1()
	time.Sleep(50 * time.Millisecond)

	// Phase 6: Publish - only ch2 should receive
	event3 := domain.Event{
		Node:      domain.Node{Name: "node3"},
		EventType: domain.EventTypeJoin,
		Timestamp: 3,
	}
	bus.Publish(event3)

	select {
	case e := <-received2:
		if e.Node.Name != "node3" {
			t.Errorf("Expected 'node3', got '%s'", e.Node.Name)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("Timeout waiting for event3")
	}

	// Phase 7: Cleanup
	cancel2()
	time.Sleep(50 * time.Millisecond)

	bus.mu.RLock()
	finalCount := len(bus.subscribers)
	bus.mu.RUnlock()

	if finalCount != 0 {
		t.Errorf("Expected 0 subscribers, got %d", finalCount)
	}
}

func TestEventsBus_EventTypes(t *testing.T) {
	bus := NewEventsBus()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch := bus.Subscribe(ctx)

	// Setup receiver
	received := make(chan domain.Event, 2)
	go func() {
		for {
			select {
			case e := <-ch:
				received <- e
			case <-ctx.Done():
				return
			}
		}
	}()

	time.Sleep(10 * time.Millisecond)

	// Test both event types
	events := []domain.Event{
		{
			Node:      domain.Node{Name: "joining-node"},
			EventType: domain.EventTypeJoin,
			Timestamp: time.Now().Unix(),
		},
		{
			Node:      domain.Node{Name: "leaving-node"},
			EventType: domain.EventTypeLeave,
			Timestamp: time.Now().Unix(),
		},
	}

	for _, event := range events {
		bus.Publish(event)

		select {
		case receivedEvent := <-received:
			if receivedEvent.EventType != event.EventType {
				t.Errorf("Expected event type %d, got %d", event.EventType, receivedEvent.EventType)
			}
			if receivedEvent.Node.Name != event.Node.Name {
				t.Errorf("Expected node '%s', got '%s'", event.Node.Name, receivedEvent.Node.Name)
			}
		case <-time.After(100 * time.Millisecond):
			t.Errorf("Timeout waiting for event type %d", event.EventType)
		}
	}
}

func TestEventsBus_ThreadSafety(t *testing.T) {
	bus := NewEventsBus()

	const numOperations = 100

	var wg sync.WaitGroup
	wg.Add(3)

	// Concurrent subscribers
	go func() {
		defer wg.Done()
		for i := 0; i < numOperations; i++ {
			ctx, cancel := context.WithCancel(context.Background())
			bus.Subscribe(ctx)
			time.Sleep(time.Microsecond)
			cancel()
		}
	}()

	// Concurrent publishers
	go func() {
		defer wg.Done()
		for i := 0; i < numOperations; i++ {
			event := domain.Event{
				Node:      domain.Node{Name: "test"},
				EventType: domain.EventTypeJoin,
				Timestamp: int64(i),
			}
			bus.Publish(event)
			time.Sleep(time.Microsecond)
		}
	}()

	// Concurrent readers
	go func() {
		defer wg.Done()
		bus.mu.RLock()
		_ = len(bus.subscribers)
		bus.mu.RUnlock()
		time.Sleep(time.Microsecond)
	}()

	wg.Wait()
}
