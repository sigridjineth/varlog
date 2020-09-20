package main

import (
	"context"
	"testing"
	"time"

	"github.com/kakao/varlog/internal/metadata_repository"
	"github.com/kakao/varlog/pkg/varlog"
	"github.com/kakao/varlog/pkg/varlog/types"
	"github.com/kakao/varlog/pkg/varlog/util/runner"
	"github.com/kakao/varlog/pkg/varlog/util/testutil"
	varlogpb "github.com/kakao/varlog/proto/varlog"
	"go.uber.org/zap"

	. "github.com/smartystreets/goconvey/convey"
)

func TestMetadataRepositoryClientSimpleRegister(t *testing.T) {
	Convey("Given Varlog cluster", t, func(ctx C) {
		opts := VarlogClusterOptions{
			NrMR:              1,
			NrRep:             1,
			ReporterClientFac: metadata_repository.NewEmptyReporterClientFactory(),
		}
		env := NewVarlogCluster(opts)
		env.Start()
		defer env.Close()

		So(testutil.CompareWait(func() bool {
			return env.LeaderElected()
		}, time.Second), ShouldBeTrue)

		mr := env.MRs[0]
		addr := mr.GetServerAddr()

		cli, err := varlog.NewMetadataRepositoryClient(addr)
		So(err, ShouldBeNil)
		defer cli.Close()

		Convey("When register storage node by CLI", func(ctx C) {
			snId := types.StorageNodeID(time.Now().UnixNano())

			s := &varlogpb.StorageDescriptor{
				Path:  "test",
				Used:  0,
				Total: 100,
			}
			sn := &varlogpb.StorageNodeDescriptor{
				StorageNodeID: snId,
				Address:       "localhost",
			}
			sn.Storages = append(sn.Storages, s)

			err = cli.RegisterStorageNode(context.TODO(), sn)
			So(err, ShouldBeNil)

			Convey("Then getting storage node info from metadata should be success", func(ctx C) {
				meta, err := cli.GetMetadata(context.TODO())
				So(err, ShouldBeNil)
				So(meta.GetStorageNode(snId), ShouldNotEqual, nil)
			})
		})
	})
}

func TestVarlogRegisterStorageNode(t *testing.T) {
	Convey("Given Varlog cluster", t, func(ctx C) {
		opts := VarlogClusterOptions{
			NrMR:              1,
			NrRep:             1,
			ReporterClientFac: metadata_repository.NewReporterClientFactory(),
		}
		env := NewVarlogCluster(opts)
		env.Start()
		defer env.Close()

		So(testutil.CompareWait(func() bool {
			return env.LeaderElected()
		}, time.Second), ShouldBeTrue)

		mr := env.GetMR()

		Convey("When register SN & LS", func(ctx C) {
			_, err := env.AddSN()
			So(err, ShouldBeNil)

			_, err = env.AddLS()
			So(err, ShouldBeNil)

			meta, err := mr.GetMetadata(context.TODO())
			So(err, ShouldBeNil)
			So(meta, ShouldNotBeNil)
			So(len(meta.StorageNodes), ShouldBeGreaterThan, 0)
			So(len(meta.LogStreams), ShouldBeGreaterThan, 0)

			Convey("Then nrReport of MR should be updated", func(ctx C) {
				So(testutil.CompareWait(func() bool {
					return mr.GetReportCount() > 0
				}, time.Second), ShouldBeTrue)
			})
		})
	})
}

func TestVarlogAppendLog(t *testing.T) {
	Convey("Given Varlog cluster", t, func(ctx C) {
		opts := VarlogClusterOptions{
			NrMR:              1,
			NrRep:             1,
			ReporterClientFac: metadata_repository.NewReporterClientFactory(),
		}
		env := NewVarlogCluster(opts)
		env.Start()
		defer env.Close()

		So(testutil.CompareWait(func() bool {
			return env.LeaderElected()
		}, time.Second), ShouldBeTrue)

		mr := env.GetMR()

		_, err := env.AddSN()
		So(err, ShouldBeNil)

		lsID, err := env.AddLS()
		So(err, ShouldBeNil)

		meta, _ := mr.GetMetadata(context.TODO())
		So(meta, ShouldNotBeNil)

		Convey("When Append log entry to SN", func(ctx C) {
			cli, err := env.NewLogIOClient(lsID)
			So(err, ShouldBeNil)
			defer cli.Close()

			Convey("Then it should return valid GLSN", func(ctx C) {
				for i := 0; i < 100; i++ {
					glsn, err := cli.Append(context.TODO(), lsID, []byte("foo"))
					So(err, ShouldBeNil)
					So(glsn, ShouldEqual, types.GLSN(i+1))
				}
			})
		})
	})
}

