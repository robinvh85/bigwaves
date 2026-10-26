// Package auth signs users in through Auth0 (OIDC) and keeps server-side sessions in SQLite.
// Only emails already present in the users table may sign in.
package auth

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"

	"bigwaves/internal/config"
	"bigwaves/internal/platform/db"
	"bigwaves/internal/platform/db/dbq"
	"bigwaves/internal/platform/dbtype"
	"bigwaves/internal/platform/httpx"
)

const (
	sessionCookie = "bw_session"
	stateCookie   = "bw_oauth_state"
)

type User struct {
	ID    int64
	Email string
}

type ctxKey struct{}

// UserFrom returns the signed-in user put in the context by RequireSession.
func UserFrom(ctx context.Context) (User, bool) {
	u, ok := ctx.Value(ctxKey{}).(User)
	return u, ok
}

// WithUser is for tests and internal callers.
func WithUser(ctx context.Context, u User) context.Context {
	return context.WithValue(ctx, ctxKey{}, u)
}

type Service struct {
	cfg config.Config
	db  *db.DB

	once     sync.Once
	oidcErr  error
	verifier *oidc.IDTokenVerifier
	oauth    *oauth2.Config
}

func New(cfg config.Config, d *db.DB) *Service { return &Service{cfg: cfg, db: d} }

func (s *Service) devMode() bool { return s.cfg.Session.DevUserEmail != "" && !s.cfg.IsProduction() }

func (s *Service) provider(ctx context.Context) error {
	s.once.Do(func() {
		if s.cfg.Auth0.Domain == "" {
			s.oidcErr = errors.New("AUTH0_DOMAIN is not configured")
			return
		}
		issuer := "https://" + strings.TrimSuffix(strings.TrimPrefix(s.cfg.Auth0.Domain, "https://"), "/") + "/"
		p, err := oidc.NewProvider(ctx, issuer)
		if err != nil {
			s.oidcErr = fmt.Errorf("oidc discovery: %w", err)
			return
		}
		s.verifier = p.Verifier(&oidc.Config{ClientID: s.cfg.Auth0.ClientID})
		s.oauth = &oauth2.Config{
			ClientID:     s.cfg.Auth0.ClientID,
			ClientSecret: s.cfg.Auth0.ClientSecret,
			Endpoint:     p.Endpoint(),
			RedirectURL:  strings.TrimSuffix(s.cfg.AppURL, "/") + "/auth/callback",
			Scopes:       []string{oidc.ScopeOpenID, "email", "profile"},
		}
	})
	return s.oidcErr
}

// Routes mounts /auth/login, /auth/callback and /auth/logout.
func (s *Service) Routes(mux interface {
	Get(string, http.HandlerFunc)
	Post(string, http.HandlerFunc)
}) {
	mux.Get("/auth/login", s.login)
	mux.Get("/auth/callback", s.callback)
	mux.Post("/auth/logout", s.logout)
}

func (s *Service) login(w http.ResponseWriter, r *http.Request) {
	redirect := safeRedirect(r.URL.Query().Get("redirect"))
	if s.devMode() {
		u, err := s.ensureUser(r.Context(), s.cfg.Session.DevUserEmail)
		if err == nil {
			err = s.startSession(w, r, u)
		}
		if err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		slog.WarnContext(r.Context(), "dev login", "email", u.Email)
		http.Redirect(w, r, redirect, http.StatusFound) //nolint:gosec // redirect went through safeRedirect
		return
	}
	if err := s.provider(r.Context()); err != nil {
		httpx.WriteError(w, r, fmt.Errorf("auth: %w", err))
		return
	}
	state := randomToken(24)
	// Secure follows the APP_URL scheme: dev runs on plain http.
	http.SetCookie(w, &http.Cookie{ //nolint:gosec // see above
		Name: stateCookie, Value: state + "|" + url.QueryEscape(redirect), Path: "/auth",
		MaxAge: 600, HttpOnly: true, Secure: s.secure(), SameSite: http.SameSiteLaxMode,
	})
	http.Redirect(w, r, s.oauth.AuthCodeURL(state), http.StatusFound)
}

func (s *Service) callback(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if err := s.provider(ctx); err != nil {
		httpx.WriteError(w, r, fmt.Errorf("auth: %w", err))
		return
	}
	c, err := r.Cookie(stateCookie)
	if err != nil {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: stateCookie, Path: "/auth", MaxAge: -1, HttpOnly: true, Secure: s.secure(), SameSite: http.SameSiteLaxMode}) //nolint:gosec // Secure follows APP_URL
	state, rawRedirect, _ := strings.Cut(c.Value, "|")
	if state == "" || r.URL.Query().Get("state") != state {
		httpx.WriteError(w, r, httpx.NewError(http.StatusBadRequest, "invalid_state", "login expired, try again"))
		return
	}
	tok, err := s.oauth.Exchange(ctx, r.URL.Query().Get("code"))
	if err != nil {
		httpx.WriteError(w, r, httpx.NewError(http.StatusUnauthorized, "oauth_exchange_failed", "sign in failed"))
		return
	}
	rawID, _ := tok.Extra("id_token").(string)
	idTok, err := s.verifier.Verify(ctx, rawID)
	if err != nil {
		httpx.WriteError(w, r, httpx.NewError(http.StatusUnauthorized, "invalid_id_token", "sign in failed"))
		return
	}
	var claims struct {
		Email         string `json:"email"`
		EmailVerified bool   `json:"email_verified"`
	}
	if err := idTok.Claims(&claims); err != nil || claims.Email == "" {
		httpx.WriteError(w, r, httpx.NewError(http.StatusUnauthorized, "no_email", "the identity provider sent no email"))
		return
	}
	row, err := dbq.New(s.db.Read).GetUserByEmail(ctx, strings.ToLower(claims.Email))
	if errors.Is(err, sql.ErrNoRows) {
		slog.WarnContext(ctx, "login refused: unknown email", "email", claims.Email)
		httpx.WriteError(w, r, httpx.NewError(http.StatusForbidden, "not_allowed", "this account is not allowed"))
		return
	}
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if err := s.startSession(w, r, User{ID: row.ID, Email: row.Email}); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	redirect, _ := url.QueryUnescape(rawRedirect)
	http.Redirect(w, r, safeRedirect(redirect), http.StatusFound) //nolint:gosec // relative same-site paths only
}

