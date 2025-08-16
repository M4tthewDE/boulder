package boulder

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSet(t *testing.T) {
	arr := make([]int, 10)
	set(0, arr, 10)
	set(1, arr, 11)
	set(-1, arr, -1)
	assert.Equal(t, 10, arr[0])
	assert.Equal(t, 11, arr[1])
	assert.Equal(t, -1, arr[9])
}

func TestSet2d(t *testing.T) {
	arr := make([][]int, 10)
	arr[0] = make([]int, 10)
	arr[9] = make([]int, 10)

	set2d(0, 0, arr, 1)
	set2d(0, -1, arr, 2)
	set2d(-1, 0, arr, 3)
	set2d(-1, -1, arr, 4)
	assert.Equal(t, 1, arr[0][0])
	assert.Equal(t, 2, arr[0][9])
	assert.Equal(t, 3, arr[9][0])
	assert.Equal(t, 4, arr[9][9])
}

func TestGet(t *testing.T) {
	arr := make([]int, 10)
	arr[0] = 1
	arr[1] = 2
	arr[9] = 3
	assert.Equal(t, 1, get(0, arr))
	assert.Equal(t, 2, get(1, arr))
	assert.Equal(t, 3, get(-1, arr))
}

func TestGet2d(t *testing.T) {
	arr := make([][]int, 10)
	arr[0] = make([]int, 10)
	arr[9] = make([]int, 10)

	arr[0][0] = 1
	arr[9][0] = 2
	arr[0][9] = 3
	arr[9][9] = 4
	assert.Equal(t, 1, get2d(0, 0, arr))
	assert.Equal(t, 2, get2d(-1, 0, arr))
	assert.Equal(t, 3, get2d(0, -1, arr))
	assert.Equal(t, 4, get2d(-1, -1, arr))
}
