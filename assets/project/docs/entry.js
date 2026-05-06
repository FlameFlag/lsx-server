const root = document.querySelector("#openapi-docs-root");

const responseLabels = {
  "200": "OK",
  "400": "Bad Request",
  "405": "Method Not Allowed",
  "500": "Server Error",
};

function escapeHTML(value) {
  return String(value).replace(/[&<>"']/g, (char) => ({
    "&": "&amp;",
    "<": "&lt;",
    ">": "&gt;",
    "\"": "&quot;",
    "'": "&#39;",
  })[char]);
}

function cleanValue(value) {
  return value.trim().replace(/^["']|["']$/g, "");
}

function parseOpenAPISummary(source) {
  const lines = source.split(/\r?\n/);
  const title = matchValue(lines, /^  title:\s*(.+)$/) || "API";
  const version = matchValue(lines, /^  version:\s*(.+)$/) || "";
  const description = matchValue(lines, /^  description:\s*(.+)$/) || "";
  const endpoints = [];
  const componentParameters = parseComponentParameters(lines);
  let inPaths = false;
  let currentPath = "";
  let currentEndpoint = null;
  let mode = "";

  for (const line of lines) {
    if (/^paths:\s*$/.test(line)) {
      inPaths = true;
      continue;
    }
    if (inPaths && /^[A-Za-z][A-Za-z0-9_-]*:\s*$/.test(line)) {
      break;
    }
    if (!inPaths) {
      continue;
    }

    const pathMatch = line.match(/^  (\/[^:]+):\s*$/);
    if (pathMatch) {
      currentPath = pathMatch[1];
      currentEndpoint = null;
      mode = "";
      continue;
    }

    const methodMatch = line.match(/^    (get|post|put|patch|delete|head|options):\s*$/);
    if (methodMatch && currentPath) {
      currentEndpoint = {
        path: currentPath,
        method: methodMatch[1].toUpperCase(),
        summary: "",
        description: "",
        parameters: [],
        responses: [],
      };
      endpoints.push(currentEndpoint);
      mode = "";
      continue;
    }

    if (!currentEndpoint) {
      continue;
    }

    const summaryMatch = line.match(/^      summary:\s*(.+)$/);
    if (summaryMatch) {
      currentEndpoint.summary = cleanValue(summaryMatch[1]);
      continue;
    }

    const descriptionMatch = line.match(/^      description:\s*(.+)$/);
    if (descriptionMatch) {
      currentEndpoint.description = cleanValue(descriptionMatch[1]);
      continue;
    }

    if (/^      parameters:\s*$/.test(line)) {
      mode = "parameters";
      continue;
    }

    if (/^      responses:\s*$/.test(line)) {
      mode = "responses";
      continue;
    }

    const refMatch = line.match(/^        - \$ref:\s*"#\/components\/parameters\/([^"]+)"/);
    if (mode === "parameters" && refMatch) {
      currentEndpoint.parameters.push(componentParameters[refMatch[1]] || { name: refMatch[1], description: "" });
      continue;
    }

    const responseMatch = line.match(/^        "([^"]+)":\s*$/);
    if (mode === "responses" && responseMatch) {
      currentEndpoint.responses.push(responseMatch[1]);
    }
  }

  return { title, version, description, endpoints };
}

function parseComponentParameters(lines) {
  const parameters = {};
  let inParameters = false;
  let current = "";

  for (const line of lines) {
    if (/^  parameters:\s*$/.test(line)) {
      inParameters = true;
      continue;
    }
    if (inParameters && /^  responses:\s*$/.test(line)) {
      break;
    }
    if (!inParameters) {
      continue;
    }

    const keyMatch = line.match(/^    ([A-Za-z0-9_]+):\s*$/);
    if (keyMatch) {
      current = keyMatch[1];
      parameters[current] = { name: current, description: "" };
      continue;
    }

    if (!current) {
      continue;
    }

    const nameMatch = line.match(/^      name:\s*(.+)$/);
    if (nameMatch) {
      parameters[current].name = cleanValue(nameMatch[1]);
      continue;
    }

    const descriptionMatch = line.match(/^      description:\s*(.+)$/);
    if (descriptionMatch) {
      parameters[current].description = cleanValue(descriptionMatch[1]);
    }
  }

  return parameters;
}

function matchValue(lines, pattern) {
  for (const line of lines) {
    const match = line.match(pattern);
    if (match) {
      return cleanValue(match[1]);
    }
  }
  return "";
}

function renderDocs(spec) {
  root.innerHTML = `
    <section class="docs-spec-strip" aria-label="OpenAPI summary">
      <div><span>Version</span><strong>${escapeHTML(spec.version)}</strong></div>
      <div><span>Endpoints</span><strong>${escapeHTML(spec.endpoints.length)}</strong></div>
      <div><span>Format</span><strong>OpenAPI 3.1</strong></div>
    </section>
    <section class="docs-spec-card">
      <p class="docs-kicker">Contract</p>
      <h3>${escapeHTML(spec.title)}</h3>
      <p>${escapeHTML(spec.description)}</p>
    </section>
    <section class="docs-endpoints" aria-label="Endpoints">
      ${spec.endpoints.map(renderEndpoint).join("") || "<p>No paths found in the OpenAPI document.</p>"}
    </section>
  `;
}

function renderEndpoint(endpoint) {
  const parameters = endpoint.parameters.map((param) => `
    <li>
      <code>${escapeHTML(param.name)}</code>
      <span>${escapeHTML(param.description || "Query parameter")}</span>
    </li>
  `).join("");
  const responses = endpoint.responses.map((code) => `
    <span class="docs-response"><strong>${escapeHTML(code)}</strong> ${escapeHTML(responseLabels[code] || "Response")}</span>
  `).join("");

  return `
    <section class="docs-endpoint">
      <header class="docs-endpoint-head">
        <span class="docs-method">${escapeHTML(endpoint.method)}</span>
        <div>
          <h3>${escapeHTML(endpoint.path)}</h3>
          <p>${escapeHTML(endpoint.summary || "Endpoint")}</p>
        </div>
      </header>
      ${endpoint.description ? `<p class="docs-description">${escapeHTML(endpoint.description)}</p>` : ""}
      ${parameters ? `<ul class="docs-params">${parameters}</ul>` : ""}
      ${responses ? `<div class="docs-responses">${responses}</div>` : ""}
    </section>
  `;
}

async function loadDocs() {
  if (!root) {
    return;
  }
  try {
    const response = await fetch(root.dataset.openapiUrl || "/openapi.yaml", { headers: { "Accept": "application/yaml,text/yaml,*/*" } });
    if (!response.ok) {
      throw new Error(`HTTP ${response.status}`);
    }
    renderDocs(parseOpenAPISummary(await response.text()));
  } catch (error) {
    root.innerHTML = `<p class="docs-error">OpenAPI documentation could not be loaded: ${escapeHTML(error.message)}</p>`;
  }
}

loadDocs();
