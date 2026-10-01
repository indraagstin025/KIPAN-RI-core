package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/gofiber/fiber/v2/middleware/requestid"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/kipan-indonesia/sim-kipan-core/config"
	"github.com/kipan-indonesia/sim-kipan-core/internal/database"
	"github.com/kipan-indonesia/sim-kipan-core/internal/handler"
	"github.com/kipan-indonesia/sim-kipan-core/internal/middleware"
	"github.com/kipan-indonesia/sim-kipan-core/pkg/validator"
)

func main() {
	setupLogger()

	cfg, err := config.Load()
	if err != nil {
		log.Fatal().Err(err).Msg("Gagal memuat konfigurasi. Server tidak dapat berjalan.")
	}

	log.Info().
		Str("env", cfg.App.Env).
		Str("port", cfg.App.Port).
		Msg("SIM-KIPAN Core API [Fokus: Auth & Authorization] starting...")

	db, err := database.ConnectPostgres(cfg)
	if err != nil {
		log.Fatal().Err(err).Msg("Gagal konfigurasi database PostgreSQL")
	}
	if db != nil {
		defer db.Close()
	}

	rdb, err := database.ConnectRedis(cfg)
	if err != nil {
		log.Fatal().Err(err).Msg("Gagal konfigurasi Redis")
	}
	if rdb != nil {
		defer rdb.Close()
	}

	app := setupFiberApp(cfg)
	handler.RegisterRoutes(app, cfg, db, rdb, validator.New())

	startServer(app, cfg.App.Port)
}

func setupLogger() {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339})
}

func setupFiberApp(cfg *config.Config) *fiber.App {
	app := fiber.New(fiber.Config{
		AppName:           cfg.App.Name + " [Auth Core]",
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       30 * time.Second,
		BodyLimit:         2 * 1024 * 1024,
		EnablePrintRoutes: cfg.App.Debug,
		ErrorHandler:      errorHandler,
	})

	app.Use(recover.New(recover.Config{EnableStackTrace: cfg.App.Debug}))
	app.Use(requestid.New())
	app.Use(middleware.RequestLogger())
	app.Use(cors.New(cors.Config{
		AllowOrigins:     cfg.App.AllowOrigin,
		AllowMethods:     "GET,POST,PUT,PATCH,DELETE,OPTIONS",
		AllowHeaders:     "Origin, Content-Type, Accept, Authorization, X-Request-ID",
		AllowCredentials: true,
		MaxAge:           86400,
	}))

	return app
}

func startServer(app *fiber.App, port string) {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)

	go func() {
		addr := fmt.Sprintf(":%s", port)
		log.Info().Str("addr", addr).Msg("Server auth listening")
		if err := app.Listen(addr); err != nil {
			log.Fatal().Err(err).Msg("Server berhenti tidak terduga")
		}
	}()

	<-quit
	log.Info().Msg("Menerima sinyal shutdown, menutup server dengan graceful...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := app.ShutdownWithContext(ctx); err != nil {
		log.Error().Err(err).Msg("Gagal graceful shutdown")
	}

	log.Info().Msg("Server berhenti. Selesai.")
}

func errorHandler(c *fiber.Ctx, err error) error {
	code := fiber.StatusInternalServerError
	msg := "Terjadi kesalahan pada server"

	if e, ok := err.(*fiber.Error); ok {
		code = e.Code
		msg = e.Message
	}

	log.Error().
		Int("status", code).
		Str("path", c.Path()).
		Str("method", c.Method()).
		Err(err).
		Msg("Request error")

	return c.Status(code).JSON(fiber.Map{
		"success": false,
		"message": msg,
	})
}