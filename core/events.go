// Пакет core — центральный оркестратор.
// events.go — шина событий для связи компонентов без прямых зависимостей.
package core

import "sync"

// EventType — тип события.
type EventType string

const (
	EventSignalGenerated EventType = "signal.generated"
	EventSignalPublished EventType = "signal.published"
	EventSignalExecuted  EventType = "signal.executed"
	EventScanCompleted   EventType = "scan.completed"
	EventError           EventType = "error"
)

// Event — единица данных шины.
type Event struct {
	Type    EventType
	Payload interface{}
}

// Handler — обработчик событий.
type Handler func(e Event)

// EventBus — простая in-process шина событий.
type EventBus struct {
	mu       sync.RWMutex
	handlers map[EventType][]Handler
}

// NewEventBus создаёт EventBus.
func NewEventBus() *EventBus {
	return &EventBus{handlers: make(map[EventType][]Handler)}
}

// Subscribe подписывается на тип события.
func (b *EventBus) Subscribe(t EventType, h Handler) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.handlers[t] = append(b.handlers[t], h)
}

// Publish рассылает событие всем подписчикам.
func (b *EventBus) Publish(e Event) {
	b.mu.RLock()
	handlers := append([]Handler{}, b.handlers[e.Type]...)
	b.mu.RUnlock()
	for _, h := range handlers {
		h(e)
	}
}
