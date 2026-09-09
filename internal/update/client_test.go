package update

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestCheckAcceptsNewerSignedStableRelease(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	payload := []byte("new panel")
	sum := sha256.Sum256(payload)
	manifest := &Manifest{
		Release:        "1.10.0",
		Channel:        "stable",
		MinimumRelease: "1.0.0",
		Artifacts: []Artifact{{
			Path:   "panel-api",
			Target: "bin/panel-api",
			Size:   int64(len(payload)),
			SHA256: hex.EncodeToString(sum[:]),
			Mode:   0o755,
		}},
	}
	if err := Sign(manifest, priv); err != nil {
		t.Fatal(err)
	}

	server := newManifestServer(t, manifest)
	defer server.Close()
	withHTTPClient(t, server.Client())

	status, err := Check(context.Background(), Config{
		FeedURL:          server.URL,
		Channel:          "stable",
		InstalledRelease: "1.9.0",
		PublicKey:        pub,
		InstallRoot:      t.TempDir(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if status.State != "available" || status.AvailableRelease != "1.10.0" {
		t.Fatalf("unexpected status: %+v", status)
	}
	if status.InstalledRelease != "1.9.0" || status.Channel != "stable" {
		t.Fatalf("status lost configured values: %+v", status)
	}
}

func TestCheckRejectsHTTPFeed(t *testing.T) {
	_, err := Check(context.Background(), Config{
		FeedURL: "http://updates.example.test", Channel: "stable",
		InstallRoot: t.TempDir(),
	})
	if err == nil || !strings.Contains(err.Error(), "HTTPS") {
		t.Fatalf("expected HTTPS error, got %v", err)
	}
}

func TestCheckRejectsTraversalAndOversizedArtifacts(t *testing.T) {
	tests := []struct {
		name     string
		artifact Artifact
		want     string
	}{
		{
			name: "traversal path",
			artifact: Artifact{
				Path: "../panel-api", Target: "bin/panel-api", Size: 1,
				SHA256: strings.Repeat("0", 64), Mode: 0o755,
			},
			want: "path",
		},
		{
			name: "traversal target",
			artifact: Artifact{
				Path: "panel-api", Target: "../panel-api", Size: 1,
				SHA256: strings.Repeat("0", 64), Mode: 0o755,
			},
			want: "target",
		},
		{
			name: "oversized",
			artifact: Artifact{
				Path: "panel-api", Target: "bin/panel-api", Size: maxArtifactSize + 1,
				SHA256: strings.Repeat("0", 64), Mode: 0o755,
			},
			want: "size",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pub, priv, err := ed25519.GenerateKey(rand.Reader)
			if err != nil {
				t.Fatal(err)
			}
			manifest := &Manifest{
				Release: "2.0.0", Channel: "stable", MinimumRelease: "1.0.0",
				Artifacts: []Artifact{tt.artifact},
			}
			if err := Sign(manifest, priv); err != nil {
				t.Fatal(err)
			}
			server := newManifestServer(t, manifest)
			defer server.Close()
			withHTTPClient(t, server.Client())

			_, err = Check(context.Background(), Config{
				FeedURL: server.URL, Channel: "stable",
				InstalledRelease: "1.0.0", PublicKey: pub,
				InstallRoot: t.TempDir(),
			})
			if err == nil || !strings.Contains(strings.ToLower(err.Error()), tt.want) {
				t.Fatalf("expected %q error, got %v", tt.want, err)
			}
		})
	}
}

func TestCheckRejectsNonIncreasingRelease(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	manifest := validManifest("1.9.0")
	if err := Sign(manifest, priv); err != nil {
		t.Fatal(err)
	}
	server := newManifestServer(t, manifest)
	defer server.Close()
	withHTTPClient(t, server.Client())

	_, err = Check(context.Background(), Config{
		FeedURL: server.URL, Channel: "stable",
		InstalledRelease: "1.10.0", PublicKey: pub,
		InstallRoot: t.TempDir(),
	})
	if err == nil || !strings.Contains(err.Error(), "newer") {
		t.Fatalf("expected non-increasing release error, got %v", err)
	}
}

func TestCheckAllowsSameHostRedirectAndRejectsDifferentHost(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	manifest := validManifest("2.0.0")
	if err := Sign(manifest, priv); err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}

	var sameHost *httptest.Server
	sameHost = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/stable/manifest.json" {
			http.Redirect(w, r, sameHost.URL+"/redirected-manifest.json", http.StatusFound)
			return
		}
		_, _ = w.Write(raw)
	}))
	defer sameHost.Close()
	withHTTPClient(t, sameHost.Client())
	if _, err := Check(context.Background(), Config{
		FeedURL: sameHost.URL, Channel: "stable",
		InstalledRelease: "1.0.0", PublicKey: pub,
		InstallRoot: t.TempDir(),
	}); err != nil {
		t.Fatalf("same-host redirect rejected: %v", err)
	}

	other := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(raw)
	}))
	defer other.Close()
	redirector := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, other.URL+"/stable/manifest.json", http.StatusFound)
	}))
	defer redirector.Close()
	withHTTPClient(t, redirector.Client())
	if _, err := Check(context.Background(), Config{
		FeedURL: redirector.URL, Channel: "stable",
		InstalledRelease: "1.0.0", PublicKey: pub,
		InstallRoot: t.TempDir(),
	}); err == nil || !strings.Contains(err.Error(), "redirect") {
		t.Fatalf("expected cross-host redirect error, got %v", err)
	}
}

