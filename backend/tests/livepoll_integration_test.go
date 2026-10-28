//go:build integration

package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"livepoll/backend/internal/config"
	"livepoll/backend/internal/httpapi"
	"livepoll/backend/internal/live"
	"livepoll/backend/internal/model"
	"livepoll/backend/internal/store"
)

func TestLivePollEndToEnd(t *testing.T) {
	mongoURI := env("MONGO_URI", "mongodb://127.0.0.1:27017")
	redisURL := env("REDIS_URL", "redis://127.0.0.1:6379/15")
	dbName := fmt.Sprintf("livepoll_it_%d", time.Now().UnixNano())
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	database, err := store.Connect(ctx, mongoURI, dbName)
	if err != nil {
		t.Fatalf("connect test MongoDB: %v", err)
	}
	defer func() {
		cleanup, done := context.WithTimeout(context.Background(), 5*time.Second)
		defer done()
		_ = database.DB.Drop(cleanup)
		_ = database.Close(cleanup)
	}()
	realtime, err := live.Connect(redisURL)
	if err != nil {
		t.Fatalf("connect test Redis: %v", err)
	}
	defer realtime.Close()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	origin := "http://" + listener.Addr().String()
	application := httpapi.New(database, realtime, config.Config{MongoURI: mongoURI, MongoDB: dbName, RedisURL: redisURL, PublicOrigin: origin})
	server := httptest.NewUnstartedServer(application.Router())
	server.Listener = listener
	server.Start()
	defer server.Close()

	owner, ownerJar := newClient(t)
	ownerUser := signup(t, owner, ownerJar, server.URL, origin)
	createToken := cookie(t, ownerJar, server.URL, "csrf_token")
	loginClient, loginJar := newClient(t)
	requestJSON(t, loginClient, origin, http.MethodPost, server.URL+"/api/auth/login", map[string]any{"email": ownerUser["email"], "password": "test-password-123"}, "", http.StatusOK)
	loginCSRF := cookie(t, loginJar, server.URL, "csrf_token")
	requestJSON(t, loginClient, origin, http.MethodGet, server.URL+"/api/auth/me", nil, "", http.StatusOK)
	requestJSON(t, loginClient, origin, http.MethodPost, server.URL+"/api/auth/logout", map[string]any{}, loginCSRF, http.StatusOK)
	requestJSON(t, loginClient, origin, http.MethodGet, server.URL+"/api/auth/me", nil, "", http.StatusUnauthorized)
	duplicateAccount, _ := newClient(t)
	requestJSON(t, duplicateAccount, origin, http.MethodPost, server.URL+"/api/auth/signup", map[string]any{"name": "Duplicate Owner", "email": strings.ToUpper(ownerUser["email"].(string)), "password": "test-password-123"}, "", http.StatusConflict)
	poll := requestJSON(t, owner, origin, http.MethodPost, server.URL+"/api/polls", map[string]any{"question": "Which session should we host?", "options": []string{"Morning", "Afternoon"}}, createToken, http.StatusCreated)
	pollID := poll["id"].(string)
	options := poll["options"].([]any)
	firstID := options[0].(map[string]any)["id"].(string)
	secondID := options[1].(map[string]any)["id"].(string)

	viewer1, viewer1Jar := newClient(t)
	viewer2, viewer2Jar := newClient(t)
	for _, viewer := range []struct {
		client *http.Client
		jar    *cookiejar.Jar
	}{{viewer1, viewer1Jar}, {viewer2, viewer2Jar}} {
		requestJSON(t, viewer.client, origin, http.MethodGet, server.URL+"/api/polls/"+pollID+"/results", nil, "", http.StatusOK)
	}
	ws1 := openSocket(t, server.URL, origin, pollID)
	defer ws1.Close()
	ws2 := openSocket(t, server.URL, origin, pollID)
	defer ws2.Close()
	assertEvent(t, ws1, "snapshot", 0)
	assertEvent(t, ws2, "snapshot", 0)

	viewer1CSRF := cookie(t, viewer1Jar, server.URL, "csrf_token")
	viewer2CSRF := cookie(t, viewer2Jar, server.URL, "csrf_token")
	requestJSON(t, viewer1, origin, http.MethodPost, server.URL+"/api/polls", map[string]any{"question": "Should this fail?", "options": []string{"Yes", "No"}}, viewer1CSRF, http.StatusUnauthorized)
	if cookie(t, viewer1Jar, server.URL, "voter_id") == "" {
		t.Fatal("public poll view did not initialize voter identity")
	}
	uninitialized, uninitializedJar := newClient(t)
	requestJSON(t, uninitialized, origin, http.MethodGet, server.URL+"/api/polls/"+pollID+"/results", nil, "", http.StatusOK)
	uninitializedCSRF := cookie(t, uninitializedJar, server.URL, "csrf_token")
	withoutVoter, err := http.NewRequest(http.MethodPost, server.URL+"/api/polls/"+pollID+"/votes", bytes.NewBufferString(`{"optionId":"`+firstID+`"}`))
	if err != nil {
		t.Fatal(err)
	}
	withoutVoter.Header.Set("Origin", origin)
	withoutVoter.Header.Set("Content-Type", "application/json")
	withoutVoter.Header.Set("Cookie", "csrf_token="+uninitializedCSRF)
	withoutVoter.Header.Set("X-CSRF-Token", uninitializedCSRF)
	withoutVoterResponse, err := http.DefaultClient.Do(withoutVoter)
	if err != nil {
		t.Fatal(err)
	}
	_ = withoutVoterResponse.Body.Close()
	if withoutVoterResponse.StatusCode != http.StatusPreconditionRequired {
		t.Fatalf("vote without voter cookie returned %d, expected %d", withoutVoterResponse.StatusCode, http.StatusPreconditionRequired)
	}
	requestJSON(t, viewer1, origin, http.MethodPost, server.URL+"/api/polls/"+pollID+"/votes", map[string]any{"optionId": "invalid"}, viewer1CSRF, http.StatusBadRequest)
	requestJSON(t, viewer1, origin, http.MethodPost, server.URL+"/api/polls/"+pollID+"/votes", map[string]any{"optionId": firstID}, viewer1CSRF, http.StatusCreated)
	assertEvent(t, ws1, "vote", 1)
	assertEvent(t, ws2, "vote", 1)
	requestJSON(t, viewer1, origin, http.MethodPost, server.URL+"/api/polls/"+pollID+"/votes", map[string]any{"optionId": firstID}, viewer1CSRF, http.StatusConflict)
	requestJSON(t, viewer2, origin, http.MethodPost, server.URL+"/api/polls/"+pollID+"/votes", map[string]any{"optionId": secondID}, viewer2CSRF, http.StatusCreated)
	assertEvent(t, ws1, "vote", 2)
	assertEvent(t, ws2, "vote", 2)

	// Simultaneous POSTs from the same cookie jar must still yield exactly one accepted vote.
	racer, racerJar := newClient(t)
	requestJSON(t, racer, origin, http.MethodGet, server.URL+"/api/polls/"+pollID+"/results", nil, "", http.StatusOK)
	racerCSRF := cookie(t, racerJar, server.URL, "csrf_token")
	const attempts = 8
	start := make(chan struct{})
	statuses := make(chan int, attempts)
	var wait sync.WaitGroup
	for i := 0; i < attempts; i++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-start
			payload, _ := json.Marshal(map[string]string{"optionId": firstID})
			req, requestErr := http.NewRequest(http.MethodPost, server.URL+"/api/polls/"+pollID+"/votes", bytes.NewReader(payload))
			if requestErr != nil {
				statuses <- 0
				return
			}
			req.Header.Set("Origin", origin)
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-CSRF-Token", racerCSRF)
			response, requestErr := racer.Do(req)
			if requestErr != nil {
				statuses <- 0
				return
			}
			statuses <- response.StatusCode
			_ = response.Body.Close()
		}()
	}
	close(start)
	wait.Wait()
	close(statuses)
	accepted, duplicate := 0, 0
	for status := range statuses {
		if status == http.StatusCreated {
			accepted++
		} else if status == http.StatusConflict {
			duplicate++
		}
	}
	if accepted != 1 || duplicate != attempts-1 {
		t.Fatalf("concurrent same-voter submissions: accepted=%d duplicate=%d", accepted, duplicate)
	}
	assertEvent(t, ws1, "vote", 3)
	assertEvent(t, ws2, "vote", 3)

	result := requestJSON(t, viewer1, origin, http.MethodGet, server.URL+"/api/polls/"+pollID+"/results", nil, "", http.StatusOK)
	if result["totalVotes"].(float64) != 3 {
		t.Fatalf("want three durable votes, got %v", result["totalVotes"])
	}
	stored, err := database.Votes.CountDocuments(ctx, map[string]any{"pollId": pollID})
	if err != nil || stored != 3 {
		t.Fatalf("Mongo vote count = %d, err=%v", stored, err)
	}
	counts, err := realtime.R.HGetAll(ctx, "poll:"+pollID+":counts").Result()
	if err != nil || counts["__total"] != "3" || counts[firstID] != "2" || counts[secondID] != "1" {
		t.Fatalf("Redis counters incorrect: %v, err=%v", counts, err)
	}

	// Delete only this poll's active counter hash and confirm an HTTP result read rebuilds it from Mongo.
	if err = realtime.R.Del(ctx, "poll:"+pollID+":counts").Err(); err != nil {
		t.Fatal(err)
	}
	recovered, err := application.Polls.Results(ctx, model.Poll{ID: pollID, Options: []model.Option{{ID: firstID}, {ID: secondID}}})
	if err != nil || recovered.TotalVotes != 3 {
		t.Fatalf("Mongo-to-Redis recovery failed: %v, err=%v", recovered, err)
	}
	counts, err = realtime.R.HGetAll(ctx, "poll:"+pollID+":counts").Result()
	if err != nil || counts["__total"] != "3" {
		t.Fatalf("recovered Redis counters incorrect: %v, err=%v", counts, err)
	}

	// Force one temporary Redis update failure, then verify the Mongo pending marker is retried idempotently.
	third, thirdJar := newClient(t)
	requestJSON(t, third, origin, http.MethodGet, server.URL+"/api/polls/"+pollID+"/results", nil, "", http.StatusOK)
	thirdCSRF := cookie(t, thirdJar, server.URL, "csrf_token")
	if err = realtime.R.Set(ctx, "poll:"+pollID+":rebuild-lock", "test", 15*time.Second).Err(); err != nil {
		t.Fatal(err)
	}
	voteResult := requestJSON(t, third, origin, http.MethodPost, server.URL+"/api/polls/"+pollID+"/votes", map[string]any{"optionId": firstID}, thirdCSRF, http.StatusCreated)
	if voteResult["pendingRealtime"] != true {
		t.Fatalf("vote should report pending Redis propagation: %v", voteResult)
	}
	_ = realtime.R.Del(ctx, "poll:"+pollID+":rebuild-lock").Err()
	_ = realtime.R.Del(ctx, "poll:"+pollID+":counts").Err()
	pendingRebuild, err := application.Polls.Results(ctx, model.Poll{ID: pollID, Options: []model.Option{{ID: firstID}, {ID: secondID}}})
	if err != nil || pendingRebuild.TotalVotes != 4 {
		t.Fatalf("rebuild including pending vote failed: total=%d, err=%v", pendingRebuild.TotalVotes, err)
	}
	application.Polls.ReconcilePending(ctx)
	assertEvent(t, ws1, "vote", 4)
	assertEvent(t, ws2, "vote", 4)
	counts, err = realtime.R.HGetAll(ctx, "poll:"+pollID+":counts").Result()
	if err != nil || counts["__total"] != "4" || counts[firstID] != "3" {
		t.Fatalf("reconciled Redis counters incorrect: %v, err=%v", counts, err)
	}

	// A different authenticated owner cannot close the poll; the real owner can.
	other, otherJar := newClient(t)
	signup(t, other, otherJar, server.URL, origin)
	otherCSRF := cookie(t, otherJar, server.URL, "csrf_token")
	requestJSON(t, other, origin, http.MethodPost, server.URL+"/api/polls/"+pollID+"/close", map[string]any{}, otherCSRF, http.StatusNotFound)
	requestJSON(t, owner, origin, http.MethodPost, server.URL+"/api/polls/"+pollID+"/close", map[string]any{}, createToken, http.StatusOK)
	assertStatusEvent(t, ws1, "closed")
	assertStatusEvent(t, ws2, "closed")
	requestJSON(t, other, origin, http.MethodPost, server.URL+"/api/polls/"+pollID+"/reopen", map[string]any{}, otherCSRF, http.StatusNotFound)
	fourth, fourthJar := newClient(t)
	requestJSON(t, fourth, origin, http.MethodGet, server.URL+"/api/polls/"+pollID+"/results", nil, "", http.StatusOK)
	requestJSON(t, fourth, origin, http.MethodPost, server.URL+"/api/polls/"+pollID+"/votes", map[string]any{"optionId": firstID}, cookie(t, fourthJar, server.URL, "csrf_token"), http.StatusConflict)
	requestJSON(t, owner, origin, http.MethodPost, server.URL+"/api/polls/"+pollID+"/reopen", map[string]any{}, createToken, http.StatusOK)
	assertStatusEvent(t, ws1, "open")
	assertStatusEvent(t, ws2, "open")
	fifth, fifthJar := newClient(t)
	requestJSON(t, fifth, origin, http.MethodGet, server.URL+"/api/polls/"+pollID+"/results", nil, "", http.StatusOK)
	requestJSON(t, fifth, origin, http.MethodPost, server.URL+"/api/polls/"+pollID+"/votes", map[string]any{"optionId": secondID}, cookie(t, fifthJar, server.URL, "csrf_token"), http.StatusCreated)
	assertEvent(t, ws1, "vote", 5)
	assertEvent(t, ws2, "vote", 5)
}