func TestVarlogReplicateLog(t *testing.T) {
	nrRep := 2
	Convey("Given Varlog cluster", t, func(ctx C) {
		opts := VarlogClusterOptions{
			NrMR:              1,
			NrRep:             nrRep,
			ReporterClientFac: metadata_repository.NewReporterClientFactory(),
		}
		env := NewVarlogCluster(opts)
		env.Start()
		defer env.Close()

		So(testutil.CompareWait(func() bool {
			return env.LeaderElected()
		}, time.Second), ShouldBeTrue)

		mr := env.GetMR()

		for i := 0; i < nrRep; i++ {
			_, err := env.AddSN()
			So(err, ShouldBeNil)
		}

		lsID, err := env.AddLS()
		So(err, ShouldBeNil)

		meta, _ := mr.GetMetadata(context.TODO())
		So(meta, ShouldNotBeNil)

		Convey("When Append log entry to SN", func(ctx C) {
			cli, err := env.NewLogIOClient(lsID)
			So(err, ShouldBeNil)
			defer cli.Close()

			ls := meta.LogStreams[0]

			var backups []varlog.StorageNode
			backups = make([]varlog.StorageNode, nrRep-1)
			for i := 1; i < nrRep; i++ {
				replicaID := ls.Replicas[i].StorageNodeID
				replica := meta.GetStorageNode(replicaID)
				So(replica, ShouldNotBeNil)

				backups[i-1].ID = replicaID
				backups[i-1].Addr = replica.Address
			}

			Convey("Then it should return valid GLSN", func(ctx C) {
				for i := 0; i < 100; i++ {
					rctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
					defer cancel()

					glsn, err := cli.Append(rctx, lsID, []byte("foo"), backups...)
					So(err, ShouldBeNil)
					So(glsn, ShouldEqual, types.GLSN(i+1))
				}
			})
		})
	})
}

func TestVarlogSeal(t *testing.T) {
	Convey("Given Varlog cluster", t, func(ctx C) {
		opts := VarlogClusterOptions{
			NrMR:              1,
			NrRep:             1,
			ReporterClientFac: metadata_repository.NewReporterClientFactory(),
		}
		env := NewVarlogCluster(opts)
		env.Start()
		defer env.Close()

		So(testutil.CompareWait(func() bool {
			return env.LeaderElected()
		}, time.Second), ShouldBeTrue)

		mr := env.GetMR()

		snID, err := env.AddSN()
		So(err, ShouldBeNil)

		sn := env.LookupSN(snID)
		So(sn, ShouldNotBeNil)

		lsID, err := env.AddLS()
		So(err, ShouldBeNil)

		meta, _ := mr.GetMetadata(context.TODO())
		So(meta, ShouldNotBeNil)

		Convey("When seal LS", func(ctx C) {
			errC := make(chan error, 1024)
			glsnC := make(chan types.GLSN, 1024)

			appendf := func(actx context.Context) {
				cli, err := env.NewLogIOClient(lsID)
				if err != nil {
					errC <- err
					return
				}
				defer cli.Close()

				for {
					select {
					case <-actx.Done():
						return
					default:
						rctx, cancel := context.WithTimeout(actx, time.Second)
						glsn, err := cli.Append(rctx, lsID, []byte("foo"))
						cancel()
						if err != nil {
							if err != context.DeadlineExceeded {
								errC <- err
							}
						} else {
							glsnC <- glsn
						}
					}
				}
			}

			runner := runner.New("seal-test", zap.NewNop())
			defer runner.Stop()

			for i := 0; i < 5; i++ {
				runner.Run(appendf)
			}

			sealedGLSN := types.InvalidGLSN
		Loop:
			for {
				select {
				case glsn := <-glsnC:
					if sealedGLSN.Invalid() && glsn > types.GLSN(10) {
						sealedGLSN, err = mr.Seal(context.TODO(), lsID)
						So(err, ShouldBeNil)
					}

					if !sealedGLSN.Invalid() {
						status, hwm, err := sn.Seal(env.ClusterID, snID, lsID, sealedGLSN)
						So(err, ShouldBeNil)
						if status == varlogpb.LogStreamStatusSealing {
							continue Loop
						}

						So(hwm, ShouldEqual, sealedGLSN)
						break Loop
					}
				case err := <-errC:
					So(err, ShouldEqual, varlog.ErrSealed)
				}
			}

			Convey("Then it should be able to read all logs which is lower than sealedGLSN", func(ctx C) {
				cli, err := env.NewLogIOClient(lsID)
				So(err, ShouldBeNil)
				defer cli.Close()

				for glsn := types.MinGLSN; glsn <= sealedGLSN; glsn += types.GLSN(1) {
					_, err := cli.Read(context.TODO(), lsID, glsn)
					So(err, ShouldBeNil)
				}
			})
		})
	})
}

