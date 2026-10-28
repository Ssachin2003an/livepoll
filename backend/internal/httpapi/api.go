package httpapi

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"go.mongodb.org/mongo-driver/mongo"
	"livepoll/backend/internal/auth"
	"livepoll/backend/internal/config"
	"livepoll/backend/internal/live"
	"livepoll/backend/internal/model"
	"livepoll/backend/internal/service"
	"livepoll/backend/internal/store"
	"livepoll/backend/internal/validation"
)

type API struct {
	S     *store.Store
	L     *live.Live
	C     config.Config
	Auth  *service.AuthService
	Polls *service.PollService
	Up    websocket.Upgrader
}

func New(s *store.Store, l *live.Live, c config.Config) *API {
	return &API{S: s, L: l, C: c, Auth: service.NewAuth(s), Polls: service.NewPoll(s, l), Up: websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return c.AllowsOrigin(r.Header.Get("Origin")) }}}
}

func (a *API) Router() *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	g := gin.New()
	g.Use(gin.Recovery(), a.requestLimit(), a.cors(), a.rateLimit(), a.accessLog())
	g.GET("/healthz", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"status": "ok"}) })
	g.GET("/readyz", func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()
		if err := a.S.Client.Ping(ctx, nil); err != nil {
			fail(c, http.StatusServiceUnavailable, "database unavailable")
			return
		}
		if err := a.L.R.Ping(ctx).Err(); err != nil {
			fail(c, http.StatusServiceUnavailable, "realtime service unavailable")
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ready"})
	})
	// Keep the original health route as a harmless local-development alias.
	g.GET("/health", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"status": "ok"}) })

	r := g.Group("/api")
	r.POST("/auth/signup", a.signup)
	r.POST("/auth/login", a.login)
	r.POST("/auth/logout", a.logout)
	r.GET("/auth/me", a.me)
	r.POST("/polls", a.createPoll)
	r.GET("/polls", a.listPolls)
	r.GET("/polls/:id", a.getPoll)
	r.DELETE("/polls/:id", a.deletePoll)
	r.GET("/polls/:id/results", a.results)
	r.POST("/polls/:id/votes", a.vote)
	r.POST("/polls/:id/close", a.setStatus("closed"))
	r.POST("/polls/:id/reopen", a.setStatus("open"))
	r.GET("/ws/polls/:id", a.websocket)
	g.NoRoute(func(c *gin.Context) {
		if c.Request.Method != http.MethodGet || c.Request.URL.Path == "/api" || strings.HasPrefix(c.Request.URL.Path, "/api/") {
			fail(c, http.StatusNotFound, "route not found")
			return
		}
		relative := filepath.Clean(filepath.FromSlash(strings.TrimPrefix(c.Request.URL.Path, "/")))
		if relative != "." && !filepath.IsLocal(relative) {
			fail(c, http.StatusNotFound, "not found")
			return
		}
		asset := filepath.Join("frontend", "dist", relative)
		if relative != "." {
			if info, err := os.Stat(asset); err == nil && !info.IsDir() {
				c.File(asset)
				return
			}
		}
		c.File(filepath.Join("frontend", "dist", "index.html"))
	})
	return g
}

func (a *API) requestLimit() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 1<<20)
		c.Next()
	}
}

func (a *API) rateLimit() gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Request.URL.Path
		limit := int64(0)
		if c.Request.Method == http.MethodPost && (path == "/api/auth/login" || path == "/api/auth/signup") {
			limit = 12
		}
		if c.Request.Method == http.MethodPost && strings.HasSuffix(path, "/votes") {
			limit = 35
		}
		if limit == 0 {
			c.Next()
			return
		}
		digest := sha256.Sum256([]byte(c.RemoteIP()))
		key := fmt.Sprintf("%s:%s", strings.ReplaceAll(path, "/", ":"), hex.EncodeToString(digest[:12]))
		allowed, err := a.L.Allow(c.Request.Context(), key, limit, time.Minute)
		// Fail open during a Redis outage; vote uniqueness and auth still use MongoDB.
		if err == nil && !allowed {
			c.Header("Retry-After", "60")
			fail(c, http.StatusTooManyRequests, "too many attempts; try again in a minute")
			return
		}
		c.Next()
	}
}

func (a *API) accessLog() gin.HandlerFunc {
	return func(c *gin.Context) {
		started := time.Now()
		c.Next()
		slog.Info("http request", "method", c.Request.Method, "route", c.FullPath(), "status", c.Writer.Status(), "duration_ms", time.Since(started).Milliseconds())
	}
}