func newClient(t *testing.T) (*http.Client, *cookiejar.Jar) {
	t.Helper()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	return &http.Client{Jar: jar, Timeout: 10 * time.Second}, jar
}

func signup(t *testing.T, client *http.Client, jar *cookiejar.Jar, baseURL, origin string) map[string]any {
	t.Helper()
	user := fmt.Sprintf("integration-%d@example.test", time.Now().UnixNano())
	response := requestJSON(t, client, origin, http.MethodPost, baseURL+"/api/auth/signup", map[string]any{"name": "Integration Owner", "email": user, "password": "test-password-123"}, "", http.StatusOK)
	cookie(t, jar, baseURL, "session")
	return response["user"].(map[string]any)
}

func cookie(t *testing.T, jar *cookiejar.Jar, baseURL, name string) string {
	t.Helper()
	parsed, err := url.Parse(baseURL)
	if err != nil {
		t.Fatal(err)
	}
	for _, candidate := range jar.Cookies(parsed) {
		if candidate.Name == name {
			return candidate.Value
		}
	}
	t.Fatalf("cookie %q was not issued", name)
	return ""
}

func requestJSON(t *testing.T, client *http.Client, origin, method, endpoint string, payload map[string]any, csrf string, expectedStatus int) map[string]any {
	t.Helper()
	// Allow a method-first form for readability at call sites while supporting GET bodies as nil.
	if method == "" {
		method = http.MethodGet
	}
	var body *bytes.Reader
	if payload == nil {
		body = bytes.NewReader(nil)
	} else {
		encoded, err := json.Marshal(payload)
		if err != nil {
			t.Fatal(err)
		}
		body = bytes.NewReader(encoded)
	}
	request, err := http.NewRequest(method, endpoint, body)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Origin", origin)
	if payload != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	if csrf != "" {
		request.Header.Set("X-CSRF-Token", csrf)
	}
	response, err := client.Do(request)
	if err != nil {
		t.Fatalf("%s %s: %v", method, endpoint, err)
	}
	defer response.Body.Close()
	var value map[string]any
	if err = json.NewDecoder(response.Body).Decode(&value); err != nil {
		t.Fatalf("decode %s response: %v", endpoint, err)
	}
	if response.StatusCode != expectedStatus {
		t.Fatalf("%s %s: status %d, expected %d; body=%v", method, endpoint, response.StatusCode, expectedStatus, value)
	}
	return value
}

