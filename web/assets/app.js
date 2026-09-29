import { api, apiBlob, apiUpload, getToken, setToken } from "./api.js";

const views = {
  login: document.querySelector("#view-login"),
  register: document.querySelector("#view-register"),
  verify: document.querySelector("#view-verify"),
  home: document.querySelector("#view-home"),
  locations: document.querySelector("#view-locations"),
  placeTypes: document.querySelector("#view-place-types"),
  equipments: document.querySelector("#view-equipments"),
  bookings: document.querySelector("#view-bookings"),
};

let currentUser = null;
const nav = {
  level: "locations",
  location: null,
  building: null,
  floor: null,
  room: null,
};

const NAV_ITEMS = [
  ["Konto", "/app"],
  ["Standorte", "/app/locations"],
  ["Platztypen", "/app/place-types"],
  ["Ausstattung", "/app/equipments"],
  ["Buchungen", "/app/bookings"],
];

function paintNav() {
  document.querySelectorAll("[data-app-nav]").forEach((el) => {
    const active = el.getAttribute("data-app-nav");
    el.innerHTML = NAV_ITEMS.map(([label, href]) => {
      const cls = label === active ? "nav-link is-active" : "nav-link";
      return `<a href="${href}" class="${cls}">${label}</a>`;
    }).join("") + `<button type="button" class="secondary" data-logout>Abmelden</button>`;
  });
  document.querySelectorAll("[data-logout]").forEach((btn) => {
    btn.addEventListener("click", logout);
  });
}

function show(name) {
  Object.entries(views).forEach(([key, el]) => {
    el.classList.toggle("hidden", key !== name);
  });
}

function setMsg(el, type, text) {
  if (!el) return;
  el.className = `msg show ${type}`;
  el.textContent = text;
}

function clearMsg(el) {
  if (!el) return;
  el.className = "msg";
  el.textContent = "";
}

function pathName() {
  return window.location.pathname.replace(/\/+$/, "") || "/";
}

function isAdmin() {
  return currentUser && currentUser.role === "tenant_admin";
}

function applyAdminVisibility() {
  document.querySelectorAll(".admin-only").forEach((el) => {
    el.classList.toggle("hidden", !isAdmin());
  });
}

async function requireSession() {
  if (!getToken()) throw new Error("no session");
  currentUser = await api("/api/v1/auth/me", { auth: true });
  applyAdminVisibility();
  paintNav();
  return currentUser;
}

async function route() {
  const path = pathName();
  const params = new URLSearchParams(window.location.search);

  if (path === "/register") {
    show("register");
    return;
  }
  if (path === "/verify") {
    show("verify");
    const token = params.get("token") || "";
    const input = document.querySelector("#verify-token");
    if (token) input.value = token;
    if (token) await runVerify(token);
    return;
  }
  if (path === "/login" || path === "/") {
    if (getToken()) {
      try {
        await loadHome();
        history.replaceState({}, "", "/app");
        return;
      } catch {
        setToken("");
      }
    }
    show("login");
    return;
  }

  if (path.startsWith("/app")) {
    if (!getToken()) {
      history.replaceState({}, "", "/login");
      show("login");
      return;
    }
    try {
      await requireSession();
    } catch {
      setToken("");
      history.replaceState({}, "", "/login");
      show("login");
      return;
    }

    if (path === "/app/locations") {
      show("locations");
      await renderLocations();
      return;
    }
    if (path === "/app/place-types") {
      show("placeTypes");
      await renderPlaceTypes();
      return;
    }
    if (path === "/app/equipments") {
      show("equipments");
      await renderEquipments();
      return;
    }
    if (path === "/app/bookings") {
      show("bookings");
      await renderBookingsPage();
      return;
    }
    await loadHome();
    return;
  }

  history.replaceState({}, "", "/login");
  show("login");
}

async function loadHome() {
  const me = await requireSession();
  show("home");
  document.querySelector("#home-name").textContent = me.display_name || me.email;
  document.querySelector("#home-email").textContent = me.email;
  document.querySelector("#home-role").textContent = me.role;
  document.querySelector("#home-tenant").textContent = me.tenant_id || "— (Platform)";
  document.querySelector("#home-provider").textContent = me.identity_provider;
}

async function runVerify(token) {
  const msg = document.querySelector("#verify-msg");
  clearMsg(msg);
  try {
    await api("/api/v1/auth/verify", { method: "POST", body: { token } });
    setMsg(msg, "ok", "Konto bestätigt. Du kannst dich jetzt anmelden.");
  } catch (err) {
    setMsg(msg, "error", err.message || "Bestätigung fehlgeschlagen.");
  }
}

async function logout() {
  try {
    await api("/api/v1/auth/logout", { method: "POST", auth: true });
  } catch {
    /* ignore */
  }
  setToken("");
  currentUser = null;
  history.pushState({}, "", "/login");
  show("login");
}

document.querySelector("#form-login").addEventListener("submit", async (e) => {
  e.preventDefault();
  const msg = document.querySelector("#login-msg");
  const btn = e.target.querySelector("button[type=submit]");
  clearMsg(msg);
  btn.disabled = true;
  const fd = new FormData(e.target);
  try {
    const out = await api("/api/v1/auth/login", {
      method: "POST",
      body: {
        email: fd.get("email"),
        password: fd.get("password"),
        tenant_slug: fd.get("tenant_slug") || undefined,
      },
    });
    setToken(out.token);
    history.pushState({}, "", "/app");
    await loadHome();
  } catch (err) {
    setMsg(msg, "error", err.message || "Login fehlgeschlagen.");
  } finally {
    btn.disabled = false;
  }
});

document.querySelector("#form-register").addEventListener("submit", async (e) => {
  e.preventDefault();
  const msg = document.querySelector("#register-msg");
  const btn = e.target.querySelector("button[type=submit]");
  clearMsg(msg);
  btn.disabled = true;
  const fd = new FormData(e.target);
  try {
    const out = await api("/api/v1/auth/register", {
      method: "POST",
      body: {
        tenant_name: fd.get("tenant_name"),
        tenant_slug: fd.get("tenant_slug"),
        admin_email: fd.get("admin_email"),
        admin_password: fd.get("admin_password"),
        admin_display_name: fd.get("admin_display_name"),
      },
    });
    const text = "Registrierung gestartet. Bitte E-Mail bestätigen.";
    if (out.verification_token) {
      const link = `/verify?token=${encodeURIComponent(out.verification_token)}`;
      msg.className = "msg show ok";
      msg.innerHTML = `${escapeHtml(text)}<br><a href="${link}">Jetzt bestätigen</a><br><span class="mono">${escapeHtml(out.verification_token)}</span>`;
    } else {
      setMsg(msg, "ok", text);
    }
  } catch (err) {
    setMsg(msg, "error", err.message || "Registrierung fehlgeschlagen.");
  } finally {
    btn.disabled = false;
  }
});

document.querySelector("#form-verify").addEventListener("submit", async (e) => {
  e.preventDefault();
  const fd = new FormData(e.target);
  await runVerify(String(fd.get("token") || ""));
});

function escapeHtml(s) {
  return String(s)
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;");
}

function bookingModeLabel(mode) {
  return { daily: "tageweise", hourly: "stundenweise" }[mode] || mode || "";
}

function bookingStatusLabel(status) {
  return (
    {
      booked: "gebucht",
      checked_in: "eingecheckt",
      checked_out: "ausgecheckt",
      cancelled: "storniert",
      possibly_free: "möglicherweise frei",
      no_show: "No-Show",
    }[status] || status || ""
  );
}

function equipBadges(items) {
  if (!items || !items.length) return "";
  return items.map((e) => `<span class="badge">${escapeHtml(e.name)}</span>`).join("");
}

function resetNav() {
  nav.level = "locations";
  nav.location = null;
  nav.building = null;
  nav.floor = null;
  nav.room = null;
}

document.querySelector("#btn-seed-demo")?.addEventListener("click", async () => {
  const msg = document.querySelector("#locations-msg");
  clearMsg(msg);
  const btn = document.querySelector("#btn-seed-demo");
  btn.disabled = true;
  try {
    const out = await api("/api/v1/catalog/seed-demo", { method: "POST", auth: true });
    setMsg(msg, "ok", out.message || "Beispiel-Standort angelegt.");
    resetNav();
    await renderLocations();
  } catch (err) {
    setMsg(msg, "error", err.message || "Seed fehlgeschlagen.");
  } finally {
    btn.disabled = false;
  }
});

async function renderLocations() {
  const msg = document.querySelector("#locations-msg");
  clearMsg(msg);
  applyAdminVisibility();
  renderCreateForm();
  renderCrumbs();
  closePlaceEquipmentEditor();

  const list = document.querySelector("#locations-list");
  const title = document.querySelector("#locations-list-title");
  const overviewBox = document.querySelector("#locations-overview-box");
  const detailBox = document.querySelector("#location-detail-box");
  const floorPlanBox = document.querySelector("#floor-plan-box");
  const roomPlanBox = document.querySelector("#room-plan-box");
  list.innerHTML = `<li class="empty">Lade…</li>`;

  const seedBox = document.querySelector("#seed-demo-box");
  if (seedBox) {
    seedBox.classList.toggle("hidden", !(isAdmin() && nav.level === "locations"));
  }

  try {
    if (nav.level === "locations") {
      title.textContent = "Standorte";
      overviewBox?.classList.remove("hidden");
      detailBox?.classList.add("hidden");
      floorPlanBox?.classList.add("hidden");
      roomPlanBox?.classList.add("hidden");
      destroyPickerMap();
      destroyFloorPlanEditor();
      destroyRoomPlanEditor();
      const data = await api("/api/v1/locations", { auth: true });
      renderList(list, data.items, (item) => ({
        title: item.name,
        subtitle: item.address || "Adresse noch nicht gesetzt",
        onOpen: async () => {
          nav.level = "buildings";
          nav.location = item;
          nav.building = null;
          nav.floor = null;
          nav.room = null;
          await renderLocations();
        },
        onDelete: isAdmin()
          ? async () => {
              await api(`/api/v1/locations/${item.id}`, { method: "DELETE", auth: true });
              await renderLocations();
            }
          : null,
      }));
      renderLocationsOverviewMap(data.items || []);
      return;
    }

    overviewBox?.classList.add("hidden");
    destroyOverviewMap();

    if (nav.level === "buildings") {
      title.textContent = `Gebäude in ${nav.location.name}`;
      detailBox?.classList.remove("hidden");
      floorPlanBox?.classList.add("hidden");
      roomPlanBox?.classList.add("hidden");
      destroyFloorPlanEditor();
      destroyRoomPlanEditor();
      renderLocationAddressPanel(nav.location);
      const data = await api(`/api/v1/locations/${nav.location.id}/buildings`, { auth: true });
      renderList(list, data.items, (item) => ({
        title: `${item.code} — ${item.name}`,
        subtitle: item.address || (nav.location?.address ? `Standort: ${nav.location.address}` : null),
        onOpen: async () => {
          nav.level = "floors";
          nav.building = item;
          nav.floor = null;
          nav.room = null;
          await renderLocations();
        },
        onDelete: isAdmin()
          ? async () => {
              await api(`/api/v1/buildings/${item.id}`, { method: "DELETE", auth: true });
              await renderLocations();
            }
          : null,
      }));
      return;
    }

    detailBox?.classList.add("hidden");
    destroyPickerMap();

    if (nav.level === "floors") {
      title.textContent = `Stockwerke in ${nav.building.code}`;
      floorPlanBox?.classList.add("hidden");
      roomPlanBox?.classList.add("hidden");
      destroyFloorPlanEditor();
      destroyRoomPlanEditor();
      const data = await api(`/api/v1/buildings/${nav.building.id}/floors`, { auth: true });
      renderList(list, data.items, (item) => ({
        title: item.name,
        subtitle: `Ebene ${item.level}`,
        onOpen: async () => {
          nav.level = "rooms";
          nav.floor = item;
          nav.room = null;
          await renderLocations();
        },
        onDelete: isAdmin()
          ? async () => {
              await api(`/api/v1/floors/${item.id}`, { method: "DELETE", auth: true });
              await renderLocations();
            }
          : null,
      }));
      return;
    }

    if (nav.level === "rooms") {
      title.textContent = `Räume auf ${nav.floor.name}`;
      floorPlanBox?.classList.remove("hidden");
      roomPlanBox?.classList.add("hidden");
      destroyRoomPlanEditor();
      await renderFloorPlanEditor(nav.floor.id);
      const data = await api(`/api/v1/floors/${nav.floor.id}/rooms`, { auth: true });
      renderList(list, data.items, (item) => ({
        title: `${item.number}${item.name ? " — " + item.name : ""}`,
        subtitle: item.is_bookable ? "buchbar" : "gesperrt / nicht buchbar",
        onOpen: async () => {
          nav.level = "places";
          nav.room = item;
          await renderLocations();
        },
        extra: isAdmin()
          ? {
              label: item.is_bookable ? "Sperren" : "Freigeben",
              run: async () => {
                await api(`/api/v1/rooms/${item.id}`, {
                  method: "PATCH",
                  auth: true,
                  body: { is_bookable: !item.is_bookable },
                });
                await renderLocations();
              },
            }
          : null,
        onDelete: isAdmin()
          ? async () => {
              await api(`/api/v1/rooms/${item.id}`, { method: "DELETE", auth: true });
              await renderLocations();
            }
          : null,
      }));
      return;
    }

    if (nav.level === "places") {
      title.textContent = `Plätze in Raum ${nav.room.number}`;
      floorPlanBox?.classList.add("hidden");
      destroyFloorPlanEditor();
      roomPlanBox?.classList.remove("hidden");
      await renderRoomPlanEditor(nav.floor.id, nav.room.id);
      const data = await api(`/api/v1/rooms/${nav.room.id}/places`, { auth: true });
      renderList(list, data.items, (item) => ({
        title: item.label || item.number,
        subtitleHtml: `${escapeHtml(item.place_type_name || "ohne Typ")} · ${escapeHtml(bookingModeLabel(item.booking_mode))}<br>${equipBadges(item.equipments)}`,
        extra: isAdmin()
          ? {
              label: "Ausstattung",
              run: async () => openPlaceEquipmentEditor(item),
            }
          : null,
        onDelete: isAdmin()
          ? async () => {
              await api(`/api/v1/places/${item.id}`, { method: "DELETE", auth: true });
              await renderLocations();
            }
          : null,
      }));
    }
  } catch (err) {
    list.innerHTML = "";
    setMsg(msg, "error", err.message || "Laden fehlgeschlagen.");
  }
}

