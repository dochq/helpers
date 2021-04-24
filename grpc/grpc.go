package grpc

import (
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/DocHQ/logging"

	_ "github.com/joho/godotenv/autoload"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

/*

	srv, _ := NewRPCServer(":5000")
	proto.RegisterProductServiceServer(srv, &controller.Controller{})
	StartServer(srv)
*/

var listener net.Listener

func NewRPCServer(port string, opt ...grpc.ServerOption) (server *grpc.Server, err error) {
	listener, err = net.Listen("tcp", port)
	if err != nil {
		return server, err
	}

	return grpc.NewServer(opt...), nil
}

func StartServer(srv *grpc.Server) {
	reflection.Register(srv)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	logging.Info("Starting server...")

	canExit := false
	go func() {
		if e := srv.Serve(listener); e != nil {
			logging.Error(e)
		} else {
			logging.Info("Server gracefully stopping...")
		}
		canExit = true
	}()

CronLoop:
	for {
		time.Sleep(time.Second) // sleep for 1 second
		select {
		case <-stop:
			{
				logging.Info("Server is being shut down...")
				srv.GracefulStop()
			}

		default:
			if canExit {
				logging.Info("Server has been shut down...")
				break CronLoop
			}
		}
	}
}
