package main

import (
	"container/list"
	"fmt"
	"math/rand"
	"sync/atomic"
	"time"
)

// ================= Basic Types =================

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

type PriceLevel struct {
	Price  int64
	Orders *list.List
	Total  int64
}

// ================= AVL Tree for Price Levels =================

type PriceLevelNode struct {
	PL     *PriceLevel
	Left   *PriceLevelNode
	Right  *PriceLevelNode
	Height int
}

func height(n *PriceLevelNode) int {
	if n == nil {
		return 0
	}
	return n.Height
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func rotateRight(y *PriceLevelNode) *PriceLevelNode {
	x := y.Left
	T2 := x.Right
	x.Right = y
	y.Left = T2
	y.Height = max(height(y.Left), height(y.Right)) + 1
	x.Height = max(height(x.Left), height(x.Right)) + 1
	return x
}

func rotateLeft(x *PriceLevelNode) *PriceLevelNode {
	y := x.Right
	T2 := y.Left
	y.Left = x
	x.Right = T2
	x.Height = max(height(x.Left), height(x.Right)) + 1
	y.Height = max(height(y.Left), height(y.Right)) + 1
	return y
}

func getBalance(n *PriceLevelNode) int {
	if n == nil {
		return 0
	}
	return height(n.Left) - height(n.Right)
}

func insertNode(root *PriceLevelNode, pl *PriceLevel, cmp func(a, b int64) bool) *PriceLevelNode {
	if root == nil {
		return &PriceLevelNode{PL: pl, Height: 1}
	}
	if cmp(pl.Price, root.PL.Price) {
		root.Left = insertNode(root.Left, pl, cmp)
	} else if cmp(root.PL.Price, pl.Price) {
		root.Right = insertNode(root.Right, pl, cmp)
	} else {
		// Price already exists, shouldn't happen in this design
		return root
	}

	root.Height = 1 + max(height(root.Left), height(root.Right))
	balance := getBalance(root)

	// Left Left
	if balance > 1 && cmp(pl.Price, root.Left.PL.Price) {
		return rotateRight(root)
	}
	// Right Right
	if balance < -1 && cmp(root.Right.PL.Price, pl.Price) {
		return rotateLeft(root)
	}
	// Left Right
	if balance > 1 && cmp(root.Left.PL.Price, pl.Price) {
		root.Left = rotateLeft(root.Left)
		return rotateRight(root)
	}
	// Right Left
	if balance < -1 && cmp(pl.Price, root.Right.PL.Price) {
		root.Right = rotateRight(root.Right)
		return rotateLeft(root)
	}

	return root
}

// Find min/max node depending on side
func minNode(n *PriceLevelNode) *PriceLevelNode {
	current := n
	for current.Left != nil {
		current = current.Left
	}
	return current
}

func maxNode(n *PriceLevelNode) *PriceLevelNode {
	current := n
	for current.Right != nil {
		current = current.Right
	}
	return current
}

// Remove node by price
func removeNode(root *PriceLevelNode, price int64, cmp func(a, b int64) bool) *PriceLevelNode {
	if root == nil {
		return nil
	}

	if cmp(price, root.PL.Price) {
		root.Left = removeNode(root.Left, price, cmp)
	} else if cmp(root.PL.Price, price) {
		root.Right = removeNode(root.Right, price, cmp)
	} else {
		// node to delete
		if root.Left == nil || root.Right == nil {
			var temp *PriceLevelNode
			if root.Left != nil {
				temp = root.Left
			} else {
				temp = root.Right
			}
			if temp == nil {
				return nil
			} else {
				root = temp
			}
		} else {
			// get inorder successor
			temp := minNode(root.Right)
			root.PL = temp.PL
			root.Right = removeNode(root.Right, temp.PL.Price, cmp)
		}
	}

	if root == nil {
		return nil
	}

	root.Height = 1 + max(height(root.Left), height(root.Right))
	balance := getBalance(root)

	// Left Left
	if balance > 1 && getBalance(root.Left) >= 0 {
		return rotateRight(root)
	}
	// Left Right
	if balance > 1 && getBalance(root.Left) < 0 {
		root.Left = rotateLeft(root.Left)
		return rotateRight(root)
	}
	// Right Right
	if balance < -1 && getBalance(root.Right) <= 0 {
		return rotateLeft(root)
	}
	// Right Left
	if balance < -1 && getBalance(root.Right) > 0 {
		root.Right = rotateRight(root.Right)
		return rotateLeft(root)
	}
	return root
}

// ================= Price Level Tree Wrapper =================

type PriceLevelTree struct {
	root *PriceLevelNode
	cmp  func(a, b int64) bool
}

func NewPriceLevelTree(cmp func(a, b int64) bool) *PriceLevelTree {
	return &PriceLevelTree{cmp: cmp}
}

func (t *PriceLevelTree) Insert(price int64) *PriceLevel {
	pl := &PriceLevel{Price: price, Orders: list.New()}
	t.root = insertNode(t.root, pl, t.cmp)
	return pl
}

func (t *PriceLevelTree) Remove(price int64) {
	t.root = removeNode(t.root, price, t.cmp)
}

func (t *PriceLevelTree) Best() *PriceLevel {
	if t.root == nil {
		return nil
	}
	if t.cmp == nil {
		return nil
	}
	if t.cmp(1, 0) {
		return minNode(t.root).PL
	}
	return maxNode(t.root).PL
}

// ================= OrderBook =================

type OrderBook struct {
	Bids       *PriceLevelTree
	Asks       *PriceLevelTree
	OrderIndex map[uint64]*Order
}

func NewOrderBook() *OrderBook {
	return &OrderBook{
		Bids:       NewPriceLevelTree(func(a, b int64) bool { return a > b }),
		Asks:       NewPriceLevelTree(func(a, b int64) bool { return a < b }),
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
	for {
		best := ob.Asks.Best()
		if best == nil || best.Price > order.Price || order.Quantity == 0 {
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
			ob.Asks.Remove(best.Price)
		}
	}
}

func (ob *OrderBook) MatchAsk(order *Order) {
	for {
		best := ob.Bids.Best()
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
	pl := order.PriceLevel
	pl.Orders.Remove(order.Node)
	if pl.Orders.Len() == 0 {
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

// ================= Benchmark =================

func main() {
	ob := NewOrderBook()
	orderCh := make(chan *Order, 100_000)
	done := make(chan struct{})
	var totalOrders uint64 = 1_000_000
	var idCounter uint64
	var processed uint64

	go func() {
		for order := range orderCh {
			ob.AddOrder(order)
			atomic.AddUint64(&processed, 1)
		}
		done <- struct{}{}
	}()

	start := time.Now()
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
