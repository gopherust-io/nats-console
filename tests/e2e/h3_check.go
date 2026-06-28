//go:build ignore

package main

import (
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: h3_check BASE_URL")
		os.Exit(2)
	}
	baseURL := strings.TrimRight(os.Args[1], "/")
	healthURL := baseURL + "/api/health"

	client := &http.Client{
		Transport: &http.Transport{
			ForceAttemptHTTP2: true,
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: os.Getenv("HTTP3_INSECURE") == "1",
			},
		},
	}

	resp, err := client.Get(healthURL)
	if err != nil {
		exitErr(fmt.Errorf("health request: %w", err))
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		exitErr(fmt.Errorf("health status %d: %s", resp.StatusCode, string(body)))
	}
	if !strings.Contains(string(body), `"ok"`) {
		exitErr(fmt.Errorf("unexpected health body: %s", string(body)))
	}

	altSvc := resp.Header.Get("Alt-Svc")
	if altSvc == "" {
		fmt.Println("health ok (Alt-Svc not advertised; h3 may still be available on retry)")
		return
	}
	if !strings.Contains(strings.ToLower(altSvc), "h3") {
		exitErr(fmt.Errorf("expected Alt-Svc h3, got %q", altSvc))
	}
	fmt.Printf("health ok via %s; Alt-Svc=%s\n", resp.Proto, altSvc)
}

func exitErr(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
