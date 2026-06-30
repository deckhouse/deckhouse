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
	"io"
	"testing"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/metadata"

	pb "github.com/deckhouse/deckhouse/dhctl/pkg/server/pb/dhctl"
)

const testNoEventTimeout = 50 * time.Millisecond

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

	stoppedCh := startSender[*pb.CheckRequest, *pb.CheckResponse](stream, sendCh, internalErrCh)
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

	startSender[*pb.CheckRequest, *pb.CheckResponse](stream, sendCh, internalErrCh)
	sendCh <- &pb.CheckResponse{}

	select {
	case err := <-internalErrCh:
		require.ErrorContains(t, err, expectedErr.Error())
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for startSender error")
	}
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

func TestStartReceiverReturnsWhenContextCanceledWhileDeliveringRequest(t *testing.T) {
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

	select {
	case <-recvReturned:
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for Recv")
	}

	cancel()

	select {
	case <-stoppedCh:
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for startReceiver to stop after context cancellation")
	}

	select {
	case received := <-receiveCh:
		t.Fatalf("unexpected request delivered after context cancellation: %#v", received)
	case err := <-internalErrCh:
		t.Fatalf("unexpected internal error after context cancellation: %v", err)
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
