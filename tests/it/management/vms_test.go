package management

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/gogo/protobuf/proto"
	. "github.com/smartystreets/goconvey/convey"
	"go.uber.org/zap"

	"github.com/kakao/varlog/internal/metarepos"
	"github.com/kakao/varlog/internal/storagenode"
	"github.com/kakao/varlog/internal/varlogadm"
	"github.com/kakao/varlog/pkg/mrc"
	"github.com/kakao/varlog/pkg/types"
	"github.com/kakao/varlog/pkg/util/testutil"
	"github.com/kakao/varlog/pkg/util/testutil/ports"
	"github.com/kakao/varlog/pkg/verrors"
	"github.com/kakao/varlog/proto/snpb"
	"github.com/kakao/varlog/proto/varlogpb"
	"github.com/kakao/varlog/tests/it"
	"github.com/kakao/varlog/vtesting"
)

// FIXME: This test checks MRManager, move unit test or something similar.
func TestVarlogNewMRManager(t *testing.T) {
	Convey("Given that MRManager runs without any running MR", t, func(c C) {
		const (
			clusterID = types.ClusterID(1)
			portBase  = 20000
		)

		portLease, err := ports.ReserveWeaklyWithRetry(portBase)
		So(err, ShouldBeNil)

		var (
			mrRAFTAddr = fmt.Sprintf("http://127.0.0.1:%d", portLease.Base())
			mrRPCAddr  = fmt.Sprintf("127.0.0.1:%d", portLease.Base()+1)
		)

		defer func() {
			So(portLease.Release(), ShouldBeNil)
		}()

		vmsOpts := varlogadm.DefaultOptions()

		Convey("When MR does not start within specified periods", func(c C) {
			vmsOpts.InitialMRConnRetryCount = 1
			vmsOpts.InitialMRConnRetryBackoff = 100 * time.Millisecond
			vmsOpts.MetadataRepositoryAddresses = []string{mrRPCAddr}

			Convey("Then the MRManager should return an error", func(ctx C) {
				_, err := varlogadm.NewMRManager(context.TODO(), clusterID, vmsOpts.MRManagerOptions, zap.L())
				So(err, ShouldNotBeNil)
			})
		})

		Convey("When MR starts within specified periods", func(c C) {
			vmsOpts.InitialMRConnRetryCount = 30
			vmsOpts.InitialMRConnRetryBackoff = 1 * time.Second
			vmsOpts.MetadataRepositoryAddresses = []string{mrRPCAddr}

			// vms
			mrmC := make(chan varlogadm.MetadataRepositoryManager, 1)
			go func() {
				defer close(mrmC)
				mrm, err := varlogadm.NewMRManager(context.TODO(), clusterID, vmsOpts.MRManagerOptions, zap.L())
				if err != nil {
					fmt.Printf("err: %+v\n", err)
				}
				c.So(err, ShouldBeNil)
				if err == nil {
					mrmC <- mrm
				}
			}()

			time.Sleep(10 * time.Millisecond)

			// mr
			mrOpts := &metarepos.MetadataRepositoryOptions{
				RaftOptions: metarepos.RaftOptions{
					Join:      false,
					SnapCount: uint64(10),
					RaftTick:  vtesting.TestRaftTick(),
					RaftDir:   filepath.Join(t.TempDir(), "raftdir"),
					Peers:     []string{mrRAFTAddr},
				},

				ClusterID:                      clusterID,
				RaftAddress:                    mrRAFTAddr,
				LogDir:                         filepath.Join(t.TempDir(), "log"),
				RPCTimeout:                     vtesting.TimeoutAccordingToProcCnt(metarepos.DefaultRPCTimeout),
				NumRep:                         1,
				RPCBindAddress:                 mrRPCAddr,
				ReporterClientFac:              metarepos.NewReporterClientFactory(),
				StorageNodeManagementClientFac: metarepos.NewEmptyStorageNodeClientFactory(),
				Logger:                         zap.L(),
			}
			mr := metarepos.NewRaftMetadataRepository(mrOpts)
			mr.Run()

			Reset(func() {
				So(mr.Close(), ShouldBeNil)
			})

			Convey("Then the MRManager should connect to the MR", func(ctx C) {
				mrm, ok := <-mrmC
				So(ok, ShouldBeTrue)
				So(mrm.Close(), ShouldBeNil)
			})
		})

	})

	Convey("Given MR cluster", t, func(ctx C) {
		vmsOpts := varlogadm.DefaultOptions()
		vmsOpts.InitialMRConnRetryCount = 3
		env := it.NewVarlogCluster(t, it.WithVMSOptions(vmsOpts))
		defer env.Close(t)

		mr := env.GetMR(t)
		So(testutil.CompareWaitN(50, func() bool {
			return mr.GetServerAddr() != ""
		}), ShouldBeTrue)
		mrAddr := mr.GetServerAddr()

		Convey("When create MRManager with non addrs", func(ctx C) {
			vmsOpts.MRManagerOptions.MetadataRepositoryAddresses = nil
			// VMS Server
			_, err := varlogadm.NewMRManager(context.TODO(), types.ClusterID(0), vmsOpts.MRManagerOptions, zap.L())
			Convey("Then it should be fail", func(ctx C) {
				So(err, ShouldNotBeNil)
			})
		})

		Convey("When create MRManager with invalid addrs", func(ctx C) {
			// VMS Server
			vmsOpts.MRManagerOptions.MetadataRepositoryAddresses = []string{fmt.Sprintf("%s%d", mrAddr, 0)}
			_, err := varlogadm.NewMRManager(context.TODO(), types.ClusterID(0), vmsOpts.MRManagerOptions, zap.L())
			Convey("Then it should be fail", func(ctx C) {
				So(err, ShouldNotBeNil)
			})
		})

		Convey("When create MRManager with valid addrs", func(ctx C) {
			// VMS Server
			vmsOpts.MRManagerOptions.MetadataRepositoryAddresses = []string{mrAddr}
			mrm, err := varlogadm.NewMRManager(context.TODO(), types.ClusterID(0), vmsOpts.MRManagerOptions, zap.L())
			Convey("Then it should be success", func(ctx C) {
				So(err, ShouldBeNil)

				Convey("and it should work", func(ctx C) {
					cinfo, err := mrm.GetClusterInfo(context.TODO())
					So(err, ShouldBeNil)
					So(len(cinfo.GetMembers()), ShouldEqual, 1)
				})
			})
			defer mrm.Close()
		})
	})
}

