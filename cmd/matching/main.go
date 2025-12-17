package main

import (
	"container/list"
	"fmt"
	"math/rand"
	"simple-securities/pkg/datetime"
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
			append([]interface{}{datetime.Now().Format("15:04:05.000")}, args...)...)
	}
}

func slow() {
	if SLOW_MODE {
		time.Sleep(SLOW_DELAY)
	}
}

// ================= Basic Types =================

type OrderSide string

const (
	Buy  OrderSide = "BUY"  // Buy OrderSide - Lệnh Mua
	Sell OrderSide = "SELL" // Sell OrderSide - Lệnh Bán
)

type OrderType string

const (
	Limit           OrderType = "LIMIT"             // LIMIT
	Market          OrderType = "MARKET"            // MARKET
	StopLoss        OrderType = "STOP_LOSS"         // STOP_LOSS -> executed as MARKET when triggered
	StopLossLimit   OrderType = "STOP_LOSS_LIMIT"   // STOP_LOSS_LIMIT -> executed as LIMIT when triggered
	TakeProfit      OrderType = "TAKE_PROFIT"       // TAKE_PROFIT -> executed as MARKET when triggered
	TakeProfitLimit OrderType = "TAKE_PROFIT_LIMIT" // TAKE_PROFIT_LIMIT -> executed as LIMIT when triggered
	LimitMaker      OrderType = "LIMIT_MAKER"       // LIMIT_MAKER
)

type OrderStatus string

const (
	New             OrderStatus = "NEW"              // New order created
	Pending         OrderStatus = "PENDING"          // Pending (for stop/take awaiting trigger)
	PartiallyFilled OrderStatus = "PARTIALLY_FILLED" // Partially filled
	Filled          OrderStatus = "FILLED"           // Filled
	Canceled        OrderStatus = "CANCELED"         // Canceled
	PendingCancel   OrderStatus = "PENDING_CANCEL"   // pending cancel (unused)
	Rejected        OrderStatus = "REJECTED"         // Rejected
	Expired         OrderStatus = "EXPIRED"          // Expired
)

// Order represents an order; Quantity is remaining quantity.
type Order struct {
	ID             uint64
	UserID         uint64
	BaseAsset      string
	QuoteAsset     string
	Symbol         string
	Price          int64 // For LIMIT or LIMIT side after trigger
	StopPrice      int64 // For stop/take orders
	Quantity       int64 // Remaining quantity
	OrigQuantity   int64 // Original quantity at creation
	FilledQuantity int64 // Total filled quantity so far
	Side           OrderSide
	Type           OrderType
	Status         OrderStatus
	CreatedAt      time.Time
	UpdatedAt      time.Time
	Node           *list.Element
	PriceLevel     *PriceLevel
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
	// cmp is a comparator used for tree ordering:
	// For bids we use cmp: a > b so best is max (rightmost)
	// For asks we use cmp: a < b so best is min (leftmost)
	// We detect by checking cmp(1,0) (hacky but consistent in this code)
	if t.cmp(1, 0) {
		return maxNode(t.root).PL
	}
	return minNode(t.root).PL
}

// ================= OrderBook =================

type OrderBook struct {
	Bids          *PriceLevelTree
	Asks          *PriceLevelTree
	OrderIndex    map[uint64]*Order
	PendingOrders map[uint64]*Order // for stop/take orders waiting for trigger
}

func NewOrderBook() *OrderBook {
	return &OrderBook{
		Bids:          NewPriceLevelTree(func(a, b int64) bool { return a > b }),
		Asks:          NewPriceLevelTree(func(a, b int64) bool { return a < b }),
		OrderIndex:    make(map[uint64]*Order),
		PendingOrders: make(map[uint64]*Order),
	}
}

