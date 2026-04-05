package server

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
	"time"

	"github.com/fasthttp/router"
	"github.com/fasthttp/websocket"
	bifrost "github.com/maximhq/bifrost/core"
	"github.com/maximhq/bifrost/core/schemas"
	configstoreTables "github.com/maximhq/bifrost/framework/configstore/tables"
	"github.com/maximhq/bifrost/framework/kvstore"
	enterprisecfg "github.com/maximhq/bifrost/transports/bifrost-http/enterprise"
	"github.com/maximhq/bifrost/transports/bifrost-http/handlers"
	"github.com/maximhq/bifrost/transports/bifrost-http/lib"
	"github.com/valyala/fasthttp"
)

type storeUpdateMessage struct {
	Type string   `json:"type"`
	Tags []string `json:"tags"`
}

func TestPropagateClusterConfigChangeBroadcastsLocalStoreUpdateBeforePeerFanoutFailure(t *testing.T) {
	SetLogger(bifrost.NewNoOpLogger())
	handlers.SetLogger(bifrost.NewNoOpLogger())

	peer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"boom"}`))
	}))
	defer peer.Close()

	kv, err := kvstore.New(kvstore.Config{CleanupInterval: time.Minute})
	if err != nil {
		t.Fatalf("kvstore.New() error = %v", err)
	}
	defer kv.Close()

	cluster, err := enterprisecfg.NewClusterService(&enterprisecfg.ClusterConfig{
		Enabled:   true,
		Peers:     []string{peer.URL},
		AuthToken: schemas.NewEnvVar("cluster-secret"),
	}, kv, "local-node", bifrost.NewNoOpLogger())
	if err != nil {
		t.Fatalf("NewClusterService() error = %v", err)
	}
	defer cluster.Close()

	wsHandler, conn, cleanup := connectStoreUpdateWebSocket(t)
	defer cleanup()

	server := &BifrostHTTPServer{
		ClusterService:   cluster,
		WebSocketHandler: wsHandler,
	}

	err = server.PropagateClusterConfigChange(context.Background(), &handlers.ClusterConfigChange{
		Scope: handlers.ClusterConfigScopeProvider,
	})
	if err == nil {
		t.Fatal("expected peer fanout error")
	}

	msg := readStoreUpdateMessage(t, conn)
	if msg.Type != "store_update" {
		t.Fatalf("expected store_update message, got %+v", msg)
	}
	assertStoreUpdateHasTags(t, msg.Tags, "Providers", "DBKeys", "Models", "BaseModels", "ProviderGovernance", "ClusterNodes")
}

func TestApplyClusterConfigChangeBroadcastsStoreUpdateAfterPeerSideApply(t *testing.T) {
	SetLogger(bifrost.NewNoOpLogger())
	handlers.SetLogger(bifrost.NewNoOpLogger())

	store := newClusterPluginApplyStore(t)
	wsHandler, conn, cleanup := connectStoreUpdateWebSocket(t)
	defer cleanup()

	server := &BifrostHTTPServer{
		Config:           &lib.Config{ConfigStore: store},
		WebSocketHandler: wsHandler,
	}

	session := &configstoreTables.SessionsTable{
		Token:     "cluster-session-store-update",
		ExpiresAt: time.Unix(1700010000, 0).UTC(),
		CreatedAt: time.Unix(1700000000, 0).UTC(),
		UpdatedAt: time.Unix(1700000100, 0).UTC(),
	}

	if err := server.ApplyClusterConfigChange(context.Background(), &handlers.ClusterConfigChange{
		Scope:         handlers.ClusterConfigScopeSession,
		SessionToken:  session.Token,
		SessionConfig: session,
	}); err != nil {
		t.Fatalf("ApplyClusterConfigChange() error = %v", err)
	}

	stored, err := store.GetSession(context.Background(), session.Token)
	if err != nil {
		t.Fatalf("GetSession() error = %v", err)
	}
	if stored == nil || stored.Token != session.Token {
		t.Fatalf("expected session to be persisted, got %+v", stored)
	}

	msg := readStoreUpdateMessage(t, conn)
	if msg.Type != "store_update" {
		t.Fatalf("expected store_update message, got %+v", msg)
	}
	assertStoreUpdateHasTags(t, msg.Tags, "SessionState", "ClusterNodes")
}

func connectStoreUpdateWebSocket(t *testing.T) (*handlers.WebSocketHandler, *websocket.Conn, func()) {
	t.Helper()

	wsHandler := handlers.NewWebSocketHandler(context.Background(), nil)
	r := router.New()
	wsHandler.RegisterRoutes(r)

	srv := &fasthttp.Server{Handler: r.Handler}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen() error = %v", err)
	}

	go func() {
		_ = srv.Serve(listener)
	}()

	conn, _, err := websocket.DefaultDialer.Dial("ws://"+listener.Addr().String()+"/ws", nil)
	if err != nil {
		_ = srv.Shutdown()
		_ = listener.Close()
		t.Fatalf("websocket.Dial() error = %v", err)
	}

	cleanup := func() {
		_ = conn.Close()
		_ = srv.Shutdown()
		_ = listener.Close()
	}

	return wsHandler, conn, cleanup
}

func readStoreUpdateMessage(t *testing.T, conn *websocket.Conn) storeUpdateMessage {
	t.Helper()

	if conn == nil {
		t.Fatal("websocket connection is nil")
	}
	if err := conn.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatalf("SetReadDeadline() error = %v", err)
	}
	defer conn.SetReadDeadline(time.Time{})

	_, payload, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("ReadMessage() error = %v", err)
	}

	var msg storeUpdateMessage
	if err := json.Unmarshal(payload, &msg); err != nil {
		t.Fatalf("json.Unmarshal() error = %v, payload=%s", err, string(payload))
	}
	return msg
}

func assertStoreUpdateHasTags(t *testing.T, got []string, expected ...string) {
	t.Helper()

	for _, tag := range expected {
		if !slices.Contains(got, tag) {
			t.Fatalf("expected tag %q in %v", tag, got)
		}
	}
}
