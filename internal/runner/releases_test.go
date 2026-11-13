package runner

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/IPauloGermano/multitool/internal/system"
)

type fakeReleaseExecutor struct {
	*MockExecutor
	calls int
}

func (e *fakeReleaseExecutor) ResolveLatestTag(repo, fallback string) string {
	e.calls++
	return "v0.19.2"
}

func TestCraftLauncherUsesInjectedReleaseResolver(t *testing.T) {
	t.Setenv("LOCALAPPDATA", t.TempDir())
	executor := &fakeReleaseExecutor{MockExecutor: &MockExecutor{}}
	r := securityRunner(t, executor.MockExecutor, system.OSWindows)
	r.sysInfo.Arch = "arm64"
	r.executor = executor
	if err := r.installDirectWindows("craftlauncher"); err != nil {
		t.Fatal(err)
	}
	if executor.calls != 1 {
		t.Fatalf("release resolver called %d times", executor.calls)
	}
	if len(executor.executed) < 2 || !strings.Contains(strings.Join(executor.executed[0], " "), "v0.19.2/artcraft-launcher-0.19.2-windows-arm64.msi") {
		t.Fatalf("wrong release/architecture: %v", executor.executed)
	}
}

type releaseTransport func(*http.Request) (*http.Response, error)

func (f releaseTransport) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func TestReleaseResolutionWithoutNetwork(t *testing.T) {
	for _, tc := range []struct {
		name, location, want string
		status               int
		failure              error
	}{
		{name: "redirect", status: http.StatusFound, location: "https://github.com/storytold/craft-launcher/releases/tag/v0.19.2", want: "v0.19.2"},
		{name: "missing", status: http.StatusNotFound, want: "fallback"},
		{name: "no-location", status: http.StatusFound, want: "fallback"},
		{name: "timeout", failure: errors.New("request timed out"), want: "fallback"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			client := &http.Client{
				CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
				Transport: releaseTransport(func(req *http.Request) (*http.Response, error) {
					calls++
					if req.Method != http.MethodHead || req.URL.String() != "https://github.com/storytold/craft-launcher/releases/latest" {
						t.Fatalf("unexpected request: %s %s", req.Method, req.URL)
					}
					if tc.failure != nil {
						return nil, tc.failure
					}
					return &http.Response{StatusCode: tc.status, Header: http.Header{"Location": []string{tc.location}}, Body: io.NopCloser(strings.NewReader("")), Request: req}, nil
				}),
			}
			if got := resolveGitHubLatestTag(client, "storytold/craft-launcher", "fallback"); got != tc.want {
				t.Fatalf("tag=%q want=%q", got, tc.want)
			}
			if calls != 1 {
				t.Fatalf("unexpected request count: %d", calls)
			}
		})
	}
}