func TestVarlogMRManagerWithLeavedNode(t *testing.T) {
	t.Skip()
	Convey("Given MR cluster", t, func(ctx C) {
		vmsOpts := it.NewTestVMSOptions()
		env := it.NewVarlogCluster(t,
			it.WithReporterClientFactory(metarepos.NewReporterClientFactory()),
			it.WithStorageNodeManagementClientFactory(metarepos.NewEmptyStorageNodeClientFactory()),
			it.WithVMSOptions(vmsOpts),
		)
		defer env.Close(t)

		mr := env.GetMR(t)
		So(testutil.CompareWaitN(50, func() bool {
			return mr.GetServerAddr() != ""
		}), ShouldBeTrue)
		mrAddr := mr.GetServerAddr()

		// VMS Server
		vmsOpts.MRManagerOptions.MetadataRepositoryAddresses = []string{mrAddr}
		mrm, err := varlogadm.NewMRManager(context.TODO(), types.ClusterID(0), vmsOpts.MRManagerOptions, zap.L())
		So(err, ShouldBeNil)
		defer mrm.Close()

		Convey("When all the cluster configuration nodes are changed", func(ctx C) {
			env.AppendMR(t)
			env.StartMR(t, 1)

			nmr := env.GetMRByIndex(t, 1)

			rctx, cancel := context.WithTimeout(context.Background(), vtesting.TimeoutUnitTimesFactor(50))
			defer cancel()

			err = mrm.AddPeer(rctx, env.MetadataRepositoryIDAt(t, 1), env.MRPeerAtIndex(t, 1), env.MRRPCEndpointAtIndex(t, 1))
			So(err, ShouldBeNil)

			So(testutil.CompareWaitN(50, func() bool {
				return nmr.IsMember()
			}), ShouldBeTrue)

			rctx, cancel = context.WithTimeout(context.Background(), vtesting.TimeoutUnitTimesFactor(50))
			defer cancel()

			err = mrm.RemovePeer(rctx, env.MetadataRepositoryIDAt(t, 0))
			So(err, ShouldBeNil)

			So(testutil.CompareWaitN(50, func() bool {
				return !mr.IsMember()
			}), ShouldBeTrue)

			oldCL, err := mrc.NewMetadataRepositoryManagementClient(context.TODO(), mrAddr)
			So(err, ShouldBeNil)
			_, err = oldCL.GetClusterInfo(context.TODO(), types.ClusterID(0))
			So(errors.Is(err, verrors.ErrNotMember), ShouldBeTrue)
			So(oldCL.Close(), ShouldBeNil)

			So(testutil.CompareWaitN(50, func() bool {
				cinfo, err := nmr.GetClusterInfo(context.TODO(), types.ClusterID(0))
				return err == nil && len(cinfo.GetMembers()) == 1 && cinfo.GetLeader() == cinfo.GetNodeID()
			}), ShouldBeTrue)

			Convey("Then it should be success", func(ctx C) {
				rctx, cancel := context.WithTimeout(context.Background(), vtesting.TimeoutUnitTimesFactor(50))
				defer cancel()

				cinfo, err := mrm.GetClusterInfo(rctx)
				So(err, ShouldBeNil)
				So(len(cinfo.GetMembers()), ShouldEqual, 1)
			})
		})
	})
}

