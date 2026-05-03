package catalog

import (
	"net/http"

	"github.com/sikozonpc/cinema/internal/platform/web"
)

type Film struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Rows        int    `json:"rows"`
	SeatsPerRow int    `json:"seats_per_row"`
}

type Library struct {
	films []Film
}

func NewLibrary() *Library {
	return &Library{
		films: []Film{
			{ID: "inception", Title: "Inception", Rows: 5, SeatsPerRow: 8},
			{ID: "dune", Title: "Dune: Part Two", Rows: 4, SeatsPerRow: 6},
		},
	}
}

func (library *Library) All() []Film {
	films := make([]Film, len(library.films))
	copy(films, library.films)
	return films
}

type HTTPHandler struct {
	library *Library
}

func NewHTTPHandler(library *Library) *HTTPHandler {
	return &HTTPHandler{library: library}
}

func (handler *HTTPHandler) Mount(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/films", handler.listFilms)
}

func (handler *HTTPHandler) listFilms(w http.ResponseWriter, r *http.Request) {
	web.JSON(w, http.StatusOK, map[string]any{
		"films": handler.library.All(),
	})
}
