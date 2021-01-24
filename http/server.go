package http

import (
	"context"
	base_http "net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/DocHQ/logging"
)

func StartServer(srv *base_http.Server) {
	// Allow graceful exit by listening for terminate signal
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	logging.Info("Starting service...")

	// Use a go function to listen and serve and respond to a shutdown
	canExit := false
	go func() {
		if err := srv.ListenAndServe(); err != base_http.ErrServerClosed {
			logging.Info(err)
		} else {
			logging.Info("Server gracefully stopped")
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
				ctx, cancelFn := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancelFn()
				srv.Shutdown(ctx)
			}

		default:
			if canExit {
				logging.Info("Server has been shut down")
				break CronLoop
			}
		}
	}
}
