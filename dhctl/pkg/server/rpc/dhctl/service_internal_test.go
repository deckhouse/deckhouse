// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package dhctl

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	pb "github.com/deckhouse/deckhouse/dhctl/pkg/server/pb/dhctl"
)

const (
	testNoEventTimeout = 50 * time.Millisecond
	// testEventTimeout is generous: a failed send spends about a second in retries.
	testEventTimeout = 5 * time.Second
)

func TestSendResponseReturnsContextCanceledWithoutReceiver(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	err := sendResponse(ctx, make(chan *pb.CheckResponse), &pb.CheckResponse{})

	require.ErrorIs(t, err, context.Canceled)
}

func TestTerminalResponseCanUseStreamContextAfterOperationCancel(t *testing.T) {
	t.Parallel()

	opCtx, cancelOperation := context.WithCancel(t.Context())
	cancelOperation()

	streamCtx := t.Context()
	sendCh := make(chan *pb.CheckResponse)
	response := &pb.CheckResponse{}
	received := make(chan *pb.CheckResponse, 1)

	go func() {
		received <- <-sendCh
	}()

	err := sendResponse(streamCtx, sendCh, response)

	require.NoError(t, err)
	require.ErrorIs(t, opCtx.Err(), context.Canceled)
	require.Same(t, response, <-received)
}

func TestSendPhaseSwitchReturnsWhenContextCanceled(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	done := make(chan struct{})
	go func() {
		defer close(done)
		sendPhaseSwitch(ctx, make(chan error), errors.New("phase error"))
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("sendPhaseSwitch blocked after context cancellation")
	}
}

func TestStartSenderStopsWhenSendChannelClosed(t *testing.T) {
	t.Parallel()

	sendCh := make(chan *pb.CheckResponse)
	internalErrCh := make(chan error, internalErrChBufferSize)
	sentCh := make(chan *pb.CheckResponse, 1)

	stream := &testServerStream[*pb.CheckRequest, *pb.CheckResponse]{
		ctx: t.Context(),
		sendFn: func(response *pb.CheckResponse) error {
			sentCh <- response

			return nil
		},
	}

	stoppedCh := startSender[*pb.CheckRequest, *pb.CheckResponse](t.Context(), stream, sendCh, internalErrCh)
	close(sendCh)

	select {
	case <-stoppedCh:
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for startSender to stop after sendCh close")
	}

	select {
	case response := <-sentCh:
		t.Fatalf("unexpected response sent after sendCh close: %#v", response)
	case err := <-internalErrCh:
		t.Fatalf("unexpected internal error after sendCh close: %v", err)
	default:
	}
}

func TestStartSenderReportsSendError(t *testing.T) {
	t.Parallel()

	expectedErr := errors.New("send failed")
	sendCh := make(chan *pb.CheckResponse)
	internalErrCh := make(chan error, internalErrChBufferSize)

	stream := &testServerStream[*pb.CheckRequest, *pb.CheckResponse]{
		ctx: t.Context(),
		sendFn: func(*pb.CheckResponse) error {
			return expectedErr
		},
	}

	startSender[*pb.CheckRequest, *pb.CheckResponse](t.Context(), stream, sendCh, internalErrCh)
	requireSend(t, sendCh, &pb.CheckResponse{}, "response")

	require.ErrorContains(t, requireReceive(t, internalErrCh, "send error"), expectedErr.Error())
}

func TestStartSenderDrainsSendChannelAfterSendError(t *testing.T) {
	t.Parallel()

	sendCh := make(chan *pb.CheckResponse)
	internalErrCh := make(chan error, internalErrChBufferSize)
	var sendCalls atomic.Int32

	stream := &testServerStream[*pb.CheckRequest, *pb.CheckResponse]{
		ctx: t.Context(),
		sendFn: func(*pb.CheckResponse) error {
			sendCalls.Add(1)

			return errors.New("send failed")
		},
	}

	ctx, stop := context.WithCancel(t.Context())
	stoppedCh := startSender[*pb.CheckRequest, *pb.CheckResponse](ctx, stream, sendCh, internalErrCh)

	requireSend(t, sendCh, &pb.CheckResponse{}, "first response")
	require.ErrorContains(t, requireReceive(t, internalErrCh, "send error"), "send failed")
	callsAfterFailure := sendCalls.Load()

	// The operation must not get stuck on its way out: later responses are
	// consumed without touching the broken stream.
	for range 3 {
		requireSend(t, sendCh, &pb.CheckResponse{}, "response after send error")
	}

	stop()
	requireReceive(t, stoppedCh, "sender stop after context cancellation")

	require.Equal(t, callsAfterFailure, sendCalls.Load())
}

