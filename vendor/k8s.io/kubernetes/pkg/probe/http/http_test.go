/*
Copyright 2015 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package http

import (
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/kubernetes/pkg/probe"
)

const FailureCode int = -1

func setEnv(key, value string) func() {
	originalValue := os.Getenv(key)
	os.Setenv(key, value)
	if len(originalValue) > 0 {
		return func() {
			os.Setenv(key, originalValue)
		}
	}
	return func() {}
}

func unsetEnv(key string) func() {
	originalValue := os.Getenv(key)
	os.Unsetenv(key)
	if len(originalValue) > 0 {
		return func() {
			os.Setenv(key, originalValue)
		}
	}
	return func() {}
}

func TestHTTPProbeProxy(t *testing.T) {
	res := "welcome to http probe proxy"
	localProxy := "http://127.0.0.1:9098/"

	defer setEnv("http_proxy", localProxy)()
	defer setEnv("HTTP_PROXY", localProxy)()
	defer unsetEnv("no_proxy")()
	defer unsetEnv("NO_PROXY")()

	prober := New(true)

	go func() {
		http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprintf(w, res)
		})
		err := http.ListenAndServe(":9098", nil)
		if err != nil {
			t.Errorf("Failed to start foo server: localhost:9098")
		}
	}()

	// take some time to wait server boot
	time.Sleep(2 * time.Second)
	url, err := url.Parse("http://example.com")
	if err != nil {
		t.Errorf("proxy test unexpected error: %v", err)
	}
	_, response, _ := prober.Probe(url, http.Header{}, time.Second*3)

	if response == res {
		t.Errorf("proxy test unexpected error: the probe is using proxy")
	}
}

func TestHTTPProbeChecker(t *testing.T) {
	handleReq := func(s int, body string) func(w http.ResponseWriter, r *http.Request) {
		return func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(s)
			w.Write([]byte(body))
		}
	}

	// Echo handler that returns the contents of request headers in the body
	headerEchoHandler := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		output := ""
		for k, arr := range r.Header {
			for _, v := range arr {
				output += fmt.Sprintf("%s: %s\n", k, v)
			}
		}
		w.Write([]byte(output))
	}

	redirectHandler := func(s int, bad bool) func(w http.ResponseWriter, r *http.Request) {
		return func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/" {
				http.Redirect(w, r, "/new", s)
			} else if bad && r.URL.Path == "/new" {
				http.Error(w, "", http.StatusInternalServerError)
			}
		}
	}

	prober := New(true)
	testCases := []struct {
		handler    func(w http.ResponseWriter, r *http.Request)
		reqHeaders http.Header
		health     probe.Result
		accBody    string
		notBody    string
	}{
		// The probe will be filled in below.  This is primarily testing that an HTTP GET happens.
		{
			handler: handleReq(http.StatusOK, "ok body"),
			health:  probe.Success,
			accBody: "ok body",
		},
		{
			handler: headerEchoHandler,
			reqHeaders: http.Header{
				"X-Muffins-Or-Cupcakes": {"muffins"},
			},
			health:  probe.Success,
			accBody: "X-Muffins-Or-Cupcakes: muffins",
		},
		{
			handler: headerEchoHandler,
			reqHeaders: http.Header{
				"User-Agent": {"foo/1.0"},
			},
			health:  probe.Success,
			accBody: "User-Agent: foo/1.0",
		},
		{
			handler: headerEchoHandler,
			reqHeaders: http.Header{
				"User-Agent": {""},
			},
			health:  probe.Success,
			notBody: "User-Agent",
		},
		{
			handler:    headerEchoHandler,
			reqHeaders: http.Header{},
			health:     probe.Success,
			accBody:    "User-Agent: kube-probe/",
		},
		{
			// Echo handler that returns the contents of Host in the body
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(200)
				w.Write([]byte(r.Host))
			},
			reqHeaders: http.Header{
				"Host": {"muffins.cupcakes.org"},
			},
			health:  probe.Success,
			accBody: "muffins.cupcakes.org",
		},
		{
			handler: handleReq(FailureCode, "fail body"),
			health:  probe.Failure,
		},
		{
			handler: handleReq(http.StatusInternalServerError, "fail body"),
			health:  probe.Failure,
		},
		{
			handler: func(w http.ResponseWriter, r *http.Request) {
				time.Sleep(3 * time.Second)
			},
			health: probe.Failure,
		},
		{
			handler: redirectHandler(http.StatusMovedPermanently, false), // 301
			health:  probe.Success,
		},
		{
			handler: redirectHandler(http.StatusMovedPermanently, true), // 301
			health:  probe.Failure,
		},
		{
			handler: redirectHandler(http.StatusFound, false), // 302
			health:  probe.Success,
		},
		{
			handler: redirectHandler(http.StatusFound, true), // 302
			health:  probe.Failure,
		},
		{
			handler: redirectHandler(http.StatusTemporaryRedirect, false), // 307
			health:  probe.Success,
		},
		{
			handler: redirectHandler(http.StatusTemporaryRedirect, true), // 307
			health:  probe.Failure,
		},
		{
			handler: redirectHandler(http.StatusPermanentRedirect, false), // 308
			health:  probe.Success,
		},
		{
			handler: redirectHandler(http.StatusPermanentRedirect, true), // 308
			health:  probe.Failure,
		},
	}
	for i, test := range testCases {
		t.Run(fmt.Sprintf("case-%2d", i), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				test.handler(w, r)
			}))
			defer server.Close()
			u, err := url.Parse(server.URL)
			if err != nil {
				t.Errorf("case %d: unexpected error: %v", i, err)
			}
			_, port, err := net.SplitHostPort(u.Host)
			if err != nil {
				t.Errorf("case %d: unexpected error: %v", i, err)
			}
			_, err = strconv.Atoi(port)
			if err != nil {
				t.Errorf("case %d: unexpected error: %v", i, err)
			}
			health, output, err := prober.Probe(u, test.reqHeaders, 1*time.Second)
			if test.health == probe.Unknown && err == nil {
				t.Errorf("case %d: expected error", i)
			}
			if test.health != probe.Unknown && err != nil {
				t.Errorf("case %d: unexpected error: %v", i, err)
			}
			if health != test.health {
				t.Errorf("case %d: expected %v, got %v", i, test.health, health)
			}
			if health != probe.Failure && test.health != probe.Failure {
				if !strings.Contains(output, test.accBody) {
					t.Errorf("Expected response body to contain %v, got %v", test.accBody, output)
				}
				if test.notBody != "" && strings.Contains(output, test.notBody) {
					t.Errorf("Expected response not to contain %v, got %v", test.notBody, output)
				}
			}
		})
	}
}

func TestHTTPProbeChecker_NonLocalRedirects(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/redirect":
			loc, _ := url.QueryUnescape(r.URL.Query().Get("loc"))
			http.Redirect(w, r, loc, http.StatusFound)
		case "/loop":
			http.Redirect(w, r, "/loop", http.StatusFound)
		case "/success":
			w.WriteHeader(http.StatusOK)
		default:
			http.Error(w, "", http.StatusInternalServerError)
		}
	})
	server := httptest.NewServer(handler)
	defer server.Close()

	newportServer := httptest.NewServer(handler)
	defer newportServer.Close()

	testCases := map[string]struct {
		redirect             string
		expectLocalResult    probe.Result
		expectNonLocalResult probe.Result
	}{
		"local success":   {"/success", probe.Success, probe.Success},
		"local fail":      {"/fail", probe.Failure, probe.Failure},
		"newport success": {newportServer.URL + "/success", probe.Success, probe.Success},
		"newport fail":    {newportServer.URL + "/fail", probe.Failure, probe.Failure},
		"bogus nonlocal":  {"http://0.0.0.0/fail", probe.Warning, probe.Failure},
		"redirect loop":   {"/loop", probe.Failure, probe.Failure},
	}
	for desc, test := range testCases {
		t.Run(desc+"-local", func(t *testing.T) {
			prober := New(false)
			target, err := url.Parse(server.URL + "/redirect?loc=" + url.QueryEscape(test.redirect))
			require.NoError(t, err)
			result, _, _ := prober.Probe(target, nil, wait.ForeverTestTimeout)
			assert.Equal(t, test.expectLocalResult, result)
		})
		t.Run(desc+"-nonlocal", func(t *testing.T) {
			prober := New(true)
			target, err := url.Parse(server.URL + "/redirect?loc=" + url.QueryEscape(test.redirect))
			require.NoError(t, err)
			result, _, _ := prober.Probe(target, nil, wait.ForeverTestTimeout)
			assert.Equal(t, test.expectNonLocalResult, result)
		})
	}
}
