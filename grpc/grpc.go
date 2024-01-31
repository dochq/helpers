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
	"google.golang.org/grpc/keepalive"
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

	return grpc.NewServer(append(opt, ServerKeepaliveParams)...), nil
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

// ServerKeepaliveParams - gRPC Server Keepalive Parameters
var ServerKeepaliveParams = grpc.KeepaliveParams(keepalive.ServerParameters{
	// After a duration of this time if the server doesn't see any activity it
	// pings the client to see if the transport is still alive.
	// If set below 1s, a minimum value of 1s will be used instead.
	// Set to a relatively short duration to detect idle connections faster.
	Time: 30 * time.Second,

	// After having pinged for keepalive check, the server waits for a duration
	// of Timeout and if no activity is seen even after that the connection is
	// closed.
	// Set to a value higher than Time to allow for potential network delays.
	Timeout: 60 * time.Second,

	// MaxConnectionAgeGrace is an additive period after MaxConnectionAge after
	// which the connection will be forcibly closed.
	// Set conservatively to provide extra time before forcibly closing.
	MaxConnectionAgeGrace: 60 * time.Second,

	// Set to a value that accommodates your application's requirements.
	MaxConnectionAge: 5 * time.Minute,
})