func TestOperationCloseWaitsForTasksBeforeStoppingSender(t *testing.T) {
	t.Parallel()

	sent := make(chan *pb.CheckResponse, 1)
	stream := &testServerStream[*pb.CheckRequest, *pb.CheckResponse]{
		ctx: t.Context(),
		// A slow Send: close must still be waiting for it when the task is done.
		sendFn: func(response *pb.CheckResponse) error {
			time.Sleep(testNoEventTimeout)
			sent <- response

			return nil
		},
	}

	op, shutdownCalled := newTestOperation(stream)
	sendCh := make(chan *pb.CheckResponse)
	internalErrCh := make(chan error, internalErrChBufferSize)
	startOperationSender[*pb.CheckRequest, *pb.CheckResponse](op, stream, sendCh, internalErrCh)

	result := &pb.CheckResponse{}
	started := make(chan struct{})
	var (
		cleanedUp        bool
		senderErrOnClean error
		resultErr        error
	)
	op.Go(func() {
		defer func() { cleanedUp = true }()

		close(started)
		<-op.ctx.Done()

		// Slow cleanup after cancellation, then the final result is still
		// delivered: the sender is alive until the task is done.
		time.Sleep(testNoEventTimeout)
		senderErrOnClean = op.senderCtx.Err()
		resultErr = sendResponse(stream.Context(), sendCh, result)
	})

	requireReceive(t, started, "task start")
	requireReceive(t, closeAsync(op), "operation close")

	require.True(t, cleanedUp)
	require.NoError(t, senderErrOnClean, "sender stopped before the task was done")
	require.NoError(t, resultErr)

	select {
	case response := <-sent:
		require.Same(t, result, response)
	default:
		t.Fatal("close returned before the final result was sent")
	}
	require.ErrorIs(t, op.senderCtx.Err(), context.Canceled)
	require.Zero(t, requireReceive(t, shutdownCalled, "server shutdown"))
}

func TestOperationCloseWithoutSender(t *testing.T) {
	t.Parallel()

	op, shutdownCalled := newTestOperation(&testServerStream[*pb.CheckRequest, *pb.CheckResponse]{ctx: t.Context()})

	requireReceive(t, closeAsync(op), "operation close")
	require.Zero(t, requireReceive(t, shutdownCalled, "server shutdown"))
}

func TestOperationCloseGivesUpOnTaskIgnoringCancellation(t *testing.T) {
	t.Parallel()

	op, shutdownCalled := newTestOperation(&testServerStream[*pb.CheckRequest, *pb.CheckResponse]{ctx: t.Context()})
	op.closeTimeout = testNoEventTimeout
	// Giving up arms the exit deadline: it may fire after the test is over.
	op.exit = func(int) {}

	release := make(chan struct{})
	t.Cleanup(func() { close(release) })
	op.Go(func() { <-release })

	requireReceive(t, closeAsync(op), "operation close")
	require.Equal(t, 1, requireReceive(t, shutdownCalled, "server shutdown"), "abandoned operation exits as a failure")
}

func TestOperationExitsWhenShutdownHangsAfterGivingUp(t *testing.T) {
	t.Parallel()

	op, _ := newTestOperation(&testServerStream[*pb.CheckRequest, *pb.CheckResponse]{ctx: t.Context()})
	op.closeTimeout = testNoEventTimeout
	op.abandonedShutdownTimeout = testNoEventTimeout

	release := make(chan struct{})
	t.Cleanup(func() { close(release) })

	// The shutdown waits on the same stuck step as the operation.
	op.Go(func() { <-release })
	op.shutdown = func(int) { <-release }

	exitCode := make(chan int, 1)
	op.exit = func(code int) { exitCode <- code }

	requireReceive(t, closeAsync(op), "operation close")
	require.Equal(t, 1, requireReceive(t, exitCode, "process exit"))
}

func TestOperationDoesNotExitAfterCleanClose(t *testing.T) {
	t.Parallel()

	op, shutdownCalled := newTestOperation(&testServerStream[*pb.CheckRequest, *pb.CheckResponse]{ctx: t.Context()})
	op.abandonedShutdownTimeout = testNoEventTimeout

	exitCode := make(chan int, 1)
	op.exit = func(code int) { exitCode <- code }

	op.Go(func() { <-op.ctx.Done() })

	requireReceive(t, closeAsync(op), "operation close")
	require.Zero(t, requireReceive(t, shutdownCalled, "server shutdown"))

	select {
	case code := <-exitCode:
		t.Fatalf("process exit %d after a clean close", code)
	case <-time.After(4 * testNoEventTimeout):
	}
}

func TestOperationCloseStopsWaitingForSenderStuckInSend(t *testing.T) {
	t.Parallel()

	release := make(chan struct{})
	t.Cleanup(func() { close(release) })

	sending := make(chan struct{})
	stream := &testServerStream[*pb.CheckRequest, *pb.CheckResponse]{
		ctx: t.Context(),
		// A client that does not read the stream: Send blocks until the stream is canceled.
		sendFn: func(*pb.CheckResponse) error {
			close(sending)
			<-release

			return nil
		},
	}

	op, shutdownCalled := newTestOperation(stream)
	op.senderStopTimeout = testNoEventTimeout

	sendCh := make(chan *pb.CheckResponse)
	startOperationSender[*pb.CheckRequest, *pb.CheckResponse](op, stream, sendCh, make(chan error, internalErrChBufferSize))
	requireSend(t, sendCh, &pb.CheckResponse{}, "response")
	requireReceive(t, sending, "Send")

	// Nothing is abandoned: the operation is done, and the stuck Send is released
	// by the stream cancellation once the handler returns.
	started := time.Now()
	requireReceive(t, closeAsync(op), "operation close")
	require.GreaterOrEqual(t, time.Since(started), op.senderStopTimeout, "close did not wait for the sender")
	require.Zero(t, requireReceive(t, shutdownCalled, "server shutdown"))
}

