package boulder

func set[T any](i int, arr []T, val T) {
	if i < 0 {
		arr[len(arr)+i] = val
	} else {
		arr[i] = val
	}
}

func set2d(i int, j int, arr [][]int, val int) {
	if i < 0 {
		set(j, arr[len(arr)+i], val)
	} else {
		set(j, arr[i], val)
	}
}

func set3d(i int, j int, z int, arr [][][]int, val int) {
	if i < 0 {
		set2d(j, z, arr[len(arr)+i], val)
	} else {
		set2d(j, z, arr[i], val)
	}
}

func get(i int, arr []int) int {
	if i < 0 {
		return arr[len(arr)+i]
	} else {
		return arr[i]
	}
}

func get2d(i int, j int, arr [][]int) int {
	if i < 0 {
		return get(j, arr[len(arr)+i])
	} else {
		return get(j, arr[i])
	}
}

func get3d(i int, j int, z int, arr [][][]int) int {
	if i < 0 {
		return get2d(j, z, arr[len(arr)+i])
	} else {
		return get2d(j, z, arr[i])
	}
}