async function openPlaceEquipmentEditor(place) {
  const modal = document.querySelector("#equip-modal");
  const form = document.querySelector("#form-place-equipments");
  const filter = document.querySelector("#equip-filter");
  const countEl = document.querySelector("#equip-selected-count");
  document.querySelector("#equip-place-label").textContent = place.label || place.number;
  document.querySelector("#equip-place-id").value = place.id;
  if (filter) filter.value = "";

  const data = await api("/api/v1/equipments", { auth: true });
  placeEquipEditorState = {
    items: data.items || [],
    selected: new Set((place.equipments || []).map((e) => e.id)),
  };
  renderPlaceEquipmentCheckboxes();
  modal?.classList.remove("hidden");
  document.body.classList.add("modal-open");
  queueMicrotask(() => filter?.focus());
}

function closePlaceEquipmentEditor() {
  const modal = document.querySelector("#equip-modal");
  modal?.classList.add("hidden");
  document.body.classList.remove("modal-open");
  placeEquipEditorState = null;
  const filter = document.querySelector("#equip-filter");
  if (filter) filter.value = "";
}

function updatePlaceEquipmentCount() {
  const countEl = document.querySelector("#equip-selected-count");
  if (!countEl || !placeEquipEditorState) return;
  const total = placeEquipEditorState.items.length;
  const selected = placeEquipEditorState.selected.size;
  const q = (document.querySelector("#equip-filter")?.value || "").trim().toLowerCase();
  const visible = q
    ? placeEquipEditorState.items.filter((e) => (e.name || "").toLowerCase().includes(q)).length
    : total;
  if (!total) {
    countEl.textContent = "";
    return;
  }
  countEl.textContent =
    visible === total
      ? `${selected} von ${total} ausgewählt`
      : `${selected} ausgewählt · ${visible} von ${total} angezeigt`;
}

function renderPlaceEquipmentCheckboxes() {
  const box = document.querySelector("#equip-checkboxes");
  if (!box || !placeEquipEditorState) return;
  const q = (document.querySelector("#equip-filter")?.value || "").trim().toLowerCase();
  const items = placeEquipEditorState.items;
  if (!items.length) {
    box.innerHTML = `<p class="equip-empty">Keine Ausstattungen definiert. Bitte unter „Ausstattung“ anlegen.</p>`;
    updatePlaceEquipmentCount();
    return;
  }
  const filtered = q ? items.filter((e) => (e.name || "").toLowerCase().includes(q)) : items;
  if (!filtered.length) {
    box.innerHTML = `<p class="equip-empty">Keine Treffer für „${escapeHtml(document.querySelector("#equip-filter")?.value || "")}“.</p>`;
    updatePlaceEquipmentCount();
    return;
  }
  box.innerHTML = filtered
    .map(
      (e) =>
        `<label><input type="checkbox" name="equipment_id" value="${e.id}" ${
          placeEquipEditorState.selected.has(e.id) ? "checked" : ""
        }/> ${escapeHtml(e.name)}</label>`
    )
    .join("");
  updatePlaceEquipmentCount();
}

let placeEquipEditorState = null;

document.querySelector("#equip-filter")?.addEventListener("input", () => {
  renderPlaceEquipmentCheckboxes();
});

document.querySelector("#equip-checkboxes")?.addEventListener("change", (e) => {
  const input = e.target;
  if (!placeEquipEditorState || input?.name !== "equipment_id") return;
  if (input.checked) placeEquipEditorState.selected.add(input.value);
  else placeEquipEditorState.selected.delete(input.value);
  updatePlaceEquipmentCount();
});

document.querySelector("#equip-modal")?.addEventListener("click", (e) => {
  if (e.target.closest("[data-equip-close]")) {
    e.preventDefault();
    closePlaceEquipmentEditor();
  }
});

document.addEventListener("keydown", (e) => {
  if (e.key !== "Escape") return;
  const modal = document.querySelector("#equip-modal");
  if (!modal || modal.classList.contains("hidden")) return;
  closePlaceEquipmentEditor();
});

document.querySelector("#form-place-equipments").addEventListener("submit", async (e) => {
  e.preventDefault();
  const msg = document.querySelector("#locations-msg");
  clearMsg(msg);
  const placeId = document.querySelector("#equip-place-id").value;
  const ids = placeEquipEditorState
    ? [...placeEquipEditorState.selected]
    : [...e.target.querySelectorAll("input[name=equipment_id]:checked")].map((el) => el.value);
  try {
    await api(`/api/v1/places/${placeId}/equipments`, {
      method: "PUT",
      auth: true,
      body: { equipment_ids: ids },
    });
    closePlaceEquipmentEditor();
    setMsg(msg, "ok", "Ausstattung gespeichert.");
    await renderLocations();
  } catch (err) {
    setMsg(msg, "error", err.message || "Speichern fehlgeschlagen.");
  }
});

function renderCrumbs() {
  const el = document.querySelector("#locations-crumbs");
  const parts = [`<a href="#" data-crumb="Standorte">Standorte</a>`];
  if (nav.location) parts.push(`<span class="sep">/</span>`, `<a href="#" data-crumb="loc">${escapeHtml(nav.location.name)}</a>`);
  if (nav.building) parts.push(`<span class="sep">/</span>`, `<a href="#" data-crumb="bld">${escapeHtml(nav.building.code)}</a>`);
  if (nav.floor) parts.push(`<span class="sep">/</span>`, `<a href="#" data-crumb="flr">${escapeHtml(nav.floor.name)}</a>`);
  if (nav.room) parts.push(`<span class="sep">/</span>`, `<span>Raum ${escapeHtml(nav.room.number)}</span>`);
  el.innerHTML = parts.join(" ");
  el.querySelectorAll("a[data-crumb]").forEach((a) => {
    a.addEventListener("click", async (ev) => {
      ev.preventDefault();
      const key = a.getAttribute("data-crumb");
      if (key === "Standorte") resetNav();
      else if (key === "loc") {
        nav.level = "buildings";
        nav.building = null;
        nav.floor = null;
        nav.room = null;
      } else if (key === "bld") {
        nav.level = "floors";
        nav.floor = null;
        nav.room = null;
      } else if (key === "flr") {
        nav.level = "rooms";
        nav.room = null;
      }
      await renderLocations();
    });
  });
}

function renderList(listEl, items, mapItem) {
  if (!items || items.length === 0) {
    listEl.innerHTML = `<li class="empty">Noch keine Einträge.</li>`;
    return;
  }
  listEl.innerHTML = "";
  items.forEach((item) => {
    const cfg = mapItem(item);
    const li = document.createElement("li");
    const main = document.createElement("div");
    main.className = "item-main";
    main.innerHTML = cfg.subtitleHtml || cfg.subtitle
      ? `<strong>${escapeHtml(cfg.title)}</strong><span>${cfg.subtitleHtml || escapeHtml(cfg.subtitle)}</span>`
      : `<strong>${escapeHtml(cfg.title)}</strong>`;
    if (cfg.onOpen) {
      main.style.cursor = "pointer";
      main.addEventListener("click", () => cfg.onOpen());
    }
    const actions = document.createElement("div");
    actions.className = "item-actions";
    if (cfg.onOpen) {
      const openBtn = document.createElement("button");
      openBtn.type = "button";
      openBtn.textContent = "Öffnen";
      openBtn.addEventListener("click", () => cfg.onOpen());
      actions.appendChild(openBtn);
    }
    if (cfg.extra) {
      const extraBtn = document.createElement("button");
      extraBtn.type = "button";
      extraBtn.className = "secondary";
      extraBtn.textContent = cfg.extra.label;
      extraBtn.addEventListener("click", async () => {
        try {
          await cfg.extra.run();
        } catch (err) {
          setMsg(document.querySelector("#locations-msg") || document.querySelector("#bookings-msg"), "error", err.message || "Aktion fehlgeschlagen.");
        }
      });
      actions.appendChild(extraBtn);
    }
    if (cfg.onDelete) {
      const delBtn = document.createElement("button");
      delBtn.type = "button";
      delBtn.className = "secondary";
      delBtn.textContent = "Löschen";
      delBtn.addEventListener("click", async () => {
        if (!confirm("Wirklich löschen?")) return;
        try {
          await cfg.onDelete();
        } catch (err) {
          setMsg(document.querySelector("#locations-msg"), "error", err.message || "Löschen fehlgeschlagen.");
        }
      });
      actions.appendChild(delBtn);
    }
    if (cfg.actions) {
      cfg.actions.forEach((act) => {
        const btn = document.createElement("button");
        btn.type = "button";
        btn.className = act.secondary ? "secondary" : "";
        btn.textContent = act.label;
        btn.addEventListener("click", act.run);
        actions.appendChild(btn);
      });
    }
    li.appendChild(main);
    li.appendChild(actions);
    listEl.appendChild(li);
  });
}

let overviewMap = null;
let pickerMap = null;
let pickerMarker = null;
let overviewRefreshTimer = null;
let pickerRefreshTimer = null;
let overviewRefreshToken = 0;
let pickerRefreshToken = 0;

function ensureLeaflet() {
  if (typeof L === "undefined") {
    throw new Error("Kartenbibliothek (Leaflet) nicht geladen.");
  }
}

function osmTiles(map) {
  L.tileLayer("https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png", {
    maxZoom: 19,
    attribution: '&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a>',
  }).addTo(map);
}

function mapIsAlive(map) {
  return !!(map && map._leaflet_id && map._container && map._container._leaflet_id);
}

function destroyOverviewMap() {
  overviewRefreshToken += 1;
  if (overviewRefreshTimer) {
    clearTimeout(overviewRefreshTimer);
    overviewRefreshTimer = null;
  }
  if (!overviewMap) return;
  const map = overviewMap;
  overviewMap = null;
  try {
    map.off();
    map.remove();
  } catch {
    /* already torn down */
  }
}

function destroyPickerMap() {
  pickerRefreshToken += 1;
  if (pickerRefreshTimer) {
    clearTimeout(pickerRefreshTimer);
    pickerRefreshTimer = null;
  }
  pickerMarker = null;
  if (!pickerMap) return;
  const map = pickerMap;
  pickerMap = null;
  try {
    map.off();
    map.remove();
  } catch {
    /* already torn down */
  }
}

function refreshMapView(map, applyView, kind) {
  if (!map) return;
  const token = kind === "picker" ? ++pickerRefreshToken : ++overviewRefreshToken;
  const run = () => {
    if (kind === "picker" && token !== pickerRefreshToken) return;
    if (kind === "overview" && token !== overviewRefreshToken) return;
    if (!mapIsAlive(map)) return;
    if (kind === "picker" && map !== pickerMap) return;
    if (kind === "overview" && map !== overviewMap) return;
    try {
      map.invalidateSize();
      if (typeof applyView === "function") applyView();
    } catch {
      /* ignore stale map */
    }
  };
  requestAnimationFrame(() => {
    run();
    const timer = setTimeout(run, 120);
    if (kind === "picker") pickerRefreshTimer = timer;
    else overviewRefreshTimer = timer;
  });
}

