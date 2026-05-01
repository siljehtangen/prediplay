package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"prediplay/backend/bzzoiro"
	"prediplay/backend/config"
	"prediplay/backend/db"
	"prediplay/backend/handlers"
	"prediplay/backend/services"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/rs/cors"
)

func main() {
	cfg := config.Load()

	database := db.Init(cfg.DatabasePath)

	bzzoiroClient := bzzoiro.New(cfg.BzzoiroBaseURL, cfg.BzzoiroToken)
	predSvc := services.NewPredictionService(database, bzzoiroClient)
	go runSync(predSvc)

	h := handlers.New(bzzoiroClient, predSvc)

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Compress(5))

	c := cors.New(cors.Options{
		AllowedOrigins:   cfg.CORSOrigins,
		AllowedMethods:   []string{"GET", "POST", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Content-Type", "Authorization"},
		AllowCredentials: true,
	})
	r.Use(c.Handler)

	r.Route("/api", func(r chi.Router) {
		r.Get("/leagues", h.GetLeagues)
		r.Get("/teams", h.GetTeams)
		r.Get("/events", h.GetEvents)
		r.Get("/live", h.GetLive)
		r.Get("/predictions", h.GetPredictions)
		r.Get("/players", h.GetPlayers)
		r.Get("/players/{id}/photo", h.GetPlayerPhoto)
		r.Get("/players/{id}/stats", h.GetPlayerStats)

		r.Get("/predict/player/{id}", h.GetPlayerPrediction)
		r.Get("/dashboard", h.GetDashboard)
		r.Get("/predict/top", h.GetTopPredictions)
		r.Get("/predict/redflags", h.GetRedFlags)
		r.Get("/predict/benchwarmers", h.GetBenchwarmers)
		r.Get("/predict/synergy", h.GetSynergy)
		r.Get("/predict/momentum", h.GetMomentum)
	})

	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Printf("Prediplay backend listening on :%s", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server…")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}
	log.Println("Server stopped")
}

// runSync runs SyncPlayers in the background, recovering from any panic so it
// does not bring down the server.
func runSync(svc *services.PredictionService) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[sync] panic recovered: %v", r)
		}
	}()
	svc.SyncPlayers()
}