func (a *API) cors() gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin != "" && !a.C.AllowsOrigin(origin) {
			fail(c, http.StatusForbidden, "origin not allowed")
			return
		}
		if origin != "" {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Access-Control-Allow-Credentials", "true")
			c.Header("Access-Control-Allow-Headers", "Content-Type, X-CSRF-Token")
			c.Header("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
			c.Header("Vary", "Origin")
		}
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		if (c.Request.Method == http.MethodPost || c.Request.Method == http.MethodDelete) &&
			strings.HasPrefix(c.Request.URL.Path, "/api/") &&
			c.Request.URL.Path != "/api/auth/login" &&
			c.Request.URL.Path != "/api/auth/signup" {
			if !a.C.AllowsOrigin(origin) {
				fail(c, http.StatusForbidden, "request origin required")
				return
			}
			cookieToken, _ := c.Cookie("csrf_token")
			headerToken := c.GetHeader("X-CSRF-Token")
			if cookieToken == "" || headerToken == "" || subtle.ConstantTimeCompare([]byte(cookieToken), []byte(headerToken)) != 1 {
				fail(c, http.StatusForbidden, "CSRF validation failed")
				return
			}
		}
		c.Next()
	}
}

func (a *API) cookie(c *gin.Context, name, value string, maxAge int, httpOnly bool) {
	if a.C.SecureCookies {
		c.SetSameSite(http.SameSiteNoneMode)
	} else {
		c.SetSameSite(http.SameSiteLaxMode)
	}
	c.SetCookie(name, value, maxAge, "/", a.C.CookieDomain, a.C.SecureCookies, httpOnly)
}

func decode(c *gin.Context, target any) error {
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return errors.New("request must contain one JSON value")
	}
	return nil
}

