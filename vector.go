package main

type Vector[T any] struct {
	data  []T
	index int
}

func (v *Vector[T]) Insert(i int, item T) {
	if len(v.data) == 0 {
		v.data = make([]T, 8)
	}

	if i >= len(v.data) {
		holder := make([]T, len(v.data)*2)
		copy(holder, v.data)
		v.data = holder
	}

	v.data[i] = item
}

func (v *Vector[T]) Reset() {
	v.index = 0
}

func (v *Vector[T]) At(i int) T {
	return v.data[i]
}

func (v *Vector[T]) Append(item T) {
	v.Insert(v.index, item)
	v.index++
}

func (v *Vector[T]) Pop() T {
	v.index--
	return v.data[v.index]
}

func (v *Vector[T]) List() []T {
	return v.data[:v.index]
}

func (v *Vector[T]) Index() int {
	return v.index
}
