// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/mitchellh/mapstructure"
)

var pluginName = "infratographer/identity-auth"

// HandlerRegisterer is the symbol the plugin loader will try to load. It must implement the Registerer interface
var HandlerRegisterer = registerer(pluginName)

type AuthCtxKey struct{}

type registerer string

func (r registerer) RegisterHandlers(f func(
	name string,
	handler func(context.Context, map[string]interface{}, http.Handler) (http.Handler, error),
)) {
	f(string(r), r.registerHandlers)
}

type Config struct {
	AuthEndpoint   string `mapstructure:"auth_endpoint"`
	TimeoutSeconds int    `mapstructure:"timeout_in_sec"`
}

func (r registerer) registerHandlers(_ context.Context, extra map[string]interface{}, h http.Handler) (http.Handler, error) {
	var cfg Config
	err := mapstructure.Decode(extra[pluginName], &cfg)
	if err != nil {
		logger.Error("failed decoding: error: %w", err)
		logger.Fatal("failed to decode config for " + pluginName)
	}

	fmt.Printf("\n\n\nDEBUG:\nextra config: %+v\n", extra[pluginName])
	fmt.Printf("parsed config: %+v\n", cfg)

	if cfg.TimeoutSeconds == 0 {
		cfg.TimeoutSeconds = 2
	}

	// return the actual handler wrapping or your custom logic so it can be used as a replacement for the default http handler
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {

		authReq, err := http.NewRequest(http.MethodGet, cfg.AuthEndpoint, nil)
		if err != nil {
			log.Fatal(err)
		}

		ctx, cancel := context.WithTimeout(req.Context(), time.Duration(cfg.TimeoutSeconds)*time.Second)
		defer cancel()
		authReq = authReq.WithContext(ctx)

		// pass the authorization header through to the auth service
		authReq.Header.Add("Authorization", req.Header.Get("Authorization"))

		c := &http.Client{}
		res, err := c.Do(authReq)
		if err != nil {
			log.Fatal(err)
		}
		defer res.Body.Close()

		if res.StatusCode != http.StatusOK {
			w.WriteHeader(http.StatusUnauthorized)
			w.Header().Set("Content-Type", "application/json")
			resp := make(map[string]string)
			resp["error"] = "authorization failed"
			jsonResp, err := json.Marshal(resp)
			if err != nil {
				logger.Error("Error happened in JSON marshal. Err:", err)
			}
			w.Write(jsonResp)
			return
		}

		out, err := io.ReadAll(res.Body)
		if err != nil {
			logger.Error(err)

			fmt.Fprintf(w, `{"error": "%s"}`, err.Error())
			w.WriteHeader(http.StatusInternalServerError)
			w.Header().Set("Content-Type", "application/json")
			resp := make(map[string]string)
			resp["error"] = err.Error()
			jsonResp, err := json.Marshal(resp)
			if err != nil {
				logger.Error("Error happened in JSON marshal. Err:", err)
			}
			w.Write(jsonResp)
			return
		}

		authResp := struct {
			URN string `json:"urn"`
		}{}

		err = json.Unmarshal(out, &authResp)
		if err != nil {
			logger.Error("failed to unmarshall auth response")
		}

		req.Header.Del("Authorization")
		req.Header.Del("Infratographer-Auth")
		req.Header.Add("Infratographer-Auth", authResp.URN)

		// if req.URL.Path != path {

		h.ServeHTTP(w, req)
		// return
		// }

	}), nil
}

func main() {}

// This logger is replaced by the RegisterLogger method to load the one from KrakenD
var logger Logger = noopLogger{}

func (registerer) RegisterLogger(v interface{}) {
	l, ok := v.(Logger)
	if !ok {
		return
	}
	logger = l
	logger.Debug(fmt.Sprintf("[PLUGIN: %s] Logger loaded", HandlerRegisterer))
}

type Logger interface {
	Debug(v ...interface{})
	Info(v ...interface{})
	Warning(v ...interface{})
	Error(v ...interface{})
	Critical(v ...interface{})
	Fatal(v ...interface{})
}

// Empty logger implementation
type noopLogger struct{}

func (n noopLogger) Debug(_ ...interface{})    {}
func (n noopLogger) Info(_ ...interface{})     {}
func (n noopLogger) Warning(_ ...interface{})  {}
func (n noopLogger) Error(_ ...interface{})    {}
func (n noopLogger) Critical(_ ...interface{}) {}
func (n noopLogger) Fatal(_ ...interface{})    {}
