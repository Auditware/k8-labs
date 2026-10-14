package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"

	"backend/internal/drivers"
)

// Admin vetting surface for the research driver catalog. An admin lists the
// queue, inspects a module (metadata + raw download), and allow/denies it into
// the shared catalog. Mirrors the /admin/labs handler shapes. See
// docs/architecture/research-driver-catalog.md.

// adminListDrivers godoc
// @Summary     List research drivers for vetting
// @Description Filter by ?status=pending|approved|denied (default pending). Admin only.
// @Tags        admin
// @Produce     json
// @Success     200 {array} drivers.Driver
// @Security    BearerAuth
// @Router      /admin/drivers [get]
func adminListDrivers(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		status := r.URL.Query().Get("status")
		if status == "" {
			status = drivers.StatusPending
		}
		switch status {
		case drivers.StatusPending, drivers.StatusApproved, drivers.StatusDenied:
		default:
			http.Error(w, "invalid status filter", http.StatusBadRequest)
			return
		}
		list, err := drivers.ListByStatus(r.Context(), pool, status)
		if err != nil {
			http.Error(w, "failed to list drivers", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(list)
	}
}

// adminDriverDecision godoc
// @Summary     Vet a research driver
// @Description Approve (optionally into the shared catalog) or deny an uploaded module. Admin only.
// @Tags        admin
// @Accept      json
// @Produce     json
// @Param       id path string true "Driver ID"
// @Success     200 {object} map[string]string
// @Failure     400 {string} string "bad request"
// @Failure     404 {string} string "not found"
// @Security    BearerAuth
// @Router      /admin/drivers/{id}/decision [post]
func adminDriverDecision(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		var body struct {
			// Decision is "approve" or "deny". Catalog promotes an approved module
			// to the shared catalog (visible to every research user); false keeps
			// it usable only by its uploader.
			Decision string `json:"decision"`
			Catalog  bool   `json:"catalog"`
			Reason   string `json:"reason"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		var status, visibility string
		switch body.Decision {
		case "approve":
			status = drivers.StatusApproved
			if body.Catalog {
				visibility = drivers.VisibilityCatalog
			} else {
				visibility = drivers.VisibilityPrivate
			}
		case "deny":
			status = drivers.StatusDenied
		default:
			http.Error(w, `decision must be "approve" or "deny"`, http.StatusBadRequest)
			return
		}

		actor := contextUser(r)
		err := drivers.SetDecision(r.Context(), pool, id, drivers.Decision{
			Status: status, Visibility: visibility, Reason: body.Reason, VettedBy: actor.ID,
		})
		if errors.Is(err, drivers.ErrNotFound) {
			http.Error(w, "unknown driver", http.StatusNotFound)
			return
		}
		if err != nil {
			http.Error(w, "failed to record decision", http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}
}

// adminDownloadDriver serves a driver's raw .ko for out-of-band inspection on an
// admin's own analysis box. Admin only. Deliberately unannotated for swaggo - it
// returns octet-stream bytes, not JSON.
func adminDownloadDriver(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		data, filename, err := drivers.GetBytes(r.Context(), pool, id)
		if errors.Is(err, drivers.ErrNotFound) {
			http.NotFound(w, r)
			return
		}
		if err != nil {
			http.Error(w, "failed to load module", http.StatusInternalServerError)
			return
		}
		// Never let a browser sniff or render this; it is an untrusted binary an
		// admin is downloading to inspect, not to open in-page.
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
		w.Header().Set("Cache-Control", "private, no-store")
		w.Write(data)
	}
}
