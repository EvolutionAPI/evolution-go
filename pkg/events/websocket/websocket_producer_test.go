package websocket_producer

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/evolution-foundation/evolution-go/pkg/config"
	logger_wrapper "github.com/evolution-foundation/evolution-go/pkg/logger"
	"github.com/gorilla/websocket"
)

func newTestLoggerManager(t *testing.T) *logger_wrapper.LoggerManager {
	t.Helper()
	return logger_wrapper.NewLoggerManager(&config.Config{
		LogDirectory:  t.TempDir(),
		LogMaxSize:    1,
		LogMaxBackups: 1,
		LogMaxAge:     1,
		LogCompress:   false,
	})
}

// dialTestWS starts a local websocket server, dials a client, and returns the
// server-side connection registered for Produce writes.
func dialTestWS(t *testing.T) (serverConn *websocket.Conn, cleanup func()) {
	t.Helper()

	upgraded := make(chan *websocket.Conn, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("upgrade: %v", err)
			return
		}
		upgraded <- conn
		// Keep the connection open until the test finishes.
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}))

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")
	client, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		srv.Close()
		t.Fatalf("dial: %v", err)
	}

	var serverConnResult *websocket.Conn
	select {
	case serverConnResult = <-upgraded:
	case <-time.After(2 * time.Second):
		client.Close()
		srv.Close()
		t.Fatal("timed out waiting for server upgrade")
	}

	// Drain client reads so WriteJSON on the server does not block on buffer full.
	go func() {
		for {
			if _, _, err := client.ReadMessage(); err != nil {
				return
			}
		}
	}()

	cleanup = func() {
		_ = client.Close()
		if serverConnResult != nil {
			_ = serverConnResult.Close()
		}
		srv.Close()
	}
	return serverConnResult, cleanup
}

func TestProduceConcurrentWrites(t *testing.T) {
	const (
		goroutines = 50
		messages   = 20
	)

	t.Run("concurrent instance writes", func(t *testing.T) {
		serverConn, cleanup := dialTestWS(t)
		defer cleanup()

		producer := NewWebsocketProducer(newTestLoggerManager(t))
		const instanceID = "instance-race-test"
		producer.AddClient(instanceID, serverConn)

		var wg sync.WaitGroup
		errCh := make(chan error, goroutines*messages)

		for g := 0; g < goroutines; g++ {
			wg.Add(1)
			go func(n int) {
				defer wg.Done()
				for i := 0; i < messages; i++ {
					payload := []byte(fmt.Sprintf(`{"event":"Receipt","n":%d}`, n))
					if err := producer.Produce("instance-race-test.receipt", payload, instanceID, ""); err != nil {
						errCh <- err
						return
					}
				}
			}(g)
		}

		wg.Wait()
		close(errCh)

		for err := range errCh {
			t.Fatalf("Produce failed under concurrency: %v", err)
		}
	})

	t.Run("concurrent broadcast writes", func(t *testing.T) {
		serverConn, cleanup := dialTestWS(t)
		defer cleanup()

		producer := NewWebsocketProducer(newTestLoggerManager(t))
		producer.AddBroadcastClient(serverConn)

		var wg sync.WaitGroup
		errCh := make(chan error, goroutines*messages)

		for g := 0; g < goroutines; g++ {
			wg.Add(1)
			go func(n int) {
				defer wg.Done()
				for i := 0; i < messages; i++ {
					payload := []byte(fmt.Sprintf(`{"event":"Message","n":%d}`, n))
					if err := producer.Produce("broadcast.message", payload, "any-instance", ""); err != nil {
						errCh <- err
						return
					}
				}
			}(g)
		}

		wg.Wait()
		close(errCh)

		for err := range errCh {
			t.Fatalf("Produce failed under concurrency: %v", err)
		}
	})
}
