package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"wedding-invitation/internal/guests"
)

const basePath = "/wedding/invitation"

type makeReq struct {
	First string `json:"f"`
	Last  string `json:"s"`
}

type answerPayload struct {
	FirstName   string `json:"first_name"`
	LastName    string `json:"last_name"`
	Attendance  string `json:"attendance"`
	Transfer    string `json:"transfer"`
	Alcohol     string `json:"alcohol"`
	Hash        string `json:"hash"`
	SubmittedAt string `json:"submitted_at"`
}

// Таймаут для вызова Google Apps Script (медленный cold start).
var sheetHTTPClient = &http.Client{Timeout: 45 * time.Second}

func main() {
	addr := getenv("LISTEN_ADDR", ":8080")
	dataDir := getenv("DATA_DIR", "./data")
	guestPath := getenv("GUEST_DB", dataDir+"/guests.json")
	adminToken := os.Getenv("ADMIN_TOKEN")
	sheetsURL := os.Getenv("GOOGLE_SHEETS_WEBAPP_URL")
	tlsCert := os.Getenv("TLS_CERT_FILE")
	tlsKey := os.Getenv("TLS_KEY_FILE")

	if strings.TrimSpace(sheetsURL) == "" {
		log.Print("WARNING: GOOGLE_SHEETS_WEBAPP_URL is not set — answers are only appended to data/rsvp-submissions.jsonl. Set the variable to the Apps Script web app URL (…/exec), or add the same URL to <meta name=\"google-sheets-webapp\" content=\"…\"> in static/index.html for browser-side delivery.")
	} else {
		log.Print("Google Sheets: GOOGLE_SHEETS_WEBAPP_URL is set (server will POST each RSVP to the web app)")
	}

	st, err := guests.New(guestPath)
	if err != nil {
		log.Fatalf("guest store: %v", err)
	}

	mux := http.NewServeMux()

	mux.HandleFunc("POST "+basePath+"/make", func(w http.ResponseWriter, r *http.Request) {
		if adminToken != "" && r.Header.Get("X-Admin-Token") != adminToken {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		first, last, ok := parseMakeBody(r)
		if !ok || strings.TrimSpace(first) == "" || strings.TrimSpace(last) == "" {
			http.Error(w, `need f and s (first and last name)`, http.StatusBadRequest)
			return
		}
		h, err := st.Create(guests.Guest{FirstName: strings.TrimSpace(first), LastName: strings.TrimSpace(last)})
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		rel := fmt.Sprintf("%s/%s", basePath, h)
		writeJSON(w, http.StatusOK, map[string]any{
			"hash": h,
			"path": rel,
			"url":  rel,
		})
	})

	mux.HandleFunc("GET "+basePath+"/api/sheets-config", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"google_sheets_configured": strings.TrimSpace(sheetsURL) != "",
		})
	})

	mux.HandleFunc("GET "+basePath+"/api/guest/{hash}", func(w http.ResponseWriter, r *http.Request) {
		h := r.PathValue("hash")
		if h == "" {
			writeJSON(w, http.StatusOK, map[string]any{"guest": nil})
			return
		}
		g, err := st.Get(h)
		if errors.Is(err, guests.ErrNotFound) {
			writeJSON(w, http.StatusOK, map[string]any{"guest": nil})
			return
		}
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"guest": map[string]string{
				"first_name": g.FirstName,
				"last_name":  g.LastName,
			},
		})
	})

	answerHandler := func(w http.ResponseWriter, r *http.Request) {
		h := ""
		if r.PathValue("hash") != "" {
			h = r.PathValue("hash")
		}
		var body answerPayload
		if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&body); err != nil {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}
		if h == "" {
			h = strings.TrimSpace(body.Hash)
		}
		body.Hash = h
		if ts := strings.TrimSpace(body.SubmittedAt); ts == "" {
			body.SubmittedAt = time.Now().UTC().Format(time.RFC3339)
		}
		g, err := st.Get(h)
		if err == nil {
			if strings.TrimSpace(body.FirstName) == "" {
				body.FirstName = g.FirstName
			}
			if strings.TrimSpace(body.LastName) == "" {
				body.LastName = g.LastName
			}
		}
		if strings.TrimSpace(body.FirstName) == "" || strings.TrimSpace(body.LastName) == "" {
			http.Error(w, "first_name and last_name required", http.StatusBadRequest)
			return
		}
		if err := appendRSVPLog(dataDir, body); err != nil {
			log.Printf("rsvp log: %v", err)
			http.Error(w, "could not save response", http.StatusInternalServerError)
			return
		}
		if err := postToSheets(r.Context(), sheetsURL, body); err != nil {
			log.Printf("sheets: %v", err)
			http.Error(w, "could not sync to Google Sheets (response saved on server)", http.StatusBadGateway)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	}

	mux.HandleFunc("POST "+basePath+"/answer/{hash}", answerHandler)
	mux.HandleFunc("POST "+basePath+"/answer", answerHandler)

	fs := http.FileServer(http.Dir("./static"))
	mux.Handle("GET /static/", http.StripPrefix("/static/", fs))

	mux.HandleFunc("GET "+basePath+"/", servePage)
	mux.HandleFunc("GET "+basePath+"/{hash}", servePage)

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

func parseMakeBody(r *http.Request) (first, last string, ok bool) {
	if err := r.ParseForm(); err != nil {
		return "", "", false
	}
	if f, s := strings.TrimSpace(r.FormValue("f")), strings.TrimSpace(r.FormValue("s")); f != "" && s != "" {
		return f, s, true
	}
	q := r.URL.Query()
	if f, s := strings.TrimSpace(q.Get("f")), strings.TrimSpace(q.Get("s")); f != "" && s != "" {
		return f, s, true
	}
	ct := r.Header.Get("Content-Type")
	if strings.Contains(ct, "application/json") {
		var m makeReq
		if err := json.NewDecoder(io.LimitReader(r.Body, 1<<16)).Decode(&m); err != nil {
			return "", "", false
		}
		return strings.TrimSpace(m.First), strings.TrimSpace(m.Last), true
	}
	return "", "", false
}

func appendRSVPLog(dataDir string, p answerPayload) error {
	dir := strings.TrimSpace(dataDir)
	if dir == "" {
		dir = "."
	}
	path := filepath.Join(dir, "rsvp-submissions.jsonl")
	b, err := json.Marshal(p)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(append(b, '\n'))
	return err
}

func postToSheets(ctx context.Context, url string, p answerPayload) error {
	if strings.TrimSpace(url) == "" {
		log.Printf("GOOGLE_SHEETS_WEBAPP_URL is not set: RSVP stored only in data/rsvp-submissions.jsonl — add the Apps Script web app URL to sync Google Sheets")
		return nil
	}
	payload, err := json.Marshal(p)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "wedding-invitation/1.0")
	resp, err := sheetHTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		slurp, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("sheets webhook: %s: %s", resp.Status, string(slurp))
	}
	return nil
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
