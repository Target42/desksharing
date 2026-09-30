package booking

import (
	"context"
	"fmt"
	"time"

	"desksharing/internal/auth"

	"github.com/google/uuid"
)

const (
	businessStartHour = 8
	businessEndHour   = 18
)

type DayStatus string

const (
	DayFree    DayStatus = "free"
	DayPartial DayStatus = "partial"
	DayFull    DayStatus = "full"
	DayBlocked DayStatus = "blocked"
)

type RoomWeekOverview struct {
	WeekStart string         `json:"week_start"`
	WeekEnd   string         `json:"week_end"`
	Days      []string       `json:"days"`
	Room      RoomInfo       `json:"room"`
	Places    []PlaceWeekRow `json:"places"`
}

type RoomInfo struct {
	ID    uuid.UUID `json:"id"`
	Label string    `json:"label"`
	Name  string    `json:"name"`
}

type PlaceWeekRow struct {
	ID          uuid.UUID      `json:"id"`
	Label       string         `json:"label"`
	BookingMode string         `json:"booking_mode"`
	Days        []PlaceDayCell `json:"days"`
}

type PlaceDayCell struct {
	Date         string       `json:"date"`
	Status       DayStatus    `json:"status"`
	BookingCount int          `json:"booking_count"`
	Segments     []DaySegment `json:"segments,omitempty"`
}

type DaySegment struct {
	Kind     string  `json:"kind"` // busy | free
	StartPct float64 `json:"start_pct"`
	EndPct   float64 `json:"end_pct"`
}

type TimeSlot struct {
	Start    string    `json:"start"`
	End      string    `json:"end"`
	StartsAt time.Time `json:"starts_at"`
	EndsAt   time.Time `json:"ends_at"`
}

type PlaceDayDetail struct {
	PlaceID       uuid.UUID    `json:"place_id"`
	Label         string       `json:"label"`
	BookingMode   string       `json:"booking_mode"`
	Date          string       `json:"date"`
	BusinessStart string       `json:"business_start"`
	BusinessEnd   string       `json:"business_end"`
	Status        DayStatus    `json:"status"`
	Busy          []TimeSlot   `json:"busy"`
	Free          []TimeSlot   `json:"free"`
	Segments      []DaySegment `json:"segments"`
}

type MyWeekCalendar struct {
	WeekStart string          `json:"week_start"`
	WeekEnd   string          `json:"week_end"`
	Days      []MyCalendarDay `json:"days"`
}

type MyCalendarDay struct {
	Date     string    `json:"date"`
	Bookings []Booking `json:"bookings"`
}

type timeInterval struct {
	placeID uuid.UUID
	start   time.Time
	end     time.Time
	mode    string
}

func BerlinLocation() *time.Location {
	return berlinLocation()
}

func onQuarterHour(t time.Time) bool {
	local := t.In(berlinLocation())
	return local.Second() == 0 && local.Nanosecond() == 0 && local.Minute()%15 == 0
}

func berlinLocation() *time.Location {
	loc, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		return time.Local
	}
	return loc
}

// MondayOf returns Monday 00:00 of the week containing date (in Berlin).
func MondayOf(t time.Time) time.Time {
	loc := berlinLocation()
	t = t.In(loc)
	day := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, loc)
	wd := int(day.Weekday())
	if wd == 0 {
		wd = 7
	}
	return day.AddDate(0, 0, -(wd - 1))
}

func ParseWeekStart(raw string) (time.Time, error) {
	if raw == "" {
		return MondayOf(time.Now()), nil
	}
	t, err := time.ParseInLocation("2006-01-02", raw, berlinLocation())
	if err != nil {
		return time.Time{}, auth.ErrInvalidInput("week_start must be YYYY-MM-DD")
	}
	return MondayOf(t), nil
}

