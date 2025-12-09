package main

import (
	"container/list"
	"fmt"
	"math/rand"
	"time"
)

var (
	AUDIT      = true                   // full audit on/off
	SLOW_MODE  = true                   // slow motion on/off
	SLOW_DELAY = 200 * time.Millisecond // adjust speed here
)

func audit(format string, args ...interface{}) {
	if AUDIT {
		fmt.Printf("[%s] "+format+"\n",
			append([]interface{}{time.Now().Format("15:04:05.000")}, args...)...)
	}
}

func slow() {
	if SLOW_MODE {
		time.Sleep(SLOW_DELAY)
	}
}

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
	CreatedAt  time.Time
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

func findNode(root *PriceLevelNode, price int64, cmp func(a, b int64) bool) *PriceLevelNode {
	if root == nil {
		return nil
	}
	if price == root.PL.Price {
		return root
	}
	if cmp(price, root.PL.Price) {
		return findNode(root.Left, price, cmp)
	}
	return findNode(root.Right, price, cmp)
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

func (t *PriceLevelTree) GetOrCreate(price int64) *PriceLevel {
	// 1. tìm trước
	node := findNode(t.root, price, t.cmp)
	if node != nil {
		audit("PRICE LEVEL USED → %d", price)
		return node.PL
	}

	// 2. nếu chưa có thì mới tạo
	pl := &PriceLevel{
		Price:  price,
		Orders: list.New(),
	}
	audit("PRICE LEVEL CREATED → %d", price)
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
	audit("NEW ORDER → ID=%d Side=%v Price=%d Qty=%d Time=%s",
		order.ID, order.Side, order.Price, order.Quantity,
		order.CreatedAt.Format("15:04:05.000"))

	ob.OrderIndex[order.ID] = order
	slow()

	if order.Side == Bid {
		ob.MatchBid(order)
		if order.Quantity > 0 {
			pl := ob.Bids.GetOrCreate(order.Price)
			order.PriceLevel = pl
			order.Node = pl.Orders.PushBack(order)
			pl.Total += order.Quantity

			audit("BOOKED BID → ID=%d Price=%d Qty=%d LevelTotal=%d",
				order.ID, pl.Price, order.Quantity, pl.Total)
			slow()
		}
	} else {
		ob.MatchAsk(order)
		if order.Quantity > 0 {
			pl := ob.Asks.GetOrCreate(order.Price)
			order.PriceLevel = pl
			order.Node = pl.Orders.PushBack(order)
			pl.Total += order.Quantity

			audit("BOOKED ASK → ID=%d Price=%d Qty=%d LevelTotal=%d",
				order.ID, pl.Price, order.Quantity, pl.Total)
			slow()
		}
	}
}

func (ob *OrderBook) MatchBid(order *Order) {
	for {
		best := ob.Asks.Best()
		if best == nil || best.Price > order.Price || order.Quantity == 0 {
			return
		}

		for e := best.Orders.Front(); e != nil && order.Quantity > 0; {
			o := e.Value.(*Order)

			tradeQty := min(order.Quantity, o.Quantity)

			audit("TRADE → BuyID=%d SellID=%d Price=%d Qty=%d",
				order.ID, o.ID, best.Price, tradeQty)

			slow()

			order.Quantity -= tradeQty
			o.Quantity -= tradeQty
			best.Total -= tradeQty

			audit("UPDATE → BuyRemain=%d SellRemain=%d LevelTotal=%d",
				order.Quantity, o.Quantity, best.Total)

			next := e.Next()

			if o.Quantity == 0 {
				audit("ORDER FILLED → ID=%d", o.ID)
				best.Orders.Remove(e)
				delete(ob.OrderIndex, o.ID)
				slow()
			}

			e = next
		}

		if best.Orders.Len() == 0 {
			audit("REMOVE ASK LEVEL → Price=%d", best.Price)
			ob.Asks.Remove(best.Price)
			slow()
		}
	}
}

func (ob *OrderBook) MatchAsk(order *Order) {
	for {
		best := ob.Bids.Best()
		if best == nil || best.Price < order.Price || order.Quantity == 0 {
			return
		}

		for e := best.Orders.Front(); e != nil && order.Quantity > 0; {
			o := e.Value.(*Order)

			tradeQty := min(order.Quantity, o.Quantity)

			audit("TRADE → SellID=%d BuyID=%d Price=%d Qty=%d",
				order.ID, o.ID, best.Price, tradeQty)

			slow()

			order.Quantity -= tradeQty
			o.Quantity -= tradeQty
			best.Total -= tradeQty

			audit("UPDATE → SellRemain=%d BuyRemain=%d LevelTotal=%d",
				order.Quantity, o.Quantity, best.Total)

			next := e.Next()

			if o.Quantity == 0 {
				audit("ORDER FILLED → ID=%d", o.ID)
				best.Orders.Remove(e)
				delete(ob.OrderIndex, o.ID)
				slow()
			}

			e = next
		}

		if best.Orders.Len() == 0 {
			audit("REMOVE BID LEVEL → Price=%d", best.Price)
			ob.Bids.Remove(best.Price)
			slow()
		}
	}
}

func (ob *OrderBook) Cancel(orderID uint64) bool {
	order, ok := ob.OrderIndex[orderID]
	if !ok {
		audit("CANCEL FAILED → ID=%d NOT FOUND", orderID)
		return false
	}

	pl := order.PriceLevel
	pl.Orders.Remove(order.Node)

	audit("CANCELLED → ID=%d Side=%v Price=%d Qty=%d",
		order.ID, order.Side, order.Price, order.Quantity)

	if pl.Orders.Len() == 0 {
		if order.Side == Bid {
			audit("REMOVE BID LEVEL (CANCEL) → Price=%d", order.Price)
			ob.Bids.Remove(order.Price)
		} else {
			audit("REMOVE ASK LEVEL (CANCEL) → Price=%d", order.Price)
			ob.Asks.Remove(order.Price)
		}
	}

	delete(ob.OrderIndex, orderID)
	slow()
	return true
}

func min(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

func (ob *OrderBook) DumpBook() {
	fmt.Println("------ ORDER BOOK SNAPSHOT ------")
	fmt.Println("ASKS:")
	ob.dumpTree(ob.Asks.root)
	fmt.Println("BIDS:")
	ob.dumpTree(ob.Bids.root)
	fmt.Println("---------------------------------")
}

func (ob *OrderBook) dumpTree(n *PriceLevelNode) {
	if n == nil {
		return
	}
	ob.dumpTree(n.Left)
	fmt.Printf("Price=%d Total=%d Orders=%d\n",
		n.PL.Price, n.PL.Total, n.PL.Orders.Len())
	for e := n.PL.Orders.Front(); e != nil; e = e.Next() {
		o := e.Value.(*Order)
		fmt.Printf("  OrderID=%d Qty=%d Time=%s\n",
			o.ID, o.Quantity, o.CreatedAt.Format("15:04:05.000"))
	}
	ob.dumpTree(n.Right)
}

// ================= Benchmark =================

func main() {
	AUDIT = true
	SLOW_MODE = true
	totalOrders := 20

	ob := NewOrderBook()

	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	for i := 0; i < totalOrders; i++ {
		id := uint64(i + 1)
		price := r.Int63n(5) + 95
		qty := r.Int63n(5) + 1
		side := Bid
		if r.Intn(2) == 0 {
			side = Ask
		}

		ob.AddOrder(&Order{
			ID:        id,
			Price:     price,
			Quantity:  qty,
			Side:      side,
			CreatedAt: time.Now(),
		})

		ob.DumpBook()
		fmt.Println()
		time.Sleep(500 * time.Millisecond)
	}
}
