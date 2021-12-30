package logstream

import (
	"context"

	"github.com/kakao/varlog/internal/varlogctl"
	"github.com/kakao/varlog/internal/varlogctl/result"
	"github.com/kakao/varlog/pkg/types"
	"github.com/kakao/varlog/pkg/varlog"
)

const resourceType = "logstream"

func Add(topicID types.TopicID) varlogctl.ExecuteFunc {
	return func(ctx context.Context, adm varlog.Admin) *result.Result {
		res := result.New(resourceType)
		if lsd, err := adm.AddLogStream(ctx, topicID, nil); err != nil {
			res.AddErrors(err)
		} else {
			res.AddDataItems(lsd)
		}
		return res
	}
}

func Seal(topicID types.TopicID, logStreamID types.LogStreamID) varlogctl.ExecuteFunc {
	return func(ctx context.Context, adm varlog.Admin) *result.Result {
		res := result.New(resourceType)
		rsp, err := adm.Seal(ctx, topicID, logStreamID)
		if err != nil {
			res.AddErrors(err)
			return res
		}
		res.AddDataItems(rsp)
		return res
	}
}

func Unseal(topicID types.TopicID, logStreamID types.LogStreamID) varlogctl.ExecuteFunc {
	return func(ctx context.Context, adm varlog.Admin) *result.Result {
		res := result.New(resourceType)
		if lsd, err := adm.Unseal(ctx, topicID, logStreamID); err != nil {
			res.AddErrors(err)
		} else {
			res.AddDataItems(lsd)
		}
		return res
	}
}

func Sync(topicID types.TopicID, logStreamID types.LogStreamID, src, dst types.StorageNodeID) varlogctl.ExecuteFunc {
	panic("not implemented")
}

func Describe(topicID types.TopicID, logStreamID ...types.LogStreamID) varlogctl.ExecuteFunc {
	panic("not implemented")
}