function renderLocationsOverviewMap(items) {
  const el = document.querySelector("#locations-overview-map");
  const hint = document.querySelector("#locations-overview-hint");
  if (!el) return;
  ensureLeaflet();
  destroyOverviewMap();

  const map = L.map(el);
  overviewMap = map;
  osmTiles(map);

  const withCoords = (items || []).filter(
    (it) => typeof it.latitude === "number" && typeof it.longitude === "number"
  );

  const applyView = () => {
    if (!mapIsAlive(map) || map !== overviewMap) return;
    if (!withCoords.length) {
      map.setView([51.16, 10.45], 6);
      return;
    }
    const bounds = L.latLngBounds(withCoords.map((it) => [it.latitude, it.longitude]));
    if (withCoords.length === 1) {
      map.setView([withCoords[0].latitude, withCoords[0].longitude], 14);
    } else {
      map.fitBounds(bounds.pad(0.35));
    }
  };

  withCoords.forEach((it) => {
    const marker = L.marker([it.latitude, it.longitude]).addTo(map);
    marker.bindPopup(`<strong>${escapeHtml(it.name)}</strong><br>${escapeHtml(it.address || "")}`);
  });

  if (!withCoords.length) {
    if (hint) {
      hint.textContent = "Noch keine Standorte mit Position — Adresse im Standort festlegen.";
    }
  } else if (hint) {
    hint.textContent = `${withCoords.length} Standort${withCoords.length === 1 ? "" : "e"} auf der Karte · Mausrad zum Zoomen.`;
  }

  refreshMapView(map, applyView, "overview");
}

async function nominatimSearch(query) {
  const url = new URL("https://nominatim.openstreetmap.org/search");
  url.searchParams.set("format", "json");
  url.searchParams.set("q", query);
  url.searchParams.set("limit", "6");
  url.searchParams.set("addressdetails", "1");
  url.searchParams.set("countrycodes", "de,at,ch");
  const res = await fetch(url.toString(), {
    headers: { Accept: "application/json", "Accept-Language": "de" },
  });
  if (!res.ok) throw new Error("Adresssuche fehlgeschlagen");
  return res.json();
}

async function nominatimReverse(lat, lng) {
  const url = new URL("https://nominatim.openstreetmap.org/reverse");
  url.searchParams.set("format", "json");
  url.searchParams.set("lat", String(lat));
  url.searchParams.set("lon", String(lng));
  url.searchParams.set("zoom", "18");
  url.searchParams.set("addressdetails", "1");
  const res = await fetch(url.toString(), {
    headers: { Accept: "application/json", "Accept-Language": "de" },
  });
  if (!res.ok) return null;
  return res.json();
}

function setPickerPosition(lat, lng, address) {
  const latEl = document.querySelector("#loc-lat");
  const lngEl = document.querySelector("#loc-lng");
  const addrEl = document.querySelector("#loc-address");
  const hint = document.querySelector("#loc-coords-hint");
  if (!latEl || !lngEl || !mapIsAlive(pickerMap)) return;
  latEl.value = String(lat);
  lngEl.value = String(lng);
  if (address != null && addrEl) addrEl.value = address;
  try {
    if (pickerMarker) {
      pickerMarker.setLatLng([lat, lng]);
    } else {
      pickerMarker = L.marker([lat, lng], { draggable: true }).addTo(pickerMap);
      pickerMarker.on("dragend", async () => {
        if (!mapIsAlive(pickerMap) || !pickerMarker) return;
        const pos = pickerMarker.getLatLng();
        setPickerPosition(pos.lat, pos.lng, addrEl?.value || "");
        try {
          const rev = await nominatimReverse(pos.lat, pos.lng);
          if (rev?.display_name && addrEl) addrEl.value = rev.display_name;
        } catch {
          /* ignore */
        }
      });
    }
    pickerMap.setView([lat, lng], Math.max(pickerMap.getZoom() || 0, 16));
  } catch {
    return;
  }
  if (hint) hint.textContent = `${lat.toFixed(5)}, ${lng.toFixed(5)} · Mausrad zum Zoomen, Pin ziehbar`;
}

function initLocationPicker(initial = null) {
  const el = document.querySelector("#loc-picker-map");
  if (!el || typeof L === "undefined") return;
  destroyPickerMap();

  const hasPos = initial?.latitude != null && initial?.longitude != null;
  const startLat = hasPos ? initial.latitude : 51.16;
  const startLng = hasPos ? initial.longitude : 10.45;
  const startZoom = hasPos ? 16 : 6;

  const map = L.map(el);
  pickerMap = map;
  osmTiles(map);
  map.on("click", async (e) => {
    if (!mapIsAlive(map) || map !== pickerMap) return;
    setPickerPosition(e.latlng.lat, e.latlng.lng, "");
    try {
      const rev = await nominatimReverse(e.latlng.lat, e.latlng.lng);
      if (rev?.display_name) {
        const addrEl = document.querySelector("#loc-address");
        if (addrEl) addrEl.value = rev.display_name;
      }
    } catch {
      /* ignore */
    }
  });

  refreshMapView(
    map,
    () => {
      if (!mapIsAlive(map) || map !== pickerMap) return;
      if (hasPos) {
        setPickerPosition(initial.latitude, initial.longitude, initial.address || "");
      } else {
        map.setView([startLat, startLng], startZoom);
      }
    },
    "picker"
  );

  const searchBtn = document.querySelector("#btn-loc-search");
  const searchInput = document.querySelector("#loc-address-search");
  const results = document.querySelector("#loc-search-results");

  async function runSearch() {
    const q = (searchInput?.value || "").trim();
    if (!q || !results) return;
    results.innerHTML = `<li><button type="button" disabled>Suche…</button></li>`;
    try {
      const hits = await nominatimSearch(q);
      if (!hits.length) {
        results.innerHTML = `<li><button type="button" disabled>Keine Treffer</button></li>`;
        return;
      }
      results.innerHTML = hits
        .map(
          (h, i) =>
            `<li><button type="button" data-i="${i}">${escapeHtml(h.display_name)}</button></li>`
        )
        .join("");
      results.querySelectorAll("button[data-i]").forEach((btn) => {
        btn.addEventListener("click", () => {
          const hit = hits[Number(btn.getAttribute("data-i"))];
          setPickerPosition(Number(hit.lat), Number(hit.lon), hit.display_name);
          results.innerHTML = "";
          if (searchInput) searchInput.value = hit.display_name;
        });
      });
    } catch (err) {
      results.innerHTML = `<li><button type="button" disabled>${escapeHtml(err.message || "Suche fehlgeschlagen")}</button></li>`;
    }
  }

  searchBtn?.addEventListener("click", (e) => {
    e.preventDefault();
    runSearch();
  });
  searchInput?.addEventListener("keydown", (e) => {
    if (e.key === "Enter") {
      e.preventDefault();
      runSearch();
    }
  });
}

function renderLocationAddressPanel(location) {
  const form = document.querySelector("#form-location-address");
  const readonly = document.querySelector("#location-detail-readonly");
  if (!form || !readonly) return;

  if (!isAdmin()) {
    form.classList.add("hidden");
    readonly.classList.remove("hidden");
    readonly.textContent = location.address
      ? location.address
      : "Für diesen Standort ist noch keine Adresse hinterlegt.";
    destroyPickerMap();
    return;
  }

  readonly.classList.add("hidden");
  form.classList.remove("hidden");
  form.name.value = location.name || "";
  form.address.value = location.address || "";
  form.latitude.value = location.latitude != null ? String(location.latitude) : "";
  form.longitude.value = location.longitude != null ? String(location.longitude) : "";
  const search = document.querySelector("#loc-address-search");
  if (search) search.value = location.address || "";
  const results = document.querySelector("#loc-search-results");
  if (results) results.innerHTML = "";
  queueMicrotask(() =>
    initLocationPicker({
      latitude: location.latitude,
      longitude: location.longitude,
      address: location.address || "",
    })
  );
}

let floorPlanState = null;
let roomPlanState = null;

function destroyFloorPlanEditor() {
  if (floorPlanState?.objectUrl) URL.revokeObjectURL(floorPlanState.objectUrl);
  if (floorPlanState?.onResize) window.removeEventListener("resize", floorPlanState.onResize);
  floorPlanState = null;
  const img = document.querySelector("#floor-plan-image");
  if (img) img.removeAttribute("src");
  const canvas = document.querySelector("#floor-plan-canvas");
  if (canvas) canvas.getContext("2d")?.clearRect(0, 0, canvas.width, canvas.height);
}

function destroyRoomPlanEditor() {
  if (roomPlanState?.objectUrl) URL.revokeObjectURL(roomPlanState.objectUrl);
  if (roomPlanState?.onResize) window.removeEventListener("resize", roomPlanState.onResize);
  if (roomPlanState?.imageEl) roomPlanState.imageEl.onload = null;
  roomPlanState = null;
  const canvas = document.querySelector("#room-plan-canvas");
  if (canvas) canvas.getContext("2d")?.clearRect(0, 0, canvas.width, canvas.height);
}

function paintNormRect(ctx, width, height, rect, fill, stroke, label, lineWidth = 2) {
  if (!rect) return;
  const x = rect.x * width;
  const y = rect.y * height;
  const w = rect.w * width;
  const h = rect.h * height;
  ctx.fillStyle = fill;
  ctx.strokeStyle = stroke;
  ctx.lineWidth = lineWidth;
  ctx.fillRect(x, y, w, h);
  ctx.strokeRect(x, y, w, h);
  if (label) {
    ctx.fillStyle = stroke;
    ctx.font = "600 12px Manrope, sans-serif";
    ctx.fillText(label, x + 4, y + 14);
  }
}

function absToLocal(abs, roomRect) {
  if (!abs || !roomRect || roomRect.w <= 0 || roomRect.h <= 0) return null;
  return {
    x: (abs.x - roomRect.x) / roomRect.w,
    y: (abs.y - roomRect.y) / roomRect.h,
    w: abs.w / roomRect.w,
    h: abs.h / roomRect.h,
  };
}

function localToAbs(local, roomRect) {
  if (!local || !roomRect) return null;
  return {
    x: roomRect.x + local.x * roomRect.w,
    y: roomRect.y + local.y * roomRect.h,
    w: local.w * roomRect.w,
    h: local.h * roomRect.h,
  };
}

function distributeRectsInRoom(roomRect, count) {
  if (count < 1) return [];
  const cols = Math.ceil(Math.sqrt(count));
  const rows = Math.ceil(count / cols);
  const gapX = roomRect.w * 0.04;
  const gapY = roomRect.h * 0.04;
  const cellW = Math.max(0.01, (roomRect.w - gapX * (cols + 1)) / cols);
  const cellH = Math.max(0.01, (roomRect.h - gapY * (rows + 1)) / rows);
  const out = [];
  for (let i = 0; i < count; i++) {
    const c = i % cols;
    const r = Math.floor(i / cols);
    out.push({
      x: roomRect.x + gapX + c * (cellW + gapX),
      y: roomRect.y + gapY + r * (cellH + gapY),
      w: cellW,
      h: cellH,
    });
  }
  return out;
}

function getFloorPlanDrawRect(canvas) {
  const img = document.querySelector("#floor-plan-image");
  if (!img?.naturalWidth || !floorPlanState) return null;
  const view = floorPlanState.view || { zoom: 1, panX: 0, panY: 0 };
  const base = Math.min(canvas.width / img.naturalWidth, canvas.height / img.naturalHeight) * 0.98;
  const scale = base * view.zoom;
  const dw = img.naturalWidth * scale;
  const dh = img.naturalHeight * scale;
  const dx = (canvas.width - dw) / 2 + view.panX;
  const dy = (canvas.height - dh) / 2 + view.panY;
  return { dx, dy, dw, dh, scale, nw: img.naturalWidth, nh: img.naturalHeight };
}

function syncFloorPlanCanvasSize() {
  const canvas = document.querySelector("#floor-plan-canvas");
  const stage = document.querySelector("#floor-plan-stage");
  const img = document.querySelector("#floor-plan-image");
  if (!canvas || !stage || !img?.naturalWidth) return;
  const w = Math.max(320, stage.clientWidth || 640);
  const h = Math.max(420, stage.clientHeight || 480);
  canvas.width = Math.round(w);
  canvas.height = Math.round(h);
  drawFloorPlanOverlay();
}

