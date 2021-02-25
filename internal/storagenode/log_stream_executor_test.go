package storagenode

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	. "github.com/smartystreets/goconvey/convey"
	"go.uber.org/zap"

	"github.com/kakao/varlog/pkg/types"
	"github.com/kakao/varlog/pkg/util/testutil"
	"github.com/kakao/varlog/pkg/verrors"
	"github.com/kakao/varlog/proto/varlogpb"
)

func TestLogStreamExecutorNew(t *testing.T) {
	Convey("LogStreamExecutor", t, func() {
		Convey("it should not be created with nil storage", func() {
			_, err := NewLogStreamExecutor(zap.L(), types.LogStreamID(0), nil, newNopTelmetryStub(), &LogStreamExecutorOptions{})
			So(err, ShouldNotBeNil)
		})

		Convey("it should not be sealed at first", func() {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			storage := NewMockStorage(ctrl)
			storage.EXPECT().RestoreLogStreamContext(gomock.Any()).Return(false)
			storage.EXPECT().RestoreStorage(gomock.Any(), gomock.Any())
			lse, err := NewLogStreamExecutor(zap.L(), types.LogStreamID(0), storage, newNopTelmetryStub(), &LogStreamExecutorOptions{})
			So(err, ShouldBeNil)
			So(lse.(*logStreamExecutor).isSealed(), ShouldBeFalse)
		})
	})
}

func TestLogStreamExecutorRunClose(t *testing.T) {
	Convey("LogStreamExecutor", t, func() {
		Convey("it should be run and closed", func() {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			storage := NewMockStorage(ctrl)
			storage.EXPECT().RestoreLogStreamContext(gomock.Any()).Return(false)
			storage.EXPECT().RestoreStorage(gomock.Any(), gomock.Any())
			storage.EXPECT().Close().Return(nil).AnyTimes()
			lse, err := NewLogStreamExecutor(zap.L(), types.LogStreamID(0), storage, newNopTelmetryStub(), &LogStreamExecutorOptions{})
			So(err, ShouldBeNil)

			err = lse.Run(context.TODO())
			So(err, ShouldBeNil)

			lse.Close()
		})
	})
}

