package metadata_repository

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/kakao/varlog/pkg/snc"
	"github.com/kakao/varlog/pkg/types"
	"github.com/kakao/varlog/pkg/util/mathutil"
	"github.com/kakao/varlog/pkg/verrors"
	"github.com/kakao/varlog/proto/mrpb"
	"github.com/kakao/varlog/proto/snpb"
	"github.com/kakao/varlog/proto/varlogpb"
)

type StateMachineSyncer struct {
	nrReplica int
	clients   []snc.StorageNodeManagementClient
}

type commitResultContext struct {
	prevCommitResults *mrpb.LogStreamCommitResults
	commitInfos       map[types.LogStreamID]map[types.StorageNodeID]snpb.LogStreamCommitInfo
	highestLLSNs      map[types.LogStreamID]types.LLSN
	sortedLSIDs       []types.LogStreamID
	expectedCommit    uint64
	numCommit         uint64

	commitResults *mrpb.LogStreamCommitResults
}

const StateMachineSyncerTimeout = 2 * time.Second

type snConnFunc func(context.Context, string) (snc.StorageNodeManagementClient, error)

func NewStateMachineSyncer(addrs []string, nrReplica int, connf snConnFunc) (*StateMachineSyncer, error) {
	s := &StateMachineSyncer{
		nrReplica: nrReplica,
		clients:   make([]snc.StorageNodeManagementClient, 0, len(addrs)),
	}

	for _, addr := range addrs {
		ctx, cancel := context.WithTimeout(context.Background(), StateMachineSyncerTimeout)
		defer cancel()
		cli, err := connf(ctx, addr)
		if err != nil {
			s.Close()
			return nil, err
		}

		s.clients = append(s.clients, cli)

		meta, err := cli.GetMetadata(ctx)
		if err != nil {
			s.Close()
			return nil, err
		}

		sn := meta.GetStorageNode()
		if sn == nil {
			continue
		}
	}

	return s, nil
}

func (s *StateMachineSyncer) Close() {
	for _, cli := range s.clients {
		cli.Close()
	}
}

func (s *StateMachineSyncer) syncMetadata(ctx context.Context, storage *MetadataStorage) error {
	var err error

	collectedLSs := make(map[types.LogStreamID][]*varlogpb.LogStreamMetadataDescriptor)
	for _, cli := range s.clients {
		meta, err := cli.GetMetadata(ctx)
		fmt.Printf("syncMetadata:: sn[%v] GetMetadata %+v. err:%v\n",
			cli.PeerStorageNodeID(), meta, err)

		if err != nil {
			return err
		}

		sn := meta.GetStorageNode()
		if sn == nil {
			continue
		}

		// sync StorageNodeDescriptor
		if err := storage.RegisterStorageNode(sn, 0, 0); err != nil && err != verrors.ErrAlreadyExists {
			return err
		}

		// collect LogStreamDescriptor
		for _, tmp := range meta.GetLogStreams() {
			ls := tmp
			ls.StorageNodeID = sn.StorageNodeID

			collectedLSs[ls.LogStreamID] = append(collectedLSs[ls.LogStreamID], &ls)
		}
	}

	for _, ls := range storage.GetLogStreams() {
		if _, ok := collectedLSs[ls.LogStreamID]; !ok {
			return fmt.Errorf("sync metadata error. ls[%v] should exist in the collected log streams", ls.LogStreamID)
		}
	}

	// sync LogStreamDescriptor
	for lsID, collectedLS := range collectedLSs {
		oldLS := storage.LookupLogStream(lsID)
		if oldLS != nil {
			if compareLogStreamReplica(oldLS.Replicas, collectedLS) {
				// already exist logstream
				continue
			}
		}

		if len(collectedLS) < s.nrReplica {
			if oldLS != nil {
				return fmt.Errorf("sync metadata error. ls[%d] # of collectedLS < repFactor", lsID)
			}

			for _, r := range collectedLS {
				if !r.HighWatermark.Invalid() {
					return fmt.Errorf("sync metadata error. newbie ls[%d] # of collectedLS < repFactor & has valid HWM", lsID)
				}
			}

			// not yet created logstream
			continue
		}

		if len(collectedLS) > s.nrReplica {
			collectedLS = s.selectReplicas(collectedLS)
		}

		ls := &varlogpb.LogStreamDescriptor{
			LogStreamID: lsID,
			Status:      varlogpb.LogStreamStatusSealed,
		}

		for _, collectedReplica := range collectedLS {
			r := &varlogpb.ReplicaDescriptor{
				StorageNodeID: collectedReplica.StorageNodeID,
				Path:          collectedReplica.Path,
			}

			ls.Replicas = append(ls.Replicas, r)
		}

		if oldLS == nil {
			err = storage.RegisterLogStream(ls, 0, 0)
		} else {
			err = storage.UpdateLogStream(ls, 0, 0)
		}
	}

	return err
}