func (ob *OrderBook) AddOrder(order *Order) {
	// initialize tracking fields
	order.OrigQuantity = order.Quantity
	order.FilledQuantity = 0
	order.CreatedAt = datetime.Now()
	order.UpdatedAt = datetime.Now()

	audit("NEW ORDER → ID=%d Type=%s Side=%v Price=%d StopPrice=%d Qty=%d Time=%s",
		order.ID, order.Type, order.Side, order.Price, order.StopPrice, order.Quantity,
		order.CreatedAt.Format("15:04:05.000"))

	// Index the order always (so we can cancel even when pending)
	ob.OrderIndex[order.ID] = order
	slow()

	// Handle STOP / TAKE orders -> store as pending until triggered
	switch order.Type {
	case StopLoss, StopLossLimit, TakeProfit, TakeProfitLimit:
		order.Status = Pending
		ob.PendingOrders[order.ID] = order
		audit("PENDING ORDER STORED → ID=%d Type=%s StopPrice=%d", order.ID, order.Type, order.StopPrice)
		return
	}

	// For Market orders we try to match immediately, do not place on book
	if order.Type == Market {
		order.Status = New
		if order.Side == Buy {
			ob.MatchBid(order)
		} else {
			ob.MatchAsk(order)
		}
		// After matching, check status
		if order.Quantity == 0 {
			order.Status = Filled
			audit("INCOMING ORDER FILLED → ID=%d FilledQty=%d", order.ID, order.FilledQuantity)
			delete(ob.OrderIndex, order.ID) // fully executed, remove from index if desired
		} else if order.FilledQuantity > 0 {
			order.Status = PartiallyFilled
			audit("INCOMING ORDER PARTIAL → ID=%d FilledQty=%d Remain=%d", order.ID, order.FilledQuantity, order.Quantity)
		} else {
			// Market not filled at all (no liquidity) -> reject
			order.Status = Rejected
			audit("MARKET ORDER REJECTED (NO LIQUIDITY) → ID=%d", order.ID)
			delete(ob.OrderIndex, order.ID)
		}
		return
	}

	// Limit orders: attempt matching first, then if remaining book them
	order.Status = New
	if order.Side == Buy {
		ob.MatchBid(order)
		if order.Quantity > 0 {
			pl := ob.Bids.GetOrCreate(order.Price)
			order.PriceLevel = pl
			order.Node = pl.Orders.PushBack(order)
			pl.Total += order.Quantity

			audit("BOOKED BID → ID=%d Price=%d Qty=%d LevelTotal=%d",
				order.ID, pl.Price, order.Quantity, pl.Total)
			slow()
		} else {
			// fully matched already
			order.Status = Filled
			audit("LIMIT ORDER FILLED IMMEDIATELY → ID=%d", order.ID)
			delete(ob.OrderIndex, order.ID)
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
		} else {
			order.Status = Filled
			audit("LIMIT ORDER FILLED IMMEDIATELY → ID=%d", order.ID)
			delete(ob.OrderIndex, order.ID)
		}
	}
}

