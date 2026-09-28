// Copyright 2024 Flant JSC
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
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"runtime/debug"
	"runtime/pprof"
	"strings"
	"sync"
	"time"

	"github.com/gogo/protobuf/proto"
	"google.golang.org/grpc"
	"k8s.io/utils/ptr"

	dhlog "github.com/deckhouse/lib-dhctl/pkg/logger"
	"github.com/deckhouse/lib-dhctl/pkg/retry"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	infraexec "github.com/deckhouse/deckhouse/dhctl/pkg/infrastructure/exec"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/check"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/phases"
	pb "github.com/deckhouse/deckhouse/dhctl/pkg/server/pb/dhctl"
	"github.com/deckhouse/deckhouse/dhctl/pkg/server/pkg/fsm"
	"github.com/deckhouse/deckhouse/dhctl/pkg/server/pkg/logger"
	"github.com/deckhouse/deckhouse/dhctl/pkg/state/cache"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/tomb"
)

const (
	// operationCloseTimeout bounds how long close waits for a canceled operation: the
	// worst case of a forced stop of its infrastructure utility, plus time to save the
	// state and clean up. Only a step that ignores cancellation can take longer.
	operationCloseTimeout = infraexec.ForcedStopTimeout + 5*time.Minute

	// senderStopTimeout bounds the wait for the sender once the operation is done. A
	// sender blocked in Send on a stream the client does not read holds nothing, and
	// returning from the handler cancels the stream and releases it anyway.
	senderStopTimeout = 30 * time.Second

	// abandonedShutdownTimeout bounds the server shutdown after close gave up: its
	// callbacks wait for the infrastructure utility and its state saver too, and a
	// step stuck in the operation can hold them forever as well.
	abandonedShutdownTimeout = 2 * time.Minute
)

type Service struct {
	pb.UnimplementedDHCTLServer

	params ServiceParams
}

type ServiceParams struct {
	TmpDir        string
	CacheDir      string
	PodName       string
	PodNamespace  string
	SchemaStore   *config.SchemaStore
	IsDebug       bool
	GlobalOptions *options.GlobalOptions
}

func New(params ServiceParams) *Service {
	return &Service{
		params: params,
	}
}

// operation owns every goroutine a stream handler starts, and the handler must
// not return until they are done or until close gives up. The one-shot dhctl
// server exits as soon as its stream is over, killing whatever is still running at
// that moment, so the deferred cleanup of an unfinished operation (ssh sessions,
// kube proxies on the remote hosts, temporary files) would never run.
type operation struct {
	ctx    context.Context
	cancel context.CancelFunc
	tasks  sync.WaitGroup

	// The sender lives on its own context: it must outlast the operation,
	// which still delivers its result after being canceled. Logs and progress
	// are tied to the operation context and are dropped once it is canceled.
	senderCtx     context.Context
	stopSender    context.CancelFunc
	senderStopped <-chan struct{}

	closeTimeout      time.Duration
	senderStopTimeout time.Duration
	shutdown          func(code int)

	abandonedShutdownTimeout time.Duration
	exit                     func(code int)
}

func newOperation(server grpc.ServerStream) *operation {
	streamCtx := server.Context()

	var name string
	switch server.(type) {
	case pb.DHCTL_CheckServer:
		name = "check"
	case pb.DHCTL_BootstrapServer:
		name = "bootstrap"
	case pb.DHCTL_ConvergeServer:
		name = "converge"
	case pb.DHCTL_DestroyServer:
		name = "destroy"
	case pb.DHCTL_AbortServer:
		name = "abort"
	case pb.DHCTL_CommanderAttachServer:
		name = "commander/attach"
	case pb.DHCTL_CommanderDetachServer:
		name = "commander/detach"
	default:
		name = "unknown"
	}

	// Nobody is there to interrupt a stuck infrastructure utility twice: bound its
	// stop, otherwise a canceled operation, and close with it, could wait forever.
	//
	// A Cancel message leaves the stream open: the client waits for the result, and
	// the utility gets its grace period to finish the work in flight. A closed stream
	// means the client has given up on the operation and may already retry it, so
	// the utility is stopped right away instead of overlapping the retry.
	ctx, cancel := context.WithCancel(infraexec.WithForcedStop(streamCtx, streamCtx.Done()))
	senderCtx, stopSender := context.WithCancel(streamCtx)

	return &operation{
		ctx:               logger.ToContext(ctx, logger.L(streamCtx).With(slog.String("operation", name))),
		cancel:            cancel,
		senderCtx:         senderCtx,
		stopSender:        stopSender,
		closeTimeout:      operationCloseTimeout,
		senderStopTimeout: senderStopTimeout,
		shutdown:          tomb.Shutdown,

		abandonedShutdownTimeout: abandonedShutdownTimeout,
		exit:                     os.Exit,
	}
}

// Go runs fn in a goroutine that close waits for.
func (o *operation) Go(fn func()) {
	o.tasks.Go(fn)
}