type testSnHandler struct {
	hbC     chan types.StorageNodeID
	reportC chan *snpb.StorageNodeMetadataDescriptor
}

func newTestSnHandler() *testSnHandler {
	return &testSnHandler{
		hbC:     make(chan types.StorageNodeID),
		reportC: make(chan *snpb.StorageNodeMetadataDescriptor),
	}
}

func (sh *testSnHandler) HandleHeartbeatTimeout(ctx context.Context, snID types.StorageNodeID) {
	select {
	case sh.hbC <- snID:
	default:
	}
}

func (sh *testSnHandler) HandleReport(ctx context.Context, sn *snpb.StorageNodeMetadataDescriptor) {
	select {
	case sh.reportC <- sn:
	default:
	}
}

func TestVarlogSNWatcher(t *testing.T) {
	t.Skip("Tests for MRManager and SNWatcher")

	Convey("Given MR cluster", t, func(ctx C) {
		opts := []it.Option{
			it.WithReporterClientFactory(metarepos.NewReporterClientFactory()),
			it.WithStorageNodeManagementClientFactory(metarepos.NewEmptyStorageNodeClientFactory()),
		}

		Convey("cluster", it.WithTestCluster(t, opts, func(env *it.VarlogCluster) {
			mr := env.GetMR(t)

			So(testutil.CompareWaitN(50, func() bool {
				return mr.GetServerAddr() != ""
			}), ShouldBeTrue)
			mrAddr := mr.GetServerAddr()

			snID := env.AddSN(t)
			topicID := env.AddTopic(t)
			lsID := env.AddLS(t, topicID)

			vmsOpts := it.NewTestVMSOptions()
			vmsOpts.MRManagerOptions.MetadataRepositoryAddresses = []string{mrAddr}
			mrMgr, err := varlogadm.NewMRManager(context.TODO(), env.ClusterID(), vmsOpts.MRManagerOptions, env.Logger())
			So(err, ShouldBeNil)

			cmView := mrMgr.ClusterMetadataView()
			snMgr, err := varlogadm.NewStorageNodeManager(context.TODO(), env.ClusterID(), cmView, zap.NewNop())
			So(err, ShouldBeNil)

			snHandler := newTestSnHandler()

			wopts := varlogadm.WatcherOptions{
				Tick:             varlogadm.DefaultTick,
				ReportInterval:   varlogadm.DefaultReportInterval,
				HeartbeatTimeout: varlogadm.DefaultHeartbeatTimeout,
				RPCTimeout:       varlogadm.DefaultWatcherRPCTimeout,
			}
			snWatcher := varlogadm.NewStorageNodeWatcher(wopts, cmView, snMgr, snHandler, zap.NewNop())
			snWatcher.Run()
			defer snWatcher.Close()

			Convey("When seal LS", func(ctx C) {
				snID := env.PrimaryStorageNodeIDOf(t, lsID)
				_, _, err := env.SNClientOf(t, snID).Seal(context.TODO(), topicID, lsID, types.InvalidGLSN)
				So(err, ShouldBeNil)

				Convey("Then it should be reported by watcher", func(ctx C) {
					So(testutil.CompareWaitN(100, func() bool {
						select {
						case meta := <-snHandler.reportC:
							replica, exist := meta.FindLogStream(lsID)
							return exist && replica.GetStatus().Sealed()
						case <-time.After(varlogadm.DefaultTick * time.Duration(2*varlogadm.DefaultReportInterval)):
						}

						return false
					}), ShouldBeTrue)
				})
			})

			Convey("When close SN", func(ctx C) {
				sn := env.PrimaryStorageNodeIDOf(t, lsID)
				env.CloseSN(t, sn)

				Convey("Then it should be heartbeat timeout", func(ctx C) {
					So(testutil.CompareWaitN(50, func() bool {
						select {
						case hsnid := <-snHandler.hbC:
							return hsnid == snID
						case <-time.After(varlogadm.DefaultTick * time.Duration(2*varlogadm.DefaultHeartbeatTimeout)):
						}

						return false
					}), ShouldBeTrue)
				})
			})
		}))
	})
}

