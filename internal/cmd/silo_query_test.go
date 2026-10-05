package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/invopop/client.go/invopop"
)

// queryServer fakes the query endpoints, serving pages keyed by cursor and
// recording each request's parameters.
func queryServer(t *testing.T, pages map[string]string) (string, *[]url.Values) {
	t.Helper()
	var seen []url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.URL.Query())
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/silo/v1/entries/query":
			body, ok := pages[r.URL.Query().Get("cursor")]
			if !ok {
				http.NotFound(w, r)
				return
			}
			_, _ = w.Write([]byte(body))
		case "/silo/v1/entries/query/count":
			_, _ = w.Write([]byte(`{"schema":"bill/invoice","count":7}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	t.Setenv("INVOPOP_TOKEN", "key")
	t.Setenv(workspaceEnv, "")
	return srv.URL, &seen
}

func TestSiloEntriesQuery(t *testing.T) {
	host, seen := queryServer(t, map[string]string{
		"": `{"list":[{"id":"e1"}],"schema":"bill/invoice","limit":50,"next_cursor":"p2"}`,
	})

	out, err := run(t, "--host", host, "silo", "entries", "query", "bill/invoice",
		"--filter", `state = "sent" && folder = "sales"`, "--order-by", "issue_date desc",
		"--limit", "20", "--fields", "id,code", "--count")
	if err != nil {
		t.Fatal(err)
	}

	q := (*seen)[0]
	want := url.Values{
		"schema":   {"bill/invoice"},
		"filter":   {`state = "sent" && folder = "sales"`},
		"order_by": {"issue_date desc"},
		"limit":    {"20"},
		"fields":   {"id,code"},
		"count":    {"true"},
	}
	for k, v := range want {
		if q.Get(k) != v[0] {
			t.Errorf("%s = %q, want %q", k, q.Get(k), v[0])
		}
	}
	if len(q) != len(want) {
		t.Errorf("sent %v, want only %v", q, want)
	}

	var col invopop.SiloQueryEntryCollection
	if err := json.Unmarshal([]byte(out), &col); err != nil {
		t.Fatalf("%v in %s", err, out)
	}
	if len(col.List) != 1 || col.NextCursor != "p2" {
		t.Errorf("got %+v", col)
	}
}

func TestSiloEntriesQueryAll(t *testing.T) {
	host, seen := queryServer(t, map[string]string{
		"":   `{"list":[{"id":"e1"},{"id":"e2"}],"schema":"bill/invoice","next_cursor":"p2","count":3}`,
		"p2": `{"list":[{"id":"e3"}],"schema":"bill/invoice","cursor":"p2","watermark":"2026-10-01T00:00:00.000Z"}`,
	})

	out, err := run(t, "--host", host, "silo", "entries", "query", "bill/invoice", "--all", "--count")
	if err != nil {
		t.Fatal(err)
	}

	var col invopop.SiloQueryEntryCollection
	if err := json.Unmarshal([]byte(out), &col); err != nil {
		t.Fatalf("%v in %s", err, out)
	}
	if len(col.List) != 3 || col.List[2].ID != "e3" {
		t.Errorf("list = %+v", col.List)
	}
	if col.Count != 3 || col.Cursor != "" || col.NextCursor != "" {
		t.Errorf("count, cursors = %d, %q, %q", col.Count, col.Cursor, col.NextCursor)
	}
	if col.Watermark != "2026-10-01T00:00:00.000Z" {
		t.Errorf("watermark = %q, want the last page's", col.Watermark)
	}
	if len(*seen) != 2 || (*seen)[1].Get("count") != "" {
		t.Errorf("requests = %v, want the count asked for on the first page only", *seen)
	}
}

func TestSiloEntriesQueryAllRepeatedCursor(t *testing.T) {
	host, _ := queryServer(t, map[string]string{
		"":   `{"list":[],"next_cursor":"p2"}`,
		"p2": `{"list":[],"next_cursor":"p2"}`,
	})
	_, err := run(t, "--host", host, "silo", "entries", "query", "bill/invoice", "--all")
	if err == nil || !strings.Contains(err.Error(), "twice") {
		t.Fatalf("err = %v", err)
	}
}

func TestSiloEntriesQueryAllWithCursor(t *testing.T) {
	host, seen := queryServer(t, nil)
	_, err := run(t, "--host", host, "silo", "entries", "query", "bill/invoice", "--all", "--cursor", "p2")
	if err == nil || !strings.Contains(err.Error(), "cannot be used together") {
		t.Fatalf("err = %v", err)
	}
	if len(*seen) != 0 {
		t.Errorf("made %d requests, want none", len(*seen))
	}
}

func TestSiloEntriesCount(t *testing.T) {
	host, seen := queryServer(t, nil)
	out, err := run(t, "--host", host, "silo", "entries", "count", "bill/invoice", "--filter", `state = "sent"`)
	if err != nil {
		t.Fatal(err)
	}
	if q := (*seen)[0]; q.Get("schema") != "bill/invoice" || q.Get("filter") != `state = "sent"` {
		t.Errorf("sent %v", q)
	}
	var res invopop.SiloEntryCount
	if err := json.Unmarshal([]byte(out), &res); err != nil || res.Count != 7 {
		t.Errorf("got %+v, %v from %s", res, err, out)
	}
}
