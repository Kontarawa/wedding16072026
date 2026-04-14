package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

const basePath = "/wedding/invitation"

type answerPayload struct {
	FirstName   string `json:"first_name"`
	LastName    string `json:"last_name"`
	Attendance  string `json:"attendance"`
	Transfer    string `json:"transfer"`
	Alcohol     string `json:"alcohol"`
	SubmittedAt string `json:"submitted_at"`
}

type storedAnswer struct {
	ID          string `json:"id"`
	FirstName   string `json:"first_name"`
	LastName    string `json:"last_name"`
	Attendance  string `json:"attendance"`
	Transfer    string `json:"transfer"`
	Alcohol     string `json:"alcohol"`
	Hash        string `json:"hash"`
	SubmittedAt string `json:"submitted_at"`
}

var rsvpFileMu sync.Mutex

func main() {
	addr := getenv("LISTEN_ADDR", ":8080")
	tlsCert := os.Getenv("TLS_CERT_FILE")
	tlsKey := os.Getenv("TLS_KEY_FILE")
	sheetsWebhookURL := strings.TrimSpace(os.Getenv("GOOGLE_SHEETS_WEBHOOK_URL"))
	sheetsWebhookToken := strings.TrimSpace(os.Getenv("GOOGLE_SHEETS_WEBHOOK_TOKEN"))
	if sheetsWebhookURL == "" {
		sheetsWebhookURL = "https://script.google.com/macros/s/AKfycbw79jR09DpJsQcwUnNf5BoLfri7Mj2tWMLB_RNSmSDrHGxl7mpFdD1pyXN7LNDgRc47/exec"
	}
	if sheetsWebhookToken == "" {
		sheetsWebhookToken = "wjdhqjlkQWD-PyBGnI-85EpEcmrZYDun18"
	}

	log.Printf("RSVP answers go to Google Sheets webhook (configured: %v)", sheetsWebhookURL != "" && sheetsWebhookToken != "")

	mux := http.NewServeMux()

	answerHandler := func(w http.ResponseWriter, r *http.Request) {
		var body answerPayload
		if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&body); err != nil {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}
		if ts := strings.TrimSpace(body.SubmittedAt); ts == "" {
			body.SubmittedAt = time.Now().UTC().Format(time.RFC3339)
		}
		if strings.TrimSpace(body.FirstName) == "" || strings.TrimSpace(body.LastName) == "" {
			http.Error(w, "first_name and last_name required", http.StatusBadRequest)
			return
		}
		if sheetsWebhookURL == "" || sheetsWebhookToken == "" {
			http.Error(w, "google sheets not configured", http.StatusInternalServerError)
			return
		}
		if err := postToGoogleSheets(sheetsWebhookURL, sheetsWebhookToken, body); err != nil {
			log.Printf("google sheets webhook: %v", err)
			http.Error(w, "could not save response", http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	}

	mux.HandleFunc("POST "+basePath+"/answer", answerHandler)

	fs := http.FileServer(http.Dir("./static"))
	mux.Handle("GET /static/", http.StripPrefix("/static/", fs))

	mux.HandleFunc("GET "+basePath+"/", servePage)

	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		http.Redirect(w, r, basePath+"/", http.StatusFound)
	})

	log.Printf("listening on %s (TLS: %v)", addr, tlsCert != "" && tlsKey != "")
	var listenErr error
	if tlsCert != "" && tlsKey != "" {
		listenErr = http.ListenAndServeTLS(addr, tlsCert, tlsKey, logReq(mux))
	} else {
		listenErr = http.ListenAndServe(addr, logReq(mux))
	}
	if listenErr != nil {
		log.Fatal(listenErr)
	}
}

func servePage(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "./static/index.html")
}

func clientIP(r *http.Request) string {
	if xff := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); xff != "" {
		if i := strings.IndexByte(xff, ','); i >= 0 {
			xff = strings.TrimSpace(xff[:i])
		}
		if xff != "" {
			return xff
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func logReq(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s", r.Method, r.URL.Path)
		next.ServeHTTP(w, r)
	})
}

func postToGoogleSheets(webhookURL, token string, p answerPayload) error {
	payload := map[string]any{
		"token":        token,
		"submitted_at": strings.TrimSpace(p.SubmittedAt),
		"first_name":   strings.TrimSpace(p.FirstName),
		"last_name":    strings.TrimSpace(p.LastName),
		"attendance":   strings.TrimSpace(p.Attendance),
		"transfer":     strings.TrimSpace(p.Transfer),
		"alcohol":      strings.TrimSpace(p.Alcohol),
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", webhookURL, strings.NewReader(string(b)))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 8 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(res.Body, 1<<16))
		return fmt.Errorf("webhook status %d: %s", res.StatusCode, strings.TrimSpace(string(body)))
	}
	return nil
}
