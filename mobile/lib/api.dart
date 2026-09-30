import 'dart:async';
import 'dart:convert';

import 'package:http/http.dart' as http;

import 'models.dart';

class ApiException implements Exception {
  const ApiException(this.message, {this.status});

  final String message;
  final int? status;

  bool get isUnauthorized => status == 401;

  @override
  String toString() => message;
}

String normalizeBaseUrl(
  String raw, {
  String fallback = 'http://127.0.0.1:8081',
}) {
  var value = raw.trim();
  if (value.isEmpty) return fallback;
  if (!value.contains('://')) value = 'http://$value';
  while (value.endsWith('/')) {
    value = value.substring(0, value.length - 1);
  }
  return value;
}

String apiErrorMessage(String body, int statusCode) {
  try {
    final decoded = jsonDecode(body);
    if (decoded is Map && decoded['message'] is String) {
      final message = (decoded['message'] as String).trim();
      if (message.isNotEmpty) return message;
    }
  } catch (_) {}
  if (statusCode == 401) return 'Anmeldung abgelaufen oder ungültig.';
  return 'Anfrage fehlgeschlagen ($statusCode).';
}

class DesksharingApi {
  DesksharingApi({required this.baseUrl, this.token, http.Client? client})
    : _client = client ?? http.Client();

  final String baseUrl;
  final String? token;
  final http.Client _client;

  Future<LoginResult> login({
    required String email,
    required String password,
    String tenantSlug = '',
  }) async {
    final body = <String, String>{
      'email': email,
      'password': password,
      if (tenantSlug.trim().isNotEmpty) 'tenant_slug': tenantSlug.trim(),
    };
    final json = await _send('POST', '/api/v1/auth/login', body: body);
    final token = jstr(json['token']);
    final userRaw = json['user'];
    if (token.isEmpty || userRaw is! Map) {
      throw const ApiException('Anmeldung ohne gültige Sitzung.');
    }
    return LoginResult(
      token: token,
      user: UserAccount.fromJson(Map<String, dynamic>.from(userRaw)),
    );
  }

  Future<void> logout() async {
    await _send('POST', '/api/v1/auth/logout');
  }

  Future<UserAccount> me() async {
    final json = await _send('GET', '/api/v1/auth/me');
    return UserAccount.fromJson(json);
  }

  Future<List<LocationItem>> locations() async {
    final json = await _send('GET', '/api/v1/locations');
    return jlist(json['items'], LocationItem.fromJson);
  }

  Future<List<BuildingItem>> buildings(String locationId) async {
    final json = await _send('GET', '/api/v1/locations/$locationId/buildings');
    return jlist(json['items'], BuildingItem.fromJson);
  }

  Future<List<FloorItem>> floors(String buildingId) async {
    final json = await _send('GET', '/api/v1/buildings/$buildingId/floors');
    return jlist(json['items'], FloorItem.fromJson);
  }

  Future<List<RoomItem>> rooms(String floorId) async {
    final json = await _send('GET', '/api/v1/floors/$floorId/rooms');
    return jlist(json['items'], RoomItem.fromJson);
  }

  Future<RoomWeek> roomWeek({
    required String roomId,
    required String weekStart,
  }) async {
    final json = await _send(
      'GET',
      '/api/v1/bookings/room-week',
      query: {'room_id': roomId, 'week_start': weekStart},
    );
    return RoomWeek.fromJson(json);
  }

  Future<PlaceDayDetail> placeDay({
    required String placeId,
    required String date,
  }) async {
    final json = await _send(
      'GET',
      '/api/v1/bookings/place-day',
      query: {'place_id': placeId, 'date': date},
    );
    return PlaceDayDetail.fromJson(json);
  }

  Future<List<ResourceGroup>> resourceGroups() async {
    final json = await _send('GET', '/api/v1/resource-groups');
    return jlist(json['items'], ResourceGroup.fromJson);
  }