function drawFloorPlanOverlay() {
  const canvas = document.querySelector("#floor-plan-canvas");
  const img = document.querySelector("#floor-plan-image");
  if (!canvas || !floorPlanState?.layout || !img?.naturalWidth) return;
  const ctx = canvas.getContext("2d");
  const draw = getFloorPlanDrawRect(canvas);
  if (!draw) return;

  ctx.setTransform(1, 0, 0, 1, 0, 0);
  ctx.clearRect(0, 0, canvas.width, canvas.height);
  ctx.fillStyle = "#d8e1db";
  ctx.fillRect(0, 0, canvas.width, canvas.height);
  ctx.drawImage(img, 0, 0, draw.nw, draw.nh, draw.dx, draw.dy, draw.dw, draw.dh);

  const paintAbs = (rect, fill, stroke, label) => {
    if (!rect) return;
    const x = draw.dx + rect.x * draw.dw;
    const y = draw.dy + rect.y * draw.dh;
    const w = rect.w * draw.dw;
    const h = rect.h * draw.dh;
    ctx.fillStyle = fill;
    ctx.strokeStyle = stroke;
    ctx.lineWidth = 2;
    ctx.fillRect(x, y, w, h);
    ctx.strokeRect(x, y, w, h);
    if (label) {
      ctx.fillStyle = stroke;
      ctx.font = "600 12px Manrope, sans-serif";
      ctx.fillText(label, x + 4, y + 14);
    }
  };

  (floorPlanState.layout.rooms || []).forEach((rm) => {
    paintAbs(rm.rectangle, "rgba(15, 122, 77, 0.16)", "#0f7a65", rm.number || rm.name);
  });
  if (floorPlanState.draft) {
    paintAbs(floorPlanState.draft, "rgba(40, 90, 180, 0.28)", "#285ab4", "");
  }
}

function floorPlanScreenToNorm(e) {
  const canvas = document.querySelector("#floor-plan-canvas");
  const draw = getFloorPlanDrawRect(canvas);
  if (!canvas || !draw || draw.dw <= 0 || draw.dh <= 0) return null;
  const rect = canvas.getBoundingClientRect();
  const x = ((e.clientX - rect.left) / rect.width) * canvas.width;
  const y = ((e.clientY - rect.top) / rect.height) * canvas.height;
  return {
    x: Math.min(1, Math.max(0, (x - draw.dx) / draw.dw)),
    y: Math.min(1, Math.max(0, (y - draw.dy) / draw.dh)),
  };
}

function floorPlanZoomAt(factor, clientX, clientY) {
  if (!floorPlanState?.view) return;
  const canvas = document.querySelector("#floor-plan-canvas");
  if (!canvas) return;
  const before = getFloorPlanDrawRect(canvas);
  const nextZoom = Math.min(8, Math.max(0.4, floorPlanState.view.zoom * factor));
  if (before && clientX != null && clientY != null) {
    const rect = canvas.getBoundingClientRect();
    const mx = ((clientX - rect.left) / rect.width) * canvas.width;
    const my = ((clientY - rect.top) / rect.height) * canvas.height;
    const lx = (mx - before.dx) / before.dw;
    const ly = (my - before.dy) / before.dh;
    floorPlanState.view.zoom = nextZoom;
    const after = getFloorPlanDrawRect(canvas);
    if (after) {
      floorPlanState.view.panX += mx - (after.dx + lx * after.dw);
      floorPlanState.view.panY += my - (after.dy + ly * after.dh);
    }
  } else {
    floorPlanState.view.zoom = nextZoom;
  }
  drawFloorPlanOverlay();
}

function floorPlanResetView() {
  if (!floorPlanState) return;
  floorPlanState.view = { zoom: 1, panX: 0, panY: 0 };
  drawFloorPlanOverlay();
}

function fillFloorPlanTargets() {
  const targetSel = document.querySelector("#floor-plan-target");
  const hint = document.querySelector("#floor-plan-hint");
  if (!targetSel || !floorPlanState?.layout) return;
  const rooms = floorPlanState.layout.rooms || [];
  const prev = targetSel.value;
  targetSel.innerHTML = rooms.length
    ? rooms
        .map(
          (r) =>
            `<option value="${r.id}">Raum ${escapeHtml(r.number)}${r.name ? " — " + escapeHtml(r.name) : ""}${r.rectangle ? "" : " · ohne Rechteck"}</option>`
        )
        .join("")
    : `<option value="">Keine Räume</option>`;
  if (prev && [...targetSel.options].some((o) => o.value === prev)) targetSel.value = prev;
  targetSel.disabled = !rooms.length;
  if (hint) {
    hint.textContent = isAdmin()
      ? "Mausrad zoomen, mittlere Maustaste/Alt+Ziehen verschieben, links Rechteck zeichnen."
      : "Mausrad zoomen, mittlere Maustaste oder Alt+Ziehen verschieben.";
  }
  drawFloorPlanOverlay();
}

async function renderFloorPlanEditor(floorId) {
  const uploadBox = document.querySelector("#floor-plan-upload");
  const editor = document.querySelector("#floor-plan-editor");
  const empty = document.querySelector("#floor-plan-empty");
  const toolbar = document.querySelector("#floor-plan-toolbar");
  const msg = document.querySelector("#locations-msg");

  destroyFloorPlanEditor();
  wireFloorPlanCanvasOnce();
  uploadBox?.classList.toggle("hidden", !isAdmin());
  toolbar?.classList.toggle("hidden", !isAdmin());

  try {
    const layout = await api(`/api/v1/floors/${floorId}/layout`, { auth: true });
    floorPlanState = {
      floorId,
      layout,
      draft: null,
      drawing: false,
      panning: false,
      start: null,
      panLast: null,
      view: { zoom: 1, panX: 0, panY: 0 },
      objectUrl: null,
      onResize: () => syncFloorPlanCanvasSize(),
    };
    window.addEventListener("resize", floorPlanState.onResize);

    if (!layout.plan?.image_url) {
      editor?.classList.add("hidden");
      empty?.classList.remove("hidden");
      empty.textContent = "Noch kein Plan für dieses Stockwerk — als Admin oben ein Bild hochladen.";
      return;
    }

    empty?.classList.add("hidden");
    editor?.classList.remove("hidden");
    const blob = await apiBlob(layout.plan.image_url, { auth: true });
    floorPlanState.objectUrl = URL.createObjectURL(blob);
    const img = document.querySelector("#floor-plan-image");
    img.onload = () => {
      syncFloorPlanCanvasSize();
      fillFloorPlanTargets();
    };
    img.src = floorPlanState.objectUrl;
  } catch (err) {
    editor?.classList.add("hidden");
    empty?.classList.remove("hidden");
    empty.textContent = "Plan konnte nicht geladen werden.";
    setMsg(msg, "error", err.message || "Stockwerkplan laden fehlgeschlagen.");
  }
}

let floorPlanCanvasWired = false;

function wireFloorPlanCanvasOnce() {
  if (floorPlanCanvasWired) return;
  floorPlanCanvasWired = true;

  const canvas = document.querySelector("#floor-plan-canvas");
  const uploadBtn = document.querySelector("#btn-floor-plan-upload");
  const clearBtn = document.querySelector("#btn-floor-plan-clear-target");
  const zoomInBtn = document.querySelector("#btn-floor-plan-zoom-in");
  const zoomOutBtn = document.querySelector("#btn-floor-plan-zoom-out");
  const zoomResetBtn = document.querySelector("#btn-floor-plan-zoom-reset");

  zoomInBtn?.addEventListener("click", () => floorPlanZoomAt(1.25));
  zoomOutBtn?.addEventListener("click", () => floorPlanZoomAt(1 / 1.25));
  zoomResetBtn?.addEventListener("click", () => floorPlanResetView());

  uploadBtn?.addEventListener("click", async () => {
    const msg = document.querySelector("#locations-msg");
    clearMsg(msg);
    const file = document.querySelector("#floor-plan-file")?.files?.[0];
    if (!file || !nav.floor?.id) {
      setMsg(msg, "error", "Bitte zuerst eine Bilddatei wählen.");
      return;
    }
    const fd = new FormData();
    fd.append("file", file);
    uploadBtn.disabled = true;
    const prevLabel = uploadBtn.textContent;
    uploadBtn.textContent = "Lade hoch…";
    try {
      await apiUpload(`/api/v1/floors/${nav.floor.id}/plan`, fd, { auth: true });
      setMsg(msg, "ok", "Stockwerkplan hochgeladen.");
      await renderFloorPlanEditor(nav.floor.id);
    } catch (err) {
      setMsg(msg, "error", err.message || "Upload fehlgeschlagen.");
    } finally {
      uploadBtn.disabled = false;
      uploadBtn.textContent = prevLabel;
    }
  });

  clearBtn?.addEventListener("click", async () => {
    const msg = document.querySelector("#locations-msg");
    clearMsg(msg);
    const targetId = document.querySelector("#floor-plan-target")?.value;
    if (!targetId) return;
    try {
      await api(`/api/v1/rooms/${targetId}/rectangle`, { method: "PATCH", auth: true, body: { rectangle: null } });
      setMsg(msg, "ok", "Raum-Rechteck entfernt.");
      await renderFloorPlanEditor(nav.floor.id);
    } catch (err) {
      setMsg(msg, "error", err.message || "Löschen fehlgeschlagen.");
    }
  });

  canvas?.addEventListener(
    "wheel",
    (e) => {
      if (!floorPlanState) return;
      e.preventDefault();
      floorPlanZoomAt(e.deltaY < 0 ? 1.12 : 1 / 1.12, e.clientX, e.clientY);
    },
    { passive: false }
  );

  canvas?.addEventListener("pointerdown", (e) => {
    if (!floorPlanState) return;
    const wantsPan =
      e.button === 1 || e.altKey || e.button === 2 || !isAdmin() || !document.querySelector("#floor-plan-target")?.value;
    canvas.setPointerCapture(e.pointerId);
    if (wantsPan) {
      e.preventDefault();
      floorPlanState.panning = true;
      floorPlanState.panLast = { x: e.clientX, y: e.clientY };
      canvas.classList.add("is-panning");
      canvas.classList.remove("is-drawing");
      return;
    }
    if (e.button !== 0) return;
    const local = floorPlanScreenToNorm(e);
    if (!local) return;
    floorPlanState.drawing = true;
    floorPlanState.start = local;
    floorPlanState.draft = { x: local.x, y: local.y, w: 0, h: 0 };
    canvas.classList.add("is-drawing");
  });

  canvas?.addEventListener("pointermove", (e) => {
    if (!floorPlanState) return;
    if (floorPlanState.panning && floorPlanState.panLast) {
      const rect = canvas.getBoundingClientRect();
      const scaleX = canvas.width / rect.width;
      const scaleY = canvas.height / rect.height;
      floorPlanState.view.panX += (e.clientX - floorPlanState.panLast.x) * scaleX;
      floorPlanState.view.panY += (e.clientY - floorPlanState.panLast.y) * scaleY;
      floorPlanState.panLast = { x: e.clientX, y: e.clientY };
      drawFloorPlanOverlay();
      return;
    }
    if (!floorPlanState.drawing || !floorPlanState.start) return;
    const cur = floorPlanScreenToNorm(e);
    if (!cur) return;
    floorPlanState.draft = {
      x: Math.min(floorPlanState.start.x, cur.x),
      y: Math.min(floorPlanState.start.y, cur.y),
      w: Math.abs(cur.x - floorPlanState.start.x),
      h: Math.abs(cur.y - floorPlanState.start.y),
    };
    drawFloorPlanOverlay();
  });

  const endPointer = async () => {
    if (!floorPlanState) return;
    if (floorPlanState.panning) {
      floorPlanState.panning = false;
      floorPlanState.panLast = null;
      canvas.classList.remove("is-panning");
      return;
    }
    if (!floorPlanState.drawing) return;
    floorPlanState.drawing = false;
    canvas.classList.remove("is-drawing");
    const draft = floorPlanState.draft;
    floorPlanState.draft = null;
    drawFloorPlanOverlay();
    if (!draft || draft.w < 0.008 || draft.h < 0.008) return;
    const msg = document.querySelector("#locations-msg");
    clearMsg(msg);
    const targetId = document.querySelector("#floor-plan-target")?.value;
    if (!targetId) return;
    try {
      await api(`/api/v1/rooms/${targetId}/rectangle`, { method: "PATCH", auth: true, body: { rectangle: draft } });
      setMsg(msg, "ok", "Raum-Rechteck gespeichert.");
      await renderFloorPlanEditor(nav.floor.id);
    } catch (err) {
      setMsg(msg, "error", err.message || "Speichern fehlgeschlagen.");
      drawFloorPlanOverlay();
    }
  };

  canvas?.addEventListener("pointerup", endPointer);
  canvas?.addEventListener("pointercancel", endPointer);
  canvas?.addEventListener("contextmenu", (e) => e.preventDefault());
}