func (s *StateMachineSyncer) selectReplicas(replicas []*varlogpb.LogStreamMetadataDescriptor) []*varlogpb.LogStreamMetadataDescriptor {
	if len(replicas) <= s.nrReplica {
		return replicas
	}

	sort.Slice(replicas, func(i, j int) bool {
		if replicas[i].HighWatermark == replicas[j].HighWatermark {
			return replicas[i].UpdatedTime.After(replicas[j].UpdatedTime)
		}

		return replicas[i].HighWatermark > replicas[j].HighWatermark
	})

	return replicas[:s.nrReplica]
}

func compareLogStreamReplica(orig []*varlogpb.ReplicaDescriptor, diff []*varlogpb.LogStreamMetadataDescriptor) bool {
	if len(orig) != len(diff) {
		return false
	}

	sort.Slice(orig, func(i, j int) bool { return orig[i].StorageNodeID < orig[j].StorageNodeID })
	sort.Slice(diff, func(i, j int) bool { return diff[i].StorageNodeID < diff[j].StorageNodeID })

	for i := 0; i < len(orig); i++ {
		if !orig[i].Equal(diff[i]) {
			return false
		}
	}

	return true
}

func (s *StateMachineSyncer) SyncCommitResults(ctx context.Context, storage *MetadataStorage) error {
	if err := s.syncMetadata(ctx, storage); err != nil {
		return err
	}

	for {
		cc, err := s.initCommitResultContext(ctx, storage.GetLastCommitResults())
		if err != nil {
			return fmt.Errorf("sync commit result init fail. %v", err)
		}

		if cc.commitResults.HighWatermark.Invalid() {
			break
		}

		if err := cc.buildCommitResults(); err != nil {
			return fmt.Errorf("sync commit result build fail. %v. info:%+v, prev:%+v",
				err, cc.commitInfos, cc.prevCommitResults)
		}

		if err := cc.validate(); err != nil {
			return fmt.Errorf("sync commit result validate fail. %v. info:%+v, prev:%+v, cur:%+v",
				err, cc.commitInfos, cc.prevCommitResults, cc.commitResults)
		}

		storage.AppendLogStreamCommitHistory(cc.commitResults)
	}

	return nil
}

func (s *StateMachineSyncer) initCommitResultContext(ctx context.Context, prevCommitResults *mrpb.LogStreamCommitResults) (*commitResultContext, error) {
	cc := &commitResultContext{
		prevCommitResults: prevCommitResults,
		commitResults:     &mrpb.LogStreamCommitResults{},
		commitInfos:       make(map[types.LogStreamID]map[types.StorageNodeID]snpb.LogStreamCommitInfo),
		highestLLSNs:      make(map[types.LogStreamID]types.LLSN),
	}

	for _, cli := range s.clients {
		snID := cli.PeerStorageNodeID()
		commitInfo, err := cli.GetPrevCommitInfo(ctx, prevCommitResults.GetHighWatermark())

		if err != nil {
			return nil, err
		}

		for _, lsCommitInfo := range commitInfo.CommitInfos {
			if lsCommitInfo.Status == snpb.GetPrevCommitStatusInconsistent {
				return nil, fmt.Errorf("inconsistency commit info[snID:%v, hwm:%v]",
					snID, prevCommitResults.GetHighWatermark())
			} else if lsCommitInfo.Status == snpb.GetPrevCommitStatusOK {
				cc.commitResults.HighWatermark = lsCommitInfo.HighWatermark
				cc.commitResults.PrevHighWatermark = lsCommitInfo.PrevHighWatermark
			}

			r, ok := cc.commitInfos[lsCommitInfo.LogStreamID]
			if !ok {
				r = make(map[types.StorageNodeID]snpb.LogStreamCommitInfo)
				cc.commitInfos[lsCommitInfo.LogStreamID] = r
				cc.sortedLSIDs = append(cc.sortedLSIDs, lsCommitInfo.LogStreamID)
			}

			r[snID] = *lsCommitInfo

			if highestLLSN, ok := cc.highestLLSNs[lsCommitInfo.LogStreamID]; !ok || highestLLSN > lsCommitInfo.HighestWrittenLLSN {
				cc.highestLLSNs[lsCommitInfo.LogStreamID] = lsCommitInfo.HighestWrittenLLSN
			}
		}
	}

	if !cc.commitResults.HighWatermark.Invalid() {
		sort.Slice(cc.sortedLSIDs, func(i, j int) bool { return cc.sortedLSIDs[i] < cc.sortedLSIDs[j] })
		cc.commitResults.CommitResults = make([]*snpb.LogStreamCommitResult, 0, len(cc.sortedLSIDs))
		cc.expectedCommit = uint64(cc.commitResults.HighWatermark - cc.commitResults.PrevHighWatermark)
	}

	return cc, nil
}

