// Copyright 2021, Mikhail Vitsen (@porfirion)
package trie

// !!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!
// YOU CAN COPY THIS FILE AND REPLACE ValueType ALIAS TO GET DEFINITELY TYPED TRIE.
// !!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!
//
// Type of value stored in Trie (prepare for generics %))
// Must be nillable (interface or pointer)!
type ValueType = interface{}

// type ValueType = *string
// type ValueType = *CustomStruct

// Sparse radix trie. Create it just as &Trie{} and add required data.
// Also there are some convenience constructors (for example for one line initialization)
// Makes zero allocation on Get and LongestPrefixOf operations and two allocations per Put
//
// type Trie[type ValueType] struct {...}
type Trie struct {
	Prefix   []byte
	Value    ValueType
	Children *[256]*Trie
}

// Convenience method for Put()
func (t *Trie) PutString(prefix string, value ValueType) {
	t.Put([]byte(prefix), value)
}

// Put adds new entry into trie or replaces existing with specified prefix. Prefix can have zero length - associated value would be added into root Trie
//
// WARNING! nil shouldn't be stored as value: you wouldn't be able to find it nor by LongestPrefixOf, nor by Get, nor by Iterate
// If you don't need any value (you need only prefixes) - you can use struct{}{}. See checker.FromStrings
//
// There was some variants of control of replace. Handle it inplace results in dependency on ValueType
// (interfaces and pointers are handled differently). Also there are some incomparable types, that cause panic :(.
// Store separate function like OnReplaceCallback - requires passing it to children. And if you update it in parent -
// it wouldn't be updated in children automatically. Another problem - we can't know whole prefix of value, since
// we don't know our parent!
// With current realization caller of Put knows whole prefix and we shouldn't collect it inside. IMHO, much easier.
//
// Also previously there was check for non zero prefix length. But what to do if it is? Panic? return error?
// Seems to me the best approach is to generalize Trie for storing zero length prefix too.
func (t *Trie) Put(newPrefix []byte, val ValueType) (oldValue ValueType) {
	var curPrefix = t.Prefix
	var ind int
	for ind < len(curPrefix) && ind < len(newPrefix) && curPrefix[ind] == newPrefix[ind] {
		// find common part of current and new prefixes
		ind++
	}

	if ind == len(curPrefix) {
		// reached curPrefix end
		if ind == len(newPrefix) {
			// complete match  of prefixes
			// put or replace current value
			// also for case of zero length prefix
			if t.Value != nil {
				oldValue = t.Value
			}
			t.Value = val
		} else {
			// ind < len(newPrefix) - newPrefix longer than existing
			if len(t.Prefix) == 0 && t.Children == nil && t.Value == nil {
				// case for empty Trie and first insertion
				// insert prefix and value into Trie itself
				t.Prefix = newPrefix
				t.Value = val
			} else {
				// our trie is not empty (we already have value or children)
				// rest of newPrefix would be added into proper child
				oldValue = t.getChildOrCreate(newPrefix[ind]).Put(newPrefix[ind:], val)
			}
		}
	} else {
		// ind < len(curPrefix)

		// cur ****|**
		// new ****|
		// OR
		// cur ****|**
		// new ****|*

		// newPrefix shorted than existing or they diverged.
		// Split current prefix into parts (common part [0:ind] and the rest [ind:])
		// and place all current fields into newChild
		var newChild = &Trie{
			Prefix:   curPrefix[ind:], // take only diverging part of prefix
			Value:    t.Value,
			Children: t.Children,
		}

		// reset current Trie and add newChild
		t.Prefix = curPrefix[:ind] // common part (in worst case - it would be empty slice)
		t.Value = nil              // no value - it's prefix only
		t.Children = &[256]*Trie{} // it would have a child anyway
		t.Children[newChild.Prefix[0]] = newChild

		// what to do with new value?
		if ind == len(newPrefix) {
			// newPrefix equals common part! Current Trie becomes value
			t.Value = val
		} else {
			// newPrefix longer than common part. Rest of newPrefix would be set into proper child
			oldValue = t.getChildOrCreate(newPrefix[ind]).Put(newPrefix[ind:], val)
		}
	}

	return
}

func (t *Trie) getChildOrCreate(ind byte) *Trie {
	if t.Children == nil {
		t.Children = &[256]*Trie{}
		t.Children[ind] = &Trie{}
	} else if t.Children[ind] == nil {
		t.Children[ind] = &Trie{}
	}
	return t.Children[ind]
}

