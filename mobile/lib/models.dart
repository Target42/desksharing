String jstr(dynamic value) => value is String ? value : '';

int jint(dynamic value) {
  if (value is int) return value;
  if (value is num) return value.toInt();
  return int.tryParse('$value') ?? 0;
}

double jdouble(dynamic value) {
  if (value is num) return value.toDouble();
  return double.tryParse('$value') ?? 0;
}

bool jbool(dynamic value) => value == true;

List<T> jlist<T>(dynamic value, T Function(Map<String, dynamic>) parse) {
  if (value is! List) return const [];
  final out = <T>[];
  for (final item in value) {
    if (item is Map) {
      out.add(parse(Map<String, dynamic>.from(item)));
    }
  }
  return out;
}

class UserAccount {
  const UserAccount({
    required this.id,
    required this.email,
    required this.displayName,
    required this.role,
  });

  final String id;
  final String email;
  final String displayName;
  final String role;

  String get label => displayName.trim().isEmpty ? email : displayName;

  factory UserAccount.fromJson(Map<String, dynamic> json) {
    final email = jstr(json['email']);
    final name = jstr(json['display_name']).trim();
    return UserAccount(
      id: jstr(json['id']),
      email: email,
      displayName: name.isEmpty ? email : name,
      role: jstr(json['role']),
    );
  }
}

class LoginResult {
  const LoginResult({required this.token, required this.user});

  final String token;
  final UserAccount user;
}

class LocationItem {
  const LocationItem({required this.id, required this.name});

  final String id;
  final String name;

  factory LocationItem.fromJson(Map<String, dynamic> json) {
    return LocationItem(id: jstr(json['id']), name: jstr(json['name']));
  }
}

class BuildingItem {
  const BuildingItem({
    required this.id,
    required this.name,
    required this.code,
  });

  final String id;
  final String name;
  final String code;

  String get label => code.isEmpty ? name : '$code — $name';

  factory BuildingItem.fromJson(Map<String, dynamic> json) {
    return BuildingItem(
      id: jstr(json['id']),
      name: jstr(json['name']),
      code: jstr(json['code']),
    );
  }
}

class FloorItem {
  const FloorItem({required this.id, required this.name, required this.level});

  final String id;
  final String name;
  final int level;

  String get label => '$name (Ebene $level)';

  factory FloorItem.fromJson(Map<String, dynamic> json) {
    return FloorItem(
      id: jstr(json['id']),
      name: jstr(json['name']),
      level: jint(json['level']),
    );
  }
}

class RoomItem {
  const RoomItem({
    required this.id,
    required this.number,
    required this.name,
    required this.isBookable,
  });

  final String id;
  final String number;
  final String name;
  final bool isBookable;

  String get label {
    final base = name.trim().isEmpty ? 'Raum $number' : '$number — $name';
    return isBookable ? base : '$base (gesperrt)';
  }

  factory RoomItem.fromJson(Map<String, dynamic> json) {
    return RoomItem(
      id: jstr(json['id']),
      number: jstr(json['number']),
      name: jstr(json['name']),
      isBookable: jbool(json['is_bookable']),
    );
  }
}

class DaySegment {
  const DaySegment({
    required this.kind,
    required this.startPct,
    required this.endPct,
  });

  final String kind;
  final double startPct;
  final double endPct;

  factory DaySegment.fromJson(Map<String, dynamic> json) {
    return DaySegment(
      kind: jstr(json['kind']),
      startPct: jdouble(json['start_pct']),
      endPct: jdouble(json['end_pct']),
    );
  }
}

class PlaceDay {
  const PlaceDay({
    required this.date,
    required this.status,
    required this.bookingCount,
    required this.segments,
  });

  final String date;
  final String status;
  final int bookingCount;
  final List<DaySegment> segments;

  factory PlaceDay.fromJson(Map<String, dynamic> json) {
    return PlaceDay(
      date: jstr(json['date']),
      status: jstr(json['status']),
      bookingCount: jint(json['booking_count']),
      segments: jlist(json['segments'], DaySegment.fromJson),
    );
  }
}

class PlaceWeek {
  const PlaceWeek({
    required this.id,
    required this.label,
    required this.bookingMode,
    required this.days,
  });

  final String id;
  final String label;
  final String bookingMode;
  final List<PlaceDay> days;

  factory PlaceWeek.fromJson(Map<String, dynamic> json) {
    return PlaceWeek(
      id: jstr(json['id']),
      label: jstr(json['label']),
      bookingMode: jstr(json['booking_mode']),
      days: jlist(json['days'], PlaceDay.fromJson),
    );
  }
}

