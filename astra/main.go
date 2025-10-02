package main

import (
	"astra/astra/config"
	"astra/astra/controllers"
	"astra/astra/routes"
	"astra/astra/sources/psql"
	"astra/astra/sources/psql/dao"
	"astra/astra/sources/storage"
	"astra/astra/utils/logging"
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func main() {
	logging.InitLogger()
	cfg := config.LoadConfig()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	db, err := psql.NewDatabase(ctx, cfg)
	if err != nil {
		logging.Logger.Error("database connection error", "error", err)
		os.Exit(1)
	}
	defer db.Close()
	userDAO := dao.NewUserDAO(db.DB)
	chatDAO := dao.NewChatMessageDAO(db.DB)
	authCtrl := controllers.NewAuthController(userDAO, cfg)
	userCtrl := controllers.NewUserController(userDAO)
	chatCtrl := controllers.NewChatController(chatDAO)

	// Initialize MinIO
	minioClient, err := storage.NewMinIOClient(cfg)
	if err != nil {
		logging.Logger.Error("minio connection error", "error", err)
		os.Exit(1)
	}
	scrapeCtrl, err := controllers.NewScrapeController(minioClient)
	if err != nil {
		logging.Logger.Error("minio connection error", "error", err)
		os.Exit(1)
	}

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))

	r.Mount("/auth", routes.AuthRoutes(authCtrl))
	r.Mount("/users", routes.UserRoutes(userCtrl, cfg))
	r.Mount("/chat", routes.ChatRoutes(chatCtrl, cfg))
	r.Mount("/test", routes.ScrapeRoutes(scrapeCtrl, cfg))

	srv := &http.Server{
		Addr:    ":8000", // Or load from env
		Handler: r,
	}
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logging.Logger.Error("server listen error", "error", err)
		}
	}()
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logging.Logger.Error("server shutdown error", "error", err)
		logging.Logger.Info("server shutdown complete")
	}
}