  Future<List<BookableResource>> searchResources({
    required String mode,
    String? groupId,
    String? locationId,
    String startsAt = '',
    String endsAt = '',
    String startDate = '',
    String endDate = '',
    bool onlyAvailable = true,
  }) async {
    final json = await _send(
      'GET',
      '/api/v1/bookings/resources',
      query: {
        'mode': mode,
        if (groupId != null && groupId.isNotEmpty) 'group_id': groupId,
        if (locationId != null && locationId.isNotEmpty)
          'location_id': locationId,
        if (mode == 'hourly') 'starts_at': startsAt,
        if (mode == 'hourly') 'ends_at': endsAt,
        if (mode == 'daily') 'start_date': startDate,
        if (mode == 'daily') 'end_date': endDate,
        if (onlyAvailable) 'only_available': '1',
      },
    );
    return jlist(json['items'], BookableResource.fromJson);
  }

  Future<void> createResourceBooking({
    required String resourceId,
    required String mode,
    String startsAt = '',
    String endsAt = '',
    String startDate = '',
    String endDate = '',
  }) async {
    await _send(
      'POST',
      '/api/v1/bookings',
      body: {
        'resource_id': resourceId,
        'mode': mode,
        if (mode == 'hourly') 'starts_at': startsAt,
        if (mode == 'hourly') 'ends_at': endsAt,
        if (mode == 'daily') 'start_date': startDate,
        if (mode == 'daily') 'end_date': endDate,
      },
    );
  }

  Future<void> createBooking({
    required String placeId,
    required String startsAt,
    required String endsAt,
  }) async {
    await _send(
      'POST',
      '/api/v1/bookings',
      body: {'place_id': placeId, 'starts_at': startsAt, 'ends_at': endsAt},
    );
  }

  Future<List<BookingItem>> myBookings() async {
    final json = await _send('GET', '/api/v1/bookings/mine');
    final raw = json['items'];
    if (raw is! List) return const [];
    final out = <BookingItem>[];
    for (final item in raw) {
      if (item is! Map) continue;
      final booking = BookingItem.fromJson(Map<String, dynamic>.from(item));
      if (booking != null) out.add(booking);
    }
    return out;
  }

  Future<void> cancelBooking(String id) async {
    await _send('POST', '/api/v1/bookings/$id/cancel');
  }

  Future<Map<String, dynamic>> _send(
    String method,
    String path, {
    Map<String, String>? query,
    Map<String, String>? body,
  }) async {
    final uri = Uri.parse(baseUrl).replace(
      path: path,
      queryParameters: query == null || query.isEmpty ? null : query,
    );
    final headers = <String, String>{
      'Accept': 'application/json',
      if (body != null) 'Content-Type': 'application/json',
      if (token != null && token!.isNotEmpty) 'Authorization': 'Bearer $token',
    };

    late http.Response response;
    try {
      final future = switch (method) {
        'POST' => _client.post(
          uri,
          headers: headers,
          body: body == null ? null : jsonEncode(body),
        ),
        'GET' => _client.get(uri, headers: headers),
        _ => throw ArgumentError('unsupported method $method'),
      };
      response = await future.timeout(const Duration(seconds: 20));
    } on TimeoutException {
      throw const ApiException(
        'Zeitüberschreitung. Der Server antwortet nicht.',
      );
    } on http.ClientException {
      throw const ApiException('Server nicht erreichbar. Prüfe die Adresse.');
    }

    if (response.statusCode < 200 || response.statusCode >= 300) {
      throw ApiException(
        apiErrorMessage(response.body, response.statusCode),
        status: response.statusCode,
      );
    }
    if (response.body.trim().isEmpty) return {};
    final decoded = jsonDecode(response.body);
    if (decoded is Map<String, dynamic>) return decoded;
    if (decoded is Map) return Map<String, dynamic>.from(decoded);
    throw const ApiException('Unerwartete Serverantwort.');
  }
}