func TestLogStreamExecutorOperations(t *testing.T) {
	Convey("LogStreamExecutor", t, func() {
		const logStreamID = types.LogStreamID(1)
		const N = 1000

		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		storage := NewMockStorage(ctrl)
		storage.EXPECT().RestoreLogStreamContext(gomock.Any()).Return(false)
		storage.EXPECT().RestoreStorage(gomock.Any(), gomock.Any())
		storage.EXPECT().Close().Return(nil).AnyTimes()
		replicator := NewMockReplicator(ctrl)

		opts := DefaultLogStreamExecutorOptions()
		lse, err := NewLogStreamExecutor(zap.L(), logStreamID, storage, newNopTelmetryStub(), &opts)
		So(err, ShouldBeNil)

		err = lse.Run(context.TODO())
		So(err, ShouldBeNil)

		Reset(func() {
			lse.Close()
		})

		Convey("read operation should reply uncertainties if it doesn't know", func() {
			_, err := lse.Read(context.TODO(), types.MinGLSN)
			So(errors.Is(err, verrors.ErrUndecidable), ShouldBeTrue)
		})

		Convey("read operation should reply written data", func() {
			storage.EXPECT().Read(gomock.Any()).Return(types.LogEntry{Data: []byte("log")}, nil)
			lse.(*logStreamExecutor).lsc.localLowWatermark = 0
			lse.(*logStreamExecutor).lsc.localHighWatermark = 10
			logEntry, err := lse.Read(context.TODO(), types.GLSN(0))
			So(err, ShouldBeNil)
			So(string(logEntry.Data), ShouldEqual, "log")
		})

		Convey("append operation should not write data when sealed", func() {
			lse.(*logStreamExecutor).sealItself()
			_, err := lse.Append(context.TODO(), []byte("never"))
			So(errors.Is(err, verrors.ErrSealed), ShouldBeTrue)
		})

		Convey("append operation should not write data when the storage is failed", func() {
			storage.EXPECT().Write(gomock.Any(), gomock.Any()).Return(verrors.ErrInternal)
			_, err := lse.Append(context.TODO(), []byte("never"))
			So(err, ShouldNotBeNil)
			sealed := lse.(*logStreamExecutor).isSealed()
			So(sealed, ShouldBeTrue)
		})

		Convey("append operation should not write data when the replication is failed", func() {
			storage.EXPECT().Write(gomock.Any(), gomock.Any()).Return(nil)
			c := make(chan error, 1)
			c <- verrors.ErrInternal
			replicator.EXPECT().Replicate(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(c)
			replicator.EXPECT().Close().AnyTimes()
			lse.(*logStreamExecutor).replicator = replicator
			_, err := lse.Append(context.TODO(), []byte("never"), Replica{})
			So(err, ShouldNotBeNil)
		})

		Convey("append operation should write data", func() {
			waitCommitDone := func(knownNextGLSN types.GLSN) {
				for {
					lse.(*logStreamExecutor).lsc.rcc.mu.RLock()
					updatedKnownNextGLSN := lse.(*logStreamExecutor).lsc.rcc.globalHighwatermark
					lse.(*logStreamExecutor).lsc.rcc.mu.RUnlock()
					if knownNextGLSN != updatedKnownNextGLSN {
						break
					}
					time.Sleep(time.Millisecond)
				}
			}
			waitWriteDone := func(uncommittedLLSNEnd types.LLSN) {
				for uncommittedLLSNEnd == lse.(*logStreamExecutor).lsc.uncommittedLLSNEnd.Load() {
					time.Sleep(time.Millisecond)
				}
			}

			storage.EXPECT().Write(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
			storage.EXPECT().StoreCommitContext(gomock.Any()).Return(nil).AnyTimes()
			storage.EXPECT().Commit(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
			for i := types.MinGLSN; i < N; i++ {
				lse.(*logStreamExecutor).lsc.rcc.mu.RLock()
				knownHWM := lse.(*logStreamExecutor).lsc.rcc.globalHighwatermark
				lse.(*logStreamExecutor).lsc.rcc.mu.RUnlock()
				uncommittedLLSNEnd := lse.(*logStreamExecutor).lsc.uncommittedLLSNEnd.Load()
				var wg sync.WaitGroup
				wg.Add(1)
				go func(uncommittedLLSNEnd types.LLSN, knownNextGLSN types.GLSN) {
					defer wg.Done()
					waitWriteDone(uncommittedLLSNEnd)
					lse.Commit(context.TODO(), CommittedLogStreamStatus{
						LogStreamID:         logStreamID,
						HighWatermark:       i,
						PrevHighWatermark:   i - 1,
						CommittedGLSNOffset: i,
						CommittedGLSNLength: 1,
					})
					waitCommitDone(knownNextGLSN)
				}(uncommittedLLSNEnd, knownHWM)
				glsn, err := lse.Append(context.TODO(), []byte("log"))
				So(err, ShouldBeNil)
				So(glsn, ShouldEqual, i)
				wg.Wait()
			}
		})
	})
}

func TestLogStreamExecutorAppend(t *testing.T) {
	Convey("Given that a LogStreamExecutor.Append is called", t, func() {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		storage := NewMockStorage(ctrl)
		storage.EXPECT().Close().Return(nil).AnyTimes()
		storage.EXPECT().RestoreLogStreamContext(gomock.Any()).Return(false)
		storage.EXPECT().RestoreStorage(gomock.Any(), gomock.Any())
		replicator := NewMockReplicator(ctrl)
		lse, err := NewLogStreamExecutor(zap.L(), types.LogStreamID(1), storage, newNopTelmetryStub(), &LogStreamExecutorOptions{
			AppendCTimeout:    DefaultLSEAppendCTimeout,
			CommitWaitTimeout: DefaultLSECommitWaitTimeout,
			TrimCTimeout:      DefaultLSETrimCTimeout,
			CommitCTimeout:    DefaultLSECommitCTimeout,
		})
		So(err, ShouldBeNil)

		lse.(*logStreamExecutor).replicator = replicator
		replicator.EXPECT().Run(gomock.Any()).AnyTimes()
		replicator.EXPECT().Close().AnyTimes()

		err = lse.Run(context.TODO())
		So(err, ShouldBeNil)

		Reset(func() {
			lse.Close()
		})

		Convey("When the context passed to the Append is cancelled", func() {
			storage.EXPECT().Write(gomock.Any(), gomock.Any()).Return(nil).MaxTimes(1)

			rC := make(chan error, 1)
			rC <- nil
			replicator.EXPECT().Replicate(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(rC).MaxTimes(1)

			// FIXME: This is a very ugly test because it is not deterministic.
			Convey("Then the LogStreamExecutor should return cancellation error", func(c C) {
				ctx, cancel := context.WithCancel(context.TODO())
				stop := make(chan struct{})
				go func() {
					_, err := lse.Append(ctx, nil, Replica{})
					c.So(errors.Is(err, context.Canceled), ShouldBeTrue)
					close(stop)
				}()
				time.Sleep(time.Millisecond * time.Duration(rand.Intn(10)))
				cancel()
				<-stop
			})
		})

		Convey("When the appendC in the LogStreamExecutor is blocked", func() {
			stop := make(chan struct{})
			storage.EXPECT().Write(gomock.Any(), gomock.Any()).DoAndReturn(
				func(types.LLSN, []byte) error {
					<-stop
					return nil
				},
			).AnyTimes()
			defer close(stop)

			// add dummy appendtask to block next requests
			lse.(*logStreamExecutor).appendC <- newAppendTask(nil, nil, types.MinLLSN, &lse.(*logStreamExecutor).trackers, lse.(*logStreamExecutor).stopped)
			Convey("And the Append is blocked more than configured", func() {
				lse.(*logStreamExecutor).options.AppendCTimeout = time.Duration(0)
				Convey("Then the LogStreamExecutor should return timeout error", func() {
					_, err := lse.Append(context.TODO(), nil)
					So(errors.Is(err, context.DeadlineExceeded), ShouldBeTrue)
				})
			})

			Convey("And the context passed to the Append is cancelled", func() {
				ctx, cancel := context.WithCancel(context.TODO())
				cancel()
				Convey("Then the LogStreamExecutor should return cancellation error", func() {
					_, err := lse.Append(ctx, nil)
					So(errors.Is(err, context.Canceled), ShouldBeTrue)
				})
			})
		})

		Convey("When the Storage.Write operation is blocked", func() {
			stop := make(chan struct{})
			block := func(f func()) {
				storage.EXPECT().Write(gomock.Any(), gomock.Any()).DoAndReturn(
					func(types.LLSN, []byte) error {
						f()
						<-stop
						return nil
					},
				).MaxTimes(1)
			}

			Reset(func() {
				close(stop)
			})

			Convey("And the Append is blocked more than configured", func() {
				lse.(*logStreamExecutor).options.AppendCTimeout = time.Duration(0)
				lse.(*logStreamExecutor).options.CommitWaitTimeout = time.Duration(0)
				block(func() {})
				Convey("Then the LogStreamExecutor should return timeout error", func() {
					_, err := lse.Append(context.TODO(), nil)
					So(errors.Is(err, context.DeadlineExceeded), ShouldBeTrue)
				})
			})

			Convey("And the context passed to the Append is cancelled", func() {
				ctx, cancel := context.WithCancel(context.TODO())
				block(func() {
					cancel()
				})

				Convey("Then the LogStreamExecutor should return cancellation error", func() {
					_, err := lse.Append(ctx, nil)
					So(errors.Is(err, context.Canceled), ShouldBeTrue)
				})
			})
		})

		Convey("When the replication is blocked", func() {
			storage.EXPECT().Write(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
			stop := make(chan struct{})
			block := func(f func()) {
				replicator.EXPECT().Replicate(
					gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(),
				).DoAndReturn(
					func(context.Context, types.LLSN, []byte, []Replica) <-chan error {
						f()
						<-stop
						c := make(chan error, 1)
						c <- nil
						return c
					},
				)
			}

			Reset(func() {
				close(stop)
			})

			Convey("And it is blocked more than configured", func() {
				Convey("Then the LogStreamExecutor should return timeout error", func() {
					Convey("This isn't yet implemented", nil)
				})
			})

			Convey("And the context passed to the Append is cancelled", func() {
				ctx, cancel := context.WithCancel(context.TODO())
				block(func() {
					cancel()
				})

				Convey("Then the LogStreamExecutor should return cancellation error", func() {
					_, err := lse.Append(ctx, nil, Replica{})
					So(errors.Is(err, context.Canceled), ShouldBeTrue)
				})
			})
		})

		Convey("When the commit is not notified", func() {
			storage.EXPECT().Write(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
			replicator.EXPECT().Replicate(
				gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(),
			).DoAndReturn(
				func(context.Context, types.LLSN, []byte, []Replica) <-chan error {
					defer func() {
						lse.Commit(context.TODO(), CommittedLogStreamStatus{
							LogStreamID:         lse.LogStreamID(),
							HighWatermark:       types.MinGLSN,
							PrevHighWatermark:   types.InvalidGLSN,
							CommittedGLSNOffset: types.MinGLSN,
							CommittedGLSNLength: 1,
						})
					}()
					c := make(chan error, 1)
					c <- nil
					return c
				},
			).AnyTimes()

			stop := make(chan struct{})
			block := func(f func()) {
				storage.EXPECT().StoreCommitContext(gomock.Any()).Return(nil).MaxTimes(1)
				storage.EXPECT().Commit(gomock.Any(), gomock.Any()).DoAndReturn(
					func(types.LLSN, types.GLSN) error {
						f()
						<-stop
						return nil
					},
				).MaxTimes(1)
			}

			Reset(func() {
				close(stop)
			})

			Convey("And the Append is blocked more than configured", func() {
				lse.(*logStreamExecutor).options.CommitCTimeout = time.Duration(0)
				lse.(*logStreamExecutor).options.CommitWaitTimeout = time.Duration(0)
				block(func() {})
				Convey("Then the LogStreamExecutor should return timeout error", func() {
					_, err := lse.Append(context.TODO(), nil, Replica{})
					So(errors.Is(err, context.DeadlineExceeded), ShouldBeTrue)
				})
			})

			Convey("And the context passed to the Append is cancelled", func() {
				ctx, cancel := context.WithCancel(context.TODO())
				block(func() {
					cancel()
				})

				Convey("Then the LogStreamExecutor should return cancellation error", func(c C) {
					wait := make(chan struct{})
					go func() {
						_, err := lse.Append(ctx, nil, Replica{})
						c.So(errors.Is(err, context.Canceled), ShouldBeTrue)
						close(wait)
					}()
					<-wait
				})
			})
		})
	})
}

func TestLogStreamExecutorRead(t *testing.T) {
	Convey("Given that a LogStreamExecutor.Read is called", t, func() {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		storage := NewMockStorage(ctrl)
		storage.EXPECT().RestoreLogStreamContext(gomock.Any()).Return(false)
		storage.EXPECT().RestoreStorage(gomock.Any(), gomock.Any())
		storage.EXPECT().Close().Return(nil).AnyTimes()
		lse, err := NewLogStreamExecutor(zap.L(), types.LogStreamID(1), storage, newNopTelmetryStub(), &LogStreamExecutorOptions{})
		So(err, ShouldBeNil)

		err = lse.Run(context.TODO())
		So(err, ShouldBeNil)

		Reset(func() {
			lse.Close()
		})

		Convey("When the context passed to the Read is cancelled", func() {
			lse.(*logStreamExecutor).lsc.localHighWatermark.Store(types.MaxGLSN)

			stop := make(chan struct{})
			storage.EXPECT().Read(gomock.Any()).DoAndReturn(func(types.GLSN) ([]byte, error) {
				<-stop
				return []byte("foo"), nil
			}).MaxTimes(1)

			Reset(func() {
				close(stop)
			})

			Convey("Then the LogStreamExecutor should return cancellation error", func(c C) {
				wait := make(chan struct{})
				ctx, cancel := context.WithCancel(context.TODO())
				go func() {
					_, err := lse.Read(ctx, types.MinGLSN)
					c.So(errors.Is(err, context.Canceled), ShouldBeTrue)
					close(wait)
				}()
				time.Sleep(time.Millisecond * time.Duration(rand.Intn(10)))
				cancel()
				<-wait
			})
		})
	})
}

func TestLogStreamExecutorSubscribe(t *testing.T) {
	Convey("Given LogStreamExecutor.Subscribe", t, func() {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		storage := NewMockStorage(ctrl)
		storage.EXPECT().RestoreLogStreamContext(gomock.Any()).Return(false)
		storage.EXPECT().RestoreStorage(gomock.Any(), gomock.Any())
		storage.EXPECT().Close().Return(nil).AnyTimes()
		lse, err := NewLogStreamExecutor(zap.L(), types.LogStreamID(1), storage, newNopTelmetryStub(), &LogStreamExecutorOptions{})
		So(err, ShouldBeNil)

		err = lse.Run(context.TODO())
		So(err, ShouldBeNil)

		Reset(func() {
			lse.Close()
		})

		Convey("When the GLSN passed to it is less than LowWatermark", func() {
			lse.(*logStreamExecutor).lsc.localLowWatermark.Store(2)
			Convey("Then the LogStreamExecutor.Subscribe should return an error", func() {
				_, err := lse.Subscribe(context.TODO(), 2, 3)
				So(err, ShouldNotBeNil)
			})
		})

		Convey("When Storage.Scan returns an error", func() {
			storage.EXPECT().Scan(gomock.Any(), gomock.Any()).Return(nil, verrors.ErrInternal)
			lse.(*logStreamExecutor).lsc.localLowWatermark.Store(1)
			lse.(*logStreamExecutor).lsc.localHighWatermark.Store(10)
			Convey("Then the LogStreamExecutor.Subscribe should return a channel that has the error", func() {
				c, err := lse.Subscribe(context.TODO(), 1, 11)
				So(err, ShouldBeNil)
				So((<-c).Err, ShouldNotBeNil)
			})
		})

		Convey("When Storage.Scan returns a valid scanner", func() {
			scanner := NewMockScanner(ctrl)
			scanner.EXPECT().Close().Return(nil).AnyTimes()
			storage.EXPECT().Scan(gomock.Any(), gomock.Any()).Return(scanner, nil)

			lse.(*logStreamExecutor).lsc.localLowWatermark.Store(1)
			lse.(*logStreamExecutor).lsc.localHighWatermark.Store(10)

			Convey("And the Scanner.Next returns an error", func() {
				scanner.EXPECT().Next().Return(NewInvalidScanResult(verrors.ErrInternal))

				Convey("Then the LogStreamExecutor.Subscribe should return a channel that has the error", func() {
					c, err := lse.Subscribe(context.TODO(), 1, 11)
					So(err, ShouldBeNil)
					So((<-c).Err, ShouldNotBeNil)
				})
			})

			Convey("And the Scannext.Next returns log entries out of order", func() {
				const repeat = 3
				var cs []*gomock.Call
				for i := 0; i < repeat; i++ {
					logEntry := types.LogEntry{
						LLSN: types.MinLLSN + types.LLSN(i),
						GLSN: types.MinGLSN + types.GLSN(i),
					}
					if i == repeat-1 {
						logEntry.LLSN += types.LLSN(1)
					}
					c := scanner.EXPECT().Next().Return(ScanResult{LogEntry: logEntry})
					cs = append(cs, c)
				}
				for i := len(cs) - 1; i > 0; i-- {
					cs[i].After(cs[i-1])
				}
				Convey("Then the LogStreamExecutor.Subscribe should return a channel that has the error", func() {
					c, err := lse.Subscribe(context.TODO(), types.MinGLSN, types.MaxGLSN)
					So(err, ShouldBeNil)
					for i := 0; i < repeat-1; i++ {
						So((<-c).Err, ShouldBeNil)
					}
					So((<-c).Err, ShouldNotBeNil)
				})
			})
		})

	})
}

func TestLogStreamExecutorSeal(t *testing.T) {
	Convey("Given LogStreamExecutor", t, func() {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		const lsid = types.LogStreamID(1)
		storage := NewMockStorage(ctrl)
		storage.EXPECT().RestoreLogStreamContext(gomock.Any()).Return(false)
		storage.EXPECT().RestoreStorage(gomock.Any(), gomock.Any())
		opts := DefaultLogStreamExecutorOptions()
		lseI, err := NewLogStreamExecutor(zap.L(), lsid, storage, newNopTelmetryStub(), &opts)
		So(err, ShouldBeNil)
		lse := lseI.(*logStreamExecutor)
		updatedAt := lse.LastUpdated()

		So(lseI.Run(context.TODO()), ShouldBeNil)

		Reset(func() {
			storage.EXPECT().Close().Return(nil)
			lseI.Close()
		})

		Convey("When LogStreamExecutor.sealItself is called", func() {
			lse.sealItself()

			Convey("Then status of the LogStreamExecutor is SEALING", func() {
				sealed := lse.isSealed()
				So(sealed, ShouldBeTrue)
				lse.muStatus.RLock()
				status := lse.status
				lse.muStatus.RUnlock()
				So(status, ShouldEqual, varlogpb.LogStreamStatusSealing)

				So(lse.LastUpdated(), ShouldNotEqual, updatedAt)
			})
		})

		Convey("When LogStreamExecutor.Seal is called (localHWM < lastCommittedGLSN)", func() {
			lse.lsc.localHighWatermark.Store(types.MinGLSN)

			Convey("Then status of LogStreamExecutor is SEALING", func() {
				status, _ := lse.Seal(types.MaxGLSN)
				So(status, ShouldEqual, varlogpb.LogStreamStatusSealing)

				So(lse.LastUpdated(), ShouldNotEqual, updatedAt)
			})
		})

		Convey("When LogStreamExecutor.Seal is called (localHWM = lastCommittedGLSN)", func() {
			lse.lsc.localHighWatermark.Store(types.MinGLSN)
			storage.EXPECT().Read(gomock.Any()).Return(types.LogEntry{}, nil)
			storage.EXPECT().DeleteUncommitted(gomock.Any()).Return(nil)
			Convey("Then status of LogStreamExecutor is SEALED", func() {
				status, _ := lse.Seal(types.MinGLSN)
				So(status, ShouldEqual, varlogpb.LogStreamStatusSealed)

				So(lse.LastUpdated(), ShouldNotEqual, updatedAt)
			})
		})

		Convey("When LogStreamExecutor.Seal is called (localHWM > lastCommittedGLSN)", func() {
			lse.lsc.localHighWatermark.Store(types.MaxGLSN)

			Convey("Then panic is occurred", func() {
				So(func() { lse.Seal(types.MinGLSN) }, ShouldPanic)

				So(lse.LastUpdated(), ShouldEqual, updatedAt)
			})
		})

	})
}

func TestLogStreamExecutorAndStorage(t *testing.T) {
	Convey("Sealing initial LS with InvalidGLSN", t, func() {
		stg, err := NewStorage(InMemoryStorageName, WithLogger(zap.L()))
		So(err, ShouldBeNil)

		opts := DefaultLogStreamExecutorOptions()
		lse, err := NewLogStreamExecutor(zap.L(), logStreamID, stg, newNopTelmetryStub(), &opts)
		So(err, ShouldBeNil)

		lse.Run(context.TODO())
		defer lse.Close()

		status, sealedGLSN := lse.Seal(types.InvalidGLSN)
		So(status, ShouldEqual, varlogpb.LogStreamStatusSealed)
		So(sealedGLSN, ShouldEqual, types.InvalidGLSN)
	})

	Convey("LogStreamExecutor and Storage", t, func(c C) {
		const (
			logStreamID = types.LogStreamID(1)
			repeat      = 100
		)

		stg, err := NewStorage(InMemoryStorageName, WithLogger(zap.L()))
		So(err, ShouldBeNil)

		opts := DefaultLogStreamExecutorOptions()
		lse, err := NewLogStreamExecutor(zap.L(), logStreamID, stg, newNopTelmetryStub(), &opts)
		So(err, ShouldBeNil)

		lse.Run(context.TODO())
		defer lse.Close()

		// check initial state
		So(lse.GetReport().KnownHighWatermark, ShouldEqual, 0)

		waitForCommitted := func(highWatermark, prevHighWatermark, committedGLSNOffset types.GLSN, committedGLSNLength uint64) <-chan error {
			c := make(chan error, 1)
			go func() {
				defer close(c)
				for {
					status := lse.GetReport()

					// commit ok
					if status.KnownHighWatermark == highWatermark {
						c <- nil
						return
					}

					// bad lse
					if status.KnownHighWatermark > highWatermark {
						c <- errors.New("bad LSE status")
						return
					}

					// no written entry
					if status.UncommittedLLSNLength < 1 {
						time.Sleep(time.Millisecond)
						continue
					}

					// send commit
					lse.Commit(context.TODO(), CommittedLogStreamStatus{
						LogStreamID:         logStreamID,
						HighWatermark:       highWatermark,
						PrevHighWatermark:   prevHighWatermark,
						CommittedGLSNOffset: committedGLSNOffset,
						CommittedGLSNLength: committedGLSNLength,
					})
				}
			}()
			return c
		}

		for hwm := types.MinGLSN; hwm <= repeat; hwm++ {
			expectedData := []byte(fmt.Sprintf("log-%03d", hwm))
			// Trim future GLSN
			err = lse.Trim(context.TODO(), hwm)
			So(err, ShouldNotBeNil)

			// Append
			errC := waitForCommitted(hwm, hwm-1, hwm, 1)
			glsn, err := lse.Append(context.TODO(), expectedData)
			So(err, ShouldBeNil)
			So(glsn, ShouldEqual, hwm)
			So(<-errC, ShouldBeNil)

			// Read
			actualLogEntry, err := lse.Read(context.TODO(), hwm)
			So(err, ShouldBeNil)
			So(expectedData, ShouldResemble, actualLogEntry.Data)
		}

		_, err = lse.Subscribe(context.TODO(), types.MinGLSN+repeat, types.MinGLSN+repeat+1)
		So(errors.Is(err, verrors.ErrUndecidable), ShouldBeTrue)

		// Subscribe
		mid := types.GLSN(repeat / 2)
		subC, err := lse.Subscribe(context.TODO(), types.MinGLSN, types.MinGLSN+mid)
		So(err, ShouldBeNil)

		for expectedGLSN := types.MinGLSN; expectedGLSN < types.MinGLSN+mid; expectedGLSN++ {
			sub := <-subC
			zap.L().Debug("scanned", zap.Any("result", sub))
			So(sub.Err, ShouldBeNil)
			So(sub.LogEntry.GLSN, ShouldEqual, expectedGLSN)
			So(sub.LogEntry.LLSN, ShouldEqual, types.LLSN(expectedGLSN))
		}
		So((<-subC).Err, ShouldEqual, ErrEndOfRange)
		testutil.CompareWait(func() bool {
			_, more := <-subC
			return !more

		}, time.Minute)

		// Trim
		_, err = lse.Read(context.TODO(), 3)
		So(err, ShouldBeNil)
		err = lse.Trim(context.TODO(), 3)
		So(err, ShouldBeNil)
		// Trim is async, so wait until it is complete
		testutil.CompareWait(func() bool {
			_, err = lse.Read(context.TODO(), 3)
			return err != nil
		}, time.Minute)
		err = lse.Trim(context.TODO(), 3)
		So(err, ShouldBeNil)

		// Subscribe trimmed range
		_, err = lse.Subscribe(context.TODO(), types.MinGLSN, 4)
		So(errors.Is(err, verrors.ErrTrimmed), ShouldBeTrue)

		// Now, no appending
		So(lse.GetReport().UncommittedLLSNLength, ShouldEqual, 0)

		// 3 written, but not committed failed logs
		cctx, ccancel := context.WithCancel(context.TODO())
		defer ccancel()
		var wgClient sync.WaitGroup
		stoppedClient := int32(0)
		stoppedClientRetC := make(chan struct {
			glsn types.GLSN
			err  error
		}, 3)
		for i := 0; i < 3; i++ {
			wgClient.Add(1)
			go func(i int) {
				defer wgClient.Done()
				data := []byte(fmt.Sprintf("uncommitted-%03d", i))
				glsn, err := lse.Append(cctx, data)
				stoppedClientRetC <- struct {
					glsn types.GLSN
					err  error
				}{
					glsn: glsn,
					err:  err,
				}
				atomic.AddInt32(&stoppedClient, 1)
			}(i)
		}
		for {
			status := lse.GetReport()
			if status.UncommittedLLSNLength == 3 {
				break
			}
			time.Sleep(time.Millisecond)
		}

		// Client status check: clients are stuck with append request
		So(atomic.LoadInt32(&stoppedClient), ShouldEqual, 0)

		// LSE status check: 3 written/not-committed logs
		// NOTE (jun): When clients cancel their append request, the LSE doesn't change
		// status, it is still LogStreamStatusRunning.
		So(lse.Status(), ShouldEqual, varlogpb.LogStreamStatusRunning)
		So(lse.GetReport().UncommittedLLSNLength, ShouldEqual, 3)

		// LSE
		// GLSN(4), GLSN(5), GLSN(6), ..., GLSN(repeat)
		// LLSN(4), LLSN(5), LLSN(6), ..., LLSN(repeat), LLSN(repeat+1), LLSN(repeat+2), LLSN(repeat+3)
		// FIXME (jun): This is a very bad assertion: use public or interface method to
		// check storage status.
		written := stg.(*InMemoryStorage).written
		So(written[len(written)-1].llsn, ShouldEqual, repeat+3)

		Convey("Sealing the LS with InvalidGLSN", func() {
			So(func() { lse.Seal(types.InvalidGLSN) }, ShouldPanic)
			ccancel()
		})

		Convey("MR is behind of LSE", func() {
			So(func() { lse.Seal(repeat - 1) }, ShouldPanic)
			ccancel()
		})

		Convey("MR is ahead of LSE", func() {
			status, sealedGLSN := lse.Seal(repeat + 1)
			So(status, ShouldEqual, varlogpb.LogStreamStatusSealing)
			So(sealedGLSN, ShouldEqual, repeat)

			// FIXME (jun): See above.
			// LogStreamStatusSealing can't delete uncommitted logs.
			written = stg.(*InMemoryStorage).written
			So(written[len(written)-1].llsn, ShouldEqual, repeat+3)

			// LogStreamStatusSealing can't unseal
			err = lse.Unseal()
			So(err, ShouldNotBeNil)
			So(lse.Status(), ShouldEqual, varlogpb.LogStreamStatusSealing)

			errC := waitForCommitted(repeat+1, repeat, repeat+1, 1)
			So(<-errC, ShouldBeNil)

			// append request of 1 client succeeds.
			testutil.CompareWait(func() bool {
				return atomic.LoadInt32(&stoppedClient) == 1
			}, time.Minute)

			stoppedClientRet := <-stoppedClientRetC
			So(stoppedClientRet.err, ShouldBeNil)
			So(stoppedClientRet.glsn, ShouldEqual, repeat+1)

			status, sealedGLSN = lse.Seal(repeat + 1)
			So(status, ShouldEqual, varlogpb.LogStreamStatusSealed)
			So(sealedGLSN, ShouldEqual, repeat+1)

			// wait for appending clients to fail
			testutil.CompareWait(func() bool {
				return atomic.LoadInt32(&stoppedClient) == 3
			}, time.Minute)

			close(stoppedClientRetC)
			for stoppedClientRet := range stoppedClientRetC {
				So(stoppedClientRet.err, ShouldNotBeNil)
			}

			// append after sealing is failed.
			_, err := lse.Append(context.TODO(), []byte("never"))
			So(err, ShouldNotBeNil)

			// LogStreamStatusSealed can unseal
			err = lse.Unseal()
			So(err, ShouldBeNil)
			So(lse.Status(), ShouldEqual, varlogpb.LogStreamStatusRunning)
		})

		Convey("MR and LSE are on the same line", func() {
			status, sealedGLSN := lse.Seal(repeat)
			So(status, ShouldEqual, varlogpb.LogStreamStatusSealed)
			So(sealedGLSN, ShouldEqual, repeat)

			// FIXME (jun): See above.
			// LogStreamStatusSealed can delete uncommitted logs.
			written = stg.(*InMemoryStorage).written
			So(written[len(written)-1].llsn, ShouldEqual, repeat)

			// wait for appending clients to fail
			testutil.CompareWait(func() bool {
				return atomic.LoadInt32(&stoppedClient) == 3
			}, time.Minute)

			close(stoppedClientRetC)
			for stoppedClientRet := range stoppedClientRetC {
				So(stoppedClientRet.err, ShouldNotBeNil)
			}

			// append after sealing is failed.
			_, err := lse.Append(context.TODO(), []byte("never"))
			So(err, ShouldNotBeNil)

			// LogStreamStatusSealing can unseal
			err = lse.Unseal()
			So(err, ShouldBeNil)
			So(lse.Status(), ShouldEqual, varlogpb.LogStreamStatusRunning)
		})

		wgClient.Wait()
	})
}

func TestLogStreamExecutorCommit(t *testing.T) {
	Convey("Given a LogStreamExecutor", t, func() {
		const (
			logStreamID = types.LogStreamID(1)
		)

		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		storage := NewMockStorage(ctrl)
		storage.EXPECT().Close().Return(nil).AnyTimes()
		storage.EXPECT().RestoreLogStreamContext(gomock.Any()).Return(false)
		storage.EXPECT().RestoreStorage(gomock.Any(), gomock.Any())
		options := DefaultLogStreamExecutorOptions()
		lse, err := NewLogStreamExecutor(
			zap.L(),
			logStreamID,
			storage,
			newNopTelmetryStub(),
			&options,
		)
		So(err, ShouldBeNil)
		So(lse.Run(context.TODO()), ShouldBeNil)

		Reset(func() {
			lse.Close()
		})

		Convey("When empty commit result comes", func() {
			cr := CommittedLogStreamStatus{
				LogStreamID:         logStreamID,
				HighWatermark:       types.GLSN(10),
				PrevHighWatermark:   types.GLSN(0),
				CommittedGLSNOffset: types.GLSN(1),
				CommittedGLSNLength: 0,
			}

			Convey("Then CommitContext should be saved", func() {
				called := make(chan bool, 1)
				storage.EXPECT().StoreCommitContext(gomock.Any()).DoAndReturn(func(_ CommitContext) error {
					defer close(called)
					called <- true
					return nil
				})

				lse.Commit(context.TODO(), cr)
				So(<-called, ShouldBeTrue)
			})

		})

	})
}