function getRoomPlanSource() {
  const img = roomPlanState?.imageEl;
  const room = roomPlanState?.roomRect;
  if (!img?.naturalWidth || !room) return null;
  return {
    sx: room.x * img.naturalWidth,
    sy: room.y * img.naturalHeight,
    sw: Math.max(1, room.w * img.naturalWidth),
    sh: Math.max(1, room.h * img.naturalHeight),
  };
}

function getRoomPlanDrawRect(canvas) {
  const src = getRoomPlanSource();
  if (!src || !roomPlanState) return null;
  const view = roomPlanState.view || { zoom: 1, panX: 0, panY: 0 };
  const base = Math.min(canvas.width / src.sw, canvas.height / src.sh) * 0.92;
  const scale = base * view.zoom;
  const dw = src.sw * scale;
  const dh = src.sh * scale;
  const dx = (canvas.width - dw) / 2 + view.panX;
  const dy = (canvas.height - dh) / 2 + view.panY;
  return { ...src, dx, dy, dw, dh, scale };
}

function syncRoomPlanCanvasSize() {
  const canvas = document.querySelector("#room-plan-canvas");
  const stage = document.querySelector("#room-plan-stage");
  if (!canvas || !stage || !roomPlanState?.imageEl?.naturalWidth || !roomPlanState?.roomRect) return;
  const w = Math.max(320, stage.clientWidth || 640);
  const h = Math.max(420, stage.clientHeight || 480);
  canvas.width = Math.round(w);
  canvas.height = Math.round(h);
  drawRoomPlanView();
}

function drawRoomPlanView() {
  const canvas = document.querySelector("#room-plan-canvas");
  if (!canvas || !roomPlanState?.imageEl?.naturalWidth || !roomPlanState?.roomRect) return;
  const ctx = canvas.getContext("2d");
  const img = roomPlanState.imageEl;
  const draw = getRoomPlanDrawRect(canvas);
  if (!draw) return;

  ctx.setTransform(1, 0, 0, 1, 0, 0);
  ctx.clearRect(0, 0, canvas.width, canvas.height);
  ctx.fillStyle = "#d8e1db";
  ctx.fillRect(0, 0, canvas.width, canvas.height);
  ctx.drawImage(img, draw.sx, draw.sy, draw.sw, draw.sh, draw.dx, draw.dy, draw.dw, draw.dh);
  ctx.strokeStyle = "rgba(15, 122, 101, 0.55)";
  ctx.lineWidth = 2;
  ctx.strokeRect(draw.dx + 0.5, draw.dy + 0.5, draw.dw - 1, draw.dh - 1);

  const paintLocal = (local, fill, stroke, label) => {
    if (!local) return;
    const x = draw.dx + local.x * draw.dw;
    const y = draw.dy + local.y * draw.dh;
    const w = local.w * draw.dw;
    const h = local.h * draw.dh;
    ctx.fillStyle = fill;
    ctx.strokeStyle = stroke;
    ctx.lineWidth = 2;
    ctx.fillRect(x, y, w, h);
    ctx.strokeRect(x, y, w, h);
    if (label) {
      ctx.fillStyle = stroke;
      ctx.font = "600 12px Manrope, sans-serif";
      ctx.fillText(label, x + 4, y + 14);
    }
  };

  (roomPlanState.places || []).forEach((pl) => {
    paintLocal(absToLocal(pl.rectangle, roomPlanState.roomRect), "rgba(196, 122, 46, 0.35)", "#a65d12", pl.number);
  });
  if (roomPlanState.draft) {
    paintLocal(roomPlanState.draft, "rgba(40, 90, 180, 0.3)", "#285ab4", "");
  }
}

function roomPlanScreenToLocal(e) {
  const canvas = document.querySelector("#room-plan-canvas");
  const draw = getRoomPlanDrawRect(canvas);
  if (!canvas || !draw || draw.dw <= 0 || draw.dh <= 0) return null;
  const rect = canvas.getBoundingClientRect();
  const x = ((e.clientX - rect.left) / rect.width) * canvas.width;
  const y = ((e.clientY - rect.top) / rect.height) * canvas.height;
  return {
    x: Math.min(1, Math.max(0, (x - draw.dx) / draw.dw)),
    y: Math.min(1, Math.max(0, (y - draw.dy) / draw.dh)),
  };
}

function roomPlanZoomAt(factor, clientX, clientY) {
  if (!roomPlanState?.view) return;
  const canvas = document.querySelector("#room-plan-canvas");
  if (!canvas) return;
  const before = getRoomPlanDrawRect(canvas);
  const nextZoom = Math.min(8, Math.max(0.4, roomPlanState.view.zoom * factor));
  if (before && clientX != null && clientY != null) {
    const rect = canvas.getBoundingClientRect();
    const mx = ((clientX - rect.left) / rect.width) * canvas.width;
    const my = ((clientY - rect.top) / rect.height) * canvas.height;
    const lx = (mx - before.dx) / before.dw;
    const ly = (my - before.dy) / before.dh;
    roomPlanState.view.zoom = nextZoom;
    const after = getRoomPlanDrawRect(canvas);
    if (after) {
      roomPlanState.view.panX += mx - (after.dx + lx * after.dw);
      roomPlanState.view.panY += my - (after.dy + ly * after.dh);
    }
  } else {
    roomPlanState.view.zoom = nextZoom;
  }
  drawRoomPlanView();
}

function roomPlanResetView() {
  if (!roomPlanState) return;
  roomPlanState.view = { zoom: 1, panX: 0, panY: 0 };
  drawRoomPlanView();
}

function fillRoomPlanTargets() {
  const sel = document.querySelector("#room-plan-target");
  const hint = document.querySelector("#room-plan-hint");
  if (!sel || !roomPlanState) return;
  const places = roomPlanState.places || [];
  const prev = sel.value;
  sel.innerHTML = places.length
    ? places
        .map(
          (p) =>
            `<option value="${p.id}">${escapeHtml(p.label || p.number)}${p.rectangle ? "" : " · ohne Rechteck"}</option>`
        )
        .join("")
    : `<option value="">Keine Plätze — zuerst anlegen</option>`;
  if (prev && [...sel.options].some((o) => o.value === prev)) sel.value = prev;
  sel.disabled = !places.length;
  if (hint) {
    hint.textContent = isAdmin()
      ? "Mausrad zoomen, mittlere Maustaste/Alt+Ziehen verschieben, links Rechteck zeichnen."
      : "Mausrad zoomen, mittlere Maustaste oder Alt+Ziehen verschieben.";
  }
  drawRoomPlanView();
}

async function renderRoomPlanEditor(floorId, roomId) {
  const editor = document.querySelector("#room-plan-editor");
  const empty = document.querySelector("#room-plan-empty");
  const toolbar = document.querySelector("#room-plan-toolbar");
  const msg = document.querySelector("#locations-msg");

  destroyRoomPlanEditor();
  wireRoomPlanCanvasOnce();
  toolbar?.classList.toggle("hidden", !isAdmin());

  try {
    const layout = await api(`/api/v1/floors/${floorId}/layout`, { auth: true });
    const room = (layout.rooms || []).find((r) => r.id === roomId);
    if (!layout.plan?.image_url) {
      editor?.classList.add("hidden");
      empty?.classList.remove("hidden");
      empty.textContent = "Für dieses Stockwerk fehlt noch ein Plan.";
      return;
    }
    if (!room?.rectangle) {
      editor?.classList.add("hidden");
      empty?.classList.remove("hidden");
      empty.textContent = "Für diesen Raum fehlt noch ein Rechteck auf dem Stockwerkplan — bitte zuerst dort markieren.";
      return;
    }

    empty?.classList.add("hidden");
    editor?.classList.remove("hidden");

    const blob = await apiBlob(layout.plan.image_url, { auth: true });
    const objectUrl = URL.createObjectURL(blob);
    const imageEl = new Image();
    roomPlanState = {
      floorId,
      roomId,
      roomRect: room.rectangle,
      places: (layout.places || []).filter((p) => p.room_id === roomId),
      draft: null,
      drawing: false,
      panning: false,
      start: null,
      panLast: null,
      view: { zoom: 1, panX: 0, panY: 0 },
      objectUrl,
      imageEl,
      onResize: () => syncRoomPlanCanvasSize(),
    };
    window.addEventListener("resize", roomPlanState.onResize);
    imageEl.onload = () => {
      syncRoomPlanCanvasSize();
      fillRoomPlanTargets();
    };
    imageEl.src = objectUrl;
  } catch (err) {
    editor?.classList.add("hidden");
    empty?.classList.remove("hidden");
    empty.textContent = "Raumplan konnte nicht geladen werden.";
    setMsg(msg, "error", err.message || "Raumplan laden fehlgeschlagen.");
  }
}

let roomPlanCanvasWired = false;

function wireRoomPlanCanvasOnce() {
  if (roomPlanCanvasWired) return;
  roomPlanCanvasWired = true;

  const canvas = document.querySelector("#room-plan-canvas");
  const clearBtn = document.querySelector("#btn-room-plan-clear-target");
  const distributeBtn = document.querySelector("#btn-room-plan-distribute");
  const zoomInBtn = document.querySelector("#btn-room-plan-zoom-in");
  const zoomOutBtn = document.querySelector("#btn-room-plan-zoom-out");
  const zoomResetBtn = document.querySelector("#btn-room-plan-zoom-reset");

  zoomInBtn?.addEventListener("click", () => roomPlanZoomAt(1.25));
  zoomOutBtn?.addEventListener("click", () => roomPlanZoomAt(1 / 1.25));
  zoomResetBtn?.addEventListener("click", () => roomPlanResetView());

  clearBtn?.addEventListener("click", async () => {
    const msg = document.querySelector("#locations-msg");
    clearMsg(msg);
    const targetId = document.querySelector("#room-plan-target")?.value;
    if (!targetId || !nav.floor?.id || !nav.room?.id) return;
    try {
      await api(`/api/v1/places/${targetId}/rectangle`, { method: "PATCH", auth: true, body: { rectangle: null } });
      setMsg(msg, "ok", "Platz-Rechteck entfernt.");
      await renderRoomPlanEditor(nav.floor.id, nav.room.id);
    } catch (err) {
      setMsg(msg, "error", err.message || "Löschen fehlgeschlagen.");
    }
  });

  distributeBtn?.addEventListener("click", async () => {
    const msg = document.querySelector("#locations-msg");
    clearMsg(msg);
    if (!roomPlanState?.roomRect) return;
    const places = roomPlanState.places || [];
    if (!places.length) {
      setMsg(msg, "error", "Keine Plätze im Raum — zuerst anlegen.");
      return;
    }
    const rects = distributeRectsInRoom(roomPlanState.roomRect, places.length);
    distributeBtn.disabled = true;
    try {
      for (let i = 0; i < places.length; i++) {
        await api(`/api/v1/places/${places[i].id}/rectangle`, {
          method: "PATCH",
          auth: true,
          body: { rectangle: rects[i] },
        });
      }
      setMsg(msg, "ok", `${places.length} Plätze verteilt.`);
      await renderRoomPlanEditor(nav.floor.id, nav.room.id);
    } catch (err) {
      setMsg(msg, "error", err.message || "Verteilen fehlgeschlagen.");
    } finally {
      distributeBtn.disabled = false;
    }
  });

  canvas?.addEventListener(
    "wheel",
    (e) => {
      if (!roomPlanState) return;
      e.preventDefault();
      roomPlanZoomAt(e.deltaY < 0 ? 1.12 : 1 / 1.12, e.clientX, e.clientY);
    },
    { passive: false }
  );

  canvas?.addEventListener("pointerdown", (e) => {
    if (!roomPlanState?.roomRect) return;
    const wantsPan = e.button === 1 || e.altKey || e.button === 2 || !isAdmin() || !document.querySelector("#room-plan-target")?.value;
    canvas.setPointerCapture(e.pointerId);
    if (wantsPan) {
      e.preventDefault();
      roomPlanState.panning = true;
      roomPlanState.panLast = { x: e.clientX, y: e.clientY };
      canvas.classList.add("is-panning");
      canvas.classList.remove("is-drawing");
      return;
    }
    if (e.button !== 0) return;
    const local = roomPlanScreenToLocal(e);
    if (!local) return;
    roomPlanState.drawing = true;
    roomPlanState.start = local;
    roomPlanState.draft = { x: local.x, y: local.y, w: 0, h: 0 };
    canvas.classList.add("is-drawing");
  });

  canvas?.addEventListener("pointermove", (e) => {
    if (!roomPlanState) return;
    if (roomPlanState.panning && roomPlanState.panLast) {
      const rect = canvas.getBoundingClientRect();
      const scaleX = canvas.width / rect.width;
      const scaleY = canvas.height / rect.height;
      roomPlanState.view.panX += (e.clientX - roomPlanState.panLast.x) * scaleX;
      roomPlanState.view.panY += (e.clientY - roomPlanState.panLast.y) * scaleY;
      roomPlanState.panLast = { x: e.clientX, y: e.clientY };
      drawRoomPlanView();
      return;
    }
    if (!roomPlanState.drawing || !roomPlanState.start) return;
    const cur = roomPlanScreenToLocal(e);
    if (!cur) return;
    roomPlanState.draft = {
      x: Math.min(roomPlanState.start.x, cur.x),
      y: Math.min(roomPlanState.start.y, cur.y),
      w: Math.abs(cur.x - roomPlanState.start.x),
      h: Math.abs(cur.y - roomPlanState.start.y),
    };
    drawRoomPlanView();
  });

  const endPointer = async (e) => {
    if (!roomPlanState) return;
    if (roomPlanState.panning) {
      roomPlanState.panning = false;
      roomPlanState.panLast = null;
      canvas.classList.remove("is-panning");
      return;
    }
    if (!roomPlanState.drawing) return;
    roomPlanState.drawing = false;
    canvas.classList.remove("is-drawing");
    const draft = roomPlanState.draft;
    roomPlanState.draft = null;
    drawRoomPlanView();
    if (!draft || draft.w < 0.02 || draft.h < 0.02) return;
    const abs = localToAbs(draft, roomPlanState.roomRect);
    const msg = document.querySelector("#locations-msg");
    clearMsg(msg);
    const targetId = document.querySelector("#room-plan-target")?.value;
    if (!targetId) return;
    try {
      await api(`/api/v1/places/${targetId}/rectangle`, { method: "PATCH", auth: true, body: { rectangle: abs } });
      setMsg(msg, "ok", "Platz-Rechteck gespeichert.");
      await renderRoomPlanEditor(nav.floor.id, nav.room.id);
    } catch (err) {
      setMsg(msg, "error", err.message || "Speichern fehlgeschlagen.");
      drawRoomPlanView();
    }
  };

  canvas?.addEventListener("pointerup", endPointer);
  canvas?.addEventListener("pointercancel", endPointer);
  canvas?.addEventListener("contextmenu", (e) => e.preventDefault());
}

