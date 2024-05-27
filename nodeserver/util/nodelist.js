class Node {
    constructor(data) {
        this.data = data;
        this.next = null;
    }
}

class LinkedList {
    constructor() {
        this.head = null;
        this.tail = null;
    }

	empty() {
		return this.head == null;
	}
    // 添加元素到链表尾部
    push(data) {
        const newNode = new Node(data);
        if (!this.head) {
            this.head = newNode;
            this.tail = newNode;
        } else {
            this.tail.next = newNode;
            this.tail = newNode;
        }
    }

    // 移除并返回链表头部的元素，实现先入先出
    pop() {
        if (!this.head) return null; // 如果链表为空，返回null
        
        let removedNode = this.head;
        this.head = this.head.next; // 将头指针移动到下一个节点

        // 如果移除的是最后一个节点，更新tail
        if (removedNode === this.tail) {
            this.tail = null;
        }

        return removedNode.data;
    }
}
  
module.exports = {
	LinkedList
}