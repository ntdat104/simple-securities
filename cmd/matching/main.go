package main

import (
	"container/heap"
	"context"
	"fmt"
	"math/rand"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

// Simple exchange simulation in Go
// Features:
// - Order gateway receiving orders concurrently
// - Per-symbol matching engine (sharded)
// - In-memory order books (price-time priority)
// - Wallet/reserve checks
// - Event-driven trade publishing
// - Async persistence (simulated)
// - Basic rate-limiting per user

// Run: go run main.go

// ---------- Types ----------

type Side int

const (
	Buy Side = iota
	Sell
)

type Order struct {
	ID        uint64
	User      string
	Symbol    string
	Side      Side
	Price     float64
	Qty       float64
	Timestamp time.Time
}

type Trade struct {
	BuyOrderID  uint64
	SellOrderID uint64
	Price       float64
	Qty         float64
	Symbol      string
	Timestamp   time.Time
}

// ---------- Priority queues for orderbook ----------

type OrderItem struct {
	order *Order
	index int
}

// Buy heap: highest price first, earlier timestamp first
// Sell heap: lowest price first, earlier timestamp first

type OrderHeap struct {
	items []*OrderItem
	isBuy bool
}

func (h OrderHeap) Len() int { return len(h.items) }
func (h OrderHeap) Less(i, j int) bool {
	a := h.items[i].order
	b := h.items[j].order
	if h.isBuy {
		if a.Price == b.Price {
			return a.Timestamp.Before(b.Timestamp)
		}
		return a.Price > b.Price
	}
	if a.Price == b.Price {
		return a.Timestamp.Before(b.Timestamp)
	}
	return a.Price < b.Price
}
func (h OrderHeap) Swap(i, j int) {
	h.items[i], h.items[j] = h.items[j], h.items[i]
	h.items[i].index = i
	h.items[j].index = j
}
func (h *OrderHeap) Push(x interface{}) {
	it := x.(*OrderItem)
	it.index = len(h.items)
	h.items = append(h.items, it)
}
func (h *OrderHeap) Pop() interface{} {
	old := h.items
	n := len(old)
	it := old[n-1]
	old[n-1] = nil
	h.items = old[:n-1]
	return it
}

// ---------- OrderBook ----------

type OrderBook struct {
	symbol string
	buys   *OrderHeap
	sells  *OrderHeap
	lock   sync.Mutex
}

func NewOrderBook(symbol string) *OrderBook {
	b := &OrderHeap{isBuy: true}
	s := &OrderHeap{isBuy: false}
	heap.Init(b)
	heap.Init(s)
	return &OrderBook{
		symbol: symbol,
		buys:   b,
		sells:  s,
	}
}

// ---------- Wallet & Risk ----------

type Wallets struct {
	balances sync.Map // map[user]map[symbol]float64 or map[string]float64
}

func (w *Wallets) Ensure(user string) *sync.Map {
	v, ok := w.balances.Load(user)
	if ok {
		return v.(*sync.Map)
	}
	m := &sync.Map{}
	w.balances.Store(user, m)
	return m
}

func (w *Wallets) GetBalance(user, asset string) float64 {
	m := w.Ensure(user)
	v, ok := m.Load(asset)
	if !ok {
		return 0
	}
	return v.(float64)
}

func (w *Wallets) Add(user, asset string, amt float64) {
	m := w.Ensure(user)
	v, _ := m.LoadOrStore(asset, 0.0)
	m.Store(asset, v.(float64)+amt)
}

func (w *Wallets) Sub(user, asset string, amt float64) bool {
	m := w.Ensure(user)
	v, _ := m.LoadOrStore(asset, 0.0)
	bal := v.(float64)
	if bal < amt {
		return false
	}
	m.Store(asset, bal-amt)
	return true
}

// ---------- Matching Engine ----------

type MatchingEngine struct {
	symbol      string
	book        *OrderBook
	orderCh     chan *Order
	tradeCh     chan *Trade
	persistCh   chan interface{}
	stop        chan struct{}
	wallets     *Wallets
	nextOrderID *uint64
}

func NewMatchingEngine(symbol string, wallets *Wallets, nextID *uint64, persistCh chan interface{}) *MatchingEngine {
	me := &MatchingEngine{
		symbol:      symbol,
		book:        NewOrderBook(symbol),
		orderCh:     make(chan *Order, 10000),
		tradeCh:     make(chan *Trade, 10000),
		persistCh:   persistCh,
		stop:        make(chan struct{}),
		wallets:     wallets,
		nextOrderID: nextID,
	}
	go me.loop()
	go me.tradePublisher()
	return me
}

func (me *MatchingEngine) Submit(o *Order) {
	me.orderCh <- o
}

func (me *MatchingEngine) loop() {
	for {
		select {
		case o := <-me.orderCh:
			me.processOrder(o)
		case <-me.stop:
			return
		}
	}
}

func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func (me *MatchingEngine) processOrder(o *Order) {
	me.book.lock.Lock()
	defer me.book.lock.Unlock()
	if o.Side == Buy {
		// try match with sells
		for me.book.sells.Len() > 0 {
			best := me.book.sells.items[0].order
			if best.Price > o.Price {
				break
			}
			// match
			tradeQty := min(o.Qty, best.Qty)
			trade := &Trade{BuyOrderID: o.ID, SellOrderID: best.ID, Price: best.Price, Qty: tradeQty, Symbol: o.Symbol, Timestamp: time.Now()}
			me.tradeCh <- trade
			// reduce quantities
			o.Qty -= tradeQty
			best.Qty -= tradeQty
			if best.Qty <= 0 {
				heap.Pop(me.book.sells)
			}
			if o.Qty <= 0 {
				break
			}
		}
		if o.Qty > 0 {
			heap.Push(me.book.buys, &OrderItem{order: o})
		}
	} else {
		// Sell side
		for me.book.buys.Len() > 0 {
			best := me.book.buys.items[0].order
			if best.Price < o.Price {
				break
			}
			tradeQty := min(o.Qty, best.Qty)
			trade := &Trade{BuyOrderID: best.ID, SellOrderID: o.ID, Price: best.Price, Qty: tradeQty, Symbol: o.Symbol, Timestamp: time.Now()}
			me.tradeCh <- trade
			o.Qty -= tradeQty
			best.Qty -= tradeQty
			if best.Qty <= 0 {
				heap.Pop(me.book.buys)
			}
			if o.Qty <= 0 {
				break
			}
		}
		if o.Qty > 0 {
			heap.Push(me.book.sells, &OrderItem{order: o})
		}
	}
}

func (me *MatchingEngine) tradePublisher() {
	for t := range me.tradeCh {
		// publish event to wallets and persistence asynchronously
		me.persistCh <- t
		// apply to wallets (simple immediate settlement for simulation)
		// In real systems, settlement writes would be async and idempotent
		// Here we just print
		fmt.Printf("TRADE %s: %0.8f %s @ %0.2f (buyOrder=%d sellOrder=%d)\n", t.Symbol, t.Qty, "BASE", t.Price, t.BuyOrderID, t.SellOrderID)
	}
}

// ---------- Gateway & Rate Limiting ----------

type Gateway struct {
	engines      map[string]*MatchingEngine
	wallets      *Wallets
	persistCh    chan interface{}
	rateWindow   time.Duration
	maxPerWindow int
	userCounts   sync.Map // user -> []time.Time or int counters
	nextID       *uint64
}

func NewGateway(symbols []string) *Gateway {
	wallets := &Wallets{}
	persistCh := make(chan interface{}, 100000)
	var nextID uint64 = 1
	gw := &Gateway{engines: map[string]*MatchingEngine{}, wallets: wallets, persistCh: persistCh, rateWindow: time.Second, maxPerWindow: 100, nextID: &nextID}
	for _, s := range symbols {
		gw.engines[s] = NewMatchingEngine(s, wallets, &nextID, persistCh)
	}
	// start persistence worker
	go persistenceWorker(persistCh)
	return gw
}

func (gw *Gateway) SubmitOrder(user, symbol string, side Side, price, qty float64) error {
	// basic rate limit
	cnt := gw.incUserCounter(user)
	if cnt > gw.maxPerWindow {
		return fmt.Errorf("rate limit exceeded for user %s: %d in window", user, cnt)
	}
	// basic wallet check
	if side == Sell {
		if !gw.wallets.Sub(user, "BASE", qty) {
			return fmt.Errorf("insufficient BASE balance for user %s", user)
		}
	} else {
		// Buy: ensure user has quote funds price*qty
		cost := price * qty
		if !gw.wallets.Sub(user, "QUOTE", cost) {
			return fmt.Errorf("insufficient QUOTE balance for user %s", user)
		}
	}
	id := atomic.AddUint64(gw.nextID, 1)
	o := &Order{ID: id, User: user, Symbol: symbol, Side: side, Price: price, Qty: qty, Timestamp: time.Now()}
	eng, ok := gw.engines[symbol]
	if !ok {
		return fmt.Errorf("unknown symbol %s", symbol)
	}
	eng.Submit(o)
	return nil
}

func (gw *Gateway) incUserCounter(user string) int {
	now := time.Now()
	v, _ := gw.userCounts.LoadOrStore(user, &[]time.Time{})
	arr := v.(*[]time.Time)
	// naive sliding window
	*arr = append(*arr, now)
	// drop old
	cut := now.Add(-gw.rateWindow)
	s := *arr
	i := sort.Search(len(s), func(i int) bool { return s[i].After(cut) || s[i].Equal(cut) })
	s = s[i:]
	*arr = s
	return len(s)
}

// ---------- Persistence Worker ----------

func persistenceWorker(ch chan interface{}) {
	for ev := range ch {
		switch v := ev.(type) {
		case *Trade:
			// simulate async DB write
			fmt.Printf("PERSIST trade: %v\n", v)
		default:
			fmt.Printf("PERSIST unknown: %v\n", v)
		}
		// simulate delay
		time.Sleep(5 * time.Millisecond)
	}
}

// ---------- Simulation & Utilities ----------

func seedBalances(w *Wallets, users []string) {
	for _, u := range users {
		w.Add(u, "BASE", 100)
		w.Add(u, "QUOTE", 100000)
	}
}

func randomUser(users []string) string {
	return users[rand.Intn(len(users))]
}

func main() {
	rand.Seed(time.Now().UnixNano())
	symbols := []string{"BTCUSDT", "ETHUSDT"}
	gw := NewGateway(symbols)
	seedBalances(gw.wallets, []string{"alice", "bob", "charlie", "dave"})

	// simple workload generator
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		users := []string{"alice", "bob", "charlie", "dave"}
		for i := 0; i < 1000; i++ {
			user := randomUser(users)
			sym := symbols[rand.Intn(len(symbols))]
			side := Side(rand.Intn(2))
			price := 50000.0 + float64(rand.Intn(1000))*(1-2*rand.Float64())
			qty := 0.001 + rand.Float64()*0.1
			err := gw.SubmitOrder(user, sym, side, price, qty)
			if err != nil {
				// in production we'd return error to user; here we just print
				fmt.Println("order rejected:", err)
			}
			// small jitter
			time.Sleep(time.Millisecond * time.Duration(1+rand.Intn(5)))
		}
		// stop after batch
		cancel()
	}()

	<-ctx.Done()
	fmt.Println("Simulation finished. Waiting for persistence to flush...")
	// give some time to flush
	time.Sleep(2 * time.Second)
}
