const statusEl = document.querySelector("#status");
const searchFilterEl = document.querySelector("#search-filter");
const roadFilterEl = document.querySelector("#road-filter");
const statusFilterEl = document.querySelector("#status-filter");
const resetFiltersEl = document.querySelector("#reset-filters");
const eventListEl = document.querySelector("#event-list");
const visibleCountEl = document.querySelector("#visible-count");
const currentCountEl = document.querySelector("#current-count");
const futureCountEl = document.querySelector("#future-count");
const dataURL = new URLSearchParams(window.location.search).get("data") || "data/real-roadworks.geojson";

const colors = {
  A8: "#c2410c",
  A9: "#2563eb",
  MaintenanceWorks: "#dc2626",
  AbnormalTraffic: "#7c3aed",
  ReroutingManagement: "#2563eb"
};

const collator = new Intl.Collator("de", { numeric: true, sensitivity: "base" });

// GitHub Pages serves this checked-in excerpt directly. Keeping it static
// avoids exposing credentials or mirroring large upstream traffic feeds.
const map = new maplibregl.Map({
  container: "map",
  center: [11.55, 48.85],
  zoom: 6.5,
  style: {
    version: 8,
    glyphs: "https://demotiles.maplibre.org/font/{fontstack}/{range}.pbf",
    sources: {
      osm: {
        type: "raster",
        tiles: ["https://tile.openstreetmap.org/{z}/{x}/{y}.png"],
        tileSize: 256,
        attribution: '&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a>'
      }
    },
    layers: [
      {
        id: "osm",
        type: "raster",
        source: "osm"
      }
    ]
  }
});

map.addControl(new maplibregl.NavigationControl(), "top-right");

let events = null;
let activeFeatureID = "";

map.on("load", async () => {
  try {
    const response = await fetch(dataURL);
    if (!response.ok) {
      throw new Error(`Could not load ${dataURL}: HTTP ${response.status}`);
    }
    events = await response.json();
  } catch (error) {
    statusEl.textContent = error.message;
    console.error(error);
    return;
  }

  map.addSource("events", {
    type: "geojson",
    data: events
  });

  map.addLayer({
    id: "event-lines",
    type: "line",
    source: "events",
    filter: ["==", ["geometry-type"], "LineString"],
    paint: {
      "line-color": colorExpression(),
      "line-width": 4,
      "line-opacity": 0.82
    }
  });

  map.addLayer({
    id: "event-points",
    type: "circle",
    source: "events",
    filter: ["==", ["geometry-type"], "Point"],
    paint: {
      "circle-color": colorExpression(),
      "circle-radius": 7,
      "circle-stroke-color": "#ffffff",
      "circle-stroke-width": 2
    }
  });

  setupFilter(events);
  bindPopups();
  updateView();
});

function setupFilter(data) {
  const roads = [...new Set(data.features.map((feature) => feature.properties.road).filter(Boolean))].sort(collator.compare);
  for (const road of roads) {
    const option = document.createElement("option");
    option.value = road;
    option.textContent = road;
    roadFilterEl.appendChild(option);
  }

  searchFilterEl.addEventListener("input", updateView);
  roadFilterEl.addEventListener("change", updateView);
  statusFilterEl.addEventListener("change", updateView);
  resetFiltersEl.addEventListener("click", resetFilters);
}

function updateView() {
  const filtered = filteredEvents();
  map.getSource("events").setData(filtered);
  renderList(filtered);
  updateSummary(filtered);
  fitToData(filtered);

  const date = events.features[0]?.properties.fetchedAt;
  const suffix = date ? `, abgerufen am ${date}` : "";
  statusEl.textContent = `${filtered.features.length} von ${events.features.length} Meldungen geladen${suffix}`;
}

function filteredEvents() {
  // Filtering stays client-side so the published demo remains plain static
  // files under /examples/maplibre/.
  const query = normalizeSearch(searchFilterEl.value);
  const road = roadFilterEl.value;
  const status = statusFilterEl.value;
  return {
    type: "FeatureCollection",
    features: events.features.filter((feature) => {
      const properties = feature.properties;
      return (!query || searchableText(properties).includes(query)) &&
        (!road || properties.road === road) &&
        (!status || properties.status === status);
    }).sort(compareFeatures)
  };
}

function renderList(data) {
  eventListEl.replaceChildren();
  if (data.features.length === 0) {
    const item = document.createElement("li");
    item.className = "event-empty";
    item.textContent = "Keine Meldungen für diesen Filter.";
    eventListEl.appendChild(item);
    return;
  }

  for (const feature of data.features) {
    const item = document.createElement("li");
    const button = document.createElement("button");
    button.className = "event-button";
    button.type = "button";
    button.dataset.featureId = feature.properties.id || "";
    button.setAttribute("aria-pressed", String(button.dataset.featureId === activeFeatureID));
    button.innerHTML = `
      <span class="event-topline">
        <span class="badge">${escapeHtml(feature.properties.road || feature.properties.type || "Meldung")} · ${statusLabel(feature.properties.status)}</span>
        <span class="event-start">${formatDate(feature.properties.start)}</span>
      </span>
      <span class="event-title">${escapeHtml(feature.properties.title || "Ohne Titel")}</span>
      <span class="event-meta">${escapeHtml(feature.properties.subtitle || feature.properties.start || "")}</span>
    `;
    button.addEventListener("click", () => {
      activeFeatureID = button.dataset.featureId;
      markActiveListItem();
      fitToData({ type: "FeatureCollection", features: [feature] });
      showPopup(feature);
    });
    item.appendChild(button);
    eventListEl.appendChild(item);
  }
}

