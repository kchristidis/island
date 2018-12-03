package cmap

// A doubly-linked list node.
type node struct {
	key, val   interface{}
	prev, next *node
}

func removeNode(existingNode *node) {
	prev, next := existingNode.prev, existingNode.next
	prev.next, next.prev = next, prev
}

func addToTail(fakeTail, newNode *node) {
	fakeTail.prev.next = newNode
	newNode.prev, fakeTail.prev = fakeTail.prev, newNode
	newNode.next = fakeTail
}

func moveToTail(fakeTail, existingNode *node) {
	removeNode(existingNode)
	addToTail(fakeTail, existingNode)
}
