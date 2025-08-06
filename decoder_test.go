package boulder

import (
	"testing"
)

func TestDecode(t *testing.T) {
	decoder := NewDecoder()

	filePath := "data/argon_coveragetool_av1_base_and_extended_profiles_v2.1/profile0_core/streams/test10001.obu"
	decoder.Decode(filePath)
}
