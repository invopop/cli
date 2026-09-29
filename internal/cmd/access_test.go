package cmd

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/invopop/cli/internal/auth"
)

// signedIn points the CLI at a fake API listing two orgs, with a stored
// sign-in whose default is w1.
func signedIn(t *testing.T) string {
	t.Helper()
	mux := http.NewServeMux()
	// Workspace tokens are minted from the user token, and name their
	// workspace so the fake can tell which one a request is for.
	mux.HandleFunc("POST /access/v1/workspace/{id}/token", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer user" {
			http.Error(w, "bad user token", http.StatusUnauthorized)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": "ws-" + r.PathValue("id"),
			"expires_at":   time.Now().Add(30 * time.Minute).Unix(),
		})
	})
	mux.HandleFunc("GET /access/v1/workspace", func(w http.ResponseWriter, r *http.Request) {
		id, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ws-")
		if !ok {
			http.Error(w, "not a workspace token", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json") // resty decodes only JSON
		_ = json.NewEncoder(w).Encode(map[string]string{"id": id})
	})
	mux.HandleFunc("GET /access/v1/orgs", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer user" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode([]*auth.Org{
			{ID: "o1", Name: "Acme", Workspaces: []*auth.Workspace{
				{ID: "w1", Name: "Acme", Slug: "acme"},
				{ID: "w2", Name: "Acme test", Slug: "acme", Sandbox: true},
			}},
			{ID: "o2", Name: "Other", Workspaces: []*auth.Workspace{
				{ID: "w3", Name: "Other", Slug: "other"},
			}},
		})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir) // os.UserConfigDir on Linux
	t.Setenv("HOME", dir)            // and on macOS
	t.Setenv("INVOPOP_TOKEN", "")
	t.Setenv(workspaceEnv, "")
	store, err := auth.DefaultStore()
	if err != nil {
		t.Fatal(err)
	}
	err = store.Save(srv.URL, &auth.Credentials{
		ClientID:    "cli",
		AccessHost:  srv.URL,
		AccessToken: "user",
		ExpiresAt:   time.Now().Add(time.Hour),
		WorkspaceID: "w1",
	})
	if err != nil {
		t.Fatal(err)
	}
	return srv.URL
}

// run executes the CLI and returns what it printed to stdout.
func run(t *testing.T, args ...string) (string, error) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout := os.Stdout
	os.Stdout = w
	root := Root()
	root.SetArgs(args)
	root.SetErr(io.Discard)
	runErr := root.Execute()
	os.Stdout = stdout
	w.Close() //nolint:errcheck,gosec
	out, _ := io.ReadAll(r)
	return string(out), runErr
}

func TestAccessWorkspaces(t *testing.T) {
	host := signedIn(t)

	list := func(t *testing.T, args ...string) map[string]*auth.WorkspaceListing {
		t.Helper()
		out, err := run(t, append([]string{"--host", host, "access", "workspaces"}, args...)...)
		if err != nil {
			t.Fatal(err)
		}
		var got []*auth.WorkspaceListing
		if err := json.Unmarshal([]byte(out), &got); err != nil {
			t.Fatalf("%v in %s", err, out)
		}
		byID := make(map[string]*auth.WorkspaceListing)
		for _, w := range got {
			byID[w.ID] = w
		}
		return byID
	}

	got := list(t)
	if len(got) != 3 {
		t.Fatalf("got %d workspaces, want 3", len(got))
	}
	if w := got["w2"]; w.Ref != "sandbox/acme" || !w.Sandbox || w.OrgName != "Acme" {
		t.Errorf("w2 = %+v", w)
	}
	if !got["w1"].Default || !got["w1"].InUse || got["w3"].Default || got["w3"].InUse {
		t.Errorf("default in use: w1 = %+v, w3 = %+v", got["w1"], got["w3"])
	}

	got = list(t, "-w", "sandbox/acme")
	if !got["w1"].Default || got["w1"].InUse || !got["w2"].InUse {
		t.Errorf("with -w: w1 = %+v, w2 = %+v", got["w1"], got["w2"])
	}

	t.Setenv(workspaceEnv, "other")
	if got = list(t); !got["w3"].InUse || got["w1"].InUse {
		t.Errorf("with %s: w1 = %+v, w3 = %+v", workspaceEnv, got["w1"], got["w3"])
	}
}

func TestAccessWorkspacesNeedsSignIn(t *testing.T) {
	host := signedIn(t)
	t.Setenv("INVOPOP_TOKEN", "key")
	_, err := run(t, "--host", host, "access", "workspaces")
	if err == nil || !strings.Contains(err.Error(), "needs a sign-in") {
		t.Fatalf("err = %v", err)
	}
}

func TestAccessWorkspacesNotSignedIn(t *testing.T) {
	host := signedIn(t)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(t.TempDir(), "empty"))
	t.Setenv("HOME", filepath.Join(t.TempDir(), "empty"))
	_, err := run(t, "--host", host, "access", "workspaces")
	if err == nil || !strings.Contains(err.Error(), "not logged in") {
		t.Fatalf("err = %v", err)
	}
}

func TestSignedInClientUsesWorkspaceToken(t *testing.T) {
	host := signedIn(t)
	for args, want := range map[string]string{
		"":                  "w1",
		"-w sandbox/acme":   "w2",
		"--workspace other": "w3",
	} {
		cmd := []string{"--host", host, "access", "workspace"}
		if args != "" {
			cmd = append(cmd, strings.Fields(args)...)
		}
		out, err := run(t, cmd...)
		if err != nil {
			t.Fatalf("%s: %v", args, err)
		}
		var ws struct{ ID string }
		if err := json.Unmarshal([]byte(out), &ws); err != nil || ws.ID != want {
			t.Errorf("%q: workspace = %q (%v), want %s", args, ws.ID, err, want)
		}
	}
}
