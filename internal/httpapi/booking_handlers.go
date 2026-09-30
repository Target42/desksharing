package httpapi

import (
	"net/http"
	"time"

	"desksharing/internal/auth"
	"desksharing/internal/booking"

	"github.com/google/uuid"
)

type BookingHandlers struct {
	Service *booking.Service
}

func (h BookingHandlers) tenantUser(r *http.Request) (uuid.UUID, uuid.UUID) {
	user, _ := UserFromContext(r.Context())
	return *user.TenantID, user.ID
}

func (h BookingHandlers) Search(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := h.tenantUser(r)
	q := r.URL.Query()

	startsAt, err := time.Parse(time.RFC3339, q.Get("starts_at"))
	if err != nil {
		writeError(w, auth.ErrInvalidInput("starts_at must be RFC3339"))
		return
	}
	endsAt, err := time.Parse(time.RFC3339, q.Get("ends_at"))
	if err != nil {
		writeError(w, auth.ErrInvalidInput("ends_at must be RFC3339"))
		return
	}

	filter := booking.SearchFilter{
		StartsAt:      startsAt,
		EndsAt:        endsAt,
		OnlyAvailable: q.Get("only_available") == "1" || q.Get("only_available") == "true",
	}
	if raw := q.Get("location_id"); raw != "" {
		id, err := uuid.Parse(raw)
		if err != nil {
			writeError(w, auth.ErrInvalidInput("invalid location_id"))
			return
		}
		filter.LocationID = &id
	}
	if raw := q.Get("place_type_id"); raw != "" {
		id, err := uuid.Parse(raw)
		if err != nil {
			writeError(w, auth.ErrInvalidInput("invalid place_type_id"))
			return
		}
		filter.PlaceTypeID = &id
	}
	equipmentIDs, err := booking.ParseEquipmentIDs(q.Get("equipment_ids"))
	if err != nil {
		writeError(w, err)
		return
	}
	filter.EquipmentIDs = equipmentIDs

	items, err := h.Service.SearchPlaces(r.Context(), tenantID, filter)
	if err != nil {
		writeError(w, err)
		return
	}
	if items == nil {
		items = []booking.AvailablePlace{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h BookingHandlers) Create(w http.ResponseWriter, r *http.Request) {
	tenantID, userID := h.tenantUser(r)
	var body struct {
		PlaceID    string `json:"place_id"`
		ResourceID string `json:"resource_id"`
		Mode       string `json:"mode"`
		StartsAt   string `json:"starts_at"`
		EndsAt     string `json:"ends_at"`
		StartDate  string `json:"start_date"`
		EndDate    string `json:"end_date"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, auth.ErrInvalidInput("invalid JSON body"))
		return
	}
	if body.ResourceID != "" {
		h.createResource(w, r, tenantID, userID, body.ResourceID, body.Mode, body.StartsAt, body.EndsAt, body.StartDate, body.EndDate)
		return
	}
	placeID, err := uuid.Parse(body.PlaceID)
	if err != nil {
		writeError(w, auth.ErrInvalidInput("place_id must be a UUID"))
		return
	}
	startsAt, err := time.Parse(time.RFC3339, body.StartsAt)
	if err != nil {
		writeError(w, auth.ErrInvalidInput("starts_at must be RFC3339"))
		return
	}
	endsAt, err := time.Parse(time.RFC3339, body.EndsAt)
	if err != nil {
		writeError(w, auth.ErrInvalidInput("ends_at must be RFC3339"))
		return
	}
	item, err := h.Service.Create(r.Context(), tenantID, userID, placeID, startsAt, endsAt)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

func (h BookingHandlers) createResource(w http.ResponseWriter, r *http.Request, tenantID, userID uuid.UUID, resourceRaw, mode, startsRaw, endsRaw, startDate, endDate string) {
	resourceID, err := uuid.Parse(resourceRaw)
	if err != nil {
		writeError(w, auth.ErrInvalidInput("resource_id must be a UUID"))
		return
	}
	var startsAt, endsAt time.Time
	switch mode {
	case "daily":
		startsAt, err = time.ParseInLocation("2006-01-02", startDate, booking.BerlinLocation())
		if err != nil {
			writeError(w, auth.ErrInvalidInput("start_date must be YYYY-MM-DD"))
			return
		}
		endsAt, err = time.ParseInLocation("2006-01-02", endDate, booking.BerlinLocation())
		if err != nil {
			writeError(w, auth.ErrInvalidInput("end_date must be YYYY-MM-DD"))
			return
		}
	case "hourly":
		startsAt, err = time.Parse(time.RFC3339, startsRaw)
		if err != nil {
			writeError(w, auth.ErrInvalidInput("starts_at must be RFC3339"))
			return
		}
		endsAt, err = time.Parse(time.RFC3339, endsRaw)
		if err != nil {
			writeError(w, auth.ErrInvalidInput("ends_at must be RFC3339"))
			return
		}
	default:
		writeError(w, auth.ErrInvalidInput("mode muss hourly oder daily sein"))
		return
	}
	item, err := h.Service.CreateResourceBooking(r.Context(), tenantID, userID, resourceID, mode, startsAt, endsAt)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

func (h BookingHandlers) SearchResources(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := h.tenantUser(r)
	q := r.URL.Query()
	mode := q.Get("mode")
	var startsAt, endsAt time.Time
	var err error
	switch mode {
	case "daily":
		startsAt, err = time.ParseInLocation("2006-01-02", q.Get("start_date"), booking.BerlinLocation())
		if err != nil {
			writeError(w, auth.ErrInvalidInput("start_date must be YYYY-MM-DD"))
			return
		}
		endsAt, err = time.ParseInLocation("2006-01-02", q.Get("end_date"), booking.BerlinLocation())
		if err != nil {
			writeError(w, auth.ErrInvalidInput("end_date must be YYYY-MM-DD"))
			return
		}
		startsAt, endsAt, err = booking.DailyWindow(startsAt, endsAt)
	case "hourly":
		startsAt, err = time.Parse(time.RFC3339, q.Get("starts_at"))
		if err != nil {
			writeError(w, auth.ErrInvalidInput("starts_at must be RFC3339"))
			return
		}
		endsAt, err = time.Parse(time.RFC3339, q.Get("ends_at"))
		if err != nil {
			writeError(w, auth.ErrInvalidInput("ends_at must be RFC3339"))
			return
		}
		startsAt, endsAt, err = booking.HourlyWindow(startsAt, endsAt)
	default:
		writeError(w, auth.ErrInvalidInput("mode muss hourly oder daily sein"))
		return
	}
	if err != nil {
		writeError(w, err)
		return
	}

	filter := booking.ResourceSearchFilter{
		StartsAt:      startsAt,
		EndsAt:        endsAt,
		OnlyAvailable: q.Get("only_available") == "1" || q.Get("only_available") == "true",
	}
	if raw := q.Get("group_id"); raw != "" {
		id, err := uuid.Parse(raw)
		if err != nil {
			writeError(w, auth.ErrInvalidInput("invalid group_id"))
			return
		}
		filter.GroupID = &id
	}
	if raw := q.Get("location_id"); raw != "" {
		id, err := uuid.Parse(raw)
		if err != nil {
			writeError(w, auth.ErrInvalidInput("invalid location_id"))
			return
		}
		filter.LocationID = &id
	}
	items, err := h.Service.SearchResources(r.Context(), tenantID, filter)
	if err != nil {
		writeError(w, err)
		return
	}
	if items == nil {
		items = []booking.AvailableResource{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h BookingHandlers) MyWeek(w http.ResponseWriter, r *http.Request) {
	tenantID, userID := h.tenantUser(r)
	weekStart, err := booking.ParseWeekStart(r.URL.Query().Get("week_start"))
	if err != nil {
		writeError(w, err)
		return
	}
	item, err := h.Service.MyWeek(r.Context(), tenantID, userID, weekStart)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (h BookingHandlers) PlaceDay(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := h.tenantUser(r)
	placeID, err := uuid.Parse(r.URL.Query().Get("place_id"))
	if err != nil {
		writeError(w, auth.ErrInvalidInput("place_id must be a UUID"))
		return
	}
	dateRaw := r.URL.Query().Get("date")
	if dateRaw == "" {
		writeError(w, auth.ErrInvalidInput("date must be YYYY-MM-DD"))
		return
	}
	day, err := time.ParseInLocation("2006-01-02", dateRaw, booking.BerlinLocation())
	if err != nil {
		writeError(w, auth.ErrInvalidInput("date must be YYYY-MM-DD"))
		return
	}
	item, err := h.Service.PlaceDay(r.Context(), tenantID, placeID, day)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (h BookingHandlers) RoomWeek(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := h.tenantUser(r)
	roomID, err := uuid.Parse(r.URL.Query().Get("room_id"))
	if err != nil {
		writeError(w, auth.ErrInvalidInput("room_id must be a UUID"))
		return
	}
	weekStart, err := booking.ParseWeekStart(r.URL.Query().Get("week_start"))
	if err != nil {
		writeError(w, err)
		return
	}
	item, err := h.Service.RoomWeek(r.Context(), tenantID, roomID, weekStart)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (h BookingHandlers) ListMine(w http.ResponseWriter, r *http.Request) {
	tenantID, userID := h.tenantUser(r)
	items, err := h.Service.ListMine(r.Context(), tenantID, userID)
	if err != nil {
		writeError(w, err)
		return
	}
	if items == nil {
		items = []booking.Booking{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h BookingHandlers) CheckIn(w http.ResponseWriter, r *http.Request) {
	tenantID, userID := h.tenantUser(r)
	id, err := parseID(r, "id")
	if err != nil {
		writeError(w, err)
		return
	}
	item, err := h.Service.CheckIn(r.Context(), tenantID, userID, id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (h BookingHandlers) CheckOut(w http.ResponseWriter, r *http.Request) {
	tenantID, userID := h.tenantUser(r)
	id, err := parseID(r, "id")
	if err != nil {
		writeError(w, err)
		return
	}
	item, err := h.Service.CheckOut(r.Context(), tenantID, userID, id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (h BookingHandlers) Cancel(w http.ResponseWriter, r *http.Request) {
	tenantID, userID := h.tenantUser(r)
	id, err := parseID(r, "id")
	if err != nil {
		writeError(w, err)
		return
	}
	item, err := h.Service.Cancel(r.Context(), tenantID, userID, id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}