func (s *Service) MyWeek(ctx context.Context, tenantID, userID uuid.UUID, weekStart time.Time) (*MyWeekCalendar, error) {
	_ = s.ReleaseNoShows(ctx, tenantID)
	loc := berlinLocation()
	weekStart = MondayOf(weekStart)
	weekEnd := weekStart.AddDate(0, 0, 7)

	rows, err := s.pool.Query(ctx, bookingListFrom+`
		WHERE bk.tenant_id = $1
		  AND bk.user_id = $2
		  AND bk.status IN ('booked', 'checked_in', 'checked_out', 'possibly_free')
		  AND bk.starts_at < $4
		  AND bk.ends_at > $3
		ORDER BY bk.starts_at
	`, tenantID, userID, weekStart.UTC(), weekEnd.UTC())
	if err != nil {
		return nil, fmt.Errorf("my week: %w", err)
	}
	defer rows.Close()

	byDay := map[string][]Booking{}
	days := make([]string, 0, 7)
	for i := 0; i < 7; i++ {
		d := weekStart.AddDate(0, 0, i).Format("2006-01-02")
		days = append(days, d)
		byDay[d] = []Booking{}
	}

	now := time.Now().UTC()
	for rows.Next() {
		var b Booking
		if err := scanBookingList(rows, &b); err != nil {
			return nil, err
		}
		decorateBooking(&b, now)
		// attach booking to each overlapping calendar day in the week
		for i := 0; i < 7; i++ {
			dayStart := weekStart.AddDate(0, 0, i)
			dayEnd := dayStart.AddDate(0, 0, 1)
			if b.StartsAt.Before(dayEnd.UTC()) && b.EndsAt.After(dayStart.UTC()) {
				key := dayStart.Format("2006-01-02")
				byDay[key] = append(byDay[key], b)
			}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	outDays := make([]MyCalendarDay, 0, 7)
	for _, d := range days {
		outDays = append(outDays, MyCalendarDay{Date: d, Bookings: byDay[d]})
	}
	_ = loc
	return &MyWeekCalendar{
		WeekStart: weekStart.Format("2006-01-02"),
		WeekEnd:   weekStart.AddDate(0, 0, 6).Format("2006-01-02"),
		Days:      outDays,
	}, nil
}

func (s *Service) RoomWeek(ctx context.Context, tenantID, roomID uuid.UUID, weekStart time.Time) (*RoomWeekOverview, error) {
	_ = s.ReleaseNoShows(ctx, tenantID)
	weekStart = MondayOf(weekStart)
	weekEnd := weekStart.AddDate(0, 0, 7)

	var room RoomInfo
	var roomBookable bool
	err := s.pool.QueryRow(ctx, `
		SELECT rm.id,
		       l.name || ' / ' || b.code || ' / ' || f.name || ' / ' || rm.number,
		       rm.name,
		       rm.is_bookable
		FROM rooms rm
		JOIN floors f ON f.id = rm.floor_id
		JOIN buildings b ON b.id = f.building_id
		JOIN locations l ON l.id = b.location_id
		WHERE rm.id = $1 AND l.tenant_id = $2
	`, roomID, tenantID).Scan(&room.ID, &room.Label, &room.Name, &roomBookable)
	if err != nil {
		return nil, auth.ErrNotFound("room not found")
	}

	placeRows, err := s.pool.Query(ctx, `
		SELECT p.id,
		       b.code || '-' || f.level::text || '-' || rm.number || '-' || p.number,
		       p.booking_mode::text,
		       p.is_active
		FROM places p
		JOIN rooms rm ON rm.id = p.room_id
		JOIN floors f ON f.id = rm.floor_id
		JOIN buildings b ON b.id = f.building_id
		WHERE p.room_id = $1
		ORDER BY p.number
	`, roomID)
	if err != nil {
		return nil, fmt.Errorf("list places: %w", err)
	}
	defer placeRows.Close()

	type placeMeta struct {
		id     uuid.UUID
		label  string
		mode   string
		active bool
	}
	var metas []placeMeta
	placeIDs := []uuid.UUID{}
	for placeRows.Next() {
		var m placeMeta
		if err := placeRows.Scan(&m.id, &m.label, &m.mode, &m.active); err != nil {
			return nil, err
		}
		metas = append(metas, m)
		placeIDs = append(placeIDs, m.id)
	}
	if err := placeRows.Err(); err != nil {
		return nil, err
	}

	var intervals []timeInterval
	if len(placeIDs) > 0 {
		bRows, err := s.pool.Query(ctx, `
			SELECT place_id, starts_at, ends_at, COALESCE(
				(SELECT booking_mode::text FROM places WHERE id = bookings.place_id),
				'daily'
			)
			FROM bookings
			WHERE tenant_id = $1
			  AND place_id = ANY($2)
			  AND status IN ('booked', 'checked_in')
			  AND starts_at < $4
			  AND ends_at > $3
		`, tenantID, placeIDs, weekStart.UTC(), weekEnd.UTC())
		if err != nil {
			return nil, fmt.Errorf("week bookings: %w", err)
		}
		defer bRows.Close()
		for bRows.Next() {
			var it timeInterval
			if err := bRows.Scan(&it.placeID, &it.start, &it.end, &it.mode); err != nil {
				return nil, err
			}
			intervals = append(intervals, it)
		}
		if err := bRows.Err(); err != nil {
			return nil, err
		}

		blockRows, err := s.pool.Query(ctx, `
			SELECT place_id, starts_at, ends_at
			FROM blocks
			WHERE tenant_id = $1
			  AND place_id = ANY($2)
			  AND starts_at < $4
			  AND ends_at > $3
			UNION ALL
			SELECT p.id, bl.starts_at, bl.ends_at
			FROM blocks bl
			JOIN places p ON p.room_id = bl.room_id
			WHERE bl.tenant_id = $1
			  AND bl.room_id = $5
			  AND bl.starts_at < $4
			  AND bl.ends_at > $3
		`, tenantID, placeIDs, weekStart.UTC(), weekEnd.UTC(), roomID)
		if err != nil {
			return nil, fmt.Errorf("week blocks: %w", err)
		}
		defer blockRows.Close()
		for blockRows.Next() {
			var it timeInterval
			if err := blockRows.Scan(&it.placeID, &it.start, &it.end); err != nil {
				return nil, err
			}
			it.mode = "block"
			intervals = append(intervals, it)
		}
		if err := blockRows.Err(); err != nil {
			return nil, err
		}
	}

	days := make([]string, 0, 7)
	for i := 0; i < 7; i++ {
		days = append(days, weekStart.AddDate(0, 0, i).Format("2006-01-02"))
	}

	placesOut := make([]PlaceWeekRow, 0, len(metas))
	for _, m := range metas {
		row := PlaceWeekRow{
			ID:          m.id,
			Label:       m.label,
			BookingMode: m.mode,
			Days:        make([]PlaceDayCell, 0, 7),
		}
		for i := 0; i < 7; i++ {
			dayStart := weekStart.AddDate(0, 0, i)
			dayEnd := dayStart.AddDate(0, 0, 1)
			cell := PlaceDayCell{Date: dayStart.Format("2006-01-02"), Status: DayFree}
			if !roomBookable || !m.active {
				cell.Status = DayBlocked
				row.Days = append(row.Days, cell)
				continue
			}

			var dayIntervals []timeInterval
			for _, it := range intervals {
				if it.placeID != m.id {
					continue
				}
				if it.start.Before(dayEnd.UTC()) && it.end.After(dayStart.UTC()) {
					dayIntervals = append(dayIntervals, it)
				}
			}
			cell.Status = statusForDay(dayStart, dayIntervals, true)
			for _, it := range dayIntervals {
				if it.mode != "block" {
					cell.BookingCount++
				}
			}
			cell.Segments = daySegments(dayStart, dayIntervals, cell.Status)
			row.Days = append(row.Days, cell)
		}
		placesOut = append(placesOut, row)
	}

	return &RoomWeekOverview{
		WeekStart: weekStart.Format("2006-01-02"),
		WeekEnd:   weekStart.AddDate(0, 0, 6).Format("2006-01-02"),
		Days:      days,
		Room:      room,
		Places:    placesOut,
	}, nil
}

func (s *Service) PlaceDay(ctx context.Context, tenantID, placeID uuid.UUID, date time.Time) (*PlaceDayDetail, error) {
	_ = s.ReleaseNoShows(ctx, tenantID)
	loc := berlinLocation()
	dayStart := time.Date(date.In(loc).Year(), date.In(loc).Month(), date.In(loc).Day(), 0, 0, 0, 0, loc)
	dayEnd := dayStart.AddDate(0, 0, 1)

	var (
		label  string
		mode   string
		active bool
		roomOK bool
	)
	err := s.pool.QueryRow(ctx, `
		SELECT b.code || '-' || f.level::text || '-' || rm.number || '-' || p.number,
		       p.booking_mode::text, p.is_active, rm.is_bookable
		FROM places p
		JOIN rooms rm ON rm.id = p.room_id
		JOIN floors f ON f.id = rm.floor_id
		JOIN buildings b ON b.id = f.building_id
		JOIN locations l ON l.id = b.location_id
		WHERE p.id = $1 AND l.tenant_id = $2
	`, placeID, tenantID).Scan(&label, &mode, &active, &roomOK)
	if err != nil {
		return nil, auth.ErrNotFound("place not found")
	}

	rows, err := s.pool.Query(ctx, `
		SELECT starts_at, ends_at, 'busy'
		FROM bookings
		WHERE tenant_id = $1
		  AND place_id = $2
		  AND status IN ('booked', 'checked_in')
		  AND starts_at < $4
		  AND ends_at > $3
		UNION ALL
		SELECT bl.starts_at, bl.ends_at, 'block'
		FROM blocks bl
		WHERE bl.tenant_id = $1
		  AND bl.place_id = $2
		  AND bl.starts_at < $4
		  AND bl.ends_at > $3
		UNION ALL
		SELECT bl.starts_at, bl.ends_at, 'block'
		FROM blocks bl
		JOIN places p ON p.room_id = bl.room_id
		WHERE bl.tenant_id = $1
		  AND p.id = $2
		  AND bl.room_id IS NOT NULL
		  AND bl.starts_at < $4
		  AND bl.ends_at > $3
		ORDER BY 1
	`, tenantID, placeID, dayStart.UTC(), dayEnd.UTC())
	if err != nil {
		return nil, fmt.Errorf("place day: %w", err)
	}
	defer rows.Close()

	var intervals []timeInterval
	for rows.Next() {
		var it timeInterval
		if err := rows.Scan(&it.start, &it.end, &it.mode); err != nil {
			return nil, err
		}
		it.placeID = placeID
		intervals = append(intervals, it)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	status := statusForDay(dayStart, intervals, roomOK && active)

	busySpans := busyBusinessSpans(dayStart, intervals)
	freeSpans := freeBusinessSpans(dayStart, busySpans)

	busySlots := make([]TimeSlot, 0, len(busySpans))
	for _, sp := range busySpans {
		busySlots = append(busySlots, TimeSlot{
			Start: sp.a.Format("15:04"), End: sp.b.Format("15:04"),
			StartsAt: sp.a, EndsAt: sp.b,
		})
	}
	freeSlots := make([]TimeSlot, 0, len(freeSpans))
	for _, sp := range freeSpans {
		freeSlots = append(freeSlots, TimeSlot{
			Start: sp.a.Format("15:04"), End: sp.b.Format("15:04"),
			StartsAt: sp.a, EndsAt: sp.b,
		})
	}

	return &PlaceDayDetail{
		PlaceID:       placeID,
		Label:         label,
		BookingMode:   mode,
		Date:          dayStart.Format("2006-01-02"),
		BusinessStart: fmt.Sprintf("%02d:00", businessStartHour),
		BusinessEnd:   fmt.Sprintf("%02d:00", businessEndHour),
		Status:        status,
		Busy:          busySlots,
		Free:          freeSlots,
		Segments:      daySegments(dayStart, intervals, status),
	}, nil
}

type span struct{ a, b time.Time }

func businessWindow(dayStart time.Time) (time.Time, time.Time) {
	bizStart := time.Date(dayStart.Year(), dayStart.Month(), dayStart.Day(), businessStartHour, 0, 0, 0, dayStart.Location())
	bizEnd := time.Date(dayStart.Year(), dayStart.Month(), dayStart.Day(), businessEndHour, 0, 0, 0, dayStart.Location())
	return bizStart, bizEnd
}

func busyBusinessSpans(dayStart time.Time, intervals []timeInterval) []span {
	bizStart, bizEnd := businessWindow(dayStart)
	var spans []span
	for _, it := range intervals {
		a := it.start.In(dayStart.Location())
		b := it.end.In(dayStart.Location())
		if a.Before(bizStart) {
			a = bizStart
		}
		if b.After(bizEnd) {
			b = bizEnd
		}
		if b.After(a) {
			spans = append(spans, span{a, b})
		}
	}
	return mergeSpans(spans)
}

func freeBusinessSpans(dayStart time.Time, busy []span) []span {
	bizStart, bizEnd := businessWindow(dayStart)
	if len(busy) == 0 {
		return []span{{bizStart, bizEnd}}
	}
	var free []span
	cursor := bizStart
	for _, b := range busy {
		if b.a.After(cursor) {
			free = append(free, span{cursor, b.a})
		}
		if b.b.After(cursor) {
			cursor = b.b
		}
	}
	if cursor.Before(bizEnd) {
		free = append(free, span{cursor, bizEnd})
	}
	return free
}

func mergeSpans(spans []span) []span {
	if len(spans) == 0 {
		return spans
	}
	out := append([]span(nil), spans...)
	changed := true
	for changed {
		changed = false
		for i := 0; i < len(out); i++ {
			for j := i + 1; j < len(out); {
				if out[j].a.Before(out[i].b) && out[j].b.After(out[i].a) {
					if out[j].a.Before(out[i].a) {
						out[i].a = out[j].a
					}
					if out[j].b.After(out[i].b) {
						out[i].b = out[j].b
					}
					out = append(out[:j], out[j+1:]...)
					changed = true
					continue
				}
				j++
			}
		}
	}
	// sort by start
	for i := 0; i < len(out); i++ {
		for j := i + 1; j < len(out); j++ {
			if out[j].a.Before(out[i].a) {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	return out
}

func daySegments(dayStart time.Time, intervals []timeInterval, status DayStatus) []DaySegment {
	if status == DayBlocked {
		return []DaySegment{{Kind: "busy", StartPct: 0, EndPct: 100}}
	}
	if status == DayFree {
		return []DaySegment{{Kind: "free", StartPct: 0, EndPct: 100}}
	}
	if status == DayFull {
		return []DaySegment{{Kind: "busy", StartPct: 0, EndPct: 100}}
	}
	bizStart, bizEnd := businessWindow(dayStart)
	total := bizEnd.Sub(bizStart).Seconds()
	if total <= 0 {
		return nil
	}
	busy := busyBusinessSpans(dayStart, intervals)
	free := freeBusinessSpans(dayStart, busy)
	var segs []DaySegment
	appendSeg := func(kind string, a, b time.Time) {
		startPct := a.Sub(bizStart).Seconds() / total * 100
		endPct := b.Sub(bizStart).Seconds() / total * 100
		if endPct > startPct {
			segs = append(segs, DaySegment{Kind: kind, StartPct: startPct, EndPct: endPct})
		}
	}
	for _, sp := range free {
		appendSeg("free", sp.a, sp.b)
	}
	for _, sp := range busy {
		appendSeg("busy", sp.a, sp.b)
	}
	// sort by start
	for i := 0; i < len(segs); i++ {
		for j := i + 1; j < len(segs); j++ {
			if segs[j].StartPct < segs[i].StartPct {
				segs[i], segs[j] = segs[j], segs[i]
			}
		}
	}
	return segs
}

// statusForDay classifies a business day from the actual occupied intervals.
// A shorter booking stays partial even when the place is booked by the day,
// so the remaining hours can still be reserved.
func statusForDay(dayStart time.Time, intervals []timeInterval, bookable bool) DayStatus {
	if !bookable {
		return DayBlocked
	}
	bookingCount := 0
	hasBlock := false
	for _, it := range intervals {
		if it.mode == "block" {
			hasBlock = true
			continue
		}
		bookingCount++
	}
	if hasBlock && bookingCount == 0 {
		return DayBlocked
	}
	if bookingCount == 0 {
		return DayFree
	}
	if coverageFullBusinessDay(dayStart, intervals) {
		return DayFull
	}
	return DayPartial
}

func coverageFullBusinessDay(dayStart time.Time, intervals []timeInterval) bool {
	bizStart, bizEnd := businessWindow(dayStart)
	need := bizEnd.Sub(bizStart)
	spans := busyBusinessSpans(dayStart, intervals)
	if len(spans) == 0 {
		return false
	}
	var covered time.Duration
	for _, s := range spans {
		covered += s.b.Sub(s.a)
	}
	return covered >= need-time.Minute
}