func (cc *commitResultContext) buildCommitResults() error {
	for _, lsID := range cc.sortedLSIDs {
		c := &snpb.LogStreamCommitResult{
			LogStreamID:         lsID,
			CommittedLLSNOffset: types.InvalidLLSN,
			CommittedGLSNOffset: types.InvalidGLSN,
			CommittedGLSNLength: 0,
			HighWatermark:       cc.commitResults.HighWatermark,
			PrevHighWatermark:   cc.commitResults.PrevHighWatermark,
		}

		commitInfo, _ := cc.commitInfos[lsID]

	SET_COMMIT_INFO:
		for _, r := range commitInfo {
			if r.Status == snpb.GetPrevCommitStatusOK {
				c.CommittedLLSNOffset = r.CommittedLLSNOffset
				c.CommittedGLSNOffset = r.CommittedGLSNOffset
				c.CommittedGLSNLength = r.CommittedGLSNLength

				break SET_COMMIT_INFO
			}
		}
		cc.numCommit += c.CommittedGLSNLength
		cc.commitResults.CommitResults = append(cc.commitResults.CommitResults, c)
	}

	if err := cc.fillCommitResult(); err != nil {
		return err
	}

	return nil
}

func (cc *commitResultContext) validate() error {
	i := 0
	j := 0

	nrCommitted := uint64(0)
	for i < len(cc.prevCommitResults.GetCommitResults()) && j < len(cc.commitResults.GetCommitResults()) {
		prev := cc.prevCommitResults.CommitResults[i]
		cur := cc.commitResults.CommitResults[j]
		if prev.LogStreamID < cur.LogStreamID {
			return fmt.Errorf("new commit reuslts should include all prev commit results")
		} else if prev.LogStreamID > cur.LogStreamID {
			if cur.CommittedLLSNOffset != types.MinLLSN {
				return fmt.Errorf("newbie LS[%v] should start from MinLLSN", cur.LogStreamID)
			}

			nrCommitted += cur.CommittedGLSNLength
			j++
		} else {
			if prev.CommittedLLSNOffset+types.LLSN(prev.CommittedGLSNLength) != cur.CommittedLLSNOffset {
				return fmt.Errorf("invalid commit result")
			}

			nrCommitted += cur.CommittedGLSNLength
			i++
			j++
		}
	}

	if i < len(cc.prevCommitResults.GetCommitResults()) {
		return fmt.Errorf("new commit reuslts should include all prev commit results")
	}

	for j < len(cc.commitResults.CommitResults) {
		cur := cc.commitResults.CommitResults[j]
		if cur.CommittedLLSNOffset != types.MinLLSN {
			return fmt.Errorf("newbie LS[%v] should start from MinLLSN", cur.LogStreamID)
		}

		nrCommitted += cur.CommittedGLSNLength
		j++
	}

	if nrCommitted != uint64(cc.commitResults.HighWatermark-cc.commitResults.PrevHighWatermark) {
		return fmt.Errorf("invalid commit length")
	}

	return nil
}

func (cc *commitResultContext) fillCommitResult() error {
	committedGLSNOffset := cc.prevCommitResults.GetHighWatermark() + 1
	for i, commitResult := range cc.commitResults.CommitResults {
		if !commitResult.CommittedGLSNOffset.Invalid() {
			if committedGLSNOffset != commitResult.CommittedGLSNOffset {
				return fmt.Errorf("committedGLSNOffset mismatch. lsid:%v, expectedGLSN:%v, recvGLSN:%v",
					commitResult.GetLogStreamID(), committedGLSNOffset, commitResult.GetCommittedGLSNOffset())
			}

			committedGLSNOffset = commitResult.CommittedGLSNOffset + types.GLSN(commitResult.CommittedGLSNLength)
			continue
		}

		lastCommittedLLSN := types.InvalidLLSN
		highestLLSN, _ := cc.highestLLSNs[commitResult.LogStreamID]

		prevCommitResult := cc.prevCommitResults.LookupCommitResult(commitResult.LogStreamID)
		if prevCommitResult != nil {
			lastCommittedLLSN = prevCommitResult.CommittedLLSNOffset + types.LLSN(prevCommitResult.CommittedGLSNLength) - 1
		}

		if highestLLSN < lastCommittedLLSN {
			return fmt.Errorf("invalid commit info. ls:%v, highestLLSN:%v, lastCommittedLLSN:%v",
				commitResult.LogStreamID, highestLLSN, lastCommittedLLSN)
		}

		numUncommit := uint64(highestLLSN - lastCommittedLLSN)
		boundary := uint64(boundaryCommittedGLSNOffset(cc.commitResults.CommitResults[i+1:]) - committedGLSNOffset)

		commitResult.CommittedGLSNLength = mathutil.MinUint64(cc.expectedCommit-cc.numCommit,
			mathutil.MinUint64(numUncommit, boundary))
		commitResult.CommittedLLSNOffset = lastCommittedLLSN + 1
		commitResult.CommittedGLSNOffset = committedGLSNOffset

		cc.numCommit += commitResult.CommittedGLSNLength
		committedGLSNOffset += types.GLSN(commitResult.CommittedGLSNLength)
	}

	return nil
}

func boundaryCommittedGLSNOffset(commitResults []*snpb.LogStreamCommitResult) types.GLSN {
	for _, commitResult := range commitResults {
		if !commitResult.CommittedGLSNOffset.Invalid() {
			return commitResult.CommittedGLSNOffset
		}
	}

	return types.MaxGLSN
}
