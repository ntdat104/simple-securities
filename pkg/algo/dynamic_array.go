package algo

import (
	"errors"
	"fmt"
)

// DynamicArray represents a dynamic array (resizable array) that can store elements of any type T.
type DynamicArray[T any] struct {
	data     []T // Underlying slice to store elements of type T
	size     int // Current number of elements in the array
	capacity int // Current allocated capacity of the underlying slice
}

// NewDynamicArray creates and returns a new DynamicArray with the given initial capacity.
// It takes a type parameter T, allowing it to be generic.
// If the provided capacity is 0 or negative, it defaults to an initial capacity of 1.
func NewDynamicArray[T any](capacity int) *DynamicArray[T] {
	if capacity <= 0 {
		capacity = 1 // Ensure a positive initial capacity
	}
	return &DynamicArray[T]{
		data:     make([]T, capacity), // Initialize the underlying slice with type T
		size:     0,                   // Initially empty
		capacity: capacity,            // Set the initial capacity
	}
}

// Get returns the element at the specified index and an error if the index is out of bounds.
func (da *DynamicArray[T]) Get(i int) (T, error) {
	if i < 0 || i >= da.size {
		// Return the zero value for type T and an error
		var zero T
		return zero, errors.New("index out of bounds when getting element")
	}
	return da.data[i], nil // Return the element and nil (no error)
}

// It returns an error if the index is out of bounds.
func (da *DynamicArray[T]) Set(i int, n T) error {
	if i < 0 || i >= da.size {
		return errors.New("index out of bounds when setting element")
	}
	da.data[i] = n
	return nil // Return nil (no error)
}

// Pushback adds an element 'n' to the end of the dynamic array.
// If the array reaches its capacity, it automatically resizes (doubles capacity)
// before adding the new element.
func (da *DynamicArray[T]) Pushback(n T) {
	if da.size == da.capacity {
		da.resize()
	}
	da.data[da.size] = n
	da.size++
}

// Popback removes and returns the last element of the dynamic array.
// It returns an error if the array is empty.
func (da *DynamicArray[T]) Popback() (T, error) {
	if da.size == 0 {
		var zero T
		return zero, errors.New("array is empty, cannot pop from back")
	}
	da.size--
	val := da.data[da.size] // Get the last element before decrementing size
	// Zero out the reference for GC efficiency
	var zero T
	da.data[da.size] = zero
	return val, nil // Return the element and nil (no error)
}

// resize is an internal helper method that doubles the capacity of the dynamic array.
// It creates a new, larger underlying slice and copies existing elements into it.
func (da *DynamicArray[T]) resize() {
	newCapacity := da.capacity * 2
	if newCapacity == 0 { // Handle edge case if initial capacity was 0 and somehow got here
		newCapacity = 1
	}
	newData := make([]T, newCapacity)
	copy(newData, da.data[:da.size]) // Copy only the active elements
	da.data = newData
	da.capacity = newCapacity
	fmt.Printf("DEBUG: Resized array. New Capacity: %d\n", da.capacity) // For demonstration
}

// GetSize returns the current number of elements in the dynamic array.
func (da *DynamicArray[T]) GetSize() int {
	return da.size
}

// GetCapacity returns the current allocated capacity of the dynamic array.
func (da *DynamicArray[T]) GetCapacity() int {
	return da.capacity
}

// String provides a human-readable representation of the DynamicArray for printing.
func (da *DynamicArray[T]) String() string {
	return fmt.Sprintf("DynamicArray{Data (active): %v, Size: %d, Capacity: %d}", da.data[:da.size], da.size, da.capacity)
}
