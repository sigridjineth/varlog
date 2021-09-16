package app

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/kakao/varlog/internal/vms"
)

func Main(opts *vms.Options) error {
	loggerConfig := zap.NewProductionConfig()
	loggerConfig.Sampling = nil
	loggerConfig.Level = zap.NewAtomicLevelAt(zap.InfoLevel)
	loggerConfig.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	logger, err := loggerConfig.Build(zap.AddStacktrace(zap.DPanicLevel))
	if err != nil {
		return err
	}
	defer logger.Sync()

	opts.Logger = logger

	// TODO: add VMSInitTimeout to options
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	cm, err := vms.NewClusterManager(ctx, opts)
	if err != nil {
		logger.Error("could not create cluster manager server", zap.Error(err))
		return err
	}

	if err = cm.Run(); err != nil {
		logger.Error("could not run cluster manager server", zap.Error(err))
		return err
	}

	// TODO (jun): handle SIGQUIT (it should be able to produce core dump)
	sigC := make(chan os.Signal, 1)
	signal.Notify(sigC, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigC
		cm.Close()
	}()

	cm.Wait()
	return nil
}
