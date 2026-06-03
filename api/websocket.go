// Пакет api — взаимодействие с Bybit V5 API.
// websocket.go — постоянное WebSocket-соединение с автопереподключением.
package api

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// WSMessage — сырое WS-сообщение от Bybit.
type WSMessage struct {
	Topic string          `json:"topic"`
	Type  string          `json:"type"`
	Data  json.RawMessage `json:"data"`
	Ts    int64           `json:"ts"`
}

// WSHandler — функция-обработчик входящего сообщения.
type WSHandler func(msg WSMessage)

// WSClient управляет постоянным WS-соединением.
type WSClient struct {
	url       string
	conn      *websocket.Conn
	mu        sync.Mutex
	handlers  map[string][]WSHandler
	done      chan struct{}
	reconnect bool
}

// NewWSClient создаёт новый WS-клиент.
func NewWSClient(wsURL string) *WSClient {
	return &WSClient{
		url: wsURL, handlers: make(map[string][]WSHandler),
		done: make(chan struct{}), reconnect: true,
	}
}

// Subscribe добавляет обработчик для топика.
func (w *WSClient) Subscribe(topic string, h WSHandler) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.handlers[topic] = append(w.handlers[topic], h)
}

// Connect открывает соединение и запускает горутины.
func (w *WSClient) Connect() error {
	conn, _, err := websocket.DefaultDialer.Dial(w.url, nil)
	if err != nil {
		return fmt.Errorf("ws dial: %w", err)
	}
	w.conn = conn
	topics := make([]string, 0, len(w.handlers))
	for t := range w.handlers {
		topics = append(topics, t)
	}
	if err := w.sendSub(topics); err != nil {
		return err
	}
	go w.readLoop()
	go w.pingLoop()
	return nil
}

// Close завершает соединение.
func (w *WSClient) Close() {
	w.reconnect = false
	close(w.done)
	if w.conn != nil {
		w.conn.Close()
	}
}

func (w *WSClient) sendSub(topics []string) error {
	if len(topics) == 0 {
		return nil
	}
	msg, _ := json.Marshal(map[string]interface{}{"op": "subscribe", "args": topics})
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.conn.WriteMessage(websocket.TextMessage, msg)
}

func (w *WSClient) readLoop() {
	for {
		select {
		case <-w.done:
			return
		default:
		}
		_, msg, err := w.conn.ReadMessage()
		if err != nil {
			if w.reconnect {
				time.Sleep(3 * time.Second)
				w.reconnectLoop()
			}
			return
		}
		var m WSMessage
		if err := json.Unmarshal(msg, &m); err != nil || m.Topic == "" {
			continue
		}
		w.mu.Lock()
		handlers := append([]WSHandler{}, w.handlers[m.Topic]...)
		w.mu.Unlock()
		for _, h := range handlers {
			h(m)
		}
	}
}

func (w *WSClient) pingLoop() {
	t := time.NewTicker(20 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-w.done:
			return
		case <-t.C:
			w.mu.Lock()
			if w.conn != nil {
				_ = w.conn.WriteMessage(websocket.TextMessage, []byte(`{"op":"ping"}`))
			}
			w.mu.Unlock()
		}
	}
}

func (w *WSClient) reconnectLoop() {
	for {
		select {
		case <-w.done:
			return
		default:
		}
		conn, _, err := websocket.DefaultDialer.Dial(w.url, nil)
		if err != nil {
			time.Sleep(5 * time.Second)
			continue
		}
		w.mu.Lock()
		w.conn = conn
		topics := make([]string, 0, len(w.handlers))
		for t := range w.handlers {
			topics = append(topics, t)
		}
		w.mu.Unlock()
		_ = w.sendSub(topics)
		go w.readLoop()
		return
	}
}