func (t *Trie) TakePrefix(str string) (prefix string, ok bool) {
	_, length, ok := t.LongestPrefixOfString(str)
	if ok {
		return str[:length], true
	}

	return "", false
}

func (t *Trie) LongestPrefixOfString(str string) (value ValueType, prefixLen int, ok bool) {
	return t.LongestPrefixOf([]byte(str))
}

// LongestPrefixOf searches the longest matching prefix in input bytes.
// If input has prefix that matches any stored key
// returns associated value, prefix length, true OR nil, 0, false otherwise
func (t *Trie) LongestPrefixOf(input []byte) (value ValueType, prefixLen int, ok bool) {
	ind := 0
	for ind < len(t.Prefix) && ind < len(input) && t.Prefix[ind] == input[ind] {
		ind++
	}

	if ind < len(t.Prefix) {
		// prefix didn't match It is not this trie or it's child
		return nil, 0, false
	}

	if ind < len(input) && t.Children != nil && t.Children[input[ind]] != nil {
		// continue matching children with next bytes from input. Greedy!
		value, prefixLen, ok = t.Children[input[ind]].LongestPrefixOf(input[ind:])
	}

	if ok {
		// we found something in children!
		prefixLen += len(t.Prefix) // our prefix should be added to children's

		return value, prefixLen, ok
	}

	if t.Value != nil {
		// take our value
		return t.Value, len(t.Prefix), true
	}

	// we have no value. Explicitly return size 0 because we can have prefix, but it doesn't matter
	return nil, 0, false
}

func (t *Trie) GetString(key string) (ValueType, bool) {
	return t.Get([]byte(key))
}

// Get searches for exactly matching key in trie
func (t *Trie) Get(key []byte) (ValueType, bool) {
	ind := 0
	for ind < len(t.Prefix) && ind < len(key) && t.Prefix[ind] == key[ind] {
		ind++
	}

	if ind < len(t.Prefix) {
		// prefix didn't match
		// it is not this trie or it's child
		return nil, false
	}

	if ind < len(key) {
		// not all key bytes matched
		if t.Children != nil && t.Children[key[ind]] != nil {
			// continue matching children with next bytes of key.
			return t.Children[key[ind]].Get(key[ind:])
		}

		// we have no child with such prefix
		return nil, false
	}

	if t.Value == nil {
		// all key matched, but current trie has no value (assuming we have some children with values)
		return nil, false
	}

	return t.Value, true
}

func (t *Trie) SubTrie(mask []byte) (subTrie *Trie, ok bool) {
	var ind = 0
	for ind < len(mask) && ind < len(t.Prefix) && mask[ind] == t.Prefix[ind] {
		ind++
	}

	if ind == len(t.Prefix) {
		if ind == len(mask) {
			// complete match!
			return t, true
		} else {
			// ind < len(mask)
			// something else to match
			if t.Children != nil && t.Children[mask[ind]] != nil {
				return t.Children[mask[ind]].SubTrie(mask[ind:])
			} else {
				// no such child(
				return nil, false
			}
		}
	} else {
		// ind < len(t.Prefix)
		if ind == len(mask) {
			// we matched all musk, but have some more bytes in mask. It's ok
			return t, true
		} else {
			// mask doesn't match current
			return nil, false
		}
	}
}

// Iterate calls callback for each value stored in trie
//
// Not thread safe.
//
// Also prefix's underlying array would change on every call - so you can't rely on it after callback finishes
// (e.g. you should not pass it to another goroutine without copying)
//
// It seems like the only possible iteration order is by key (prefix):
//     0x1, 0x1 0x1, 0x1 0x2, 0x1 0x3, 0x2, 0x2 0x1, 0x2 0x2, etc...
// But it's not guarantied. You shouldn't rely on it!
func (t *Trie) Iterate(callback func(prefix []byte, value ValueType)) {
	t.iterate(make([]byte, 0, 1024), callback)
}

func (t *Trie) iterate(prefix []byte, callback func([]byte, ValueType)) {
	curPrefix := append(prefix[:len(prefix):len(prefix)], t.Prefix...)
	if t.Value != nil {
		callback(curPrefix, t.Value)
	}
	if t.Children != nil {
		for i := range t.Children {
			if t.Children[i] != nil {
				t.Children[i].iterate(curPrefix, callback)
			}
		}
	}
}

func (t *Trie) Count() int {
	if t == nil {
		return 0
	}
	var count = 0
	if t.Value != nil {
		count++
	}
	if t.Children != nil {
		for i := range t.Children {
			if t.Children[i] != nil {
				count += t.Children[i].Count()
			}
		}
	}
	return count
}