func (ob *OrderBook) MatchBid(order *Order) {
	for {
		best := ob.Asks.Best()
		if best == nil || order.Quantity == 0 {
			return
		}
		// For limit orders enforce price, for market orders skip price check
		if order.Type != Market && best.Price > order.Price {
			return
		}

		for e := best.Orders.Front(); e != nil && order.Quantity > 0; {
			o := e.Value.(*Order) // resting sell order

			tradeQty := min(order.Quantity, o.Quantity)
			tradePrice := best.Price

			audit("TRADE → BuyID=%d SellID=%d Price=%d Qty=%d",
				order.ID, o.ID, tradePrice, tradeQty)
			slow()

			// update buyer (incoming)
			order.Quantity -= tradeQty
			order.FilledQuantity += tradeQty

			// update seller (resting)
			o.Quantity -= tradeQty
			o.FilledQuantity += tradeQty
			best.Total -= tradeQty

			// update statuses
			if o.Quantity == 0 {
				o.Status = Filled
				audit("ORDER FILLED → ID=%d", o.ID)
				best.Orders.Remove(e)
				delete(ob.OrderIndex, o.ID)
				slow()
			} else {
				o.Status = PartiallyFilled
				audit("ORDER PARTIAL → ID=%d Remain=%d", o.ID, o.Quantity)
			}

			if order.Quantity == 0 {
				order.Status = Filled
				audit("INCOMING ORDER FILLED → ID=%d", order.ID)
				// we don't delete incoming from index here (caller may want it) but keep consistent
			} else {
				order.Status = PartiallyFilled
			}

			next := e.Next()
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
		if best == nil || order.Quantity == 0 {
			return
		}
		// For limit orders enforce price, for market orders skip price check
		if order.Type != Market && best.Price < order.Price {
			return
		}

		for e := best.Orders.Front(); e != nil && order.Quantity > 0; {
			o := e.Value.(*Order) // resting buy order

			tradeQty := min(order.Quantity, o.Quantity)
			tradePrice := best.Price

			audit("TRADE → SellID=%d BuyID=%d Price=%d Qty=%d",
				order.ID, o.ID, tradePrice, tradeQty)
			slow()

			// update seller (incoming)
			order.Quantity -= tradeQty
			order.FilledQuantity += tradeQty

			// update buyer (resting)
			o.Quantity -= tradeQty
			o.FilledQuantity += tradeQty
			best.Total -= tradeQty

			// update statuses
			if o.Quantity == 0 {
				o.Status = Filled
				audit("ORDER FILLED → ID=%d", o.ID)
				best.Orders.Remove(e)
				delete(ob.OrderIndex, o.ID)
				slow()
			} else {
				o.Status = PartiallyFilled
				audit("ORDER PARTIAL → ID=%d Remain=%d", o.ID, o.Quantity)
			}

			if order.Quantity == 0 {
				order.Status = Filled
				audit("INCOMING ORDER FILLED → ID=%d", order.ID)
			} else {
				order.Status = PartiallyFilled
			}

			next := e.Next()
			e = next
		}

		if best.Orders.Len() == 0 {
			audit("REMOVE BID LEVEL → Price=%d", best.Price)
			ob.Bids.Remove(best.Price)
			slow()
		}
	}
}

// Cancel cancels an order by ID. Works for both booked and pending orders.
func (ob *OrderBook) Cancel(orderID uint64) bool {
	order, ok := ob.OrderIndex[orderID]
	if !ok {
		audit("CANCEL FAILED → ID=%d NOT FOUND", orderID)
		return false
	}

	// If it's pending, remove from pending map
	if order.Status == Pending {
		delete(ob.PendingOrders, orderID)
		order.Status = Canceled
		delete(ob.OrderIndex, orderID)
		audit("CANCELLED PENDING → ID=%d", orderID)
		return true
	}

	pl := order.PriceLevel
	if pl != nil && order.Node != nil {
		pl.Orders.Remove(order.Node)
		pl.Total -= order.Quantity
		audit("CANCELLED → ID=%d Side=%v Price=%d Qty=%d",
			order.ID, order.Side, order.Price, order.Quantity)
		if pl.Orders.Len() == 0 {
			if order.Side == Buy {
				audit("REMOVE BID LEVEL (CANCEL) → Price=%d", order.Price)
				ob.Bids.Remove(order.Price)
			} else {
				audit("REMOVE ASK LEVEL (CANCEL) → Price=%d", order.Price)
				ob.Asks.Remove(order.Price)
			}
		}
		order.Status = Canceled
		delete(ob.OrderIndex, orderID)
		slow()
		return true
	}

	// If it's not on a price level (could be fully matched or market order remaining), just mark canceled
	order.Status = Canceled
	delete(ob.OrderIndex, orderID)
	audit("CANCELLED (NOT ON BOOK) → ID=%d", orderID)
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
		fmt.Printf("  OrderID=%d Qty=%d Filled=%d Status=%s Time=%s\n",
			o.ID, o.Quantity, o.FilledQuantity, o.Status, o.CreatedAt.Format("15:04:05.000"))
	}
	ob.dumpTree(n.Right)
}

// ================= Pending Trigger Processing =================

