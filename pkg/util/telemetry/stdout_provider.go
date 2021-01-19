package telemetry

import (
	"context"

	"go.opentelemetry.io/otel/exporters/stdout"
	"go.opentelemetry.io/otel/sdk/metric/controller/push"
)

type stdoutProvider struct {
	pipeline *push.Controller
}

var _ Provider = (*stdoutProvider)(nil)

func newStdoutProvider() (*stdoutProvider, error) {
	pipeline, err := stdout.InstallNewPipeline(nil, nil)
	if err != nil {
		return nil, err
	}
	return &stdoutProvider{pipeline: pipeline}, nil
}

func (sp *stdoutProvider) Close(_ context.Context) error {
	sp.pipeline.Stop()
	return nil
}