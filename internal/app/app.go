package app

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"tenders/internal/config"
	"tenders/internal/controller"
	"tenders/internal/repository"
	"tenders/internal/router"
	"tenders/internal/service"
	"time"
)

type App struct {
	repo       *repository.Repository
	service    *service.Service
	controller *controller.Controller
	stopSig    chan os.Signal
	cfg        *config.Config

	Done chan struct{}
}

type option func(*App)

func WithConfig(cfg *config.Config) option {
	return func(app *App) {
		app.cfg = cfg
	}
}

func NewApp(opts ...option) (*App, error) {
	var err error

	app := &App{
		stopSig: make(chan os.Signal, 2),
		Done:    make(chan struct{}),
	}

	for _, opt := range opts {
		opt(app)
	}

	if app.cfg == nil {
		cfg, err := config.NewConfig()
		if err != nil {
			return nil, err
		}
		app.cfg = cfg
	}

	app.repo, err = repository.NewRepository(nil, &app.cfg.PostgresConfig)
	if err != nil {
		return nil, err
	}

	app.service = service.NewService(app.repo)
	app.controller = controller.NewController(app.service)

	return app, nil
}

func (app *App) Run() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		signal.Notify(app.stopSig, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
		sig := <-app.stopSig
		log.Printf("Received signal: %s\n", sig)
		cancel()
	}()

	server := http.Server{
		Addr:         app.cfg.ServerAddress,
		Handler:      router.NewRouter(app.controller),
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	go func() {
		err := server.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			log.Println("Http server error:", err)
		}
	}()

	log.Printf("Server started at %s, listening for connections...\n", app.cfg.ServerAddress)
	<-ctx.Done()

	timeout, tcancel := context.WithTimeout(context.Background(), time.Second*10)
	defer tcancel()
	log.Println("Shutting down http server...")
	server.Shutdown(timeout)

	log.Println("Closing repository...")
	err := app.repo.Close()
	if err != nil {
		log.Println("Repository closing error:", err)
	}

	close(app.Done)
	log.Println("Exiting app.")
}