// ProcessMarketTick simulates incoming market price ticks and triggers pending stop/take orders.
// Call this method with current market price to evaluate pending orders.
func (ob *OrderBook) ProcessMarketTick(price int64) {
	audit("MARKET TICK → Price=%d PendingCount=%d", price, len(ob.PendingOrders))
	// Collect triggers first to avoid mutating map while iterating
	toTrigger := make([]*Order, 0)
	for _, o := range ob.PendingOrders {
		triggered := false
		switch o.Type {
		case StopLoss:
			// For SELL stop-loss, trigger when price <= StopPrice
			// For BUY stop-loss (stop buy), trigger when price >= StopPrice
			if o.Side == Sell && price <= o.StopPrice {
				triggered = true
			} else if o.Side == Buy && price >= o.StopPrice {
				triggered = true
			}
		case StopLossLimit:
			if o.Side == Sell && price <= o.StopPrice {
				triggered = true
			} else if o.Side == Buy && price >= o.StopPrice {
				triggered = true
			}
		case TakeProfit:
			// For SELL take-profit, trigger when price >= StopPrice
			// For BUY take-profit, trigger when price <= StopPrice
			if o.Side == Sell && price >= o.StopPrice {
				triggered = true
			} else if o.Side == Buy && price <= o.StopPrice {
				triggered = true
			}
		case TakeProfitLimit:
			if o.Side == Sell && price >= o.StopPrice {
				triggered = true
			} else if o.Side == Buy && price <= o.StopPrice {
				triggered = true
			}
		}

		if triggered {
			toTrigger = append(toTrigger, o)
		}
	}

	// Trigger them
	for _, o := range toTrigger {
		delete(ob.PendingOrders, o.ID)
		audit("TRIGGERED PENDING → ID=%d Type=%s Side=%s StopPrice=%d", o.ID, o.Type, o.Side, o.StopPrice)

		// Convert to active order type depending on original type
		switch o.Type {
		case StopLoss, TakeProfit:
			// Trigger as MARKET
			o.Type = Market
			o.Status = New
			// attempt to match immediately
			if o.Side == Buy {
				ob.MatchBid(o)
			} else {
				ob.MatchAsk(o)
			}
			// finalize status
			if o.Quantity == 0 {
				o.Status = Filled
				delete(ob.OrderIndex, o.ID)
				audit("TRIGGERED MARKET ORDER FILLED → ID=%d", o.ID)
			} else if o.FilledQuantity > 0 {
				o.Status = PartiallyFilled
				audit("TRIGGERED MARKET ORDER PARTIAL → ID=%d Remain=%d", o.ID, o.Quantity)
			} else {
				// if couldn't fill (no liquidity), keep as rejected
				o.Status = Rejected
				audit("TRIGGERED MARKET ORDER REJECTED → ID=%d", o.ID)
				delete(ob.OrderIndex, o.ID)
			}
		case StopLossLimit, TakeProfitLimit:
			// Trigger as LIMIT using o.Price (user should have provided a limit price)
			o.Status = New
			// place on book (this will try to match immediately then book remaining)
			if o.Side == Buy {
				ob.MatchBid(o)
				if o.Quantity > 0 {
					pl := ob.Bids.GetOrCreate(o.Price)
					o.PriceLevel = pl
					o.Node = pl.Orders.PushBack(o)
					pl.Total += o.Quantity
					audit("TRIGGERED LIMIT BOOKED BID → ID=%d Price=%d Qty=%d", o.ID, o.Price, o.Quantity)
				} else {
					o.Status = Filled
					delete(ob.OrderIndex, o.ID)
					audit("TRIGGERED LIMIT FILLED → ID=%d", o.ID)
				}
			} else {
				ob.MatchAsk(o)
				if o.Quantity > 0 {
					pl := ob.Asks.GetOrCreate(o.Price)
					o.PriceLevel = pl
					o.Node = pl.Orders.PushBack(o)
					pl.Total += o.Quantity
					audit("TRIGGERED LIMIT BOOKED ASK → ID=%d Price=%d Qty=%d", o.ID, o.Price, o.Quantity)
				} else {
					o.Status = Filled
					delete(ob.OrderIndex, o.ID)
					audit("TRIGGERED LIMIT FILLED → ID=%d", o.ID)
				}
			}
		}
	}
}

// ================= Benchmark / Demo =================

func main() {
	AUDIT = true
	SLOW_MODE = false
	totalOrders := 20

	ob := NewOrderBook()

	r := rand.New(rand.NewSource(datetime.Now().UnixNano()))

	// seed some resting orders to create liquidity
	for i := 0; i < 5; i++ {
		id := uint64(100 + i)
		price := int64(98 + i) // some prices
		qty := int64(3)
		side := Sell
		if i%2 == 0 {
			side = Buy
		}
		ob.AddOrder(&Order{
			ID:        id,
			Price:     price,
			Quantity:  qty,
			Side:      side,
			Type:      Limit,
			CreatedAt: datetime.Now(),
		})
	}

	// create random orders including market and stop types
	for i := 0; i < totalOrders; i++ {
		id := uint64(i + 1)
		price := r.Int63n(5) + 95
		qty := r.Int63n(5) + 1
		side := Buy
		if r.Intn(2) == 0 {
			side = Sell
		}

		// randomly choose type
		var typ OrderType
		switch r.Intn(6) {
		case 0:
			typ = Limit
		case 1:
			typ = Market
		case 2:
			typ = StopLoss
		case 3:
			typ = StopLossLimit
		case 4:
			typ = TakeProfit
		default:
			typ = TakeProfitLimit
		}

		o := &Order{
			ID:        id,
			Price:     price,
			StopPrice: price + int64(r.Intn(4)-2), // small variation for stop
			Quantity:  qty,
			Side:      side,
			Type:      typ,
			CreatedAt: datetime.Now(),
		}

		ob.AddOrder(o)

		// occasionally simulate market ticks to trigger pending orders
		if i%3 == 0 {
			curPrice := int64(96 + r.Int63n(6))
			ob.ProcessMarketTick(curPrice)
		}

		ob.DumpBook()
		fmt.Println()
		time.Sleep(200 * time.Millisecond)
	}

	// final tick to clear pending
	ob.ProcessMarketTick(97)
	ob.DumpBook()
}
