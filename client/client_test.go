package client

// Tests against the real portal. They read PORTAL_EMAIL and PORTAL_PASSWORD
// from the environment or from ../.env, and are skipped if those are missing.
//
//	go test ./portal -v

import (
	"bufio"
	"context"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"
)

const portalURL = "https://urja-ops.flockenergy.tech"

// newTestClient returns a client for the real portal, or skips the test.
func newTestClient(t *testing.T) *Client {
	t.Helper()
	loadDotEnv("../.env")
	email, password := os.Getenv("PORTAL_EMAIL"), os.Getenv("PORTAL_PASSWORD")
	if email == "" || password == "" {
		t.Skip("PORTAL_EMAIL / PORTAL_PASSWORD not set (put them in .env)")
	}
	return New(portalURL, email, password)
}

// loadDotEnv sets KEY=VALUE lines from a file, without overriding real env vars.
func loadDotEnv(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		key, value, ok := strings.Cut(line, "=")
		if !ok || strings.HasPrefix(line, "#") {
			continue
		}
		key, value = strings.TrimSpace(key), strings.Trim(strings.TrimSpace(value), `"'`)
		if _, set := os.LookupEnv(key); !set {
			os.Setenv(key, value)
		}
	}
}

func TestSign(t *testing.T) {
	// printf 'GET\n/portal/export\npage=1\n1700000000' | openssl dgst -sha256 -hmac 'test-secret' -hex
	want := "b4b4d3dacaea949dfabeb63650cd8f500ebc3719cf8246de8306a2e31f18d4d6"
	if got := Sign("test-secret", "GET", "/portal/export", "page=1", 1700000000); got != want {
		t.Fatalf("got %s", got)
	}
}

func testCtx(t *testing.T) context.Context {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	t.Cleanup(cancel)
	return ctx
}

func TestLogin(t *testing.T) {
	c := newTestClient(t)
	if err := c.login(testCtx(t)); err != nil {
		t.Fatal(err)
	}
}

func TestMeta(t *testing.T) {
	c := newTestClient(t)
	m, err := c.Meta(testCtx(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("total=%d pageSize=%d", m.Total, m.PageSize)
	if m.Total <= 0 || m.PageSize <= 0 {
		t.Fatalf("unexpected meta: %+v", m)
	}
}

func TestExport(t *testing.T) {
	c := newTestClient(t)
	ctx := testCtx(t)

	meta, err := c.Meta(ctx)
	if err != nil {
		t.Fatal(err)
	}
	meters, err := c.Export(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("export: %d meters, first %+v", len(meters), meters[0])

	if len(meters) != meta.Total {
		t.Errorf("export has %d meters, search total is %d", len(meters), meta.Total)
	}
	seen := map[string]bool{}
	for _, m := range meters {
		if m.MeterID == "" {
			t.Fatal("meter with empty meterId")
		}
		if seen[m.MeterID] {
			t.Fatalf("duplicate meter %s", m.MeterID)
		}
		seen[m.MeterID] = true
	}
}

func TestEnergy(t *testing.T) {
	c := newTestClient(t)
	ctx := testCtx(t)

	readings, size, err := c.Energy(ctx, "J100000", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(readings) == 0 {
		t.Fatal("no readings for J100000")
	}
	t.Logf("J100000: %d readings, %d bytes, %s .. %s",
		len(readings), size, readings[0].Timestamp, readings[len(readings)-1].Timestamp)

	// Same size again means nothing new: readings come back nil.
	again, _, err := c.Energy(ctx, "J100000", size)
	if err != nil {
		t.Fatal(err)
	}
	if again != nil {
		t.Logf("size changed between two fetches (new readings arrived?)")
	}
}

func TestEnergyUnknownMeter(t *testing.T) {
	c := newTestClient(t)
	_, _, err := c.Energy(testCtx(t), "J999999", 0)
	t.Logf("unknown meter: %v", err)
	if err == nil {
		t.Log("portal returned no error for an unknown meter")
	}
}

func TestReloginAfterBadCookie(t *testing.T) {
	c := newTestClient(t)
	ctx := testCtx(t)
	if _, err := c.Meta(ctx); err != nil {
		t.Fatal(err)
	}

	// Replace the session cookie with junk, as if it had expired.
	u, _ := url.Parse(portalURL)
	c.http.Jar.SetCookies(u, []*http.Cookie{{Name: "__Secure-better-auth.session_token", Value: "expired", Path: "/", Secure: true}})

	if _, err := c.Meta(ctx); err != nil {
		t.Fatalf("expected the client to log in again: %v", err)
	}
}
