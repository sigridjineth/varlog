package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"testing"
	"time"

	"github.com/kakao/varlog/internal/metadata_repository"
	"github.com/kakao/varlog/pkg/varlog"
	"github.com/kakao/varlog/pkg/varlog/types"
	varlogpb "github.com/kakao/varlog/proto/varlog"

	. "github.com/smartystreets/goconvey/convey"
	"google.golang.org/grpc"
)

type testEnv struct {
	addr string
	lis  net.Listener
	srv  *grpc.Server
	cli  varlog.MetadataRepositoryClient
	mr   metadata_repository.MetadataRepository
}

func createServer() (net.Listener, *grpc.Server, error) {
	lis, err := net.Listen("tcp", ":0")
	if err != nil {
		log.Fatalf("could not listen: %v", err)
	}
	server := grpc.NewServer()
	return lis, server, err
}

func startServer(lis net.Listener, server *grpc.Server) {
	if err := server.Serve(lis); err != nil {
		log.Fatalf("could not serve: %v", err)
	}
}

func createMetadataRepository(server *grpc.Server) metadata_repository.MetadataRepository {
	metaRepos := metadata_repository.NewInMemoryMetadataRepository()
	service := metadata_repository.NewMetadataRepositoryService(metaRepos)
	service.Register(server)

	return metaRepos
}

func createRaftMetadataRepository(server *grpc.Server) metadata_repository.MetadataRepository {
	var cluster []string

	os.RemoveAll("raft-1")
	os.RemoveAll("raft-1-snap")

	cluster = append(cluster, "http://127.0.0.1:10000")

	metaRepos := metadata_repository.NewRaftMetadataRepository(0, 1, cluster)
	metaRepos.Start()

	service := metadata_repository.NewMetadataRepositoryService(metaRepos)
	service.Register(server)

	return metaRepos
}

func CreateEnv(t *testing.T) *testEnv {
	lis, srv, err := createServer()
	if err != nil {
		t.Fatal(err)
	}
	addr := lis.Addr()
	tcpAddr := addr.(*net.TCPAddr)
	address := fmt.Sprintf("localhost:%d", tcpAddr.Port)

	//mr = createMetadataRepository(srv)
	mr := createRaftMetadataRepository(srv)
	go startServer(lis, srv)

	cli, err := varlog.NewMetadataRepositoryClient(address)
	if err != nil {
		t.Fatal(err)
	}

	env := &testEnv{
		addr: address,
		lis:  lis,
		srv:  srv,
		cli:  cli,
		mr:   mr,
	}

	return env
}

func (env *testEnv) Close() {
	env.cli.Close()
	env.srv.GracefulStop()
	env.lis.Close()
	env.mr.Close()
}

func TestMetadataRepositoryClientSimpleRegister(t *testing.T) {
	var env *testEnv

	Convey("Create Env", t, func(ctx C) {
		env = CreateEnv(t)
	})
	defer env.Close()

	Convey("Register Storage Node", t, func(ctx C) {
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

		err := env.cli.RegisterStorageNode(context.TODO(), sn)
		So(err, ShouldEqual, nil)

		Convey("Get Storage Node info from Metadata", func(ctx C) {
			meta, err := env.cli.GetMetadata(context.TODO())
			So(err, ShouldEqual, nil)
			So(meta.GetStorageNode(snId), ShouldNotEqual, nil)
		})

		Convey("Register Exist Storage Node", func(ctx C) {
			err := env.cli.RegisterStorageNode(context.TODO(), sn)
			So(varlog.IsAlreadyExistsErr(err), ShouldEqual, true)
		})
	})

	Convey("Create Log Stream", t, func(ctx C) {
		lsId := types.LogStreamID(time.Now().UnixNano())

		ls := &varlogpb.LogStreamDescriptor{
			LogStreamID: lsId,
		}

		err := env.cli.CreateLogStream(context.TODO(), ls)
		So(err, ShouldEqual, nil)

		Convey("Get Log Steam info from Metadata", func(ctx C) {
			meta, err := env.cli.GetMetadata(context.TODO())
			So(err, ShouldEqual, nil)
			So(meta.GetLogStream(lsId), ShouldNotEqual, nil)
		})

		Convey("Create Exist Log Steam", func(ctx C) {
			err := env.cli.CreateLogStream(context.TODO(), ls)
			So(varlog.IsAlreadyExistsErr(err), ShouldEqual, true)
		})
	})
}