func TestCheckBoundsManifestRequest(t *testing.T) {
	deadlineSeen := make(chan time.Time, 1)
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		deadline, ok := request.Context().Deadline()
		if !ok {
			return nil, errors.New("manifest request has no deadline")
		}
		deadlineSeen <- deadline
		return nil, errors.New("stop after deadline inspection")
	})}
	withHTTPClient(t, client)
	started := time.Now()

	_, _ = Check(context.Background(), Config{
		FeedURL: "https://updates.example.test", Channel: "stable",
		InstallRoot: t.TempDir(),
	})

	select {
	case deadline := <-deadlineSeen:
		if deadline.After(started.Add(manifestRequestTimeout + time.Second)) {
			t.Fatalf("manifest deadline %v exceeds bound", deadline)
		}
	default:
		t.Fatal("transport did not observe a bounded request")
	}
}

func TestSemanticVersionValidationAndArbitraryNumericOrdering(t *testing.T) {
	if _, err := parseSemanticVersion("1.0.0-01"); err == nil {
		t.Fatal("numeric prerelease identifier with a leading zero was accepted")
	}
	if _, err := parseSemanticVersion("1.0.0+bad_metadata"); err == nil {
		t.Fatal("invalid build metadata was accepted")
	}
	left, err := parseSemanticVersion("999999999999999999999999999999999999999.0.0")
	if err != nil {
		t.Fatal(err)
	}
	right, err := parseSemanticVersion("1000000000000000000000000000000000000000.0.0")
	if err != nil {
		t.Fatal(err)
	}
	if compareVersions(left, right) >= 0 {
		t.Fatal("arbitrary-size core identifiers were not ordered numerically")
	}
	prereleaseLeft, err := parseSemanticVersion("1.0.0-999999999999999999999999999999999999999")
	if err != nil {
		t.Fatal(err)
	}
	prereleaseRight, err := parseSemanticVersion("1.0.0-1000000000000000000000000000000000000000")
	if err != nil {
		t.Fatal(err)
	}
	if compareVersions(prereleaseLeft, prereleaseRight) >= 0 {
		t.Fatal("arbitrary-size prerelease identifiers were not ordered numerically")
	}
	if _, err := parseSemanticVersion("1.0.0-alpha.01a+build.7"); err != nil {
		t.Fatalf("valid alphanumeric prerelease/build metadata rejected: %v", err)
	}
}

func TestArtifactModesRejectWritableAndUnexpectedPermissions(t *testing.T) {
	for _, mode := range []uint32{0o666, 0o775, 0o600, 0o700} {
		artifact := Artifact{
			Path: "panel-api", Target: "bin/panel-api", Size: 1,
			SHA256: strings.Repeat("0", 64), Mode: mode,
		}
		if err := validateArtifacts([]Artifact{artifact}); err == nil {
			t.Fatalf("unsafe or unexpected mode %o was accepted", mode)
		}
	}
	for _, mode := range []uint32{0o644, 0o755} {
		artifact := Artifact{
			Path: "panel-api", Target: "bin/panel-api", Size: 1,
			SHA256: strings.Repeat("0", 64), Mode: mode,
		}
		if err := validateArtifacts([]Artifact{artifact}); err != nil {
			t.Fatalf("safe mode %o rejected: %v", mode, err)
		}
	}
}

func validManifest(release string) *Manifest {
	return &Manifest{
		Release: release, Channel: "stable", MinimumRelease: "1.0.0",
		Artifacts: []Artifact{{
			Path: "panel-api", Target: "bin/panel-api", Size: 1,
			SHA256: strings.Repeat("0", 64), Mode: 0o755,
		}},
	}
}

func newManifestServer(t *testing.T, manifest *Manifest) *httptest.Server {
	t.Helper()
	raw, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	return httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/stable/manifest.json" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(raw)
	}))
}

func withHTTPClient(t *testing.T, client *http.Client) {
	t.Helper()
	previous := http.DefaultClient
	http.DefaultClient = client
	t.Cleanup(func() {
		http.DefaultClient = previous
	})
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (function roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}
