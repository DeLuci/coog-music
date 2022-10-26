package main

import (
	"net/http"

	// "github.com/DeLuci/coog-music/internal/config"
	"github.com/DeLuci/coog-music/internal/handlers"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func routes() http.Handler {
	mux := chi.NewRouter()
	mux.Use(middleware.Recoverer)
	mux.Use(NoSurf)
	mux.Use(SessionLoad)

	// This one works
	mux.Get("/artists", handlers.Repo.GetArtists)

	// Need to finish handlers and maybe adjust routing
	mux.Post("/song", handlers.Repo.AddSong)
	mux.Post("/user", handlers.Repo.AddUser)
	mux.Post("/song/{playlistId}", handlers.Repo.AddSongToPlaylist)
	mux.Get("/song", handlers.Repo.PlaySong)
	mux.Post("/album/{songid}", handlers.Repo.AddSongToAlbum)

	return mux
}
