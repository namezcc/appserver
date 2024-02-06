package util

type ListOrderNode struct {
	Value interface{}
	prev  *ListOrderNode
	next  *ListOrderNode
	list  *ListOrder
}

func (n *ListOrderNode) Remove() {
	n.list.removeNode(n)
	n.list = nil
}

type LessFunc func(a, b interface{}) bool

type ListOrder struct {
	head   ListOrderNode
	tail   ListOrderNode
	size   int
	LessFn LessFunc
}

func NewListOrder(lessFn LessFunc) *ListOrder {
	l := &ListOrder{
		LessFn: lessFn,
	}
	l.head.next = &l.tail
	l.tail.prev = &l.head
	return l
}

func (l *ListOrder) GetFirst() interface{} {
	if l.head.next == nil {
		return nil
	}
	return l.head.next.Value
}

func (l *ListOrder) ToArray() []interface{} {
	arr := make([]interface{}, l.size)
	n := l.head.next
	for i := 0; i < l.size; i++ {
		arr[i] = n.Value
		n = n.next
	}
	return arr
}

func (l *ListOrder) ResetBackOrder(n *ListOrderNode) {
	l.removeNode(n)
	l.insertBefore(&l.tail, n)
	l.sortNode(n)
}

func (l *ListOrder) ResetFrontOrder(n *ListOrderNode) {
	l.removeNode(n)
	l.insertAfter(&l.head, n)
	l.sortNode(n)
}

func (l *ListOrder) insertAfter(at *ListOrderNode, n *ListOrderNode) {
	next := at.next
	at.next = n
	n.prev = at
	n.next = next
	next.prev = n
	l.size++
}

func (l *ListOrder) insertBefore(at *ListOrderNode, n *ListOrderNode) {
	prev := at.prev
	at.prev = n
	n.next = at
	n.prev = prev
	prev.next = n
	l.size++
}

func (l *ListOrder) removeNode(n *ListOrderNode) {
	prev := n.prev
	next := n.next
	prev.next = next
	next.prev = prev
	l.size--
}

func (l *ListOrder) sortNode(n *ListOrderNode) {
	if n.prev == &l.head {
		l.sortNodeToNext(n)
		return
	}
	if n.next == &l.tail {
		l.sortNodeToPrev(n)
		return
	}
	if l.LessFn(n.Value, n.next.Value) {
		l.sortNodeToNext(n)
	} else {
		l.sortNodeToPrev(n)
	}
}

func (l *ListOrder) sortNodeToPrev(n *ListOrderNode) {
	prev := n.prev
	if prev == &l.head {
		return
	}
	for true {
		if l.LessFn(prev.Value, n.Value) {
			prev = prev.prev
			if prev == &l.head {
				l.removeNode(n)
				l.insertAfter(prev, n)
				break
			}
		} else {
			if prev != n.prev {
				l.removeNode(n)
				l.insertAfter(prev, n)
			}
			break
		}
	}
}

func (l *ListOrder) sortNodeToNext(n *ListOrderNode) {
	next := n.next
	if next == &l.tail {
		return
	}
	for true {
		if l.LessFn(n.Value, next.Value) {
			next = next.next
			if next == &l.tail {
				l.removeNode(n)
				l.insertBefore(next, n)
				break
			}
		} else {
			if next != n.next {
				l.removeNode(n)
				l.insertBefore(next, n)
			}
			break
		}
	}
}

func (l *ListOrder) Push(v interface{}) *ListOrderNode {
	n := &ListOrderNode{
		Value: v,
		list:  l,
	}
	l.insertAfter(&l.head, n)
	l.sortNode(n)
	return n
}

func (l *ListOrder) PushBack(v interface{}) *ListOrderNode {
	n := &ListOrderNode{
		Value: v,
		list:  l,
	}
	l.insertBefore(&l.tail, n)
	l.sortNode(n)
	return n
}

func (l *ListOrder) PopFront() interface{} {
	if l.size == 0 {
		return nil
	}
	node := l.head.next
	l.removeNode(node)
	return node.Value
}

func (l *ListOrder) Size() int {
	return l.size
}

func (l *ListOrder) Clear() {
	l.head.next = &l.tail
	l.tail.prev = &l.head
	l.size = 0
}
