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
}