function renderCreateForm() {
  const form = document.querySelector("#form-catalog-create");
  const fields = document.querySelector("#create-fields");
  const title = document.querySelector("#create-title");
  if (!isAdmin()) {
    form.classList.add("hidden");
    return;
  }
  form.classList.remove("hidden");

  if (nav.level === "locations") {
    title.textContent = "Standort anlegen";
    fields.innerHTML = `
      <label>Name<input name="name" required placeholder="Campus Nord" /></label>
      <p class="hint">Adresse und Kartenposition legst du danach in der Standort-Detailansicht fest.</p>`;
  } else if (nav.level === "buildings") {
    title.textContent = "Gebäude anlegen";
    const locAddr = nav.location?.address || "";
    fields.innerHTML = `
      <label>Name<input name="name" required placeholder="Hauptgebäude" /></label>
      <label>Kürzel<input name="code" required placeholder="HG" /></label>
      <label>Adresse (optional)<input name="address" placeholder="leer = Standort-Adresse" value="${escapeHtml(locAddr)}" /></label>
      <p class="hint">Die Kartenposition oben gilt für den Standort. Gebäude-Adresse nur bei Abweichung nötig.</p>`;
  } else if (nav.level === "floors") {
    title.textContent = "Stockwerk anlegen";
    fields.innerHTML = `
      <label>Name<input name="name" required placeholder="1. OG" /></label>
      <label>Ebene (Zahl)<input name="level" type="number" required value="1" /></label>`;
  } else if (nav.level === "rooms") {
    title.textContent = "Raum anlegen";
    fields.innerHTML = `
      <label>Nummer<input name="number" required placeholder="101" /></label>
      <label>Name<input name="name" placeholder="optional" /></label>`;
  } else if (nav.level === "places") {
    title.textContent = "Plätze anlegen";
    fields.innerHTML = `
      <label>Anzahl
        <input name="count" type="number" min="1" max="200" value="1" required />
      </label>
      <label id="place-number-wrap">Nummer (bei Anzahl 1)
        <input name="number" placeholder="01" />
      </label>
      <label>Platztyp<select name="place_type_id" required></select></label>
      <label>Buchungsmodus
        <select name="booking_mode">
          <option value="daily">tageweise</option>
          <option value="hourly">stundenweise</option>
        </select>
      </label>
      <label class="check-inline">
        <input type="checkbox" name="distribute" />
        Auf Stockwerkplan im Raum verteilen (Raum-Rechteck nötig)
      </label>
      <p class="hint">Bei Großräumen z. B. Anzahl 24 — Nummern werden automatisch vergeben (01, 02, …).</p>
      <fieldset class="equip-fieldset">
        <legend>Ausstattungen (optional, für alle)</legend>
        <div id="create-equip-checkboxes" class="check-grid"></div>
      </fieldset>`;
    loadPlaceTypeOptions(fields.querySelector("select[name=place_type_id]"));
    loadEquipmentCheckboxes(fields.querySelector("#create-equip-checkboxes"));
    const countInput = fields.querySelector("input[name=count]");
    const numberWrap = fields.querySelector("#place-number-wrap");
    const syncCountUi = () => {
      const n = Number(countInput.value || 1);
      numberWrap.classList.toggle("hidden", n !== 1);
      numberWrap.querySelector("input").required = n === 1;
    };
    countInput.addEventListener("input", syncCountUi);
    syncCountUi();
  }
}

async function loadPlaceTypeOptions(select) {
  const data = await api("/api/v1/place-types", { auth: true });
  select.innerHTML = "";
  if (!data.items.length) {
    select.innerHTML = `<option value="">Bitte zuerst Platztyp anlegen</option>`;
    return;
  }
  data.items.forEach((pt) => {
    const opt = document.createElement("option");
    opt.value = pt.id;
    opt.textContent = pt.name;
    select.appendChild(opt);
  });
}

async function loadEquipmentCheckboxes(container, selectedIds = []) {
  if (!container) return;
  const selected = new Set(selectedIds);
  const data = await api("/api/v1/equipments", { auth: true });
  if (!data.items.length) {
    container.innerHTML = `<p class="hint">Keine Ausstattungen definiert — optional unter „Ausstattung“ anlegen.</p>`;
    return;
  }
  container.innerHTML = data.items
    .map(
      (e) =>
        `<label><input type="checkbox" name="equipment_id" value="${e.id}" ${selected.has(e.id) ? "checked" : ""}/> ${escapeHtml(e.name)}</label>`
    )
    .join("");
}

document.querySelector("#form-location-address")?.addEventListener("submit", async (e) => {
  e.preventDefault();
  const msg = document.querySelector("#locations-msg");
  clearMsg(msg);
  if (!nav.location?.id) return;
  const fd = new FormData(e.target);
  const latRaw = fd.get("latitude");
  const lngRaw = fd.get("longitude");
  if (!latRaw || !lngRaw) {
    setMsg(msg, "error", "Bitte Adresse suchen oder Position auf der Karte setzen.");
    return;
  }
  try {
    const updated = await api(`/api/v1/locations/${nav.location.id}`, {
      method: "PATCH",
      auth: true,
      body: {
        name: fd.get("name"),
        address: fd.get("address") || "",
        latitude: Number(latRaw),
        longitude: Number(lngRaw),
      },
    });
    nav.location = updated;
    setMsg(msg, "ok", "Standort-Adresse gespeichert.");
    renderCrumbs();
    const title = document.querySelector("#locations-list-title");
    if (title && nav.level === "buildings") {
      title.textContent = `Gebäude in ${nav.location.name}`;
    }
    // Liste aktualisieren, damit Gebäude die Standort-Adresse anzeigen
    if (nav.level === "buildings") {
      await renderLocations();
      setMsg(msg, "ok", "Standort-Adresse gespeichert.");
    }
  } catch (err) {
    setMsg(msg, "error", err.message || "Adresse speichern fehlgeschlagen.");
  }
});

document.querySelector("#form-catalog-create").addEventListener("submit", async (e) => {
  e.preventDefault();
  const msg = document.querySelector("#locations-msg");
  clearMsg(msg);
  const fd = new FormData(e.target);
  try {
    if (nav.level === "locations") {
      await api("/api/v1/locations", {
        method: "POST",
        auth: true,
        body: { name: fd.get("name") },
      });
    } else if (nav.level === "buildings") {
      await api(`/api/v1/locations/${nav.location.id}/buildings`, {
        method: "POST",
        auth: true,
        body: { name: fd.get("name"), code: fd.get("code"), address: fd.get("address") || "" },
      });
    } else if (nav.level === "floors") {
      await api(`/api/v1/buildings/${nav.building.id}/floors`, {
        method: "POST",
        auth: true,
        body: { name: fd.get("name"), level: Number(fd.get("level")) },
      });
    } else if (nav.level === "rooms") {
      await api(`/api/v1/floors/${nav.floor.id}/rooms`, {
        method: "POST",
        auth: true,
        body: { number: fd.get("number"), name: fd.get("name") || "" },
      });
    } else if (nav.level === "places") {
      const equipmentIds = [...e.target.querySelectorAll("input[name=equipment_id]:checked")].map((el) => el.value);
      const count = Number(fd.get("count") || 1);
      const distribute = !!e.target.querySelector("input[name=distribute]")?.checked;
      if (count > 1 || distribute) {
        const result = await api(`/api/v1/rooms/${nav.room.id}/places/bulk`, {
          method: "POST",
          auth: true,
          body: {
            count: count < 1 ? 1 : count,
            place_type_id: fd.get("place_type_id"),
            booking_mode: fd.get("booking_mode"),
            equipment_ids: equipmentIds,
            distribute,
          },
        });
        setMsg(msg, "ok", `${result.count || count} Plätze angelegt${distribute ? " und auf dem Plan verteilt" : ""}.`);
      } else {
        const number = String(fd.get("number") || "").trim();
        if (!number) {
          setMsg(msg, "error", "Nummer ist erforderlich.");
          return;
        }
        await api(`/api/v1/rooms/${nav.room.id}/places`, {
          method: "POST",
          auth: true,
          body: {
            number,
            place_type_id: fd.get("place_type_id"),
            booking_mode: fd.get("booking_mode"),
            equipment_ids: equipmentIds,
          },
        });
        setMsg(msg, "ok", "Angelegt.");
      }
      e.target.reset();
      await renderLocations();
      return;
    }
    e.target.reset();
    setMsg(msg, "ok", "Angelegt.");
    await renderLocations();
  } catch (err) {
    setMsg(msg, "error", err.message || "Anlegen fehlgeschlagen.");
  }
});

async function renderPlaceTypes() {
  const msg = document.querySelector("#place-types-msg");
  const list = document.querySelector("#place-types-list");
  clearMsg(msg);
  applyAdminVisibility();
  list.innerHTML = `<li class="empty">Lade…</li>`;
  try {
    const data = await api("/api/v1/place-types", { auth: true });
    renderList(list, data.items, (item) => ({
      title: item.name,
      subtitle: item.description || "Keine Beschreibung",
      onDelete: isAdmin()
        ? async () => {
            await api(`/api/v1/place-types/${item.id}`, { method: "DELETE", auth: true });
            await renderPlaceTypes();
          }
        : null,
    }));
  } catch (err) {
    list.innerHTML = "";
    setMsg(msg, "error", err.message || "Laden fehlgeschlagen.");
  }
}

document.querySelector("#form-place-type").addEventListener("submit", async (e) => {
  e.preventDefault();
  const msg = document.querySelector("#place-types-msg");
  clearMsg(msg);
  const fd = new FormData(e.target);
  try {
    await api("/api/v1/place-types", {
      method: "POST",
      auth: true,
      body: { name: fd.get("name"), description: fd.get("description") || "" },
    });
    e.target.reset();
    setMsg(msg, "ok", "Platztyp angelegt.");
    await renderPlaceTypes();
  } catch (err) {
    setMsg(msg, "error", err.message || "Anlegen fehlgeschlagen.");
  }
});

