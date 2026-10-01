export async function api(path, { method = "GET", body } = {}) {
  const headers = { Accept: "application/json" };
  if (body !== undefined) headers["Content-Type"] = "application/json";

  const res = await fetch(path, {
    method,
    headers,
    credentials: "same-origin",
    body: body !== undefined ? JSON.stringify(body) : undefined,
  });

  let data = null;
  const text = await res.text();
  if (text) {
    try {
      data = JSON.parse(text);
    } catch {
      data = { message: text };
    }
  }

  if (!res.ok) {
    const err = new Error((data && (data.message || data.error)) || res.statusText);
    err.status = res.status;
    err.data = data;
    throw err;
  }
  return data;
}

export async function apiUpload(path, formData) {
  const res = await fetch(path, {
    method: "POST",
    headers: { Accept: "application/json" },
    credentials: "same-origin",
    body: formData,
  });
  let data = null;
  const text = await res.text();
  if (text) {
    try {
      data = JSON.parse(text);
    } catch {
      data = { message: text };
    }
  }
  if (!res.ok) {
    const err = new Error((data && (data.message || data.error)) || res.statusText);
    err.status = res.status;
    err.data = data;
    throw err;
  }
  return data;
}

export async function apiBlob(path) {
  const res = await fetch(path, { credentials: "same-origin" });
  if (!res.ok) {
    const err = new Error(res.statusText || "Bild laden fehlgeschlagen");
    err.status = res.status;
    throw err;
  }
  return res.blob();
}
