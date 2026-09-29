package httpapi

import (
	"fmt"
	"net/http"
	"time"

	"desksharing/internal/auth"
	"desksharing/internal/catalog"

	"github.com/google/uuid"
)

type CatalogHandlers struct {
	Service *catalog.Service
}

func (h CatalogHandlers) tenantID(r *http.Request) uuid.UUID {
	user, _ := UserFromContext(r.Context())
	return *user.TenantID
}

func parseID(r *http.Request, name string) (uuid.UUID, error) {
	id, err := uuid.Parse(r.PathValue(name))
	if err != nil {
		return uuid.Nil, auth.ErrInvalidInput(name + " must be a UUID")
	}
	return id, nil
}

func (h CatalogHandlers) ListAllRooms(w http.ResponseWriter, r *http.Request) {
	items, err := h.Service.ListAllRooms(r.Context(), h.tenantID(r))
	if err != nil {
		writeError(w, err)
		return
	}
	if items == nil {
		items = []catalog.RoomSummary{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h CatalogHandlers) ListLocations(w http.ResponseWriter, r *http.Request) {
	items, err := h.Service.ListLocations(r.Context(), h.tenantID(r))
	if err != nil {
		writeError(w, err)
		return
	}
	if items == nil {
		items = []catalog.Location{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h CatalogHandlers) CreateLocation(w http.ResponseWriter, r *http.Request) {
	var body catalog.CreateLocationInput
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, auth.ErrInvalidInput("invalid JSON body"))
		return
	}
	item, err := h.Service.CreateLocation(r.Context(), h.tenantID(r), body)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

func (h CatalogHandlers) UpdateLocation(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		writeError(w, err)
		return
	}
	var body catalog.CreateLocationInput
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, auth.ErrInvalidInput("invalid JSON body"))
		return
	}
	item, err := h.Service.UpdateLocation(r.Context(), h.tenantID(r), id, body)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (h CatalogHandlers) DeleteLocation(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		writeError(w, err)
		return
	}
	if err := h.Service.DeleteLocation(r.Context(), h.tenantID(r), id); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (h CatalogHandlers) ListBuildings(w http.ResponseWriter, r *http.Request) {
	locationID, err := parseID(r, "locationID")
	if err != nil {
		writeError(w, err)
		return
	}
	items, err := h.Service.ListBuildings(r.Context(), h.tenantID(r), locationID)
	if err != nil {
		writeError(w, err)
		return
	}
	if items == nil {
		items = []catalog.Building{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h CatalogHandlers) CreateBuilding(w http.ResponseWriter, r *http.Request) {
	locationID, err := parseID(r, "locationID")
	if err != nil {
		writeError(w, err)
		return
	}
	var body struct {
		Name    string `json:"name"`
		Code    string `json:"code"`
		Address string `json:"address"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, auth.ErrInvalidInput("invalid JSON body"))
		return
	}
	item, err := h.Service.CreateBuilding(r.Context(), h.tenantID(r), locationID, body.Name, body.Code, body.Address)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

func (h CatalogHandlers) DeleteBuilding(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		writeError(w, err)
		return
	}
	if err := h.Service.DeleteBuilding(r.Context(), h.tenantID(r), id); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (h CatalogHandlers) ListFloors(w http.ResponseWriter, r *http.Request) {
	buildingID, err := parseID(r, "buildingID")
	if err != nil {
		writeError(w, err)
		return
	}
	items, err := h.Service.ListFloors(r.Context(), h.tenantID(r), buildingID)
	if err != nil {
		writeError(w, err)
		return
	}
	if items == nil {
		items = []catalog.Floor{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h CatalogHandlers) CreateFloor(w http.ResponseWriter, r *http.Request) {
	buildingID, err := parseID(r, "buildingID")
	if err != nil {
		writeError(w, err)
		return
	}
	var body struct {
		Name  string `json:"name"`
		Level int    `json:"level"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, auth.ErrInvalidInput("invalid JSON body"))
		return
	}
	item, err := h.Service.CreateFloor(r.Context(), h.tenantID(r), buildingID, body.Name, body.Level)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

func (h CatalogHandlers) DeleteFloor(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		writeError(w, err)
		return
	}
	if err := h.Service.DeleteFloor(r.Context(), h.tenantID(r), id); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (h CatalogHandlers) ListRooms(w http.ResponseWriter, r *http.Request) {
	floorID, err := parseID(r, "floorID")
	if err != nil {
		writeError(w, err)
		return
	}
	items, err := h.Service.ListRooms(r.Context(), h.tenantID(r), floorID)
	if err != nil {
		writeError(w, err)
		return
	}
	if items == nil {
		items = []catalog.Room{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h CatalogHandlers) CreateRoom(w http.ResponseWriter, r *http.Request) {
	floorID, err := parseID(r, "floorID")
	if err != nil {
		writeError(w, err)
		return
	}
	var body struct {
		Number     string `json:"number"`
		Name       string `json:"name"`
		IsBookable *bool  `json:"is_bookable"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, auth.ErrInvalidInput("invalid JSON body"))
		return
	}
	isBookable := true
	if body.IsBookable != nil {
		isBookable = *body.IsBookable
	}
	item, err := h.Service.CreateRoom(r.Context(), h.tenantID(r), floorID, body.Number, body.Name, isBookable)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

func (h CatalogHandlers) PatchRoom(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		writeError(w, err)
		return
	}
	var body struct {
		IsBookable *bool `json:"is_bookable"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, auth.ErrInvalidInput("invalid JSON body"))
		return
	}
	if body.IsBookable == nil {
		writeError(w, auth.ErrInvalidInput("is_bookable is required"))
		return
	}
	item, err := h.Service.SetRoomBookable(r.Context(), h.tenantID(r), id, *body.IsBookable)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (h CatalogHandlers) DeleteRoom(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		writeError(w, err)
		return
	}
	if err := h.Service.DeleteRoom(r.Context(), h.tenantID(r), id); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (h CatalogHandlers) ListPlaceTypes(w http.ResponseWriter, r *http.Request) {
	items, err := h.Service.ListPlaceTypes(r.Context(), h.tenantID(r))
	if err != nil {
		writeError(w, err)
		return
	}
	if items == nil {
		items = []catalog.PlaceType{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h CatalogHandlers) CreatePlaceType(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, auth.ErrInvalidInput("invalid JSON body"))
		return
	}
	item, err := h.Service.CreatePlaceType(r.Context(), h.tenantID(r), body.Name, body.Description)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

func (h CatalogHandlers) DeletePlaceType(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		writeError(w, err)
		return
	}
	if err := h.Service.DeletePlaceType(r.Context(), h.tenantID(r), id); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (h CatalogHandlers) ListPlaces(w http.ResponseWriter, r *http.Request) {
	roomID, err := parseID(r, "roomID")
	if err != nil {
		writeError(w, err)
		return
	}
	items, err := h.Service.ListPlaces(r.Context(), h.tenantID(r), roomID)
	if err != nil {
		writeError(w, err)
		return
	}
	if items == nil {
		items = []catalog.Place{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h CatalogHandlers) CreatePlace(w http.ResponseWriter, r *http.Request) {
	roomID, err := parseID(r, "roomID")
	if err != nil {
		writeError(w, err)
		return
	}
	var body struct {
		Number       string   `json:"number"`
		PlaceTypeID  string   `json:"place_type_id"`
		BookingMode  string   `json:"booking_mode"`
		EquipmentIDs []string `json:"equipment_ids"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, auth.ErrInvalidInput("invalid JSON body"))
		return
	}
	placeTypeID, err := uuid.Parse(body.PlaceTypeID)
	if err != nil {
		writeError(w, auth.ErrInvalidInput("place_type_id must be a UUID"))
		return
	}
	equipmentIDs := make([]uuid.UUID, 0, len(body.EquipmentIDs))
	for _, raw := range body.EquipmentIDs {
		id, err := uuid.Parse(raw)
		if err != nil {
			writeError(w, auth.ErrInvalidInput("equipment_ids must be UUIDs"))
			return
		}
		equipmentIDs = append(equipmentIDs, id)
	}
	item, err := h.Service.CreatePlace(r.Context(), h.tenantID(r), roomID, body.Number, placeTypeID, body.BookingMode, equipmentIDs)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

func (h CatalogHandlers) CreatePlacesBulk(w http.ResponseWriter, r *http.Request) {
	roomID, err := parseID(r, "roomID")
	if err != nil {
		writeError(w, err)
		return
	}
	var body struct {
		Count        int      `json:"count"`
		PlaceTypeID  string   `json:"place_type_id"`
		BookingMode  string   `json:"booking_mode"`
		EquipmentIDs []string `json:"equipment_ids"`
		Distribute   bool     `json:"distribute"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, auth.ErrInvalidInput("invalid JSON body"))
		return
	}
	placeTypeID, err := uuid.Parse(body.PlaceTypeID)
	if err != nil {
		writeError(w, auth.ErrInvalidInput("place_type_id must be a UUID"))
		return
	}
	equipmentIDs := make([]uuid.UUID, 0, len(body.EquipmentIDs))
	for _, raw := range body.EquipmentIDs {
		id, err := uuid.Parse(raw)
		if err != nil {
			writeError(w, auth.ErrInvalidInput("equipment_ids must be UUIDs"))
			return
		}
		equipmentIDs = append(equipmentIDs, id)
	}
	items, err := h.Service.CreatePlacesBulk(r.Context(), h.tenantID(r), roomID, catalog.CreatePlacesBulkInput{
		Count:        body.Count,
		PlaceTypeID:  placeTypeID,
		BookingMode:  body.BookingMode,
		EquipmentIDs: equipmentIDs,
		Distribute:   body.Distribute,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	if items == nil {
		items = []catalog.Place{}
	}
	writeJSON(w, http.StatusCreated, map[string]any{"items": items, "count": len(items)})
}

func (h CatalogHandlers) DeletePlace(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		writeError(w, err)
		return
	}
	if err := h.Service.DeletePlace(r.Context(), h.tenantID(r), id); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (h CatalogHandlers) ListEquipments(w http.ResponseWriter, r *http.Request) {
	items, err := h.Service.ListEquipments(r.Context(), h.tenantID(r))
	if err != nil {
		writeError(w, err)
		return
	}
	if items == nil {
		items = []catalog.Equipment{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h CatalogHandlers) CreateEquipment(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name string `json:"name"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, auth.ErrInvalidInput("invalid JSON body"))
		return
	}
	item, err := h.Service.CreateEquipment(r.Context(), h.tenantID(r), body.Name)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

func (h CatalogHandlers) DeleteEquipment(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		writeError(w, err)
		return
	}
	if err := h.Service.DeleteEquipment(r.Context(), h.tenantID(r), id); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (h CatalogHandlers) SeedDemo(w http.ResponseWriter, r *http.Request) {
	item, err := h.Service.SeedDemo(r.Context(), h.tenantID(r))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

func (h CatalogHandlers) SetPlaceEquipments(w http.ResponseWriter, r *http.Request) {
	placeID, err := parseID(r, "id")
	if err != nil {
		writeError(w, err)
		return
	}
	var body struct {
		EquipmentIDs []string `json:"equipment_ids"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, auth.ErrInvalidInput("invalid JSON body"))
		return
	}
	ids := make([]uuid.UUID, 0, len(body.EquipmentIDs))
	for _, raw := range body.EquipmentIDs {
		id, err := uuid.Parse(raw)
		if err != nil {
			writeError(w, auth.ErrInvalidInput("equipment_ids must be UUIDs"))
			return
		}
		ids = append(ids, id)
	}
	item, err := h.Service.SetPlaceEquipments(r.Context(), h.tenantID(r), placeID, ids)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (h CatalogHandlers) GetFloorLayout(w http.ResponseWriter, r *http.Request) {
	floorID, err := parseID(r, "floorID")
	if err != nil {
		writeError(w, err)
		return
	}
	item, err := h.Service.GetFloorLayout(r.Context(), h.tenantID(r), floorID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (h CatalogHandlers) UploadFloorPlan(w http.ResponseWriter, r *http.Request) {
	floorID, err := parseID(r, "floorID")
	if err != nil {
		writeError(w, err)
		return
	}
	if err := r.ParseMultipartForm(12 << 20); err != nil {
		writeError(w, auth.ErrInvalidInput("invalid multipart form"))
		return
	}
	file, hdr, err := r.FormFile("file")
	if err != nil {
		writeError(w, auth.ErrInvalidInput("file is required"))
		return
	}
	defer file.Close()

	width := 0
	height := 0
	if v := r.FormValue("width_px"); v != "" {
		if _, err := fmt.Sscanf(v, "%d", &width); err != nil {
			width = 0
		}
	}
	if v := r.FormValue("height_px"); v != "" {
		if _, err := fmt.Sscanf(v, "%d", &height); err != nil {
			height = 0
		}
	}

	item, err := h.Service.UploadFloorPlan(r.Context(), h.tenantID(r), floorID, hdr.Filename, width, height, file)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

func (h CatalogHandlers) FloorPlanImage(w http.ResponseWriter, r *http.Request) {
	planID, err := parseID(r, "id")
	if err != nil {
		writeError(w, err)
		return
	}
	f, path, err := h.Service.OpenFloorPlanImage(r.Context(), h.tenantID(r), planID)
	if err != nil {
		writeError(w, err)
		return
	}
	defer f.Close()
	http.ServeContent(w, r, path, time.Time{}, f)
}

func (h CatalogHandlers) SetRoomRectangle(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		writeError(w, err)
		return
	}
	var body struct {
		Rectangle *catalog.Rect `json:"rectangle"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, auth.ErrInvalidInput("invalid JSON body"))
		return
	}
	item, err := h.Service.SetRoomRectangle(r.Context(), h.tenantID(r), id, body.Rectangle)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (h CatalogHandlers) SetPlaceRectangle(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		writeError(w, err)
		return
	}
	var body struct {
		Rectangle *catalog.Rect `json:"rectangle"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, auth.ErrInvalidInput("invalid JSON body"))
		return
	}
	item, err := h.Service.SetPlaceRectangle(r.Context(), h.tenantID(r), id, body.Rectangle)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}