func openSocket(t *testing.T, baseURL, origin, pollID string) *websocket.Conn {
	t.Helper()
	socketURL := "ws" + strings.TrimPrefix(baseURL, "http") + "/api/ws/polls/" + pollID
	dialer := websocket.Dialer{HandshakeTimeout: 5 * time.Second}
	connection, response, err := dialer.Dial(socketURL, http.Header{"Origin": []string{origin}})
	if err != nil {
		if response != nil {
			t.Fatalf("WebSocket handshake: %v, status %d", err, response.StatusCode)
		}
		t.Fatalf("WebSocket handshake: %v", err)
	}
	return connection
}

func assertEvent(t *testing.T, connection *websocket.Conn, expectedType string, expectedTotal int64) {
	t.Helper()
	_ = connection.SetReadDeadline(time.Now().Add(5 * time.Second))
	var event map[string]any
	if err := connection.ReadJSON(&event); err != nil {
		t.Fatalf("read %s WebSocket event: %v", expectedType, err)
	}
	if event["type"] != expectedType {
		t.Fatalf("want %s event, got %v", expectedType, event["type"])
	}
	if expectedType == "snapshot" || expectedType == "vote" {
		if int64(event["totalVotes"].(float64)) != expectedTotal {
			t.Fatalf("want total %d, got %v", expectedTotal, event["totalVotes"])
		}
	}
}

func assertStatusEvent(t *testing.T, connection *websocket.Conn, status string) {
	t.Helper()
	_ = connection.SetReadDeadline(time.Now().Add(5 * time.Second))
	var event map[string]any
	if err := connection.ReadJSON(&event); err != nil {
		t.Fatalf("read status event: %v", err)
	}
	if event["type"] != "status" || event["status"] != status {
		t.Fatalf("want %s status event, got %v", status, event)
	}
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
