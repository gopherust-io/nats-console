package testutil

import (
	"bytes"
	"errors"
	"io"
	"net"
	"net/http"

	"github.com/valyala/fasthttp"
	"github.com/valyala/fasthttp/fasthttputil"

	commonstrings "github.com/gopherust-io/nats-consol/pkg/common/strings"
)

var errNilRequest = errors.New("nil request")

// HTTPClient is a fasthttp-backed test client dialing an in-memory listener.
type HTTPClient struct {
	host      *fasthttp.HostClient
	basicAuth string // empty = no auto auth
}

// Request is a test HTTP request (replaces net/http.Request for Do).
type Request struct {
	Method  string
	URL     string
	Body    io.Reader
	Header  http.Header
	Cookies []*Cookie
}

// Response is a detached copy of a fasthttp response for assertions.
type Response struct {
	Body       []byte
	cookies    []*Cookie
	header     fasthttp.ResponseHeader
	StatusCode int
}

// Cookie mirrors the fields tests assert on Set-Cookie.
type Cookie struct {
	Name     string
	Value    string
	SameSite http.SameSite
	HttpOnly bool
	Secure   bool
}

// Header returns the first header value for key (case-insensitive).
func (r *Response) Header(key string) string {
	return commonstrings.BytesToString(r.header.Peek(key))
}

// Cookies returns Set-Cookie values from the response.
func (r *Response) Cookies() []*Cookie {
	return r.cookies
}

func newHTTPClient(ln *fasthttputil.InmemoryListener, basicAuth string) *HTTPClient {
	return &HTTPClient{
		basicAuth: basicAuth,
		host: &fasthttp.HostClient{
			Addr: "nats-consol.local",
			Dial: func(addr string) (net.Conn, error) {
				return ln.Dial()
			},
		},
	}
}

// Get issues a GET request.
func (c *HTTPClient) Get(url string) (*Response, error) {
	return c.Do(&Request{Method: fasthttp.MethodGet, URL: url})
}

// Post issues a POST with the given content type and body.
func (c *HTTPClient) Post(url, contentType string, body io.Reader) (*Response, error) {
	hdr := make(http.Header)
	if !commonstrings.IsEmpty(contentType) {
		hdr.Set("Content-Type", contentType)
	}
	return c.Do(&Request{Method: fasthttp.MethodPost, URL: url, Body: body, Header: hdr})
}

// Do issues an arbitrary request. Injects Basic auth when configured and Authorization is unset.
func (c *HTTPClient) Do(r *Request) (*Response, error) {
	if r == nil {
		return nil, errNilRequest
	}
	method := r.Method
	if commonstrings.IsEmpty(method) {
		method = fasthttp.MethodGet
	}

	req := fasthttp.AcquireRequest()
	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseRequest(req)
	defer fasthttp.ReleaseResponse(resp)

	req.Header.SetMethod(method)
	req.SetRequestURI(r.URL)
	if r.Header != nil {
		for key, values := range r.Header {
			for _, v := range values {
				req.Header.Add(key, v)
			}
		}
	}
	for _, ck := range r.Cookies {
		if ck == nil || commonstrings.IsEmpty(ck.Name) {
			continue
		}
		req.Header.SetCookie(ck.Name, ck.Value)
	}
	if !commonstrings.IsEmpty(c.basicAuth) && len(req.Header.Peek("Authorization")) == 0 {
		req.Header.Set("Authorization", c.basicAuth)
	}
	if r.Body != nil {
		data, err := io.ReadAll(r.Body)
		if err != nil {
			return nil, err
		}
		req.SetBody(data)
	}

	if err := c.host.Do(req, resp); err != nil {
		return nil, err
	}

	out := &Response{
		StatusCode: resp.StatusCode(),
		Body:       append([]byte(nil), resp.Body()...),
	}
	resp.Header.CopyTo(&out.header)
	out.cookies = parseSetCookies(&resp.Header)
	return out, nil
}

func parseSetCookies(h *fasthttp.ResponseHeader) []*Cookie {
	var out []*Cookie
	for key, value := range h.All() {
		if !bytes.EqualFold(key, []byte("Set-Cookie")) {
			continue
		}
		ck := fasthttp.AcquireCookie()
		err := ck.ParseBytes(value)
		if err != nil {
			fasthttp.ReleaseCookie(ck)
			continue
		}
		name := string(ck.Key())
		if commonstrings.IsEmpty(name) {
			fasthttp.ReleaseCookie(ck)
			continue
		}
		out = append(out, &Cookie{
			Name:     name,
			Value:    string(ck.Value()),
			HttpOnly: ck.HTTPOnly(),
			Secure:   ck.Secure(),
			SameSite: mapSameSite(ck.SameSite()),
		})
		fasthttp.ReleaseCookie(ck)
	}
	return out
}

func mapSameSite(ss fasthttp.CookieSameSite) http.SameSite {
	switch ss {
	case fasthttp.CookieSameSiteStrictMode:
		return http.SameSiteStrictMode
	case fasthttp.CookieSameSiteNoneMode:
		return http.SameSiteNoneMode
	case fasthttp.CookieSameSiteLaxMode:
		return http.SameSiteLaxMode
	default:
		return http.SameSiteDefaultMode
	}
}
