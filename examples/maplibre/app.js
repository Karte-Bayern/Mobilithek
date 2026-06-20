const statusEl = document.querySelector("#status");
const typeFilterEl = document.querySelector("#type-filter");
const dataURL = new URLSearchParams(window.location.search).get("data") || "data/sample-events.geojson";

const colors = {
  MaintenanceWorks: "#dc2626",
  AbnormalTraffic: "#7c3aed",
  ReroutingManagement: "#2563eb"
};

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
  fitToData(events);
  statusEl.textContent = `${events.features.length} events loaded from ${dataURL}`;
});

function setupFilter(data) {
  const types = [...new Set(data.features.map((feature) => feature.properties.type))].sort();
  for (const type of types) {
    const option = document.createElement("option");
    option.value = type;
    option.textContent = type;
    typeFilterEl.appendChild(option);
  }

  typeFilterEl.addEventListener("change", () => {
    const selected = typeFilterEl.value;
    const filtered = selected
      ? {
          type: "FeatureCollection",
          features: events.features.filter((feature) => feature.properties.type === selected)
        }
      : events;

    map.getSource("events").setData(filtered);
    fitToData(filtered);
  });
}

function bindPopups() {
  for (const layer of ["event-lines", "event-points"]) {
    map.on("click", layer, (event) => {
      const feature = event.features[0];
      const properties = feature.properties;
      new maplibregl.Popup()
        .setLngLat(event.lngLat)
        .setHTML(`
          <h2 class="popup-title">${escapeHtml(properties.title)}</h2>
          <p class="popup-meta">
            Type: ${escapeHtml(properties.type)}<br>
            Start: ${escapeHtml(properties.start || "-")}<br>
            End: ${escapeHtml(properties.end || "-")}
          </p>
        `)
        .addTo(map);
    });

    map.on("mouseenter", layer, () => {
      map.getCanvas().style.cursor = "pointer";
    });

    map.on("mouseleave", layer, () => {
      map.getCanvas().style.cursor = "";
    });
  }
}

function fitToData(data) {
  const bounds = new maplibregl.LngLatBounds();
  let hasCoordinates = false;

  for (const feature of data.features) {
    const coordinates = feature.geometry.type === "Point"
      ? [feature.geometry.coordinates]
      : feature.geometry.coordinates;

    for (const coordinate of coordinates) {
      bounds.extend(coordinate);
      hasCoordinates = true;
    }
  }

  if (hasCoordinates) {
    map.fitBounds(bounds, { padding: 72, maxZoom: 11, duration: 400 });
  }
}

function colorExpression() {
  const expression = ["match", ["get", "type"]];
  for (const [type, color] of Object.entries(colors)) {
    expression.push(type, color);
  }
  expression.push("#475569");
  return expression;
}

function escapeHtml(value) {
  return String(value).replace(/[&<>"']/g, (char) => ({
    "&": "&amp;",
    "<": "&lt;",
    ">": "&gt;",
    '"': "&quot;",
    "'": "&#39;"
  }[char]));
}
