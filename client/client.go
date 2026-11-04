// http client responssible to communicate with Urja Ops portal
package client

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

type Client struct {
	baseURL  string
	email    string
	password string
	http     *http.Client
	loginMu  sync.Mutex
	limiter  *rate.Limiter // shared by every goroutine using this client
}

// new client, allowed to send at most `perSecond` requests per second
// the portal allows about 120 requests a minute (2/s), so stay below that
func New(baseURL, email, password string, perSecond float64) *Client {
	jar, _ := cookiejar.New(nil) // for storing the cookie in the http client
	return &Client{
		baseURL:  strings.TrimRight(baseURL, "/"),
		email:    email,
		password: password,
		http:     &http.Client{Jar: jar, Timeout: 30 * time.Second},
		limiter:  rate.NewLimiter(rate.Limit(perSecond), 1),
	}
}

// login posts the login form like the portal's page does.
// Success is HTTP 200 with {"type":"redirect"} and a session cookie.
// session cookie is automatically stored in cookie jar attached to http client
// relogin updates the already stored cookie
func (c *Client) login(ctx context.Context) error {
	c.loginMu.Lock()
	defer c.loginMu.Unlock()

	// blocks until the rate limit allows the next request
	if err := c.limiter.Wait(ctx); err != nil {
		return err
	}

	form := url.Values{"email": {c.email}, "password": {c.password}}
	req, _ := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Origin", c.baseURL)
	req.Header.Set("x-sveltekit-action", "true")

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 || !strings.Contains(string(body), `"redirect"`) {
		return fmt.Errorf("login failed: HTTP %d %.200s", resp.StatusCode, body)
	}
	return nil
}

// get sends a GET and returns the body.
// It waits for the rate limit before every request (blocking the calling worker),
// logs in again once on unauthorized(401),
// and retries too many requests(429) / 5xx / network errors a few times.
func (c *Client) get(ctx context.Context, path, query string, headers func() map[string]string) ([]byte, error) {
	loggedIn := false
	var lastErr error

	for attempt := 1; attempt <= 3; attempt++ {
		// blocks until the rate limit allows the next request (or ctx is cancelled)
		if err := c.limiter.Wait(ctx); err != nil {
			return nil, err
		}

		req, _ := http.NewRequestWithContext(ctx, "GET", c.baseURL+path, nil)
		req.URL.RawQuery = query
		if headers != nil {
			for k, v := range headers() {
				req.Header.Set(k, v)
			}
		}

		resp, err := c.http.Do(req)
		if err != nil {
			lastErr = err
			time.Sleep(time.Duration(attempt) * time.Second)
			continue
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		switch {
		case resp.StatusCode == 200:
			return body, nil
		case resp.StatusCode == 401 && !loggedIn:
			if err := c.login(ctx); err != nil {
				return nil, err
			}
			loggedIn = true
			attempt-- // the login doesn't count as a retry
		case resp.StatusCode == 429:
			// rate limited: wait as long as the portal asks, or long enough for its window to reset
			lastErr = fmt.Errorf("HTTP 429")
			wait := 30 * time.Second
			if secs, err := strconv.Atoi(resp.Header.Get("Retry-After")); err == nil {
				wait = time.Duration(secs) * time.Second
			}
			time.Sleep(wait)
		case resp.StatusCode >= 500:
			lastErr = fmt.Errorf("HTTP %d", resp.StatusCode)
			time.Sleep(time.Duration(attempt) * 2 * time.Second)
		default:
			return nil, fmt.Errorf("GET %s: HTTP %d %.200s", path, resp.StatusCode, body)
		}
	}
	return nil, fmt.Errorf("GET %s: giving up: %w", path, lastErr)
}

func Sign(secret, method, path, query string, ts int64) string {
	msg := method + "\n" + path + "\n" + query + "\n" + strconv.FormatInt(ts, 10)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(msg))
	return hex.EncodeToString(mac.Sum(nil))
}