class RoomWeek {
  const RoomWeek({
    required this.weekStart,
    required this.weekEnd,
    required this.roomLabel,
    required this.places,
  });

  final String weekStart;
  final String weekEnd;
  final String roomLabel;
  final List<PlaceWeek> places;

  factory RoomWeek.fromJson(Map<String, dynamic> json) {
    final room = json['room'];
    final roomMap = room is Map
        ? Map<String, dynamic>.from(room)
        : const <String, dynamic>{};
    return RoomWeek(
      weekStart: jstr(json['week_start']),
      weekEnd: jstr(json['week_end']),
      roomLabel: jstr(roomMap['label']),
      places: jlist(json['places'], PlaceWeek.fromJson),
    );
  }
}

class TimeSlot {
  const TimeSlot({
    required this.start,
    required this.end,
    required this.startsAt,
    required this.endsAt,
  });

  final String start;
  final String end;
  final String startsAt;
  final String endsAt;

  bool get isFullDay => start == '08:00' && end == '18:00';

  factory TimeSlot.fromJson(Map<String, dynamic> json) {
    return TimeSlot(
      start: jstr(json['start']),
      end: jstr(json['end']),
      startsAt: jstr(json['starts_at']),
      endsAt: jstr(json['ends_at']),
    );
  }
}

class PlaceDayDetail {
  const PlaceDayDetail({
    required this.placeId,
    required this.label,
    required this.bookingMode,
    required this.date,
    required this.status,
    required this.free,
    required this.busy,
  });

  final String placeId;
  final String label;
  final String bookingMode;
  final String date;
  final String status;
  final List<TimeSlot> free;
  final List<TimeSlot> busy;

  TimeSlot? get fullDay {
    for (final slot in free) {
      if (slot.isFullDay) return slot;
    }
    return null;
  }

  factory PlaceDayDetail.fromJson(Map<String, dynamic> json) {
    return PlaceDayDetail(
      placeId: jstr(json['place_id']),
      label: jstr(json['label']),
      bookingMode: jstr(json['booking_mode']),
      date: jstr(json['date']),
      status: jstr(json['status']),
      free: jlist(json['free'], TimeSlot.fromJson),
      busy: jlist(json['busy'], TimeSlot.fromJson),
    );
  }
}

class ResourceGroup {
  const ResourceGroup({required this.id, required this.name});

  final String id;
  final String name;

  factory ResourceGroup.fromJson(Map<String, dynamic> json) {
    return ResourceGroup(id: jstr(json['id']), name: jstr(json['name']));
  }
}

class BookableResource {
  const BookableResource({
    required this.id,
    required this.name,
    required this.description,
    required this.seats,
    required this.groupName,
    required this.locationName,
    required this.available,
  });

  final String id;
  final String name;
  final String description;
  final int? seats;
  final String groupName;
  final String locationName;
  final bool available;

  String get subtitle {
    final bits = <String>[
      if (groupName.isNotEmpty) groupName,
      if (locationName.isNotEmpty) locationName,
      if (seats != null && seats! > 0) '$seats Sitze',
      if (description.isNotEmpty) description,
    ];
    return bits.join(' · ');
  }

  factory BookableResource.fromJson(Map<String, dynamic> json) {
    final rawSeats = json['seats'];
    return BookableResource(
      id: jstr(json['id']),
      name: jstr(json['name']),
      description: jstr(json['description']),
      seats: rawSeats == null ? null : jint(rawSeats),
      groupName: jstr(json['group_name']),
      locationName: jstr(json['location_name']),
      available: jbool(json['available']),
    );
  }
}

class BookingItem {
  const BookingItem({
    required this.id,
    required this.placeLabel,
    required this.startsAt,
    required this.endsAt,
    required this.status,
    required this.bookingMode,
  });

  final String id;
  final String placeLabel;
  final DateTime startsAt;
  final DateTime endsAt;
  final String status;
  final String bookingMode;

  bool get canCancel => status == 'booked' || status == 'possibly_free';

  static BookingItem? fromJson(Map<String, dynamic> json) {
    final id = jstr(json['id']);
    final startsAt = DateTime.tryParse(jstr(json['starts_at']));
    final endsAt = DateTime.tryParse(jstr(json['ends_at']));
    if (id.isEmpty || startsAt == null || endsAt == null) return null;
    return BookingItem(
      id: id,
      placeLabel: jstr(json['place_label']),
      startsAt: startsAt,
      endsAt: endsAt,
      status: jstr(json['status']),
      bookingMode: jstr(json['booking_mode']),
    );
  }
}
