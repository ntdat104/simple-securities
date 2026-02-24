package algo

import (
	"testing"
)

func TestNewDynamicArray_DefaultCapacity(t *testing.T) {
	da := NewDynamicArray[int](1)

	if da.GetCapacity() != 1 {
		t.Errorf("Expected capacity = 1, got %d", da.GetCapacity())
	}

	if da.GetSize() != 0 {
		t.Errorf("Expected size = 0, got %d", da.GetSize())
	}
}

func TestPushbackAndGet(t *testing.T) {
	da := NewDynamicArray[int](1)

	da.Pushback(10)
	da.Pushback(20)

	if da.GetSize() != 2 {
		t.Errorf("Expected size = 2, got %d", da.GetSize())
	}

	val, err := da.Get(1)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if val != 20 {
		t.Errorf("Expected value = 20, got %d", val)
	}
}

func TestResize(t *testing.T) {
	da := NewDynamicArray[int](1)

	da.Pushback(1)
	da.Pushback(2) // should trigger resize

	if da.GetCapacity() != 2 {
		t.Errorf("Expected capacity = 2 after resize, got %d", da.GetCapacity())
	}

	if da.GetSize() != 2 {
		t.Errorf("Expected size = 2, got %d", da.GetSize())
	}
}

func TestSet(t *testing.T) {
	da := NewDynamicArray[string](1)

	da.Pushback("A")
	da.Pushback("B")

	err := da.Set(1, "C")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	val, _ := da.Get(1)
	if val != "C" {
		t.Errorf("Expected value = C, got %s", val)
	}
}

func TestPopback(t *testing.T) {
	da := NewDynamicArray[int](1)

	da.Pushback(100)
	da.Pushback(200)

	val, err := da.Popback()
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if val != 200 {
		t.Errorf("Expected popped value = 200, got %d", val)
	}

	if da.GetSize() != 1 {
		t.Errorf("Expected size = 1 after pop, got %d", da.GetSize())
	}
}

func TestPopback_EmptyArray(t *testing.T) {
	da := NewDynamicArray[int](1)

	_, err := da.Popback()
	if err == nil {
		t.Errorf("Expected error when popping from empty array")
	}
}

func TestGet_OutOfBounds(t *testing.T) {
	da := NewDynamicArray[int](1)
	da.Pushback(1)

	_, err := da.Get(5)
	if err == nil {
		t.Errorf("Expected out of bounds error")
	}
}

func TestSet_OutOfBounds(t *testing.T) {
	da := NewDynamicArray[int](1)

	err := da.Set(1, 10)
	if err == nil {
		t.Errorf("Expected out of bounds error")
	}
}
