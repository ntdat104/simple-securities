# 705. Design HashSet

## Problem Description

Design a HashSet without using any built-in hash table libraries.

Implement `MyHashSet` class:

- `void add(key)` Inserts the value `key` into the HashSet.
- `bool contains(key)` Returns whether the value `key` exists in the HashSet or not.
- `void remove(key)` Removes the value `key` from the HashSet. If `key` does not exist in the HashSet, do nothing.

### Example 1:

**Input:**
`["MyHashSet", "add", "add", "contains", "contains", "add", "contains", "remove", "contains"]`
`[[], [1], [2], [1], [3], [2], [2], [2], [2]]`

**Output:**
`[null, null, null, true, false, null, true, null, false]`

**Explanation:**
```go
MyHashSet myHashSet = new MyHashSet();
myHashSet.add(1);      // set = [1]
myHashSet.add(2);      // set = [1, 2]
myHashSet.contains(1); // return True
myHashSet.contains(3); // return False, (not found)
myHashSet.add(2);      // set = [1, 2]
myHashSet.contains(2); // return True
myHashSet.remove(2);   // set = [1]
myHashSet.contains(2); // return False, (already removed)
```

### Constraints:

- `0 <= key <= 10^6`
- At most `10^4` calls will be made to `add`, `remove`, and `contains`.

## Solution (Golang)

### Approach: Boolean Array (Direct Mapping)

Given the constraints that $0 \le key \le 10^6$ and the number of calls is relatively small, the simplest implementation uses a boolean array.

```go
type MyHashSet struct {
    data []bool
}

func Constructor() MyHashSet {
    return MyHashSet{data: make([]bool, 1000001)}
}

func (this *MyHashSet) Add(key int) {
    this.data[key] = true
}

func (this *MyHashSet) Remove(key int) {
    this.data[key] = false
}

func (this *MyHashSet) Contains(key int) bool {
    return this.data[key]
}
```

## Explanation

1. **Storage**: We use a slice of booleans `data`. The length is set to `1000001` to cover the maximum possible key value of $10^6$.
2. **Operations**:
   - `Add(key)`: We set `data[key]` to `true`.
   - `Remove(key)`: We set `data[key]` to `false`.
   - `Contains(key)`: We return the current boolean value at `data[key]`.
3. **Optimizations**: While this uses $O(N)$ space, it provides $O(1)$ time complexity for all operations. For a more space-efficient approach (especially if the range of keys was larger), one could use a bucket array with chaining.

## Complexity

- **Time Complexity**: $O(1)$ for all operations (`Add`, `Remove`, `Contains`).
- **Space Complexity**: $O(M)$, where $M$ is the maximum possible value of the key ($10^6$).