func (a *API) signup(c *gin.Context) {
	var body struct {
		Name     string `json:"name"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if decode(c, &body) != nil {
		fail(c, http.StatusBadRequest, "invalid signup request")
		return
	}
	user, err := a.Auth.Signup(c.Request.Context(), body.Name, body.Email, body.Password)
	if errors.Is(err, service.ErrDuplicateEmail) {
		fail(c, http.StatusConflict, err.Error())
		return
	}
	if err != nil {
		var fieldErr *validation.FieldError
		if errors.As(err, &fieldErr) {
			fail(c, http.StatusBadRequest, fieldErr.Error())
		} else {
			fail(c, http.StatusInternalServerError, "could not create account")
		}
		return
	}
	a.startSession(c, user)
}

func (a *API) login(c *gin.Context) {
	var body struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if decode(c, &body) != nil {
		fail(c, http.StatusBadRequest, "invalid credentials")
		return
	}
	user, err := a.Auth.Login(c.Request.Context(), body.Email, body.Password)
	if errors.Is(err, validation.ErrInvalidCredentials) {
		fail(c, http.StatusUnauthorized, "invalid email or password")
		return
	}
	if err != nil {
		fail(c, http.StatusInternalServerError, "could not sign in")
		return
	}
	a.startSession(c, user)
}

func (a *API) startSession(c *gin.Context, user model.User) {
	tokens, err := a.Auth.CreateSession(c.Request.Context(), user)
	if err != nil {
		fail(c, http.StatusInternalServerError, "session unavailable")
		return
	}
	a.cookie(c, "session", tokens.Session, 7*24*3600, true)
	a.cookie(c, "csrf_token", tokens.CSRF, 7*24*3600, false)
	user.PasswordHash = ""
	c.JSON(http.StatusOK, gin.H{"user": user})
}

func (a *API) logout(c *gin.Context) {
	if token, err := c.Cookie("session"); err == nil {
		_ = a.Auth.Logout(c.Request.Context(), token)
	}
	a.cookie(c, "session", "", -1, true)
	a.cookie(c, "csrf_token", "", -1, false)
	c.JSON(http.StatusOK, gin.H{"message": "logged out"})
}

func (a *API) current(c *gin.Context) (model.User, bool) {
	token, err := c.Cookie("session")
	if err != nil {
		fail(c, http.StatusUnauthorized, "authentication required")
		return model.User{}, false
	}
	user, err := a.Auth.Current(c.Request.Context(), token)
	if err != nil {
		fail(c, http.StatusUnauthorized, "authentication required")
		return model.User{}, false
	}
	return user, true
}

func (a *API) me(c *gin.Context) {
	user, ok := a.current(c)
	if !ok {
		return
	}
	user.PasswordHash = ""
	c.JSON(http.StatusOK, gin.H{"user": user})
}

func (a *API) createPoll(c *gin.Context) {
	user, ok := a.current(c)
	if !ok {
		return
	}

	var body struct {
		Question      string     `json:"question"`
		Options       []string   `json:"options"`
		ClosesAt      *time.Time `json:"closesAt"`
		Anonymous     bool       `json:"anonymous"`
		AllowMultiple bool       `json:"allowMultiple"`
		MaxSelections int        `json:"maxSelections"`
	}

	if decode(c, &body) != nil {
		fail(c, http.StatusBadRequest, "invalid poll request")
		return
	}

	poll, err := a.Polls.Create(
		c.Request.Context(),
		user.ID,
		body.Question,
		body.Options,
		service.PollSettings{
			ClosesAt:      body.ClosesAt,
			Anonymous:     body.Anonymous,
			AllowMultiple: body.AllowMultiple,
			MaxSelections: body.MaxSelections,
		},
	)
	if err != nil {
		var fieldErr *validation.FieldError
		if errors.As(err, &fieldErr) {
			fail(c, http.StatusBadRequest, fieldErr.Error())
		} else {
			fail(c, http.StatusBadRequest, err.Error())
		}
		return
	}

	c.JSON(http.StatusCreated, poll)
}

func (a *API) listPolls(c *gin.Context) {
	user, ok := a.current(c)
	if !ok {
		return
	}
	polls, err := a.Polls.List(c.Request.Context(), user.ID)
	if err != nil {
		fail(c, http.StatusInternalServerError, "could not load polls")
		return
	}
	c.JSON(http.StatusOK, polls)
}

func (a *API) validPoll(c *gin.Context) (model.Poll, bool) {
	poll, err := a.Polls.Get(c.Request.Context(), c.Param("id"))
	if errors.Is(err, service.ErrPollID) {
		fail(c, http.StatusBadRequest, "invalid poll ID")
		return model.Poll{}, false
	}
	if err != nil {
		fail(c, http.StatusNotFound, "poll not found")
		return model.Poll{}, false
	}
	return poll, true
}

func (a *API) addViewerState(c *gin.Context, poll *model.Poll) {
	if token, err := c.Cookie("voter_id"); err == nil && len(token) == 64 {
		poll.HasVoted = a.Polls.HasVoted(c.Request.Context(), poll.ID, voterDigest(token))
	}
	if token, err := c.Cookie("session"); err == nil {
		if user, e := a.Auth.Current(c.Request.Context(), token); e == nil && user.ID == poll.OwnerID {
			poll.HasVoted = false
		}
	}
}

func voterDigest(token string) string {
	digest := sha256.Sum256([]byte(token))
	return hex.EncodeToString(digest[:])
}

func (a *API) ensureCSRF(c *gin.Context) {
	if token, err := c.Cookie("csrf_token"); err == nil && token != "" {
		return
	}
	token, err := auth.Token()
	if err == nil {
		a.cookie(c, "csrf_token", token, 365*24*3600, false)
	}
}

func (a *API) ensureVoter(c *gin.Context) bool {
	if token, err := c.Cookie("voter_id"); err == nil && len(token) == 64 {
		return true
	}
	token, err := auth.Token()
	if err != nil {
		fail(c, http.StatusInternalServerError, "could not initialize voter identity")
		return false
	}
	a.cookie(c, "voter_id", token, 365*24*3600, true)
	return true
}

func (a *API) getPoll(c *gin.Context) {
	poll, ok := a.validPoll(c)
	if !ok {
		return
	}
	a.ensureCSRF(c)
	if !a.ensureVoter(c) {
		return
	}
	a.addViewerState(c, &poll)
	c.JSON(http.StatusOK, poll)
}

func (a *API) result(ctx context.Context, poll model.Poll) (model.Results, error) {
	return a.Polls.Results(ctx, poll)
}

func (a *API) results(c *gin.Context) {
	poll, ok := a.validPoll(c)
	if !ok {
		return
	}
	a.ensureCSRF(c)
	if !a.ensureVoter(c) {
		return
	}
	result, err := a.result(c.Request.Context(), poll)
	if err != nil {
		fail(c, http.StatusInternalServerError, "could not load results")
		return
	}
	a.addViewerState(c, &result.Poll)
	c.JSON(http.StatusOK, result)
}

func (a *API) vote(c *gin.Context) {
	var body struct {
		OptionID  string   `json:"optionId"`
		OptionIDs []string `json:"optionIds"`
	}

	if decode(c, &body) != nil {
		fail(c, http.StatusBadRequest, "invalid vote request")
		return
	}

	optionIDs := body.OptionIDs
	if len(optionIDs) == 0 && body.OptionID != "" {
		optionIDs = []string{body.OptionID}
	}
	if len(optionIDs) == 0 {
		fail(c, http.StatusBadRequest, "at least one option is required")
		return
	}

	voterToken, err := c.Cookie("voter_id")
	if err != nil || len(voterToken) != 64 {
		fail(c, http.StatusPreconditionRequired, "open the poll first to initialize your secure voter identity")
		return
	}

	outcome, err := a.Polls.VoteOptions(
		c.Request.Context(),
		c.Param("id"),
		optionIDs,
		voterDigest(voterToken),
	)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrPollID):
			fail(c, http.StatusBadRequest, "invalid poll ID")
		case errors.Is(err, service.ErrPollNotFound):
			fail(c, http.StatusNotFound, "poll not found")
		case errors.Is(err, service.ErrInvalidChoice):
			fail(c, http.StatusBadRequest, "invalid option selection")
		case errors.Is(err, service.ErrPollClosed):
			fail(c, http.StatusConflict, "poll is closed or has expired")
		case errors.Is(err, service.ErrAlreadyVoted):
			fail(c, http.StatusConflict, "you have already voted in this poll")
		default:
			fail(c, http.StatusInternalServerError, "could not record vote")
		}
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"voteId":         outcome.VoteID,
		"accepted":       true,
		"pendingRealtime": outcome.PendingRealtime,
		"results":        outcome.Results,
	})
}

// ReconcilePending retries votes durably recorded in MongoDB but not yet confirmed in Redis.
func (a *API) ReconcilePending(ctx context.Context) {
	a.Polls.ReconcilePending(ctx)
}

func (a *API) setStatus(status string) gin.HandlerFunc {
	return func(c *gin.Context) {
		user, ok := a.current(c)
		if !ok {
			return
		}
		poll, err := a.Polls.SetStatus(c.Request.Context(), c.Param("id"), user.ID, status)
		if errors.Is(err, service.ErrPollID) {
			fail(c, http.StatusBadRequest, "invalid poll ID")
			return
		}
		if errors.Is(err, service.ErrNotOwner) {
			fail(c, http.StatusNotFound, "poll not found")
			return
		}
		if err != nil {
			fail(c, http.StatusInternalServerError, "could not update poll")
			return
		}
		c.JSON(http.StatusOK, poll)
	}
}

func (a *API) deletePoll(c *gin.Context) {
    user, ok := a.current(c)
    if !ok {
        return
    }

    pollID := c.Param("id")
    if !validation.PollID(pollID) {
        fail(c, http.StatusBadRequest, "invalid poll ID")
        return
    }

    err := a.S.DeletePoll(c.Request.Context(), pollID, user.ID)
    if errors.Is(err, mongo.ErrNoDocuments) {
        fail(c, http.StatusNotFound, "poll not found")
        return
    }
    if err != nil {
        slog.Error("poll deletion failed", "poll_id", pollID, "error", err)
        fail(c, http.StatusInternalServerError, "could not delete poll")
        return
    }

    if err := a.L.DeletePollData(c.Request.Context(), pollID); err != nil {
		slog.Error("Redis poll cleanup failed", "poll_id", pollID, "error", err)
	}

	c.JSON(http.StatusOK, gin.H{"message": "poll deleted"})
}

func (a *API) websocket(c *gin.Context) {
	poll, ok := a.validPoll(c)
	if !ok {
		return
	}
	conn, err := a.Up.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer conn.Close()
	conn.SetReadLimit(1024)
	ctx, cancel := context.WithCancel(c.Request.Context())
	defer cancel()
	subscription := a.L.Subscribe(ctx, poll.ID)
	defer subscription.Close()
	// Wait for Redis to confirm subscription before taking the snapshot, so later events cannot be missed.
	if _, err = subscription.Receive(ctx); err != nil {
		return
	}
	result, err := a.result(ctx, poll)
	if err != nil {
		return
	}
	a.addViewerState(c, &result.Poll)
	if conn.SetWriteDeadline(time.Now().Add(5*time.Second)) != nil {
		return
	}
	if err = conn.WriteJSON(gin.H{"type": "snapshot", "pollId": poll.ID, "poll": result.Poll, "totalVotes": result.TotalVotes, "counts": result.Counts, "percentages": result.Percentages}); err != nil {
		return
	}

	readDone := make(chan struct{})
	go func() {
		defer close(readDone)
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		conn.SetPongHandler(func(string) error { return conn.SetReadDeadline(time.Now().Add(60 * time.Second)) })
		for {
			if _, _, readErr := conn.ReadMessage(); readErr != nil {
				return
			}
		}
	}()
	ping := time.NewTicker(25 * time.Second)
	defer ping.Stop()
	messages := subscription.Channel()
	for {
		select {
		case message, open := <-messages:
			if !open {
				return
			}
			_ = conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
			if conn.WriteMessage(websocket.TextMessage, []byte(message.Payload)) != nil {
				return
			}
		case <-ping.C:
			_ = conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
			if conn.WriteMessage(websocket.PingMessage, nil) != nil {
				return
			}
		case <-readDone:
			return
		case <-ctx.Done():
			return
		}
	}
}

func mustToken() string {
	value, err := auth.Token()
	if err != nil {
		panic(fmt.Errorf("secure random source: %w", err))
	}
	return value
}
func fail(c *gin.Context, status int, message string) { c.JSON(status, gin.H{"error": message}) }