type dummyCMView struct {
	clusterID types.ClusterID
	addr      string
}

func (cmView *dummyCMView) ClusterMetadata(ctx context.Context) (*varlogpb.MetadataDescriptor, error) {
	cli, err := mrc.NewMetadataRepositoryClient(ctx, cmView.addr)
	if err != nil {
		return nil, err
	}
	defer cli.Close()

	meta, err := cli.GetMetadata(ctx)
	if err != nil {
		return nil, err
	}

	return meta, nil
}

func (cmView *dummyCMView) StorageNode(ctx context.Context, storageNodeID types.StorageNodeID) (*varlogpb.StorageNodeDescriptor, error) {
	meta, err := cmView.ClusterMetadata(ctx)
	if err != nil {
		return nil, err
	}
	if sndesc := meta.GetStorageNode(storageNodeID); sndesc != nil {
		return sndesc, nil
	}
	return nil, errors.New("cmview: no such storage node")
}

func TestVarlogStatRepositoryRefresh(t *testing.T) {
	t.Skip("Tests for StatRepository")

	Convey("Given MR cluster", t, func(ctx C) {
		opts := []it.Option{
			it.WithReporterClientFactory(metarepos.NewReporterClientFactory()),
			it.WithStorageNodeManagementClientFactory(metarepos.NewEmptyStorageNodeClientFactory()),
		}

		Convey("cluster", it.WithTestCluster(t, opts, func(env *it.VarlogCluster) {
			mr := env.GetMR(t)

			So(testutil.CompareWaitN(50, func() bool {
				return mr.GetServerAddr() != ""
			}), ShouldBeTrue)
			mrAddr := mr.GetServerAddr()

			snID := env.AddSN(t)
			topicID := env.AddTopic(t)
			lsID := env.AddLS(t, topicID)

			cmView := &dummyCMView{
				clusterID: env.ClusterID(),
				addr:      mrAddr,
			}
			statRepository := varlogadm.NewStatRepository(context.TODO(), cmView)

			metaIndex := statRepository.GetAppliedIndex()
			So(metaIndex, ShouldBeGreaterThan, 0)
			So(statRepository.GetStorageNode(snID), ShouldNotBeNil)
			lsStat := statRepository.GetLogStream(lsID)
			So(lsStat.Replicas, ShouldNotBeNil)

			Convey("When varlog cluster is not changed", func(ctx C) {
				Convey("Then refresh the statRepository and nothing happens", func(ctx C) {
					statRepository.Refresh(context.TODO())
					So(metaIndex, ShouldEqual, statRepository.GetAppliedIndex())
				})
			})

			Convey("When AddSN", func(ctx C) {
				snID2 := env.AddSN(t)

				Convey("Then refresh the statRepository and it should be updated", func(ctx C) {
					statRepository.Refresh(context.TODO())
					So(metaIndex, ShouldBeLessThan, statRepository.GetAppliedIndex())
					metaIndex := statRepository.GetAppliedIndex()

					So(statRepository.GetStorageNode(snID), ShouldNotBeNil)
					So(statRepository.GetStorageNode(snID2), ShouldNotBeNil)
					lsStat := statRepository.GetLogStream(lsID)
					So(lsStat.Replicas, ShouldNotBeNil)

					Convey("When UpdateLS", func(ctx C) {
						meta, err := mr.GetMetadata(context.TODO())
						So(err, ShouldBeNil)

						ls := meta.GetLogStream(lsID)
						So(ls, ShouldNotBeNil)

						newLS := proto.Clone(ls).(*varlogpb.LogStreamDescriptor)
						newLS.Replicas[0].StorageNodeID = snID2

						err = mr.UpdateLogStream(context.TODO(), newLS)
						So(err, ShouldBeNil)

						Convey("Then refresh the statRepository and it should be updated", func(ctx C) {
							statRepository.Refresh(context.TODO())
							So(metaIndex, ShouldBeLessThan, statRepository.GetAppliedIndex())

							So(statRepository.GetStorageNode(snID), ShouldNotBeNil)
							So(statRepository.GetStorageNode(snID2), ShouldNotBeNil)
							lsStat := statRepository.GetLogStream(lsID)
							So(lsStat.Replicas, ShouldNotBeNil)

							_, ok := lsStat.Replica(snID)
							So(ok, ShouldBeFalse)
							_, ok = lsStat.Replica(snID2)
							So(ok, ShouldBeTrue)
						})
					})
				})
			})

			Convey("When AddLS", func(ctx C) {
				lsID2 := env.AddLS(t, topicID)

				Convey("Then refresh the statRepository and it should be updated", func(ctx C) {
					statRepository.Refresh(context.TODO())
					So(metaIndex, ShouldBeLessThan, statRepository.GetAppliedIndex())

					So(statRepository.GetStorageNode(snID), ShouldNotBeNil)
					lsStat := statRepository.GetLogStream(lsID)
					So(lsStat.Replicas, ShouldNotBeNil)
					lsStat = statRepository.GetLogStream(lsID2)
					So(lsStat.Replicas, ShouldNotBeNil)
				})
			})

			Convey("When SealLS", func(ctx C) {
				_, err := mr.Seal(context.TODO(), lsID)
				So(err, ShouldBeNil)

				Convey("Then refresh the statRepository and it should be updated", func(ctx C) {
					statRepository.Refresh(context.TODO())
					So(metaIndex, ShouldBeLessThan, statRepository.GetAppliedIndex())
					metaIndex := statRepository.GetAppliedIndex()

					So(statRepository.GetStorageNode(snID), ShouldNotBeNil)
					lsStat := statRepository.GetLogStream(lsID)
					So(lsStat.Replicas, ShouldNotBeNil)

					Convey("When UnsealLS", func(ctx C) {
						err := mr.Unseal(context.TODO(), lsID)
						So(err, ShouldBeNil)

						Convey("Then refresh the statRepository and it should be updated", func(ctx C) {
							statRepository.Refresh(context.TODO())
							So(metaIndex, ShouldBeLessThan, statRepository.GetAppliedIndex())

							So(statRepository.GetStorageNode(snID), ShouldNotBeNil)
							lsStat := statRepository.GetLogStream(lsID)
							So(lsStat.Replicas, ShouldNotBeNil)
						})
					})
				})
			})
		}))
	})
}

