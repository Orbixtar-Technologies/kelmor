package httpserver

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/hosting-panel/panel/agent/operations"
	openapi "github.com/hosting-panel/panel/api"
	"github.com/hosting-panel/panel/internal/pkg/logging"
	"github.com/hosting-panel/panel/internal/store"
)

func TestRouterOpenAPIParity(t *testing.T) {
	api := New(store.NewMemory(), logging.New("test"), &operations.Host{Root: t.TempDir()})
	handler, ok := api.Handler().(chi.Routes)
	if !ok {
		t.Fatal("handler is not chi.Routes")
	}
	registered := map[string]bool{}
	if err := chi.Walk(handler, func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		if method == "HEAD" {
			return nil
		}
		registered[strings.ToUpper(method)+" "+normalizeRoute(route)] = true
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	documented := documentedOperations(t)
	for key := range registered {
		if !documented[key] {
			t.Errorf("undocumented route %s", key)
		}
	}
	for key := range documented {
		if !registered[key] {
			t.Errorf("stale OpenAPI path %s", key)
		}
	}
}

func TestOpenAPISecurityAndRootHealthServers(t *testing.T) {
	doc := parseOpenAPIDocument(t, string(openapi.YAML))
	if !doc.hasGlobalSecurity {
		t.Fatal("OpenAPI must declare global bearer-or-cookie security")
	}
	for _, key := range []string{"GET /healthz", "GET /readyz"} {
		op := doc.ops[key]
		if op == nil {
			t.Fatalf("missing %s", key)
			continue
		}
		if !op.public {
			t.Fatalf("%s must be a public security exception", key)
		}
		if op.server != "/" {
			t.Fatalf("%s server override %q, want /", key, op.server)
		}
	}
	refresh := doc.ops["POST /api/v1/auth/refresh"]
	if refresh == nil || refresh.public {
		t.Fatal("refresh must require an authenticated session")
	}
	login := doc.ops["POST /api/v1/auth/login"]
	if login == nil || !login.public {
		t.Fatal("login must remain a public exception")
	}
	for _, key := range []string{
		"GET /api/v1/accounts/{accountID}/files/content",
		"POST /api/v1/accounts/{accountID}/files/mkdir",
		"DELETE /api/v1/accounts/{accountID}/files",
		"PATCH /api/v1/accounts/{accountID}/files",
		"GET /api/v1/accounts/{accountID}/admin-tools",
		"GET /api/v1/accounts/{accountID}/databases/credentials",
		"POST /api/v1/server/services/{serviceName}/{action}",
	} {
		if doc.ops[key] == nil {
			t.Errorf("OpenAPI missing helper route %s", key)
		}
	}
	cron := doc.ops["POST /api/v1/accounts/{accountID}/cron"]
	if cron == nil || !cron.hasBody || !cron.hasStatus("201") {
		t.Fatalf("cron create must document body and 201, got %+v", cron)
	}
	createDB := doc.ops["POST /api/v1/accounts/{accountID}/databases"]
	if createDB == nil || !createDB.hasBody {
		t.Fatal("database create must document its request body")
	}
}

func TestProtectedOperationsRequireSession(t *testing.T) {
	st := store.NewMemory()
	if err := store.SeedDev(st, "admin", "ChangeMeOnce!2026", "admin@localhost"); err != nil {
		t.Fatal(err)
	}
	api := New(st, logging.New("test"), &operations.Host{Root: t.TempDir()})
	srv := httptest.NewServer(api.Handler())
	defer srv.Close()
	for _, path := range []string{"/api/v1/me", "/api/v1/server", "/api/v1/packages"} {
		code, body := requestJSONStatus(t, http.MethodGet, srv.URL+path, "", nil, nil)
		assertAPIErrorCode(t, code, body, http.StatusUnauthorized, "UNAUTHENTICATED")
	}
}

type openAPIOp struct {
	public   bool
	server   string
	hasBody  bool
	statuses map[string]bool
}

type openAPIDoc struct {
	hasGlobalSecurity bool
	ops               map[string]*openAPIOp
}

func documentedOperations(t *testing.T) map[string]bool {
	t.Helper()
	doc := parseOpenAPIDocument(t, string(openapi.YAML))
	out := map[string]bool{}
	for key := range doc.ops {
		out[key] = true
	}
	return out
}

func parseOpenAPIDocument(t *testing.T, raw string) openAPIDoc {
	t.Helper()
	doc := openAPIDoc{ops: map[string]*openAPIOp{}}
	lines := strings.Split(raw, "\n")
	globalServer := "/api/v1"
	inComponents := false
	var currentPath string
	var currentOp *openAPIOp
	var currentMethod string
	pathServers := []string{globalServer}
	indentOf := func(line string) int {
		return len(line) - len(strings.TrimLeft(line, " "))
	}
	for i := 0; i < len(lines); i++ {
		line := strings.TrimRight(lines[i], "\r")
		trim := strings.TrimSpace(line)
		if trim == "components:" {
			inComponents = true
			continue
		}
		if inComponents {
			continue
		}
		if strings.HasPrefix(line, "security:") && indentOf(line) == 0 {
			doc.hasGlobalSecurity = true
			continue
		}
		if indentOf(line) == 2 && strings.HasPrefix(trim, "/") && strings.HasSuffix(trim, ":") {
			currentPath = strings.TrimSuffix(trim, ":")
			pathServers = []string{globalServer}
			currentOp = nil
			currentMethod = ""
			continue
		}
		if currentPath != "" && indentOf(line) == 4 && trim == "servers:" {
			if urls := collectOpenAPIServerURLs(lines, i+1); len(urls) > 0 {
				pathServers = urls
			}
			continue
		}
		if currentPath != "" && indentOf(line) == 4 && strings.HasSuffix(trim, ":") {
			method := strings.TrimSuffix(trim, ":")
			switch method {
			case "get", "post", "put", "patch", "delete":
				currentMethod = strings.ToUpper(method)
				currentOp = &openAPIOp{server: pathServers[0], statuses: map[string]bool{}}
				registerOpenAPIOp(doc.ops, currentMethod, currentPath, pathServers, currentOp)
			}
			continue
		}
		if currentOp == nil {
			continue
		}
		if indentOf(line) == 6 && trim == "servers:" {
			if urls := collectOpenAPIServerURLs(lines, i+1); len(urls) > 0 {
				currentOp.server = urls[0]
				clearOpenAPIOp(doc.ops, currentMethod, currentPath, pathServers)
				registerOpenAPIOp(doc.ops, currentMethod, currentPath, urls, currentOp)
			}
		}
		if indentOf(line) == 6 && (trim == "security:" || strings.HasPrefix(trim, "security:")) {
			if strings.HasSuffix(trim, "[]") || (i+1 < len(lines) && strings.TrimSpace(lines[i+1]) == "[]") {
				currentOp.public = true
			}
		}
		if trim == "requestBody:" {
			currentOp.hasBody = true
		}
		if indentOf(line) >= 8 && strings.HasPrefix(trim, `"`) && strings.Contains(trim, ":") {
			code := strings.Trim(strings.SplitN(trim, ":", 2)[0], `"`)
			if len(code) == 3 && code[0] >= '1' && code[0] <= '5' {
				currentOp.statuses[code] = true
			}
		}
	}
	return doc
}

func (op *openAPIOp) hasStatus(code string) bool {
	return op != nil && op.statuses[code]
}

func collectOpenAPIServerURLs(lines []string, start int) []string {
	var urls []string
	for _, line := range lines[start:] {
		next := strings.TrimSpace(line)
		if next == "" {
			continue
		}
		if strings.HasPrefix(next, "- url:") {
			urls = append(urls, strings.TrimSpace(strings.TrimPrefix(next, "- url:")))
			continue
		}
		break
	}
	return urls
}

func registerOpenAPIOp(ops map[string]*openAPIOp, method, path string, servers []string, op *openAPIOp) {
	for _, server := range servers {
		ops[method+" "+joinServer(server, path)] = op
	}
}

func clearOpenAPIOp(ops map[string]*openAPIOp, method, path string, servers []string) {
	for _, server := range servers {
		delete(ops, method+" "+joinServer(server, path))
	}
}

func joinServer(server, path string) string {
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	if server == "/" {
		return path
	}
	server = strings.TrimSuffix(server, "/")
	if server == "" {
		server = "/api/v1"
	}
	return server + path
}

func normalizeRoute(route string) string {
	route = strings.TrimSpace(route)
	if idx := strings.Index(route, " *"); idx >= 0 {
		route = route[:idx]
	}
	if route == "/" {
		return "/"
	}
	return strings.TrimSuffix(route, "/")
}
