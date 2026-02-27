# 706. Design HashMap

## Problem Description

Design a HashMap without using any built-in hash table libraries.

Implement the `MyHashMap` class:

- `MyHashMap()` initializes the object with an empty map.
- `void put(int key, int value)` inserts a `(key, value)` pair into the HashMap. If the `key` already exists in the map, update the corresponding `value`.
- `int get(int key)` returns the `value` to which the specified `key` is mapped, or `-1` if this map contains no mapping for the `key`.
- `void remove(key)` removes the `key` and its corresponding `value` if the map contains the mapping for the `key`.

### Example 1:

**Input**
`["MyHashMap", "put", "put", "get", "get", "put", "get", "remove", "get"]`
`[[], [1, 1], [2, 2], [1], [3], [2, 1], [2], [2], [2]]`

**Output**
`[null, null, null, 1, -1, null, 1, null, -1]`

**Explanation**
```go
MyHashMap myHashMap = new MyHashMap();
myHashMap.put(1, 1); // The map is now [[1,1]]
myHashMap.put(2, 2); // The map is now [[1,1], [2,2]]
myHashMap.get(1);    // return 1, The map is now [[1,1], [2,2]]
myHashMap.get(3);    // return -1 (i.e., not found), The map is now [[1,1], [2,2]]
myHashMap.put(2, 1); // The map is now [[1,1], [2,1]] (i.e., update the existing value)
myHashMap.get(2);    // return 1, The map is now [[1,1], [2,1]]
myHashMap.remove(2); // remove the mapping for 2, The map is now [[1,1]]
myHashMap.get(2);    // return -1 (i.e., not found), The map is now [[1,1]]
```

### Constraints:

- `0 <= key, value <= 10^6`
- At most `10^4` calls will be made to `put`, `get`, and `remove`.

## Solution (Golang)

### Approach: Array with Direct Initialization

Since the maximum key is $1,000,000$, we can use an array/slice to store the values. We initialize the array with `-1` to represent the absence of a key.

```go
type MyHashMap struct {
    data []int
}

func Constructor() MyHashMap {
    data := make([]int, 1000001)
    for i := range data {
        data[i] = -1
    }
    return MyHashMap{data: data}
}

func (this *MyHashMap) Put(key int, value int) {
    this.data[key] = value
}

func (this *MyHashMap) Get(key int) int {
    return this.data[key]
}

func (this *MyHashMap) Remove(key int) {
    this.data[key] = -1
}
```

## Explanation

1. **Initialization**: We create a slice of size `1,000,001` to accommodate keys up to `1,000,000`. We fill this slice with `-1` because the problem states that a missing key should return `-1`, and valid values are non-negative.
2. **Put**: To insert or update, we simply assign `this.data[key] = value`.
3. **Get**: We return the value stored at `this.data[key]`. If no value was put there, it will naturally return the `-1` we initialized it with.
4. **Remove**: To "remove" a key, we reset the value at that index back to `-1`.

## Complexity

- **Time Complexity**: 
    - `Constructor`: $O(M)$ where $M$ is the range of keys ($10^6$).
    - `Put`, `Get`, `Remove`: $O(1)$.
- **Space Complexity**: $O(M)$ to store the array for the entire key range.
