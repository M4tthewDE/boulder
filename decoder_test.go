package boulder

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDecode(t *testing.T) {
	decoder := NewDecoder()

	filePath := "data/argon_coveragetool_av1_base_and_extended_profiles_v2.1/profile0_core/streams/test10001.obu"
	result := decoder.Decode(filePath)

	assert.Equal(t, 9, len(result.temporalUnits))

	assert.Equal(t, 1, len(result.temporalUnits[0].frameUnits))
	assert.Equal(t, 1, len(result.temporalUnits[1].frameUnits))
	assert.Equal(t, 1, len(result.temporalUnits[2].frameUnits))
	assert.Equal(t, 1, len(result.temporalUnits[3].frameUnits))
	assert.Equal(t, 2, len(result.temporalUnits[4].frameUnits))
	assert.Equal(t, 1, len(result.temporalUnits[5].frameUnits))
	assert.Equal(t, 1, len(result.temporalUnits[6].frameUnits))
	assert.Equal(t, 1, len(result.temporalUnits[7].frameUnits))
	assert.Equal(t, 1, len(result.temporalUnits[8].frameUnits))

	assert.Equal(t, 7, len(result.temporalUnits[0].frameUnits[0].obus))
	assert.Equal(t, 6, len(result.temporalUnits[1].frameUnits[0].obus))
	assert.Equal(t, 3, len(result.temporalUnits[2].frameUnits[0].obus))
	assert.Equal(t, 4, len(result.temporalUnits[3].frameUnits[0].obus))
	assert.Equal(t, 5, len(result.temporalUnits[4].frameUnits[0].obus))
	assert.Equal(t, 2, len(result.temporalUnits[4].frameUnits[1].obus))
	assert.Equal(t, 6, len(result.temporalUnits[5].frameUnits[0].obus))
	assert.Equal(t, 2, len(result.temporalUnits[6].frameUnits[0].obus))
	assert.Equal(t, 4, len(result.temporalUnits[7].frameUnits[0].obus))
	assert.Equal(t, 2, len(result.temporalUnits[8].frameUnits[0].obus))
}