func TestVarlogTrimGLS(t *testing.T) {
	Convey("Given Varlog cluster", t, func(ctx C) {
		nrLS := 2
		opts := VarlogClusterOptions{
			NrMR:              1,
			NrRep:             1,
			SnapCount:         10,
			ReporterClientFac: metadata_repository.NewReporterClientFactory(),
		}
		env := NewVarlogCluster(opts)
		env.Start()
		defer env.Close()

		So(testutil.CompareWait(func() bool {
			return env.LeaderElected()
		}, time.Second), ShouldBeTrue)

		mr := env.GetMR()

		_, err := env.AddSN()
		So(err, ShouldBeNil)

		var lsIDs []types.LogStreamID
		for i := 0; i < nrLS; i++ {
			lsID, err := env.AddLS()
			So(err, ShouldBeNil)

			lsIDs = append(lsIDs, lsID)
		}

		meta, _ := mr.GetMetadata(context.TODO())
		So(meta, ShouldNotBeNil)

		Convey("When Append Log", func(ctx C) {
			cli, err := env.NewLogIOClient(lsIDs[0])
			So(err, ShouldBeNil)
			defer cli.Close()

			glsn := types.InvalidGLSN
			for i := 0; i < 10; i++ {
				rctx, cancel := context.WithTimeout(context.TODO(), time.Second)
				defer cancel()
				glsn, _ = cli.Append(rctx, lsIDs[0], []byte("foo"))
				So(glsn, ShouldNotEqual, types.InvalidGLSN)
			}

			for i := 0; i < 10; i++ {
				rctx, cancel := context.WithTimeout(context.TODO(), time.Second)
				defer cancel()
				glsn, _ = cli.Append(rctx, lsIDs[1], []byte("foo"))
				So(glsn, ShouldNotEqual, types.InvalidGLSN)
			}

			hwm := mr.GetHighWatermark()
			So(hwm, ShouldEqual, glsn)

			Convey("Then GLS history of MR should be trimmed", func(ctx C) {
				So(testutil.CompareWait(func() bool {
					return mr.GetMinHighWatermark() == hwm-types.GLSN(1)
				}, 5*time.Second), ShouldBeTrue)
			})
		})
	})
}