// close cancels the operation and blocks until its goroutines have returned and
// their deferred cleanup is done, then stops the sender. Only goroutines that own
// nothing may outlive the handler, released by the stream cancellation that comes
// right after it returns: the receiver blocked in Recv, which cannot be interrupted
// before that, and a sender blocked in Send past senderStopTimeout.
//
// The wait for the operation is bounded by closeTimeout, long enough for an
// infrastructure utility to finish in the worst case: past it, something ignores
// cancellation, and close reports it and gives up. The server then exits with a
// failure (unless a shutdown by a signal has already started and set its code),
// and the shutdown is bounded by abandonedShutdownTimeout: the process exits even
// if it hangs.
//
// Finally the one-shot server is shut down. Asynchronously: its graceful stop
// waits for this handler to return.
func (o *operation) close() {
	exitCode := 0
	defer func() { go o.shutdown(exitCode) }()
	// Normally stopped once the operation is done, here only when close gives up.
	defer o.stopSender()

	o.cancel()

	// Outlives close only when close gives up, and then the process exits anyway.
	tasksDone := make(chan struct{})
	go func() {
		o.tasks.Wait()
		close(tasksDone)
	}()

	// A finished operation is done in no time: report only a wait worth noticing.
	slowWait := time.AfterFunc(time.Second, func() {
		logger.L(o.ctx).Info("waiting for the canceled operation to finish")
	})

	select {
	case <-tasksDone:
		slowWait.Stop()
	case <-time.After(o.closeTimeout):
		slowWait.Stop()
		o.giveUp()
		exitCode = 1
		return
	}

	o.stopSender()
	if o.senderStopped == nil {
		return
	}

	select {
	case <-o.senderStopped:
	case <-time.After(o.senderStopTimeout):
		logger.L(o.ctx).Warn("sender did not stop, leaving it to the stream cancellation",
			slog.Duration("timeout", o.senderStopTimeout),
		)
	}
}

// giveUp reports the operation close gave up waiting for, with the stacks of all
// goroutines (identical ones grouped) to find the step that ignores cancellation,
// and makes the process exit if the server shutdown hangs on the same step.
func (o *operation) giveUp() {
	var stacks strings.Builder
	_ = pprof.Lookup("goroutine").WriteTo(&stacks, 1)

	log := logger.L(o.ctx)
	log.Error("gave up waiting for the canceled operation, shutting down anyway",
		slog.Duration("timeout", o.closeTimeout),
		slog.String("goroutines", stacks.String()),
	)

	time.AfterFunc(o.abandonedShutdownTimeout, func() {
		log.Error("server shutdown did not finish, exiting",
			slog.Duration("timeout", o.abandonedShutdownTimeout),
		)
		o.exit(1)
	})
}

type serverStream[Request proto.Message, Response proto.Message] interface {
	Send(Response) error
	Recv() (Request, error)
	grpc.ServerStream
}

// internalErrChBufferSize is enough for the two goroutines that can report a
// stream failure concurrently: receiver and sender. Keeping the buffer bounded
// avoids blocking those goroutines after the main handler has already returned.
const internalErrChBufferSize = 2

func startReceiver[Request, Response proto.Message](
	server serverStream[Request, Response],
	receiveCh chan Request,
	doneCh chan struct{},
	internalErrCh chan error,
) <-chan struct{} {
	stoppedCh := make(chan struct{})

	go func() {
		defer close(stoppedCh)

		for {
			request, err := server.Recv()
			if errors.Is(err, io.EOF) {
				close(doneCh)
				return
			}
			if err != nil {
				sendInternalErr(internalErrCh, fmt.Errorf("receiving message: %w", err))
				return
			}
			select {
			case receiveCh <- request:
			case <-server.Context().Done():
				// The handler loop may be busy right now and has no other way to
				// learn the stream is gone: without this report it would wait forever.
				sendInternalErr(internalErrCh, fmt.Errorf("receiving message: %w", server.Context().Err()))
				return
			}
		}
	}()

	return stoppedCh
}

// startOperationSender starts the sender that op.close stops once the operation is done.
func startOperationSender[Request, Response proto.Message](
	op *operation,
	server serverStream[Request, Response],
	sendCh chan Response,
	internalErrCh chan error,
) {
	op.senderStopped = startSender[Request, Response](op.senderCtx, server, sendCh, internalErrCh)
}

// startSender streams responses from sendCh until ctx is done or sendCh is closed.
// grpc finishes the stream on any SendMsg error, which cancels ctx and stops the
// sender. Should a failed send leave the stream alive, the sender drains sendCh
// from then on: the operation keeps writing to it until it is canceled and done,
// and must not get blocked on the way out.
func startSender[Request, Response proto.Message](
	ctx context.Context,
	server serverStream[Request, Response],
	sendCh chan Response,
	internalErrCh chan error,
) <-chan struct{} {
	stoppedCh := make(chan struct{})

	go func() {
		defer close(stoppedCh)

		failed := false
		for {
			var response Response
			select {
			case received, ok := <-sendCh:
				if !ok {
					return
				}
				response = received
			case <-ctx.Done():
				return
			}

			if failed {
				continue
			}

			loop := retry.NewSilentLoopWithParamsOpts(
				retry.WithName("send message"),
				retry.WithAttempts(10),
				retry.WithWait(100*time.Millisecond),
			)
			err := loop.RunContext(ctx, func() error {
				return server.Send(response)
			})
			if err != nil {
				sendInternalErr(internalErrCh, fmt.Errorf("sending message: %w", err))
				failed = true
			}
		}
	}()

	return stoppedCh
}