func TestVarlogStatRepositoryReport(t *testing.T) {
	t.Skip("Tests for StatRepository")

	Convey("Given MR cluster", t, func(ctx C) {
		opts := []it.Option{
			it.WithReporterClientFactory(metarepos.NewReporterClientFactory()),
			it.WithStorageNodeManagementClientFactory(metarepos.NewEmptyStorageNodeClientFactory()),
		}

		Convey("cluster", it.WithTestCluster(t, opts, func(env *it.VarlogCluster) {
			mr := env.GetMR(t)

			So(testutil.CompareWaitN(50, func() bool {
				return mr.GetServerAddr() != ""
			}), ShouldBeTrue)
			mrAddr := mr.GetServerAddr()

			snID := env.AddSN(t)
			topicID := env.AddTopic(t)
			lsID := env.AddLS(t, topicID)

			cmView := &dummyCMView{
				clusterID: env.ClusterID(),
				addr:      mrAddr,
			}
			statRepository := varlogadm.NewStatRepository(context.TODO(), cmView)

			So(statRepository.GetStorageNode(snID), ShouldNotBeNil)
			lsStat := statRepository.GetLogStream(lsID)
			So(lsStat.Replicas, ShouldNotBeNil)

			Convey("When Report", func(ctx C) {
				sn := env.LookupSN(t, snID)

				addr := storagenode.TestGetAdvertiseAddress(t, sn)

				storagenode.TestSealLogStreamReplica(t, env.ClusterID(), topicID, lsID, types.InvalidGLSN, addr)

				snm := storagenode.TestGetStorageNodeMetadataDescriptor(t, env.ClusterID(), addr)

				statRepository.Report(context.TODO(), snm)

				Convey("Then it should be updated", func(ctx C) {
					lsStat := statRepository.GetLogStream(lsID)
					So(lsStat.Replicas, ShouldNotBeNil)

					r, ok := lsStat.Replica(snID)
					So(ok, ShouldBeTrue)

					So(r.Status.Sealed(), ShouldBeTrue)

					Convey("When AddSN and refresh the statRepository", func(ctx C) {
						_ = env.AddSN(t)
						statRepository.Refresh(context.TODO())

						Convey("Then reported info should be applied", func(ctx C) {
							lsStat := statRepository.GetLogStream(lsID)
							So(lsStat.Replicas, ShouldNotBeNil)

							r, ok := lsStat.Replica(snID)
							So(ok, ShouldBeTrue)

							So(r.Status.Sealed(), ShouldBeTrue)
						})
					})
				})
			})
		}))
	})
}
