package algo

import (
	"errors"
	"fmt"
)

// Node represents a single node in the LinkedList.
// It is generic, holding a value of type T and a pointer to the next Node.
type Node[T any] struct {
	value T
	next  *Node[T]
}

// LinkedList represents the linked list structure.
// It is generic, managing nodes of type T.
type LinkedList[T any] struct {
	head *Node[T] // Pointer to the first node
	tail *Node[T] // Pointer to the last node
	size int      // Current number of elements in the list
}

// NewLinkedList creates and returns a new empty LinkedList.
// It takes a type parameter T, allowing it to be generic.
func NewLinkedList[T any]() *LinkedList[T] {
	return &LinkedList[T]{
		head: nil,
		tail: nil,
		size: 0,
	}
}

// Get returns the element at the specified index and an error if the index is out of bounds.
func (ll *LinkedList[T]) Get(index int) (T, error) {
	if index < 0 || index >= ll.size {
		var zero T // Return the zero value for type T
		return zero, errors.New("index out of bounds when getting element")
	}

	current := ll.head
	for i := 0; i < index; i++ {
		current = current.next
	}
	return current.value, nil // Return the element and nil (no error)
}

// InsertHead adds an element 'val' to the beginning of the linked list.
func (ll *LinkedList[T]) InsertHead(val T) {
	newNode := &Node[T]{value: val, next: nil}
	if ll.head == nil {
		// List is empty, new node is both head and tail
		ll.head = newNode
		ll.tail = newNode
	} else {
		// New node points to the current head, then becomes the new head
		newNode.next = ll.head
		ll.head = newNode
	}
	ll.size++
}

// InsertTail adds an element 'val' to the end of the linked list.
func (ll *LinkedList[T]) InsertTail(val T) {
	newNode := &Node[T]{value: val, next: nil}
	if ll.tail == nil {
		// List is empty, new node is both head and tail
		ll.head = newNode
		ll.tail = newNode
	} else {
		// Current tail points to new node, then new node becomes the new tail
		ll.tail.next = newNode
		ll.tail = newNode
	}
	ll.size++
}

// Remove removes the node at the specified index.
// It returns true if the removal was successful, false otherwise (e.g., index out of bounds).
func (ll *LinkedList[T]) Remove(index int) bool {
	if index < 0 || index >= ll.size {
		return false // Index out of bounds
	}

	if index == 0 {
		// Removing the head node
		ll.head = ll.head.next
		if ll.head == nil {
			// If list becomes empty, tail should also be nil
			ll.tail = nil
		}
	} else {
		// Traverse to the node *before* the one to be removed
		current := ll.head
		for i := 0; i < index-1; i++ {
			current = current.next
		}
		// current is now the node before the target node
		targetNode := current.next
		current.next = targetNode.next // Skip the target node
		if targetNode == ll.tail {
			// If we removed the tail, update the tail pointer
			ll.tail = current
		}
	}
	ll.size--
	return true
}

// GetSize returns the current number of elements in the linked list.
func (ll *LinkedList[T]) GetSize() int {
	return ll.size
}

// GetValues returns all values in the linked list as a slice.
func (ll *LinkedList[T]) GetValues() []T {
	values := make([]T, 0, ll.size) // Pre-allocate slice with capacity
	current := ll.head
	for current != nil {
		values = append(values, current.value)
		current = current.next
	}
	return values
}

// String provides a human-readable representation of the LinkedList for printing.
func (ll *LinkedList[T]) String() string {
	return fmt.Sprintf("LinkedList{Values: %v, Size: %d}", ll.GetValues(), ll.size)
}
