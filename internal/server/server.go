package server

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
)

type Server struct {
	http *http.Server
}

func New(port string, handler *gin.Engine) *Server {
	return &Server{
		http: &http.Server{
			Addr:              ":" + port,
			Handler:           handler,
			ReadHeaderTimeout: 10 * time.Second,
			ReadTimeout:       30 * time.Second,
			WriteTimeout:      60 * time.Second,
			IdleTimeout:       120 * time.Second,
		},
	}
}

func (s *Server) Run() error {
	errCh := make(chan error, 1)

	go func() {
		log.Printf("Server berjalan di %s...", s.http.Addr)
		if err := s.http.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-errCh:
		return err
	case sig := <-quit:
		log.Printf("Menerima signal %s, memulai graceful shutdown...", sig)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := s.http.Shutdown(ctx); err != nil {
		log.Printf("Graceful shutdown gagal: %v", err)
		return s.http.Close()
	}

	log.Println("Server berhenti dengan bersih.")
	return nil
}