func TestVarlogTrimGLSWithSealedLS(t *testing.T) {
	Convey("Given Varlog cluster", t, func(ctx C) {
		nrLS := 2
		opts := VarlogClusterOptions{
			NrMR:              1,
			NrRep:             1,
			SnapCount:         10,
			ReporterClientFac: metadata_repository.NewReporterClientFactory(),
		}
		env := NewVarlogCluster(opts)
		env.Start()
		defer env.Close()

		So(testutil.CompareWait(func() bool {
			return env.LeaderElected()
		}, time.Second), ShouldBeTrue)

		snID, err := env.AddSN()
		So(err, ShouldBeNil)

		sn := env.LookupSN(snID)
		So(sn, ShouldNotBeNil)

		var lsIDs []types.LogStreamID
		for i := 0; i < nrLS; i++ {
			lsID, err := env.AddLS()
			So(err, ShouldBeNil)

			lsIDs = append(lsIDs, lsID)
		}

		mr := env.GetMR()
		meta, _ := mr.GetMetadata(context.TODO())
		So(meta, ShouldNotBeNil)

		Convey("When Append Log", func(ctx C) {
			cli, err := env.NewLogIOClient(lsIDs[0])
			So(err, ShouldBeNil)
			defer cli.Close()

			glsn := types.InvalidGLSN

			for i := 0; i < 32; i++ {
				rctx, cancel := context.WithTimeout(context.TODO(), time.Second)
				defer cancel()
				glsn, _ = cli.Append(rctx, lsIDs[i%nrLS], []byte("foo"))
				So(glsn, ShouldNotEqual, types.InvalidGLSN)
			}

			sealedLS := meta.LogStreams[0]
			runningLS := meta.LogStreams[1]

			sealedGLSN, err := mr.Seal(context.TODO(), sealedLS.LogStreamID)
			So(err, ShouldBeNil)

			_, _, err = sn.Seal(env.ClusterID, snID, sealedLS.LogStreamID, sealedGLSN)
			So(err, ShouldBeNil)

			for i := 0; i < 10; i++ {
				rctx, cancel := context.WithTimeout(context.TODO(), time.Second)
				defer cancel()
				glsn, err = cli.Append(rctx, runningLS.LogStreamID, []byte("foo"))
				So(err, ShouldBeNil)
				So(glsn, ShouldNotEqual, types.InvalidGLSN)
			}

			hwm := mr.GetHighWatermark()
			So(hwm, ShouldEqual, glsn)

			Convey("Then GLS history of MR should be trimmed", func(ctx C) {
				So(testutil.CompareWait(func() bool {
					return mr.GetMinHighWatermark() == hwm-types.GLSN(1)
				}, 5*time.Second), ShouldBeTrue)
			})
		})
	})
}

func TestVarlogFailoverMRLeaderFail(t *testing.T) {
	Convey("Given Varlog cluster", t, func(ctx C) {
		opts := VarlogClusterOptions{
			NrMR:              3,
			NrRep:             1,
			ReporterClientFac: metadata_repository.NewReporterClientFactory(),
		}
		env := NewVarlogCluster(opts)
		env.Start()
		defer env.Close()

		So(testutil.CompareWait(func() bool {
			return env.LeaderElected()
		}, time.Second), ShouldBeTrue)

		_, err := env.AddSN()
		So(err, ShouldBeNil)

		lsID, err := env.AddLS()
		So(err, ShouldBeNil)

		errC := make(chan error, 1024)
		glsnC := make(chan types.GLSN, 1024)

		appendf := func(actx context.Context) {
			cli, err := env.NewLogIOClient(lsID)
			if err != nil {
				errC <- err
				return
			}
			defer cli.Close()

			for {
				select {
				case <-actx.Done():
					return
				default:
					rctx, cancel := context.WithTimeout(actx, 5*time.Second)
					glsn, err := cli.Append(rctx, lsID, []byte("foo"))
					cancel()
					if err != nil {
						if err != context.DeadlineExceeded {
							errC <- err
						}
					} else {
						glsnC <- glsn
					}
				}
			}
		}

		runner := runner.New("failover-leader-test", zap.NewNop())
		defer runner.Stop()

		for i := 0; i < 5; i++ {
			runner.Run(appendf)
		}

		Convey("When MR leader fail", func(ctx C) {
			maxGLSN := types.InvalidGLSN
			triggerGLSN := types.GLSN(20)
			goalGLSN := types.GLSN(3) * triggerGLSN
			stopped := false
		Loop:
			for {
				select {
				case glsn := <-glsnC:
					if maxGLSN < glsn {
						maxGLSN = glsn
					}
					if !stopped && maxGLSN > triggerGLSN {
						So(env.LeaderFail(), ShouldBeTrue)
						stopped = true
					} else if maxGLSN > goalGLSN {
						break Loop
					}
				case err := <-errC:
					So(err, ShouldBeNil)
				}
			}

			Convey("Then it should be able to keep appending log", func(ctx C) {
				cli, err := env.NewLogIOClient(lsID)
				So(err, ShouldBeNil)
				defer cli.Close()

				for glsn := types.MinGLSN; glsn <= maxGLSN; glsn += types.GLSN(1) {
					_, err := cli.Read(context.TODO(), lsID, glsn)
					So(err, ShouldBeNil)
				}
			})
		})
	})
}

