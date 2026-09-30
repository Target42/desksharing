package httpapi

import (
	"context"
	"io/fs"
	"net/http"
	"strings"
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
	mux.HandleFunc("GET /invite", spa)
	mux.HandleFunc("GET /app", spa)
	mux.HandleFunc("GET /app/users", spa)
	mux.HandleFunc("GET /app/locations", spa)
	mux.HandleFunc("GET /app/place-types", spa)
	mux.HandleFunc("GET /app/equipments", spa)
	mux.HandleFunc("GET /app/resources", spa)
	mux.HandleFunc("GET /app/bookings", spa)
	mux.HandleFunc("GET /{slug}", serveTenantEntry)

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
	mux.HandleFunc("POST /api/v1/auth/accept-invite", authH.AcceptInvite)
	mux.HandleFunc("GET /api/v1/auth/me", RequireAuth(authSvc, authH.Me))

	tenant := withAuth(authSvc, RequireTenant)
	admin := withAuth(authSvc, RequireTenantAdmin)

	memberH := MemberHandlers{Service: authSvc}
	mux.HandleFunc("GET /api/v1/users", admin(memberH.List))
	mux.HandleFunc("POST /api/v1/users", admin(memberH.Create))
	mux.HandleFunc("POST /api/v1/users/import", admin(memberH.Import))
	mux.HandleFunc("GET /api/v1/users/import-template", admin(memberH.Template))
	mux.HandleFunc("POST /api/v1/users/{id}/invite", admin(memberH.ResendInvite))
	mux.HandleFunc("PATCH /api/v1/users/{id}", admin(memberH.SetDisabled))

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

	mux.HandleFunc("GET /api/v1/resource-groups", tenant(catH.ListResourceGroups))
	mux.HandleFunc("POST /api/v1/resource-groups", admin(catH.CreateResourceGroup))
	mux.HandleFunc("DELETE /api/v1/resource-groups/{id}", admin(catH.DeleteResourceGroup))
	mux.HandleFunc("GET /api/v1/resources", tenant(catH.ListResources))
	mux.HandleFunc("POST /api/v1/resources", admin(catH.CreateResource))
	mux.HandleFunc("DELETE /api/v1/resources/{id}", admin(catH.DeleteResource))

	mux.HandleFunc("GET /api/v1/rooms", tenant(catH.ListAllRooms))

	mux.HandleFunc("GET /api/v1/bookings/search", tenant(bookH.Search))
	mux.HandleFunc("GET /api/v1/bookings/mine", tenant(bookH.ListMine))
	mux.HandleFunc("GET /api/v1/bookings/my-week", tenant(bookH.MyWeek))
	mux.HandleFunc("GET /api/v1/bookings/room-week", tenant(bookH.RoomWeek))
	mux.HandleFunc("GET /api/v1/bookings/resource-week", tenant(bookH.ResourceWeek))
	mux.HandleFunc("GET /api/v1/bookings/place-day", tenant(bookH.PlaceDay))
	mux.HandleFunc("GET /api/v1/bookings/resources", tenant(bookH.SearchResources))
	mux.HandleFunc("POST /api/v1/bookings", tenant(bookH.Create))
	mux.HandleFunc("POST /api/v1/bookings/{id}/check-in", tenant(bookH.CheckIn))
	mux.HandleFunc("POST /api/v1/bookings/{id}/check-out", tenant(bookH.CheckOut))
	mux.HandleFunc("POST /api/v1/bookings/{id}/cancel", tenant(bookH.Cancel))

	return withCanonicalTenantPath(mux)
}

func serveTenantEntry(w http.ResponseWriter, r *http.Request) {
	if !auth.ValidTenantSlug(r.PathValue("slug")) {
		http.NotFound(w, r)
		return
	}
	serveIndex(w, r)
}

// canonicalTenantPath redirects /Acme and /acme/ to /acme. Reserved app paths stay untouched.
func canonicalTenantPath(path string) (string, bool) {
	if len(path) < 2 || !strings.HasPrefix(path, "/") {
		return "", false
	}
	raw := strings.TrimSuffix(path, "/")
	if strings.Contains(raw[1:], "/") {
		return "", false
	}
	slug := strings.ToLower(raw[1:])
	if !auth.ValidTenantSlug(slug) {
		return "", false
	}
	want := "/" + slug
	if path == want {
		return "", false
	}
	return want, true
}

func withCanonicalTenantPath(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet || r.Method == http.MethodHead {
			if dest, ok := canonicalTenantPath(r.URL.Path); ok {
				http.Redirect(w, r, dest, http.StatusPermanentRedirect)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
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
