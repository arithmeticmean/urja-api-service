package controllers

import "net/http"

// Docs serves the OpenAPI file and a Swagger UI page that renders it.
type Docs struct {
	spec []byte
}

func NewDocs(spec []byte) *Docs {
	return &Docs{spec: spec}
}

// GET /openapi.json
func (d *Docs) Spec(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write(d.spec)
}

// GET /docs
func (d *Docs) UI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(swaggerPage))
}

const swaggerPage = `<!doctype html>
<html>
<head>
  <meta charset="utf-8">
  <title>Urja Meter API</title>
  <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/swagger-ui-dist@5.17.14/swagger-ui.css">
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="https://cdn.jsdelivr.net/npm/swagger-ui-dist@5.17.14/swagger-ui-bundle.js"></script>
  <script>
    SwaggerUIBundle({ url: "/openapi.json", dom_id: "#swagger-ui" });
  </script>
</body>
</html>`