func TestStartReceiverClosesDoneChannelOnEOF(t *testing.T) {
	t.Parallel()

	doneCh := make(chan struct{})
	internalErrCh := make(chan error, internalErrChBufferSize)
	receiveCh := make(chan *pb.CheckRequest)

	stream := &testServerStream[*pb.CheckRequest, *pb.CheckResponse]{
		ctx: t.Context(),
		recvFn: func() (*pb.CheckRequest, error) {
			return nil, io.EOF
		},
	}

	startReceiver[*pb.CheckRequest, *pb.CheckResponse](stream, receiveCh, doneCh, internalErrCh)

	select {
	case <-doneCh:
	case err := <-internalErrCh:
		t.Fatalf("unexpected internal error: %v", err)
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for doneCh close")
	}
}

// The handler loop may be busy (e.g. in sendPhaseSwitch) when the stream is
// canceled while the receiver holds a request. The receiver is then the only
// one who knows the stream is gone, and must report it, or the loop never wakes.
func TestStartReceiverReportsContextCancellationWhileDeliveringRequest(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	recvReturned := make(chan struct{})
	doneCh := make(chan struct{})
	internalErrCh := make(chan error, internalErrChBufferSize)
	receiveCh := make(chan *pb.CheckRequest)
	request := &pb.CheckRequest{}

	stream := &testServerStream[*pb.CheckRequest, *pb.CheckResponse]{
		ctx: ctx,
		recvFn: func() (*pb.CheckRequest, error) {
			close(recvReturned)

			return request, nil
		},
	}

	stoppedCh := startReceiver[*pb.CheckRequest, *pb.CheckResponse](stream, receiveCh, doneCh, internalErrCh)

	requireReceive(t, recvReturned, "Recv")
	cancel()

	require.ErrorIs(t, requireReceive(t, internalErrCh, "receiver error"), context.Canceled)
	requireReceive(t, stoppedCh, "receiver stop after context cancellation")

	select {
	case received := <-receiveCh:
		t.Fatalf("unexpected request delivered after context cancellation: %#v", received)
	case <-doneCh:
		t.Fatal("doneCh closed on context cancellation")
	default:
	}
}

type testServerStream[Request proto.Message, Response proto.Message] struct {
	ctx    context.Context
	sendFn func(Response) error
	recvFn func() (Request, error)
}

func (s *testServerStream[Request, Response]) Send(response Response) error {
	if s.sendFn == nil {
		return nil
	}

	return s.sendFn(response)
}

func (s *testServerStream[Request, Response]) Recv() (Request, error) {
	if s.recvFn == nil {
		var zero Request

		return zero, io.EOF
	}

	return s.recvFn()
}

func (s *testServerStream[Request, Response]) SetHeader(metadata.MD) error {
	return nil
}

func (s *testServerStream[Request, Response]) SendHeader(metadata.MD) error {
	return nil
}

func (s *testServerStream[Request, Response]) SetTrailer(metadata.MD) {}

func (s *testServerStream[Request, Response]) Context() context.Context {
	if s.ctx == nil {
		return context.Background()
	}

	return s.ctx
}

func (s *testServerStream[Request, Response]) SendMsg(any) error {
	return nil
}

func (s *testServerStream[Request, Response]) RecvMsg(any) error {
	return io.EOF
}

// newTestOperation returns an operation reporting the exit code of its server
// shutdown, and failing loudly on a process exit.
func newTestOperation(stream grpc.ServerStream) (*operation, <-chan int) {
	op := newOperation(stream)

	shutdownCalled := make(chan int, 1)
	op.shutdown = func(code int) { shutdownCalled <- code }
	op.exit = func(code int) { panic(fmt.Sprintf("unexpected process exit %d", code)) }

	return op, shutdownCalled
}

func closeAsync(op *operation) <-chan struct{} {
	closed := make(chan struct{})
	go func() {
		defer close(closed)
		op.close()
	}()

	return closed
}

func requireReceive[T any](t *testing.T, ch <-chan T, what string) T {
	t.Helper()

	select {
	case v := <-ch:
		return v
	case <-time.After(testEventTimeout):
		t.Fatalf("timeout waiting for %s", what)
	}

	var zero T

	return zero
}

func requireSend[T any](t *testing.T, ch chan<- T, v T, what string) {
	t.Helper()

	select {
	case ch <- v:
	case <-time.After(testEventTimeout):
		t.Fatalf("timeout sending %s", what)
	}
}