func (s *Service) logout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(sessionCookie); err == nil {
		_ = dbq.New(s.db.Write).DeleteSession(r.Context(), c.Value)
	}
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Path: "/", MaxAge: -1, HttpOnly: true, Secure: s.secure(), SameSite: http.SameSiteLaxMode}) //nolint:gosec // Secure follows APP_URL
	w.WriteHeader(http.StatusNoContent)
}

func (s *Service) ensureUser(ctx context.Context, email string) (User, error) {
	email = strings.ToLower(email)
	row, err := dbq.New(s.db.Write).UpsertUser(ctx, dbq.UpsertUserParams{Email: email})
	if err != nil {
		return User{}, err
	}
	return User{ID: row.ID, Email: row.Email}, nil
}

// AddUser allows an email to sign in (used by bwctl).
func (s *Service) AddUser(ctx context.Context, email string) (User, error) {
	return s.ensureUser(ctx, email)
}

func (s *Service) startSession(w http.ResponseWriter, r *http.Request, u User) error {
	id := randomToken(32)
	ttl := s.cfg.Session.TTL
	ip := clientIP(r)
	ua := r.UserAgent()
	err := dbq.New(s.db.Write).CreateSession(r.Context(), dbq.CreateSessionParams{
		ID: id, UserID: u.ID, Ip: &ip, UserAgent: &ua, ExpiresAt: dbtype.NewTime(time.Now().Add(ttl)),
	})
	if err != nil {
		return err
	}
	http.SetCookie(w, &http.Cookie{ //nolint:gosec // Secure follows APP_URL; dev runs on plain http
		Name: sessionCookie, Value: id, Path: "/", MaxAge: int(ttl.Seconds()),
		HttpOnly: true, Secure: s.secure(), SameSite: http.SameSiteLaxMode,
	})
	return nil
}

func (s *Service) secure() bool { return strings.HasPrefix(s.cfg.AppURL, "https://") }

// RequireSession rejects requests without a valid session and stores the user in the context.
// Writes must also come from our own origin (defense in depth on top of SameSite=Lax).
func (s *Service) RequireSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie(sessionCookie)
		if err != nil || c.Value == "" {
			httpx.WriteError(w, r, httpx.ErrUnauthorized)
			return
		}
		row, err := dbq.New(s.db.Read).GetSessionUser(r.Context(), dbq.GetSessionUserParams{
			ID: c.Value, ExpiresAt: dbtype.Now(),
		})
		if errors.Is(err, sql.ErrNoRows) {
			httpx.WriteError(w, r, httpx.ErrUnauthorized)
			return
		}
		if err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		if isWrite(r.Method) && !s.sameOrigin(r) {
			httpx.WriteError(w, r, httpx.NewError(http.StatusForbidden, "bad_origin", "cross-origin request refused"))
			return
		}
		next.ServeHTTP(w, r.WithContext(WithUser(r.Context(), User{ID: row.ID, Email: row.Email})))
	})
}

func isWrite(m string) bool {
	return m != http.MethodGet && m != http.MethodHead && m != http.MethodOptions
}

func (s *Service) sameOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		// Non-browser clients (curl) send no Origin; the session cookie is still required.
		return true
	}
	app, err := url.Parse(s.cfg.AppURL)
	if err != nil {
		return false
	}
	o, err := url.Parse(origin)
	return err == nil && o.Scheme == app.Scheme && o.Host == app.Host
}

// CleanupExpired deletes expired sessions.
func (s *Service) CleanupExpired(ctx context.Context) (int64, error) {
	return dbq.New(s.db.Write).DeleteExpiredSessions(ctx, dbtype.Now())
}

func randomToken(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

// safeRedirect only allows same-site relative paths, so the login flow cannot be used as an
// open redirect.
func safeRedirect(p string) string {
	if p == "" || !strings.HasPrefix(p, "/") || strings.HasPrefix(p, "//") || strings.HasPrefix(p, "/\\") {
		return "/"
	}
	return p
}

// clientIP trusts X-Real-IP only from a proxy on the same host (nginx/Caddy in front of us).
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() {
		if real := r.Header.Get("X-Real-IP"); real != "" {
			return real
		}
	}
	return host
}
