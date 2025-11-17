package main

import (
	"container/list"
	"fmt"
	"math/rand"
	"sync/atomic"
	"time"
)

type Side int

const (
	Bid Side = iota
	Ask
)

type Order struct {
	ID         uint64
	Price      int64
	Quantity   int64
	Side       Side
	Node       *list.Element
	PriceLevel *PriceLevel
}

// ================= SkipList =================

type SkipListNode struct {
	Price int64
	Level []*SkipListNode
	PL    *PriceLevel
}

type SkipList struct {
	head  *SkipListNode
	level int
	cmp   func(a, b int64) bool
	rnd   *rand.Rand
}

func NewSkipList(cmp func(a, b int64) bool) *SkipList {
	return &SkipList{
		head: &SkipListNode{Level: make([]*SkipListNode, 32)},
		cmp:  cmp,
		rnd:  rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

func (sl *SkipList) randomLevel() int {
	lvl := 1
	for lvl < len(sl.head.Level) && sl.rnd.Int31n(2) == 1 {
		lvl++
	}
	return lvl
}

func (sl *SkipList) Insert(price int64) *PriceLevel {
	update := make([]*SkipListNode, len(sl.head.Level))
	curr := sl.head
	for i := sl.level - 1; i >= 0; i-- {
		for curr.Level[i] != nil && sl.cmp(curr.Level[i].Price, price) {
			curr = curr.Level[i]
		}
		update[i] = curr
	}
	next := curr.Level[0]
	if next != nil && next.Price == price {
		return next.PL
	}
	lvl := sl.randomLevel()
	if lvl > sl.level {
		for i := sl.level; i < lvl; i++ {
			update[i] = sl.head
		}
		sl.level = lvl
	}
	node := &SkipListNode{
		Price: price,
		Level: make([]*SkipListNode, lvl),
		PL:    &PriceLevel{Price: price, Orders: list.New()},
	}
	for i := 0; i < lvl; i++ {
		node.Level[i] = update[i].Level[i]
		update[i].Level[i] = node
	}
	return node.PL
}

func (sl *SkipList) First() *PriceLevel {
	node := sl.head.Level[0]
	if node != nil {
		return node.PL
	}
	return nil
}

func (sl *SkipList) Remove(price int64) {
	update := make([]*SkipListNode, len(sl.head.Level))
	curr := sl.head
	for i := sl.level - 1; i >= 0; i-- {
		for curr.Level[i] != nil && sl.cmp(curr.Level[i].Price, price) {
			curr = curr.Level[i]
		}
		update[i] = curr
	}
	target := curr.Level[0]
	if target != nil && target.Price == price {
		for i := 0; i < len(target.Level); i++ {
			if update[i].Level[i] == target {
				update[i].Level[i] = target.Level[i]
			}
		}
		for sl.level > 1 && sl.head.Level[sl.level-1] == nil {
			sl.level--
		}
	}
}

// ================= PriceLevel =================

type PriceLevel struct {
	Price  int64
	Orders *list.List
	Total  int64
}

// ================= OrderBook =================

type OrderBook struct {
	Bids       *SkipList
	Asks       *SkipList
	OrderIndex map[uint64]*Order
}

func NewOrderBook() *OrderBook {
	return &OrderBook{
		Bids:       NewSkipList(func(a, b int64) bool { return a > b }),
		Asks:       NewSkipList(func(a, b int64) bool { return a < b }),
		OrderIndex: make(map[uint64]*Order),
	}
}

func (ob *OrderBook) AddOrder(order *Order) {
	ob.OrderIndex[order.ID] = order
	if order.Side == Bid {
		ob.MatchBid(order)
		if order.Quantity > 0 {
			pl := ob.Bids.Insert(order.Price)
			order.PriceLevel = pl
			order.Node = pl.Orders.PushBack(order)
			pl.Total += order.Quantity
		}
	} else {
		ob.MatchAsk(order)
		if order.Quantity > 0 {
			pl := ob.Asks.Insert(order.Price)
			order.PriceLevel = pl
			order.Node = pl.Orders.PushBack(order)
			pl.Total += order.Quantity
		}
	}
}

func (ob *OrderBook) MatchBid(order *Order) {
	// Vòng lặp chính: match order mua với các lệnh bán (Ask) liên tục
	for {
		best := ob.Asks.First() // Lấy price level bán rẻ nhất hiện tại
		// Nếu không còn lệnh bán, hoặc giá bán cao hơn giá mua, hoặc order mua đã hết
		if best == nil || best.Price > order.Price || order.Quantity == 0 {
			break // dừng vòng lặp
		}

		// Lặp qua các lệnh bán trong price level rẻ nhất (FIFO)
		for e := best.Orders.Front(); e != nil && order.Quantity > 0; {
			o := e.Value.(*Order) // Lấy từng order bán

			// Tính số lượng khớp = min(số lượng mua còn lại, số lượng bán còn lại)
			tradeQty := min(order.Quantity, o.Quantity)

			// Cập nhật số lượng sau khi khớp
			order.Quantity -= tradeQty // giảm số lượng order mua
			o.Quantity -= tradeQty     // giảm số lượng order bán
			best.Total -= tradeQty     // giảm tổng số lượng price level

			next := e.Next() // lưu node tiếp theo trước khi xóa

			if o.Quantity == 0 {
				// Nếu order bán hết, remove khỏi list và map
				best.Orders.Remove(e)
				delete(ob.OrderIndex, o.ID)
			}

			e = next // di chuyển sang order bán tiếp theo
		}

		// Nếu không còn lệnh bán trong price level, remove price level khỏi tree
		if best.Orders.Len() == 0 {
			ob.Asks.Remove(best.Price)
		}
	}
}

func (ob *OrderBook) MatchAsk(order *Order) {
	for {
		best := ob.Bids.First()
		if best == nil || best.Price < order.Price || order.Quantity == 0 {
			break
		}
		for e := best.Orders.Front(); e != nil && order.Quantity > 0; {
			o := e.Value.(*Order)
			tradeQty := min(order.Quantity, o.Quantity)
			order.Quantity -= tradeQty
			o.Quantity -= tradeQty
			best.Total -= tradeQty
			next := e.Next()
			if o.Quantity == 0 {
				best.Orders.Remove(e)
				delete(ob.OrderIndex, o.ID)
			}
			e = next
		}
		if best.Orders.Len() == 0 {
			ob.Bids.Remove(best.Price)
		}
	}
}

func (ob *OrderBook) Cancel(orderID uint64) bool {
	order, ok := ob.OrderIndex[orderID]
	if !ok {
		return false
	}
	priceLevel := order.PriceLevel
	priceLevel.Orders.Remove(order.Node)
	if priceLevel.Orders.Len() == 0 {
		if order.Side == Bid {
			ob.Bids.Remove(order.Price)
		} else {
			ob.Asks.Remove(order.Price)
		}
	}
	delete(ob.OrderIndex, orderID)
	return true
}

func min(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

// ================= Multi-goroutine Benchmark =================

func main() {
	ob := NewOrderBook()
	orderCh := make(chan *Order, 100_000)
	done := make(chan struct{})
	var totalOrders uint64 = 1_000_000
	var idCounter uint64
	var processed uint64

	// Engine goroutine (single threaded for OrderBook)
	go func() {
		for order := range orderCh {
			ob.AddOrder(order)
			atomic.AddUint64(&processed, 1)
		}
		done <- struct{}{}
	}()

	start := time.Now()
	// 10 concurrent streams
	for i := 0; i < 10; i++ {
		go func() {
			r := rand.New(rand.NewSource(time.Now().UnixNano()))
			for j := 0; j < int(totalOrders/10); j++ {
				id := atomic.AddUint64(&idCounter, 1)
				price := r.Int63n(1000) + 90
				qty := r.Int63n(10) + 1
				side := Bid
				if r.Intn(2) == 0 {
					side = Ask
				}
				orderCh <- &Order{ID: id, Price: price, Quantity: qty, Side: side}
			}
		}()
	}

	// Wait until all orders processed
	for atomic.LoadUint64(&processed) < totalOrders {
		time.Sleep(10 * time.Millisecond)
	}
	close(orderCh)
	<-done

	duration := time.Since(start)
	opsPerSec := float64(totalOrders) / duration.Seconds()
	fmt.Printf("Processed %d orders in %v (~%.2f ops/sec)\n",
		totalOrders, duration, opsPerSec)
}
