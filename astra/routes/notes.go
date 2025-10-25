// astra/routes/notes.go
package routes

import (
	"astra/astra/config"
	"astra/astra/controllers"
	"astra/astra/middlewares"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func handleNotesJSON(handler func(r *http.Request) (any, int, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		res, status, err := handler(r)
		if err != nil {
			http.Error(w, err.Error(), status)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		json.NewEncoder(w).Encode(res)
	}
}

func NotesRoutes(ctrl *controllers.NotesController, cfg config.Config) chi.Router {
	r := chi.NewRouter()
	r.Group(func(gr chi.Router) {
		gr.Use(middlewares.AuthMiddleware(cfg))

		// Create note
		gr.Post("/", handleNotesJSON(func(r *http.Request) (any, int, error) {
			var req struct {
				UserID    int    `json:"user_id"`
				Title     string `json:"title"`
				Content   string `json:"content"`
				Favourite *bool  `json:"favourite"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				return nil, http.StatusBadRequest, err
			}
			favourite := false
			if req.Favourite != nil {
				favourite = *req.Favourite
			}
			note, err := ctrl.CreateNote(r.Context(), req.UserID, req.Title, req.Content, favourite)
			if err != nil {
				return nil, http.StatusInternalServerError, err
			}
			return note, http.StatusCreated, nil
		}))

		// List notes by user
		gr.Get("/user/{user_id}", handleNotesJSON(func(r *http.Request) (any, int, error) {
			idStr := chi.URLParam(r, "user_id")
			id, err := strconv.Atoi(idStr)
			if err != nil {
				return nil, http.StatusBadRequest, err
			}
			notes, err := ctrl.GetAllNotesByUser(r.Context(), id)
			if err != nil {
				return nil, http.StatusInternalServerError, err
			}
			return notes, http.StatusOK, nil
		}))

		// Get single note
		gr.Get("/{id}", handleNotesJSON(func(r *http.Request) (any, int, error) {
			idStr := chi.URLParam(r, "id")
			id, err := uuid.Parse(idStr)
			if err != nil {
				return nil, http.StatusBadRequest, err
			}
			note, err := ctrl.GetNoteByID(r.Context(), id)
			if err != nil {
				return nil, http.StatusInternalServerError, err
			}
			if note == nil {
				return nil, http.StatusNotFound, nil
			}
			return note, http.StatusOK, nil
		}))

		// Update note (including favourite)
		gr.Put("/{id}", handleNotesJSON(func(r *http.Request) (any, int, error) {
			idStr := chi.URLParam(r, "id")
			id, err := uuid.Parse(idStr)
			if err != nil {
				return nil, http.StatusBadRequest, err
			}
			var req struct {
				Title     *string `json:"title"`
				Content   *string `json:"content"`
				Favourite *bool   `json:"favourite"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				return nil, http.StatusBadRequest, err
			}
			updates := map[string]interface{}{}
			if req.Title != nil {
				updates["title"] = *req.Title
			}
			if req.Content != nil {
				updates["content"] = *req.Content
			}
			if req.Favourite != nil {
				updates["favourite"] = *req.Favourite
			}
			if len(updates) == 0 {
				return nil, http.StatusBadRequest, nil
			}
			if err := ctrl.UpdateNote(r.Context(), id, updates); err != nil {
				return nil, http.StatusInternalServerError, err
			}
			return map[string]string{"status": "ok"}, http.StatusOK, nil
		}))

		// Delete note
		gr.Delete("/{id}", handleNotesJSON(func(r *http.Request) (any, int, error) {
			idStr := chi.URLParam(r, "id")
			id, err := uuid.Parse(idStr)
			if err != nil {
				return nil, http.StatusBadRequest, err
			}
			if err := ctrl.DeleteNote(r.Context(), id); err != nil {
				return nil, http.StatusInternalServerError, err
			}
			return map[string]string{"status": "deleted"}, http.StatusOK, nil
		}))
	})
	return r
}
