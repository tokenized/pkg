package bitcoin

import (
	"fmt"
	"math/big"
	"testing"
)

func Test_ConvertToDifficulty(t *testing.T) {
	tests := []struct {
		bits       uint32
		hex        string // big endian
		resultBits uint32
	}{
		{
			bits:       0x181bc330,
			hex:        "1bc330000000000000000000000000000000000000000000",
			resultBits: 0x181bc330,
		},
		{
			bits:       0x1b0404cb,
			hex:        "0404cb000000000000000000000000000000000000000000000000",
			resultBits: 0x1b0404cb,
		},
		{
			bits:       0x1d00ffff, // max value
			hex:        "ffff0000000000000000000000000000000000000000000000000000",
			resultBits: 0x1d00ffff,
		},
		{
			bits:       0x1e00ffff, // over max value of 0x1d00ffff
			hex:        "ffff000000000000000000000000000000000000000000000000000000",
			resultBits: 0x1d00ffff,
		},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("0x%x", tt.bits), func(t *testing.T) {
			difficulty := ConvertToDifficulty(tt.bits)

			hex := difficulty.Text(16)
			if len(hex)%2 != 0 {
				hex = "0" + hex
			}
			t.Logf("Difficulty : %s", hex)

			if hex != tt.hex {
				t.Errorf("Wrong difficulty : \ngot  %s \nwant %s", hex, tt.hex)
			}

			bits := ConvertToBits(difficulty, MaxBits)
			t.Logf("Bits : %08x", bits)

			if bits != tt.resultBits {
				t.Errorf("Wrong bits : got %08x, want %08x", bits, tt.resultBits)
			}
		})
	}
}

func Test_ChainWork_700001(t *testing.T) {
	work := &big.Int{}
	work.SetString("12f3307cc55513a3054a34b", 16) // Chain work from 700000

	t.Logf("Work : %s", work.Text(16))

	blockWork := ConvertToWork(ConvertToDifficulty(0x1814500c)) // Bits from block 700001
	t.Logf("Block work : %s", blockWork.Text(16))
	t.Logf("Block work decimal : %s", blockWork.Text(10))
	// wantDifficulty := 54128489321.23521

	work.Add(work, blockWork)
	t.Logf("New Work : %s", work.Text(16))

	want := &big.Int{}
	want.SetString("12f331466b11eff3a5015e9", 16) // Chain work from 700001

	if work.Cmp(want) != 0 {
		t.Errorf("Wrong result work : \ngot  %s \nwant %s", work.Text(16), want.Text(16))
	}
}

func Test_ChainWork_722001(t *testing.T) {
	work := &big.Int{}
	work.SetString("13400a9d5c007cfc48a9e0a", 16) // Chain work from 722000

	t.Logf("Work : %s", work.Text(16))

	blockWork := ConvertToWork(ConvertToDifficulty(0x181249b6)) // Bits from block 722001
	t.Logf("Block work : %s", blockWork.Text(16))
	t.Logf("Block work decimal : %s", blockWork.Text(10))
	// wantDifficulty := 54128489321.23521

	work.Add(work, blockWork)
	t.Logf("New Work : %s", work.Text(16))

	want := &big.Int{}
	want.SetString("13400b7d550b34530e8bc2d", 16) // Chain work from 722001

	if work.Cmp(want) != 0 {
		t.Errorf("Wrong result work : \ngot  %s \nwant %s", work.Text(16), want.Text(16))
	}
}