async function renderEquipments() {
  const msg = document.querySelector("#equipments-msg");
  const list = document.querySelector("#equipments-list");
  clearMsg(msg);
  applyAdminVisibility();
  list.innerHTML = `<li class="empty">Lade…</li>`;
  try {
    const data = await api("/api/v1/equipments", { auth: true });
    renderList(list, data.items, (item) => ({
      title: item.name,
      onDelete: isAdmin()
        ? async () => {
            await api(`/api/v1/equipments/${item.id}`, { method: "DELETE", auth: true });
            await renderEquipments();
          }
        : null,
    }));
  } catch (err) {
    list.innerHTML = "";
    setMsg(msg, "error", err.message || "Laden fehlgeschlagen.");
  }
}

document.querySelector("#form-equipment").addEventListener("submit", async (e) => {
  e.preventDefault();
  const msg = document.querySelector("#equipments-msg");
  clearMsg(msg);
  const fd = new FormData(e.target);
  try {
    await api("/api/v1/equipments", { method: "POST", auth: true, body: { name: fd.get("name") } });
    e.target.reset();
    setMsg(msg, "ok", "Ausstattung angelegt.");
    await renderEquipments();
  } catch (err) {
    setMsg(msg, "error", err.message || "Anlegen fehlgeschlagen.");
  }
});

function toLocalInputValue(date) {
  const d = new Date(date);
  d.setMinutes(d.getMinutes() - d.getTimezoneOffset());
  return d.toISOString().slice(0, 16);
}

function fromLocalInputValue(value) {
  return new Date(value).toISOString();
}

function mondayOf(date = new Date()) {
  const d = new Date(date);
  d.setHours(0, 0, 0, 0);
  const day = d.getDay();
  const diff = day === 0 ? -6 : 1 - day;
  d.setDate(d.getDate() + diff);
  return d;
}

function formatISODate(d) {
  const x = new Date(d);
  x.setMinutes(x.getMinutes() - x.getTimezoneOffset());
  return x.toISOString().slice(0, 10);
}

function addDays(date, n) {
  const d = new Date(date);
  d.setDate(d.getDate() + n);
  return d;
}

function formatWeekLabel(startISO, endISO) {
  const opts = { day: "2-digit", month: "short" };
  const a = new Date(startISO + "T12:00:00");
  const b = new Date(endISO + "T12:00:00");
  return `${a.toLocaleDateString("de-DE", opts)} – ${b.toLocaleDateString("de-DE", opts)}`;
}

let myWeekStart = formatISODate(mondayOf());
let roomWeekStart = formatISODate(mondayOf());

async function renderBookingsPage() {
  const msg = document.querySelector("#bookings-msg");
  clearMsg(msg);
  const form = document.querySelector("#form-booking-search");
  const emptyHint = document.querySelector("#bookings-empty-hint");
  if (!form.starts_at.value) {
    const start = new Date();
    start.setMinutes(0, 0, 0);
    start.setHours(start.getHours() + 1);
    const end = new Date(start);
    end.setHours(end.getHours() + 8);
    form.starts_at.value = toLocalInputValue(start);
    form.ends_at.value = toLocalInputValue(end);
  }

  const [locs, types, equips] = await Promise.all([
    api("/api/v1/locations", { auth: true }),
    api("/api/v1/place-types", { auth: true }),
    api("/api/v1/equipments", { auth: true }),
  ]);
  if (emptyHint) {
    emptyHint.classList.toggle("hidden", (locs.items || []).length > 0);
  }

  const locSel = form.location_id;
  const typeSel = form.place_type_id;
  const eqSel = form.equipment_ids;
  const locVal = locSel.value;
  const typeVal = typeSel.value;
  locSel.innerHTML = `<option value="">alle</option>` + locs.items.map((l) => `<option value="${l.id}">${escapeHtml(l.name)}</option>`).join("");
  typeSel.innerHTML = `<option value="">alle</option>` + types.items.map((t) => `<option value="${t.id}">${escapeHtml(t.name)}</option>`).join("");
  eqSel.innerHTML = equips.items.map((e) => `<option value="${e.id}">${escapeHtml(e.name)}</option>`).join("");
  locSel.value = locVal;
  typeSel.value = typeVal;

  await initRoomWeekFilters(locs.items || []);
  await Promise.all([loadMyWeek(), loadMyBookings()]);
}

const ROOM_WEEK_FILTER_KEY = "desksharing.roomWeekFilter";

function readRoomWeekFilter() {
  try {
    return JSON.parse(sessionStorage.getItem(ROOM_WEEK_FILTER_KEY) || "{}") || {};
  } catch {
    return {};
  }
}

function writeRoomWeekFilter(partial) {
  const next = { ...readRoomWeekFilter(), ...partial };
  sessionStorage.setItem(ROOM_WEEK_FILTER_KEY, JSON.stringify(next));
}

function fillSelect(sel, items, { placeholder, value, labelFn }) {
  const prefer = value ?? "";
  sel.innerHTML =
    `<option value="">${escapeHtml(placeholder)}</option>` +
    items.map((it) => `<option value="${it.id}">${escapeHtml(labelFn(it))}</option>`).join("");
  if (prefer && [...sel.options].some((o) => o.value === prefer)) {
    sel.value = prefer;
  } else if (items.length === 1) {
    sel.value = items[0].id;
  } else {
    sel.value = "";
  }
  sel.disabled = items.length === 0;
}

async function initRoomWeekFilters(locations) {
  const locSel = document.querySelector("#room-week-location");
  const bldSel = document.querySelector("#room-week-building");
  const flrSel = document.querySelector("#room-week-floor");
  const roomSel = document.querySelector("#room-week-select");
  if (!locSel || !bldSel || !flrSel || !roomSel) return;

  const prefer = readRoomWeekFilter();
  let bootstrapping = true;

  fillSelect(locSel, locations, {
    placeholder: locations.length ? "Standort wählen" : "Kein Standort",
    value: prefer.locationId,
    labelFn: (l) => l.name,
  });

  const syncFromLocation = async () => {
    const keepBuilding = bootstrapping ? prefer.buildingId : "";
    const keepFloor = bootstrapping ? prefer.floorId : "";
    const keepRoom = bootstrapping ? prefer.roomId : "";
    writeRoomWeekFilter({ locationId: locSel.value, buildingId: "", floorId: "", roomId: "" });
    fillSelect(bldSel, [], { placeholder: "Gebäude wählen", labelFn: () => "" });
    fillSelect(flrSel, [], { placeholder: "Stockwerk wählen", labelFn: () => "" });
    fillSelect(roomSel, [], { placeholder: "Raum wählen", labelFn: () => "" });
    bldSel.disabled = !locSel.value;
    flrSel.disabled = true;
    roomSel.disabled = true;
    if (!locSel.value) {
      await loadRoomWeek();
      return;
    }
    const data = await api(`/api/v1/locations/${locSel.value}/buildings`, { auth: true });
    fillSelect(bldSel, data.items || [], {
      placeholder: "Gebäude wählen",
      value: keepBuilding,
      labelFn: (b) => `${b.code} — ${b.name}`,
    });
    await syncFromBuilding(keepFloor, keepRoom);
  };

  const syncFromBuilding = async (keepFloor = "", keepRoom = "") => {
    if (!bootstrapping) {
      keepFloor = "";
      keepRoom = "";
    }
    writeRoomWeekFilter({
      locationId: locSel.value,
      buildingId: bldSel.value,
      floorId: "",
      roomId: "",
    });
    fillSelect(flrSel, [], { placeholder: "Stockwerk wählen", labelFn: () => "" });
    fillSelect(roomSel, [], { placeholder: "Raum wählen", labelFn: () => "" });
    flrSel.disabled = !bldSel.value;
    roomSel.disabled = true;
    if (!bldSel.value) {
      await loadRoomWeek();
      return;
    }
    const data = await api(`/api/v1/buildings/${bldSel.value}/floors`, { auth: true });
    fillSelect(flrSel, data.items || [], {
      placeholder: "Stockwerk wählen",
      value: keepFloor,
      labelFn: (f) => `${f.name} (Ebene ${f.level})`,
    });
    await syncFromFloor(keepRoom);
  };

  const syncFromFloor = async (keepRoom = "") => {
    if (!bootstrapping) keepRoom = "";
    writeRoomWeekFilter({
      locationId: locSel.value,
      buildingId: bldSel.value,
      floorId: flrSel.value,
      roomId: "",
    });
    fillSelect(roomSel, [], { placeholder: "Raum wählen", labelFn: () => "" });
    roomSel.disabled = !flrSel.value;
    if (!flrSel.value) {
      await loadRoomWeek();
      return;
    }
    const data = await api(`/api/v1/floors/${flrSel.value}/rooms`, { auth: true });
    fillSelect(roomSel, data.items || [], {
      placeholder: "Raum wählen",
      value: keepRoom,
      labelFn: (r) => (r.name ? `${r.number} — ${r.name}` : `Raum ${r.number}`),
    });
    writeRoomWeekFilter({
      locationId: locSel.value,
      buildingId: bldSel.value,
      floorId: flrSel.value,
      roomId: roomSel.value,
    });
    await loadRoomWeek();
  };

  locSel.onchange = async () => {
    bootstrapping = false;
    await syncFromLocation();
  };
  bldSel.onchange = async () => {
    bootstrapping = false;
    await syncFromBuilding();
  };
  flrSel.onchange = async () => {
    bootstrapping = false;
    await syncFromFloor();
  };
  roomSel.onchange = async () => {
    writeRoomWeekFilter({
      locationId: locSel.value,
      buildingId: bldSel.value,
      floorId: flrSel.value,
      roomId: roomSel.value,
    });
    await loadRoomWeek();
  };

  await syncFromLocation();
  bootstrapping = false;
}

async function loadMyWeek() {
  const label = document.querySelector("#my-week-label");
  const cal = document.querySelector("#my-week-calendar");
  cal.innerHTML = `<div class="hint">Lade…</div>`;
  try {
    const data = await api(`/api/v1/bookings/my-week?week_start=${myWeekStart}`, { auth: true });
    myWeekStart = data.week_start;
    label.textContent = formatWeekLabel(data.week_start, data.week_end);
    const weekday = ["Mo", "Di", "Mi", "Do", "Fr", "Sa", "So"];
    cal.innerHTML = data.days
      .map((day, idx) => {
        const has = (day.bookings || []).length > 0;
        const items = (day.bookings || [])
          .slice(0, 3)
          .map((b) => `<span class="cal-item">${escapeHtml(b.place_label || "Buchung")}</span>`)
          .join("");
        const more = (day.bookings || []).length > 3 ? `<span class="cal-item">+${day.bookings.length - 3}</span>` : "";
        const d = new Date(day.date + "T12:00:00");
        return `<div class="cal-day ${has ? "has-booking" : ""}">
          <span class="cal-date">${weekday[idx]} ${d.toLocaleDateString("de-DE", { day: "2-digit", month: "2-digit" })}</span>
          ${items || '<span class="hint">—</span>'}${more}
        </div>`;
      })
      .join("");
  } catch (err) {
    cal.innerHTML = "";
    setMsg(document.querySelector("#bookings-msg"), "error", err.message || "Kalender laden fehlgeschlagen.");
  }
}