func sendInternalErr(internalErrCh chan<- error, err error) {
	select {
	case internalErrCh <- err:
	default:
	}
}

func sendResponse[T proto.Message](ctx context.Context, sendCh chan<- T, response T) error {
	select {
	case sendCh <- response:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func sendPhaseSwitch(ctx context.Context, next chan<- error, err error) {
	select {
	case next <- err:
	case <-ctx.Done():
	}
}

type fsmPhaseSwitcher[T proto.Message, OperationPhaseDataT any] struct {
	f        *fsm.FiniteStateMachine
	dataFunc func(data phases.OnPhaseFuncData[OperationPhaseDataT]) (T, error)
	sendCh   chan T
	next     chan error
}

func (b *fsmPhaseSwitcher[T, OperationPhaseDataT]) switchPhase(ctx context.Context) func(
	onPhaseData phases.OnPhaseFuncData[OperationPhaseDataT],
) error {
	return func(onPhaseData phases.OnPhaseFuncData[OperationPhaseDataT]) error {
		err := b.f.Event("wait")
		if err != nil {
			return fmt.Errorf("changing state to waiting: %w", err)
		}

		data, err := b.dataFunc(onPhaseData)
		if err != nil {
			return fmt.Errorf("switch phase data func error: %w", err)
		}

		err = sendResponse(ctx, b.sendCh, data)
		if err != nil {
			return fmt.Errorf("sending phase switch data: %w: %w", phases.ErrStopOperationCondition, err)
		}

		var (
			switchErr error
			ok        bool
		)

		select {
		case switchErr, ok = <-b.next:
			if !ok {
				return fmt.Errorf("server stopped, canceling task")
			}
		case <-ctx.Done():
			switchErr = fmt.Errorf("%w: %w", phases.ErrStopOperationCondition, ctx.Err())
		}

		return switchErr
	}
}

func onCheckResult(ctx context.Context, checkRes *check.CheckResult) error {
	printableCheckRes := *checkRes
	printableCheckRes.StatusDetails.InfrastructurePlan = nil

	printableCheckResDump, err := json.MarshalIndent(printableCheckRes, "", "  ")
	if err != nil {
		return fmt.Errorf("unable to encode check result json: %w", err)
	}

	_ = dhlog.RunProcess(ctx, dhlog.FromContext(ctx), "Check result", func(ctx context.Context) error {
		dhlog.FromContext(ctx).InfoContext(ctx, string(printableCheckResDump))
		return nil
	})

	return nil
}

func extractLastState(ctx context.Context) ([]byte, error) {
	state, err := phases.ExtractDhctlState(ctx, cache.Global())
	if err != nil {
		return nil, fmt.Errorf("extracting last state: %w", err)
	}

	data, err := json.Marshal(state)
	if err != nil {
		return nil, fmt.Errorf("marshaling last state: %w", err)
	}

	return data, nil
}

func panicResult(ctx context.Context, p any) ([]byte, error) {
	stack := string(debug.Stack())

	logger.L(ctx).Error("recovered from panic",
		slog.Any("panic", p),
		slog.String("stack", stack),
	)

	errs := []error{
		fmt.Errorf("panic: %v, %s", p, stack),
	}

	lastState, err := extractLastState(ctx)
	if err != nil {
		errs = append(errs, err)
	}

	return lastState, errors.Join(errs...)
}

type progressTracker[T proto.Message] struct {
	sendCh   chan T
	dataFunc func(progress phases.Progress) T
}

func (p *progressTracker[T]) sendProgress(ctx context.Context) phases.OnProgressFunc {
	return func(progress phases.Progress) error {
		return sendResponse(ctx, p.sendCh, p.dataFunc(progress))
	}
}

func convertProgress(p phases.Progress) *pb.Progress {
	allPhases := make([]*pb.Progress_PhaseWithSubPhases, 0, len(p.Phases))

	for _, phase := range p.Phases {
		subPhases := make([]string, 0, len(phase.SubPhases))
		for _, subPhase := range phase.SubPhases {
			subPhases = append(subPhases, string(subPhase))
		}

		allPhases = append(allPhases, &pb.Progress_PhaseWithSubPhases{
			Phase:     string(phase.Phase),
			Action:    string(ptr.Deref(phase.Action, phases.ProgressActionDefault)),
			SubPhases: subPhases,
		})
	}

	return &pb.Progress{
		Operation:         string(p.Operation),
		Progress:          p.Progress,
		CompletedPhase:    string(p.CompletedPhase),
		CurrentPhase:      string(p.CurrentPhase),
		NextPhase:         string(p.NextPhase),
		CompletedSubPhase: string(p.CompletedSubPhase),
		CurrentSubPhase:   string(p.CurrentSubPhase),
		NextSubPhase:      string(p.NextSubPhase),
		Phases:            allPhases,
	}
}
