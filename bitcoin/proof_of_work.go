package bitcoin

import (
	"math/big"
)

var (
	MaxBits = uint32(0x1d00ffff) // Maximum value of the Bitcoin header bits field

	All256Bits = &big.Int{} // 256 bits, all set
	MaxWork    = &big.Int{} // Max work value allowed
)

func init() {
	All256Bits.SetString("ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff", 16)
	MaxWork.SetString("ffffffffffffffffffffffffffffffffffffffffffffffffffffffff", 16)
}

// ConvertToDifficulty converts from a "target bits" value in a Bitcoin header to a big integer that
// represents the difficulty required in the hash of the header.
func ConvertToDifficulty(bits uint32) *big.Int {
	length := uint8((bits >> 24) & 0xff)

	// Remove leading zero to reverse negative number handling.
	if (bits & 0x00ff0000) == 0 {
		length--
		bits <<= 8
	}

	b := make([]byte, length)
	if length > 0 {
		b[0] = uint8((bits >> 16) & 0xff)
	}
	if length >= 1 {
		b[1] = uint8((bits >> 8) & 0xff)
	}
	if length > 2 {
		b[2] = uint8(bits & 0xff)
	}

	result := &big.Int{}
	result.SetBytes(b)
	return result
}

// ConvertToWork converts a difficulty number into the amount of work required to meet it.
func ConvertToWork(difficulty *big.Int) *big.Int {
	result := &big.Int{}
	result.Xor(All256Bits, difficulty)

	diffPlusOne := &big.Int{}
	diffPlusOne.Add(difficulty, big.NewInt(1))
	result.Div(result, diffPlusOne)

	result.Add(result, big.NewInt(1))
	return result
}

// ConvertToBits converts a big integer to the "target bits" value to be used in a Bitcoin header.
func ConvertToBits(difficulty *big.Int, max uint32) uint32 {
	b := difficulty.Bytes()
	length := uint32(len(b))

	// Read 3 most significant bytes into value.
	var value uint32
	for i := uint32(0); i < uint32(3); i++ {
		value <<= 8
		if i < length {
			value += uint32(b[i])
		}
	}

	// Apply maximum
	maxLength := (max >> 24) & 0xff
	maxValue := max & 0x00ffffff
	if maxLength < length || (maxLength == length && maxValue < value) {
		length = maxLength
		value = maxValue
	}

	// Pad with zero byte to prevent top bit (negative number)
	if value&0x00800000 != 0 {
		length++
		value >>= 8
	}

	result := uint32(length << 24) // set most significant byte to length
	result |= value & 0x00ffffff   // set other 3 bytes to most significant bytes of value
	return result
}
