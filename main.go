package main

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

//go:embed models/event.json
var embeddedFiles embed.FS

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	certFile := strings.TrimSpace(os.Getenv("TLS_CERT_FILE"))
	keyFile := strings.TrimSpace(os.Getenv("TLS_KEY_FILE"))
	useTLS := certFile != "" && keyFile != ""

	mux := http.NewServeMux()
	mux.HandleFunc("/health", healthHandler)
	mux.HandleFunc("/api/event", eventAPIHandler)
	mux.HandleFunc("/api/rsvp", rsvpAPIHandler)
	mux.Handle("/", securityHeaders(http.FileServer(http.Dir("."))))

	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           loggingMiddleware(mux),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    1 << 18,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		if useTLS {
			log.Printf("server: https://localhost:%s/", port)
			if err := srv.ListenAndServeTLS(certFile, keyFile); err != nil && !errors.Is(err, http.ErrServerClosed) {
				log.Fatal(err)
			}
		} else {
			log.Printf("server: http://localhost:%s/ (set TLS_CERT_FILE and TLS_KEY_FILE for HTTPS)", port)
			if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				log.Fatal(err)
			}
		}
	}()

	<-ctx.Done()
	log.Println("Stopping server...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown: %v", err)
	}
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		next.ServeHTTP(w, r)
	})
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok\n"))
}

func eventAPIHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	path := filepath.Clean(os.Getenv("EVENT_JSON_FILE"))
	if path != "" && path != "." {
		b, err := os.ReadFile(path)
		if err == nil {
			if !json.Valid(b) {
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "invalid event json file"})
				return
			}
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.Header().Set("Cache-Control", "public, max-age=60")
			_, _ = w.Write(b)
			return
		}
		if !errors.Is(err, os.ErrNotExist) {
			log.Printf("event file: %v", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not read event data"})
			return
		}
	}
	b, err := embeddedFiles.ReadFile("models/event.json")
	if err != nil {
		log.Printf("embedded event: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "event data unavailable"})
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=60")
	_, _ = w.Write(b)
}

type rsvpPayload struct {
	Name               string   `json:"name"`
	Attending          *bool    `json:"attending"`
	Transfer           string   `json:"transfer"`
	AlcoholPreferences []string `json:"alcohol_preferences"`
}

type rsvpRow struct {
	ReceivedAt         time.Time `json:"received_at"`
	Name               string    `json:"name"`
	Attending          *bool     `json:"attending"`
	Transfer           string    `json:"transfer"`
	AlcoholPreferences []string  `json:"alcohol_preferences"`
}

func rsvpAPIHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	const maxBody = 1 << 16
	r.Body = http.MaxBytesReader(w, r.Body, maxBody)
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid body"})
		return
	}
	var p rsvpPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "expected json"})
		return
	}
	p.Name = strings.TrimSpace(p.Name)
	if p.Name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name is required"})
		return
	}

	row := rsvpRow{
		ReceivedAt:         time.Now().UTC(),
		Name:               p.Name,
		Attending:          p.Attending,
		Transfer:           strings.TrimSpace(p.Transfer),
		AlcoholPreferences: p.AlcoholPreferences,
	}
	line, err := json.Marshal(row)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "encode failed"})
		return
	}

	logPath := strings.TrimSpace(os.Getenv("RSVP_LOG_FILE"))
	if logPath == "" {
		logPath = "rsvp-submissions.jsonl"
	}
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		log.Printf("rsvp log: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not save response"})
		return
	}
	_, err = f.Write(append(line, '\n'))
	_ = f.Close()
	if err != nil {
		log.Printf("rsvp write: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not save response"})
		return
	}

	log.Printf("rsvp: %s", row.Name)
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":      true,
		"message": "Спасибо! Ответ сохранён.",
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(true)
	if err := enc.Encode(v); err != nil {
		log.Printf("json encode: %v", err)
	}
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			next.ServeHTTP(w, r)
			return
		}
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start).Truncate(time.Millisecond))
	})
}
