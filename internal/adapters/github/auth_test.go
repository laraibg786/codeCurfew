package github

import (
	"crypto/rand"
	"crypto/rsa"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// countingTokenHolder is a minimal TokenHolder whose refreshToken increments an
// atomic counter, letting the test assert exactly how many times the
// "signing" step ran across concurrent GetTokenValue calls without paying for
// real RSA signing on every call.
type countingTokenHolder struct {
	mu            sync.Mutex
	value         string
	expiry        time.Time
	refreshCalls  int32
	refreshDelay  time.Duration
	refreshExpiry time.Duration
}

func (c *countingTokenHolder) lock()   { c.mu.Lock() }
func (c *countingTokenHolder) unlock() { c.mu.Unlock() }

func (c *countingTokenHolder) isExpired(ct time.Time) bool {
	return ct.After(c.expiry)
}

func (c *countingTokenHolder) currentToken() (string, error) {
	return c.value, nil
}

func (c *countingTokenHolder) resetToken() {
	c.value = ""
	c.expiry = time.Time{}
}

func (c *countingTokenHolder) refreshToken() error {
	atomic.AddInt32(&c.refreshCalls, 1)
	if c.refreshDelay > 0 {
		time.Sleep(c.refreshDelay)
	}
	c.value = "signed-token"
	c.expiry = time.Now().Add(c.refreshExpiry)
	return nil
}

// TestGetTokenValue_ConcurrentCalls_CachedTokenNotResigned proves that when the
// cached token is still valid (not within the 2-minute expiry window),
// concurrent callers of GetTokenValue reuse the cached value instead of each
// triggering a fresh sign/refresh. Run with -race to also confirm the shared
// mutex added to the token holders prevents data races on value/expiry.
func TestGetTokenValue_ConcurrentCalls_CachedTokenNotResigned(t *testing.T) {
	holder := &countingTokenHolder{refreshExpiry: time.Hour}
	// Prime the cache once, exactly like the composition root building the
	// holder once at startup and the first request populating it.
	if _, err := GetTokenValue(holder); err != nil {
		t.Fatalf("priming GetTokenValue() returned error: %v", err)
	}
	if got := atomic.LoadInt32(&holder.refreshCalls); got != 1 {
		t.Fatalf("after priming, refreshCalls = %d, want 1", got)
	}

	const n = 50
	var wg sync.WaitGroup
	errs := make(chan error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			v, err := GetTokenValue(holder)
			if err != nil {
				errs <- err
				return
			}
			if v != "signed-token" {
				errs <- fmt.Errorf("unexpected token value %q", v)
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Errorf("concurrent GetTokenValue() error: %v", err)
	}

	if got := atomic.LoadInt32(&holder.refreshCalls); got != 1 {
		t.Fatalf("refreshCalls after %d concurrent GetTokenValue() calls = %d, want 1 (token was still valid, should not be re-signed)", n, got)
	}
}

// TestGetTokenValue_ConcurrentCalls_NearExpiryStillSerializes proves that even
// when many goroutines race against a near-expired shared token, the holder's
// lock serializes the check-then-refresh-then-read sequence: refreshToken is
// still only invoked a small, bounded number of times (not once per
// goroutine), and every caller observes a consistent, valid token.
func TestGetTokenValue_ConcurrentCalls_NearExpiryStillSerializes(t *testing.T) {
	holder := &countingTokenHolder{
		// Already within the 2-minute refresh window, forcing every caller to
		// take the refresh branch unless the lock serializes them.
		expiry:        time.Now(),
		refreshExpiry: time.Hour,
		refreshDelay:  10 * time.Millisecond,
	}

	const n = 50
	var wg sync.WaitGroup
	results := make(chan string, n)
	errs := make(chan error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			v, err := GetTokenValue(holder)
			if err != nil {
				errs <- err
				return
			}
			results <- v
		}()
	}
	wg.Wait()
	close(errs)
	close(results)

	for err := range errs {
		t.Errorf("concurrent GetTokenValue() error: %v", err)
	}
	for v := range results {
		if v != "signed-token" {
			t.Errorf("unexpected token value %q", v)
		}
	}

	// Because the token is refreshed to a 1-hour expiry on the very first
	// refresh, once any goroutine wins the lock and refreshes, every
	// subsequent goroutine should see a valid (non-expired) token and skip
	// refreshing. Serialization means refreshCalls is small, definitely not n.
	if got := atomic.LoadInt32(&holder.refreshCalls); got < 1 || got >= int32(n) {
		t.Fatalf("refreshCalls = %d, want a small bounded number (>=1, < %d) proving the lock serialized refreshes", got, n)
	}
}

// TestAppToken_Concurrent_RaceFree exercises the real *appToken (the GitHub-App
// JWT holder) concurrently through GetTokenValue with real RSA signing, to
// prove -race finds no data race on the shared value/expiry fields now that
// they are guarded by appToken.mu.
func TestAppToken_Concurrent_RaceFree(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate test RSA key: %v", err)
	}
	holder := NewJWTHolder("test-app-id", key)

	const n = 20
	var wg sync.WaitGroup
	errs := make(chan error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := GetTokenValue(holder); err != nil {
				errs <- err
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Errorf("concurrent GetTokenValue() on shared appToken error: %v", err)
	}
}