function updateSummary(data) {
  visibleCountEl.textContent = String(data.features.length);
  currentCountEl.textContent = String(data.features.filter((feature) => feature.properties.status === "current").length);
  futureCountEl.textContent = String(data.features.filter((feature) => feature.properties.status === "future").length);
}

function resetFilters() {
  searchFilterEl.value = "";
  roadFilterEl.value = "";
  statusFilterEl.value = "";
  activeFeatureID = "";
  updateView();
}

function markActiveListItem() {
  for (const button of eventListEl.querySelectorAll(".event-button")) {
    button.setAttribute("aria-pressed", String(button.dataset.featureId === activeFeatureID));
  }
}

function bindPopups() {
  for (const layer of ["event-lines", "event-points"]) {
    map.on("click", layer, (event) => {
      showPopup(event.features[0], event.lngLat);
    });

    map.on("mouseenter", layer, () => {
      map.getCanvas().style.cursor = "pointer";
    });

    map.on("mouseleave", layer, () => {
      map.getCanvas().style.cursor = "";
    });
  }
}

function showPopup(feature, lngLat) {
  const properties = feature.properties;
  activeFeatureID = properties.id || activeFeatureID;
  markActiveListItem();
  // List clicks do not have a cursor position, so use the middle coordinate
  // of a line feature as a stable popup anchor.
  const anchor = lngLat || popupAnchor(feature);
  const source = properties.sourceUrl
    ? `<br>Quelle: <a href="${escapeAttribute(properties.sourceUrl)}" rel="noreferrer">${escapeHtml(properties.source || "Quelle")}</a>`
    : "";

  new maplibregl.Popup()
    .setLngLat(anchor)
    .setHTML(`
      <h2 class="popup-title">${escapeHtml(properties.title || "Meldung")}</h2>
      <p class="popup-meta">
        ${escapeHtml(properties.subtitle || properties.type || "")}<br>
        Status: ${statusLabel(properties.status)}<br>
        Start: ${escapeHtml(formatDate(properties.start))}${source}
      </p>
    `)
    .addTo(map);
}

function popupAnchor(feature) {
  if (feature.geometry.type === "Point") {
    return feature.geometry.coordinates;
  }
  const coordinates = feature.geometry.coordinates;
  return coordinates[Math.floor(coordinates.length / 2)];
}

function fitToData(data) {
  if (data.bbox) {
    map.fitBounds(boundsFromBBox(data.bbox), { padding: 72, maxZoom: 11, duration: 400 });
    return;
  }

  const bounds = new maplibregl.LngLatBounds();
  let hasCoordinates = false;

  for (const feature of data.features) {
    if (feature.bbox) {
      const featureBounds = boundsFromBBox(feature.bbox);
      bounds.extend(featureBounds[0]);
      bounds.extend(featureBounds[1]);
      hasCoordinates = true;
      continue;
    }

    for (const coordinate of featureCoordinates(feature)) {
      bounds.extend(coordinate);
      hasCoordinates = true;
    }
  }

  if (hasCoordinates) {
    map.fitBounds(bounds, { padding: 72, maxZoom: 11, duration: 400 });
  }
}

function featureCoordinates(feature) {
  return feature.geometry.type === "Point"
    ? [feature.geometry.coordinates]
    : feature.geometry.coordinates;
}

function boundsFromBBox(bbox) {
  return [
    [bbox[0], bbox[1]],
    [bbox[2], bbox[3]]
  ];
}

function colorExpression() {
  // Real excerpts use "road"; synthetic converter samples use "type".
  const expression = ["match", ["coalesce", ["get", "road"], ["get", "type"]]];
  for (const [key, color] of Object.entries(colors)) {
    expression.push(key, color);
  }
  expression.push("#475569");
  return expression;
}

function compareFeatures(left, right) {
  return collator.compare(left.properties.road || "", right.properties.road || "") ||
    statusRank(left.properties.status) - statusRank(right.properties.status) ||
    collator.compare(left.properties.title || "", right.properties.title || "");
}

function statusRank(status) {
  return status === "current" ? 0 : 1;
}

function statusLabel(status) {
  if (status === "future") {
    return "geplant";
  }
  if (status === "current") {
    return "aktuell";
  }
  return "Demo";
}

function searchableText(properties) {
  return normalizeSearch([
    properties.road,
    properties.title,
    properties.subtitle,
    properties.description,
    properties.type,
    properties.status
  ].join(" "));
}

function normalizeSearch(value) {
  return String(value)
    .toLocaleLowerCase("de")
    .normalize("NFD")
    .replace(/\p{Diacritic}/gu, "")
    .trim();
}

function formatDate(value) {
  if (!value) {
    return "-";
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }
  return new Intl.DateTimeFormat("de-DE", {
    day: "2-digit",
    month: "2-digit",
    year: "2-digit"
  }).format(date);
}

function escapeHtml(value) {
  // Popups are assembled as HTML strings, so every data value is escaped even
  // for curated demo files.
  return String(value).replace(/[&<>"']/g, (char) => ({
    "&": "&amp;",
    "<": "&lt;",
    ">": "&gt;",
    '"': "&quot;",
    "'": "&#39;"
  }[char]));
}

function escapeAttribute(value) {
  return escapeHtml(value).replace(/`/g, "&#96;");
}