func TestVarlogFailoverSNBackupFail(t *testing.T) {
	Convey("Given Varlog cluster", t, func(ctx C) {
		nrRep := 2
		nrCli := 5
		opts := VarlogClusterOptions{
			NrMR:              1,
			NrRep:             nrRep,
			ReporterClientFac: metadata_repository.NewReporterClientFactory(),
		}
		env := NewVarlogCluster(opts)
		env.Start()
		defer env.Close()

		So(testutil.CompareWait(func() bool {
			return env.LeaderElected()
		}, time.Second), ShouldBeTrue)

		for i := 0; i < nrRep; i++ {
			_, err := env.AddSN()
			So(err, ShouldBeNil)
		}

		lsID, err := env.AddLS()
		So(err, ShouldBeNil)

		meta, _ := env.GetMR().GetMetadata(context.TODO())
		So(meta, ShouldNotBeNil)

		ls := meta.LogStreams[0]

		errC := make(chan error, 1024)
		glsnC := make(chan types.GLSN, 1024)

		appendf := func(actx context.Context) {
			cli, err := env.NewLogIOClient(lsID)
			if err != nil {
				errC <- err
				return
			}
			defer cli.Close()

			var backups []varlog.StorageNode
			backups = make([]varlog.StorageNode, nrRep-1)
			for i := 1; i < nrRep; i++ {
				replicaID := ls.Replicas[i].StorageNodeID
				replica := meta.GetStorageNode(replicaID)

				backups[i-1].ID = replicaID
				backups[i-1].Addr = replica.Address
			}

			for {
				select {
				case <-actx.Done():
					return
				default:
					rctx, cancel := context.WithTimeout(actx, time.Second)
					glsn, err := cli.Append(rctx, lsID, []byte("foo"), backups...)
					cancel()
					if err != nil {
						errC <- err
						return
					} else {
						glsnC <- glsn
					}
				}
			}
		}

		runner := runner.New("backup-fail-test", zap.NewNop())
		defer func() {
			runner.Stop()
		}()

		for i := 0; i < nrCli; i++ {
			runner.Run(appendf)
		}

		Convey("When backup SN fail", func(ctx C) {
			errCnt := 0
			maxGLSN := types.InvalidGLSN
		Loop:
			for {
				select {
				case glsn := <-glsnC:
					if maxGLSN < glsn {
						maxGLSN = glsn
					}

					if glsn == types.GLSN(32) {
						sn, _ := env.GetBackupSN(lsID)
						sn.Close()
					}
				case <-errC:
					errCnt++
					if errCnt == nrCli {
						break Loop
					}
				}
			}

			Convey("Then it should not be able to append", func(ctx C) {
				sealedGLSN, _ := env.GetMR().Seal(context.TODO(), lsID)
				So(sealedGLSN, ShouldBeGreaterThanOrEqualTo, maxGLSN)

				psn, _ := env.GetPrimarySN(lsID)
				So(psn, ShouldNotBeNil)

				So(testutil.CompareWait(func() bool {
					status, _, _ := psn.Seal(env.ClusterID, ls.Replicas[0].StorageNodeID, lsID, sealedGLSN)
					return status == varlogpb.LogStreamStatusSealed
				}, 5*time.Second), ShouldBeTrue)

				_, hwm, _ := psn.Seal(env.ClusterID, ls.Replicas[0].StorageNodeID, lsID, sealedGLSN)
				So(hwm, ShouldEqual, sealedGLSN)
			})
		})
	})
}
