import 'package:flutter/foundation.dart';
import 'package:shared_preferences/shared_preferences.dart';

import 'api.dart';
import 'models.dart';

class Session extends ChangeNotifier {
  Session(this.prefs);

  final SharedPreferences prefs;

  static const tokenKey = 'auth_token';
  static const baseUrlKey = 'api_base_url';

  String? token;
  UserAccount? user;
  bool booting = true;
  String? bootError;
  int bookingsTick = 0;

  String get baseUrl {
    final saved = prefs.getString(baseUrlKey);
    return normalizeBaseUrl(saved ?? '', fallback: defaultBaseUrl());
  }

  DesksharingApi api() => DesksharingApi(baseUrl: baseUrl, token: token);

  Future<void> boot() async {
    await Future<void>.value();
    token = prefs.getString(tokenKey);
    if (token != null && token!.isEmpty) token = null;
    bootError = null;
    user = null;
    if (token != null) {
      try {
        user = await api().me();
      } on ApiException catch (error) {
        if (error.isUnauthorized) {
          token = null;
          await prefs.remove(tokenKey);
        } else {
          bootError = error.message;
        }
      }
    }
    booting = false;
    notifyListeners();
  }

  Future<void> login({
    required String baseUrl,
    required String email,
    required String password,
    String tenantSlug = '',
  }) async {
    final normalized = normalizeBaseUrl(baseUrl, fallback: defaultBaseUrl());
    final uri = Uri.tryParse(normalized);
    if (uri == null || !uri.hasScheme || uri.host.isEmpty) {
      throw const ApiException('Die Serveradresse ist ungültig.');
    }
    await prefs.setString(baseUrlKey, normalized);
    final result = await DesksharingApi(
      baseUrl: normalized,
    ).login(email: email.trim(), password: password, tenantSlug: tenantSlug);
    token = result.token;
    user = result.user;
    bootError = null;
    await prefs.setString(tokenKey, result.token);
    notifyListeners();
  }

  Future<void> logout() async {
    final client = token == null ? null : api();
    token = null;
    user = null;
    bootError = null;
    await prefs.remove(tokenKey);
    notifyListeners();
    if (client != null) {
      try {
        await client.logout();
      } catch (_) {}
    }
  }

  Future<void> expire() async {
    token = null;
    user = null;
    bootError = null;
    await prefs.remove(tokenKey);
    notifyListeners();
  }

  void bumpBookings() {
    bookingsTick += 1;
    notifyListeners();
  }
}

String defaultBaseUrl() {
  if (defaultTargetPlatform == TargetPlatform.android) {
    return 'http://10.0.2.2:8081';
  }
  return 'http://127.0.0.1:8081';
}

String? preferId(String? saved, List<String> ids) {
  if (saved != null && saved.isNotEmpty && ids.contains(saved)) return saved;
  if (ids.length == 1) return ids.single;
  return null;
}
