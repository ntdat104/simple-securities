package algo

import (
	"reflect"
	"testing"
)

func TestNewLinkedList(t *testing.T) {
	ll := NewLinkedList[int]()

	if ll.GetSize() != 0 {
		t.Errorf("Expected size = 0, got %d", ll.GetSize())
	}

	if ll.head != nil || ll.tail != nil {
		t.Errorf("Expected head and tail to be nil")
	}
}

func TestInsertHead(t *testing.T) {
	ll := NewLinkedList[int]()

	ll.InsertHead(10)
	ll.InsertHead(20)

	expected := []int{20, 10}
	if !reflect.DeepEqual(ll.GetValues(), expected) {
		t.Errorf("Expected %v, got %v", expected, ll.GetValues())
	}

	if ll.GetSize() != 2 {
		t.Errorf("Expected size = 2, got %d", ll.GetSize())
	}
}

func TestInsertTail(t *testing.T) {
	ll := NewLinkedList[int]()

	ll.InsertTail(10)
	ll.InsertTail(20)

	expected := []int{10, 20}
	if !reflect.DeepEqual(ll.GetValues(), expected) {
		t.Errorf("Expected %v, got %v", expected, ll.GetValues())
	}

	if ll.GetSize() != 2 {
		t.Errorf("Expected size = 2, got %d", ll.GetSize())
	}
}

func TestGet(t *testing.T) {
	ll := NewLinkedList[string]()
	ll.InsertTail("A")
	ll.InsertTail("B")
	ll.InsertTail("C")

	val, err := ll.Get(1)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if val != "B" {
		t.Errorf("Expected B, got %s", val)
	}
}

func Test_LinkedList_Get_OutOfBounds(t *testing.T) {
	ll := NewLinkedList[int]()
	ll.InsertTail(1)

	_, err := ll.Get(5)
	if err == nil {
		t.Errorf("Expected error for out of bounds index")
	}
}

func TestRemove_Head(t *testing.T) {
	ll := NewLinkedList[int]()
	ll.InsertTail(1)
	ll.InsertTail(2)
	ll.InsertTail(3)

	ok := ll.Remove(0)
	if !ok {
		t.Errorf("Expected remove success")
	}

	expected := []int{2, 3}
	if !reflect.DeepEqual(ll.GetValues(), expected) {
		t.Errorf("Expected %v, got %v", expected, ll.GetValues())
	}

	if ll.GetSize() != 2 {
		t.Errorf("Expected size = 2, got %d", ll.GetSize())
	}
}

func TestRemove_Tail(t *testing.T) {
	ll := NewLinkedList[int]()
	ll.InsertTail(1)
	ll.InsertTail(2)
	ll.InsertTail(3)

	ok := ll.Remove(2)
	if !ok {
		t.Errorf("Expected remove success")
	}

	expected := []int{1, 2}
	if !reflect.DeepEqual(ll.GetValues(), expected) {
		t.Errorf("Expected %v, got %v", expected, ll.GetValues())
	}

	if ll.GetSize() != 2 {
		t.Errorf("Expected size = 2, got %d", ll.GetSize())
	}
}

func TestRemove_Middle(t *testing.T) {
	ll := NewLinkedList[int]()
	ll.InsertTail(1)
	ll.InsertTail(2)
	ll.InsertTail(3)

	ok := ll.Remove(1)
	if !ok {
		t.Errorf("Expected remove success")
	}

	expected := []int{1, 3}
	if !reflect.DeepEqual(ll.GetValues(), expected) {
		t.Errorf("Expected %v, got %v", expected, ll.GetValues())
	}
}

func TestRemove_OutOfBounds(t *testing.T) {
	ll := NewLinkedList[int]()
	ll.InsertTail(1)

	ok := ll.Remove(5)
	if ok {
		t.Errorf("Expected remove to fail for out of bounds index")
	}
}

func TestLinkedList_GenericType(t *testing.T) {
	type User struct {
		Name string
	}

	ll := NewLinkedList[User]()
	ll.InsertTail(User{Name: "Alice"})
	ll.InsertTail(User{Name: "Bob"})

	values := ll.GetValues()

	if values[0].Name != "Alice" || values[1].Name != "Bob" {
		t.Errorf("Generic type test failed")
	}
}
