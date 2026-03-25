package main

import (
	"log"
	"net/http"

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
	go predSvc.SyncPlayers() // runs in background; server starts immediately

	h := handlers.New(bzzoiroClient, predSvc)

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Compress(5))

	// CORS: allow Angular dev server
	c := cors.New(cors.Options{
		AllowedOrigins:   []string{"http://localhost:4200", "http://localhost:3000"},
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

	log.Printf("Prediplay backend listening on :%s", cfg.Port)
	if err := http.ListenAndServe(":"+cfg.Port, r); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
