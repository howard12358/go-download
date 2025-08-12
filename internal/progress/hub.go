package progress

import (
	"sync"
)

const (
	cacheSize = 16
)

// Hub 管理多个任务的订阅者
type Hub struct {
	mu   sync.Mutex
	Subs map[string]map[chan int]struct{} // taskID → set of subscriber channels
}

// NewHub 创建一个新的 Hub
func NewHub() *Hub {
	return &Hub{
		Subs: make(map[string]map[chan int]struct{}),
	}
}

// NewTask 初始化一个任务的订阅列表
func (h *Hub) NewTask(id string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.Subs[id] = make(map[chan int]struct{})
}

// Subscribe 为指定任务注册一个进度通道
func (h *Hub) Subscribe(id string) chan int {
	ch := make(chan int, cacheSize) // 带缓冲，防止阻塞发布
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, ok := h.Subs[id]; !ok {
		h.Subs[id] = make(map[chan int]struct{})
	}
	h.Subs[id][ch] = struct{}{}
	return ch
}

// Publish 向所有订阅者广播一次进度更新
func (h *Hub) Publish(id string, prog int) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for ch := range h.Subs[id] {
		select {
		case ch <- prog:
		default:
			// 如果通道满了就跳过，以免阻塞
		}
	}
}

// Unsubscribe 和清理订阅者
func (h *Hub) Unsubscribe(id string, ch chan int) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.Subs[id], ch)
	close(ch)
	if len(h.Subs[id]) == 0 {
		delete(h.Subs, id)
	}
}