async function loadRoomWeek() {
  const roomSel = document.querySelector("#room-week-select");
  const label = document.querySelector("#room-week-label");
  const table = document.querySelector("#room-week-grid");
  const thead = table.querySelector("thead");
  const tbody = table.querySelector("tbody");
  if (!roomSel.value) {
    label.textContent = "—";
    thead.innerHTML = "";
    tbody.innerHTML = `<tr><td class="hint">Bitte Standort → Gebäude → Stockwerk → Raum wählen.</td></tr>`;
    return;
  }
  tbody.innerHTML = `<tr><td class="hint">Lade…</td></tr>`;
  try {
    const data = await api(
      `/api/v1/bookings/room-week?room_id=${encodeURIComponent(roomSel.value)}&week_start=${roomWeekStart}`,
      { auth: true }
    );
    roomWeekStart = data.week_start;
    label.textContent = formatWeekLabel(data.week_start, data.week_end);
    const weekday = ["Mo", "Di", "Mi", "Do", "Fr", "Sa", "So"];
    thead.innerHTML = `<tr><th class="place-col">Platz</th>${data.days
      .map((d, i) => {
        const dt = new Date(d + "T12:00:00");
        return `<th>${weekday[i]}<br>${dt.toLocaleDateString("de-DE", { day: "2-digit", month: "2-digit" })}</th>`;
      })
      .join("")}</tr>`;

    if (!data.places.length) {
      tbody.innerHTML = `<tr><td colspan="8" class="hint">Keine Plätze in diesem Raum.</td></tr>`;
      return;
    }

    const statusLabel = { free: "frei", partial: "teilw.", full: "ganz", blocked: "gesperrt" };
    tbody.innerHTML = data.places
      .map((place) => {
        const cells = place.days
          .map((day) => {
            const clickable = day.status === "free" || day.status === "partial";
            const cls = `cell-${day.status}${clickable ? " cell-bookable" : ""}`;
            const title = clickable
              ? `${place.label} am ${day.date} — Zeiten anzeigen / buchen`
              : statusLabel[day.status] || day.status;
            const bar = renderOccupancyBar(day.segments || [], day.status);
            return `<td class="${cls}" title="${title}" data-place-id="${place.id}" data-place-label="${escapeHtml(place.label)}" data-date="${day.date}" data-mode="${place.booking_mode}" data-status="${day.status}"><div>${statusLabel[day.status] || day.status}</div>${bar}</td>`;
          })
          .join("");
        return `<tr><td class="place-col">${escapeHtml(place.label)}</td>${cells}</tr>`;
      })
      .join("");

    tbody.querySelectorAll("td.cell-bookable").forEach((td) => {
      td.addEventListener("click", () => {
        openWeekBookDialog({
          placeId: td.getAttribute("data-place-id"),
          label: td.getAttribute("data-place-label"),
          date: td.getAttribute("data-date"),
          mode: td.getAttribute("data-mode") || "daily",
          status: td.getAttribute("data-status") || "free",
        });
      });
    });
  } catch (err) {
    tbody.innerHTML = "";
    setMsg(document.querySelector("#bookings-msg"), "error", err.message || "Raumwoche laden fehlgeschlagen.");
  }
}

function renderOccupancyBar(segments, status) {
  if (status === "blocked" || status === "full") {
    return `<div class="occupancy-bar"><span class="seg-busy" style="width:100%"></span></div>`;
  }
  if (status === "free" || !segments.length) {
    return `<div class="occupancy-bar"><span class="seg-free" style="width:100%"></span></div>`;
  }
  const parts = segments
    .map((s) => {
      const width = Math.max(0, (s.end_pct || 0) - (s.start_pct || 0));
      const cls = s.kind === "busy" ? "seg-busy" : "seg-free";
      return `<span class="${cls}" style="width:${width}%"></span>`;
    })
    .join("");
  return `<div class="occupancy-bar">${parts}</div>`;
}

async function openWeekBookDialog({ placeId, label, date, mode, status }) {
  const form = document.querySelector("#form-week-book");
  const summary = document.querySelector("#week-book-summary");
  const timeline = document.querySelector("#week-book-timeline");
  const slots = document.querySelector("#week-book-free-slots");
  const nice = new Date(date + "T12:00:00").toLocaleDateString("de-DE", {
    weekday: "long",
    day: "2-digit",
    month: "2-digit",
    year: "numeric",
  });
  form.place_id.value = placeId;
  form.date.value = date;
  summary.textContent = `${label} · ${nice}`;
  timeline.innerHTML = "";
  slots.innerHTML = `<p class="hint">Lade freie Zeiten…</p>`;

  const fullRadio = form.querySelector('input[name=scope][value=full]');
  const partialRadio = form.querySelector('input[name=scope][value=partial]');
  if (status === "partial" || mode === "hourly") {
    partialRadio.checked = true;
    fullRadio.checked = false;
  } else {
    fullRadio.checked = true;
    partialRadio.checked = false;
  }
  // ganzer Tag nur sinnvoll, wenn der Tag noch komplett frei ist
  fullRadio.disabled = status === "partial";
  if (status === "partial") {
    partialRadio.checked = true;
  }
  syncWeekBookPartialVisibility();
  form.classList.remove("hidden");
  form.scrollIntoView({ behavior: "smooth", block: "nearest" });

  try {
    const detail = await api(
      `/api/v1/bookings/place-day?place_id=${encodeURIComponent(placeId)}&date=${encodeURIComponent(date)}`,
      { auth: true }
    );
    timeline.innerHTML = (detail.segments || [])
      .map((s) => {
        const left = s.start_pct || 0;
        const width = Math.max(0, (s.end_pct || 0) - left);
        return `<span class="seg-${s.kind}" style="left:${left}%;width:${width}%"></span>`;
      })
      .join("");

    if (!(detail.free || []).length) {
      slots.innerHTML = `<p class="hint">Keine freien Zeiten an diesem Tag.</p>`;
      return;
    }
    slots.innerHTML =
      `<p class="hint">Freie Zeiten (08:00–18:00) — Klick übernimmt den Zeitraum:</p>` +
      detail.free
        .map(
          (slot) =>
            `<button type="button" class="secondary" data-start="${slot.start}" data-end="${slot.end}">${slot.start}–${slot.end}</button>`
        )
        .join("");
    slots.querySelectorAll("button[data-start]").forEach((btn) => {
      btn.addEventListener("click", () => {
        form.querySelector('input[name=scope][value=partial]').checked = true;
        form.start_time.value = btn.getAttribute("data-start");
        form.end_time.value = btn.getAttribute("data-end");
        syncWeekBookPartialVisibility();
      });
    });
  } catch (err) {
    slots.innerHTML = `<p class="hint">Freie Zeiten konnten nicht geladen werden.</p>`;
    setMsg(document.querySelector("#bookings-msg"), "error", err.message || "Tagesdetail fehlgeschlagen.");
  }
}

function syncWeekBookPartialVisibility() {
  const form = document.querySelector("#form-week-book");
  const partial = form.querySelector('input[name=scope][value=partial]')?.checked;
  document.querySelector("#week-book-partial-fields").classList.toggle("hidden", !partial);
}

document.querySelectorAll("#form-week-book input[name=scope]").forEach((el) => {
  el.addEventListener("change", syncWeekBookPartialVisibility);
});

document.querySelector("#btn-week-book-cancel")?.addEventListener("click", () => {
  document.querySelector("#form-week-book").classList.add("hidden");
});

document.querySelector("#form-week-book")?.addEventListener("submit", async (e) => {
  e.preventDefault();
  const form = e.target;
  const msg = document.querySelector("#bookings-msg");
  clearMsg(msg);
  const placeId = form.place_id.value;
  const date = form.date.value;
  const scope = form.querySelector('input[name=scope]:checked')?.value || "full";
  let startTime = "08:00";
  let endTime = "18:00";
  if (scope === "partial") {
    startTime = form.start_time.value;
    endTime = form.end_time.value;
    if (!startTime || !endTime || endTime <= startTime) {
      setMsg(msg, "error", "Bitte gültigen Zeitraum wählen (Bis nach Von).");
      return;
    }
  }
  const starts = new Date(`${date}T${startTime}:00`);
  const ends = new Date(`${date}T${endTime}:00`);
  const nice = starts.toLocaleDateString("de-DE");
  try {
    await api("/api/v1/bookings", {
      method: "POST",
      auth: true,
      body: {
        place_id: placeId,
        starts_at: starts.toISOString(),
        ends_at: ends.toISOString(),
      },
    });
    form.classList.add("hidden");
    setMsg(msg, "ok", `Gebucht: ${nice} ${startTime}–${endTime}`);
    await Promise.all([loadMyBookings(), loadMyWeek(), loadRoomWeek()]);
  } catch (err) {
    setMsg(msg, "error", err.message || "Buchung fehlgeschlagen.");
  }
});


document.querySelector("#btn-my-week-prev")?.addEventListener("click", async () => {
  myWeekStart = formatISODate(addDays(myWeekStart, -7));
  await loadMyWeek();
});
document.querySelector("#btn-my-week-next")?.addEventListener("click", async () => {
  myWeekStart = formatISODate(addDays(myWeekStart, 7));
  await loadMyWeek();
});
document.querySelector("#btn-room-week-prev")?.addEventListener("click", async () => {
  roomWeekStart = formatISODate(addDays(roomWeekStart, -7));
  await loadRoomWeek();
});
document.querySelector("#btn-room-week-next")?.addEventListener("click", async () => {
  roomWeekStart = formatISODate(addDays(roomWeekStart, 7));
  await loadRoomWeek();
});

async function loadMyBookings() {
  const list = document.querySelector("#my-bookings-list");
  list.innerHTML = `<li class="empty">Lade…</li>`;
  try {
    const data = await api("/api/v1/bookings/mine", { auth: true });
    renderList(list, data.items, (item) => {
      const actions = [];
      if (item.can_check_in) {
        actions.push({
          label: "Check-in",
          run: async () => {
            try {
              await api(`/api/v1/bookings/${item.id}/check-in`, { method: "POST", auth: true });
              setMsg(document.querySelector("#bookings-msg"), "ok", "Eingecheckt.");
              await Promise.all([loadMyBookings(), loadMyWeek(), loadRoomWeek()]);
            } catch (err) {
              setMsg(document.querySelector("#bookings-msg"), "error", err.message);
            }
          },
        });
      }
      if (item.can_check_out) {
        actions.push({
          label: "Check-out",
          run: async () => {
            try {
              await api(`/api/v1/bookings/${item.id}/check-out`, { method: "POST", auth: true });
              setMsg(document.querySelector("#bookings-msg"), "ok", "Ausgecheckt.");
              await Promise.all([loadMyBookings(), loadMyWeek(), loadRoomWeek()]);
            } catch (err) {
              setMsg(document.querySelector("#bookings-msg"), "error", err.message);
            }
          },
        });
      }
      if (item.status === "booked" || item.status === "possibly_free") {
        actions.push({
          label: "Stornieren",
          secondary: true,
          run: async () => {
            try {
              await api(`/api/v1/bookings/${item.id}/cancel`, { method: "POST", auth: true });
              await Promise.all([loadMyBookings(), loadMyWeek(), loadRoomWeek()]);
            } catch (err) {
              setMsg(document.querySelector("#bookings-msg"), "error", err.message);
            }
          },
        });
      }
      return {
        title: item.place_label || "Buchung",
        subtitle: `${bookingStatusLabel(item.status)} · ${new Date(item.starts_at).toLocaleString("de-DE")} – ${new Date(item.ends_at).toLocaleString("de-DE")}`,
        actions,
      };
    });
  } catch (err) {
    list.innerHTML = "";
    setMsg(document.querySelector("#bookings-msg"), "error", err.message || "Buchungen laden fehlgeschlagen.");
  }
}

document.querySelector("#form-booking-search").addEventListener("submit", async (e) => {
  e.preventDefault();
  const msg = document.querySelector("#bookings-msg");
  const list = document.querySelector("#booking-search-list");
  clearMsg(msg);
  const fd = new FormData(e.target);
  const params = new URLSearchParams({
    starts_at: fromLocalInputValue(fd.get("starts_at")),
    ends_at: fromLocalInputValue(fd.get("ends_at")),
  });
  if (fd.get("location_id")) params.set("location_id", fd.get("location_id"));
  if (fd.get("place_type_id")) params.set("place_type_id", fd.get("place_type_id"));
  const equipmentIds = [...e.target.equipment_ids.selectedOptions].map((o) => o.value);
  if (equipmentIds.length) params.set("equipment_ids", equipmentIds.join(","));
  if (fd.get("only_available")) params.set("only_available", "true");

  list.innerHTML = `<li class="empty">Suche…</li>`;
  try {
    const data = await api(`/api/v1/bookings/search?${params}`, { auth: true });
    renderList(list, data.items, (item) => ({
      title: item.label,
      subtitleHtml: `${item.available ? '<span class="badge free">frei</span>' : '<span class="badge busy">belegt</span>'} ${escapeHtml(item.place_type_name)} · ${escapeHtml(item.location_name)}<br>${equipBadges(item.equipments)}`,
      actions: item.available
        ? [
            {
              label: "Buchen",
              run: async () => {
                try {
                  await api("/api/v1/bookings", {
                    method: "POST",
                    auth: true,
                    body: {
                      place_id: item.id,
                      starts_at: params.get("starts_at"),
                      ends_at: params.get("ends_at"),
                    },
                  });
                  setMsg(msg, "ok", `${item.label} gebucht.`);
                  await Promise.all([loadMyBookings(), loadMyWeek(), loadRoomWeek()]);
                  e.target.requestSubmit();
                } catch (err) {
                  setMsg(msg, "error", err.message || "Buchung fehlgeschlagen.");
                }
              },
            },
          ]
        : [],
    }));
  } catch (err) {
    list.innerHTML = "";
    setMsg(msg, "error", err.message || "Suche fehlgeschlagen.");
  }
});

window.addEventListener("popstate", route);
route();
