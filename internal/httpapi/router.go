package httpapi

import (
	"context"
	"io/fs"
	"net/http"
	"time"

	"desksharing/internal/auth"
	"desksharing/internal/booking"
	"desksharing/internal/catalog"
	"desksharing/web"

	"github.com/jackc/pgx/v5/pgxpool"
)

func NewRouter(pool *pgxpool.Pool, authSvc *auth.Service, catalogSvc *catalog.Service, bookingSvc *booking.Service) http.Handler {
	mux := http.NewServeMux()
	authH := AuthHandlers{Service: authSvc}
	catH := CatalogHandlers{Service: catalogSvc}
	bookH := BookingHandlers{Service: bookingSvc}

	assets, err := fs.Sub(web.FS, "assets")
	if err != nil {
		panic(err)
	}
	mux.Handle("GET /assets/", http.StripPrefix("/assets/", http.FileServerFS(assets)))

	spa := serveIndex
	mux.HandleFunc("GET /{$}", spa)
	mux.HandleFunc("GET /login", spa)
	mux.HandleFunc("GET /register", spa)
	mux.HandleFunc("GET /verify", spa)
	mux.HandleFunc("GET /app", spa)
	mux.HandleFunc("GET /app/locations", spa)
	mux.HandleFunc("GET /app/place-types", spa)
	mux.HandleFunc("GET /app/equipments", spa)
	mux.HandleFunc("GET /app/bookings", spa)

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		pingCtx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		status := http.StatusOK
		body := map[string]string{"status": "ok"}
		if err := pool.Ping(pingCtx); err != nil {
			status = http.StatusServiceUnavailable
			body = map[string]string{"status": "degraded", "database": "unavailable"}
		}
		writeJSON(w, status, body)
	})

	mux.HandleFunc("GET /api", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"service": "desksharing",
			"endpoints": []string{
				"GET /api",
				"GET /healthz",
				"auth, catalog, equipments, bookings",
			},
		})
	})

	mux.HandleFunc("POST /api/v1/auth/register", authH.RegisterTenant)
	mux.HandleFunc("GET /api/v1/auth/verify", authH.Verify)
	mux.HandleFunc("POST /api/v1/auth/verify", authH.Verify)
	mux.HandleFunc("POST /api/v1/auth/login", authH.Login)
	mux.HandleFunc("POST /api/v1/auth/logout", authH.Logout)
	mux.HandleFunc("GET /api/v1/auth/me", RequireAuth(authSvc, authH.Me))

	tenant := withAuth(authSvc, RequireTenant)
	admin := withAuth(authSvc, RequireTenantAdmin)

	mux.HandleFunc("GET /api/v1/locations", tenant(catH.ListLocations))
	mux.HandleFunc("POST /api/v1/locations", admin(catH.CreateLocation))
	mux.HandleFunc("PATCH /api/v1/locations/{id}", admin(catH.UpdateLocation))
	mux.HandleFunc("DELETE /api/v1/locations/{id}", admin(catH.DeleteLocation))

	mux.HandleFunc("GET /api/v1/locations/{locationID}/buildings", tenant(catH.ListBuildings))
	mux.HandleFunc("POST /api/v1/locations/{locationID}/buildings", admin(catH.CreateBuilding))
	mux.HandleFunc("DELETE /api/v1/buildings/{id}", admin(catH.DeleteBuilding))

	mux.HandleFunc("GET /api/v1/buildings/{buildingID}/floors", tenant(catH.ListFloors))
	mux.HandleFunc("POST /api/v1/buildings/{buildingID}/floors", admin(catH.CreateFloor))
	mux.HandleFunc("DELETE /api/v1/floors/{id}", admin(catH.DeleteFloor))

	mux.HandleFunc("GET /api/v1/floors/{floorID}/rooms", tenant(catH.ListRooms))
	mux.HandleFunc("POST /api/v1/floors/{floorID}/rooms", admin(catH.CreateRoom))
	mux.HandleFunc("GET /api/v1/floors/{floorID}/layout", tenant(catH.GetFloorLayout))
	mux.HandleFunc("POST /api/v1/floors/{floorID}/plan", admin(catH.UploadFloorPlan))
	mux.HandleFunc("GET /api/v1/floor-plans/{id}/image", tenant(catH.FloorPlanImage))
	mux.HandleFunc("PATCH /api/v1/rooms/{id}/rectangle", admin(catH.SetRoomRectangle))
	mux.HandleFunc("PATCH /api/v1/places/{id}/rectangle", admin(catH.SetPlaceRectangle))
	mux.HandleFunc("PATCH /api/v1/rooms/{id}", admin(catH.PatchRoom))
	mux.HandleFunc("DELETE /api/v1/rooms/{id}", admin(catH.DeleteRoom))

	mux.HandleFunc("GET /api/v1/rooms/{roomID}/places", tenant(catH.ListPlaces))
	mux.HandleFunc("POST /api/v1/rooms/{roomID}/places", admin(catH.CreatePlace))
	mux.HandleFunc("POST /api/v1/rooms/{roomID}/places/bulk", admin(catH.CreatePlacesBulk))
	mux.HandleFunc("DELETE /api/v1/places/{id}", admin(catH.DeletePlace))
	mux.HandleFunc("PUT /api/v1/places/{id}/equipments", admin(catH.SetPlaceEquipments))

	mux.HandleFunc("GET /api/v1/place-types", tenant(catH.ListPlaceTypes))
	mux.HandleFunc("POST /api/v1/place-types", admin(catH.CreatePlaceType))
	mux.HandleFunc("DELETE /api/v1/place-types/{id}", admin(catH.DeletePlaceType))

	mux.HandleFunc("GET /api/v1/equipments", tenant(catH.ListEquipments))
	mux.HandleFunc("POST /api/v1/equipments", admin(catH.CreateEquipment))
	mux.HandleFunc("DELETE /api/v1/equipments/{id}", admin(catH.DeleteEquipment))
	mux.HandleFunc("POST /api/v1/catalog/seed-demo", admin(catH.SeedDemo))

	mux.HandleFunc("GET /api/v1/rooms", tenant(catH.ListAllRooms))

	mux.HandleFunc("GET /api/v1/bookings/search", tenant(bookH.Search))
	mux.HandleFunc("GET /api/v1/bookings/mine", tenant(bookH.ListMine))
	mux.HandleFunc("GET /api/v1/bookings/my-week", tenant(bookH.MyWeek))
	mux.HandleFunc("GET /api/v1/bookings/room-week", tenant(bookH.RoomWeek))
	mux.HandleFunc("GET /api/v1/bookings/place-day", tenant(bookH.PlaceDay))
	mux.HandleFunc("POST /api/v1/bookings", tenant(bookH.Create))
	mux.HandleFunc("POST /api/v1/bookings/{id}/check-in", tenant(bookH.CheckIn))
	mux.HandleFunc("POST /api/v1/bookings/{id}/check-out", tenant(bookH.CheckOut))
	mux.HandleFunc("POST /api/v1/bookings/{id}/cancel", tenant(bookH.Cancel))

	return mux
}

func serveIndex(w http.ResponseWriter, r *http.Request) {
	data, err := web.FS.ReadFile("index.html")
	if err != nil {
		http.Error(w, "index not found", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}
