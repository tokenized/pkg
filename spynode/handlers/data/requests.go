package data

import (
	"time"

	"github.com/tokenized/pkg/bitcoin"
	"github.com/tokenized/pkg/wire"

	"github.com/pkg/errors"
)

const (
	// Max concurrent block requests
	maxRequestedBlocks = 10

	// Max bytes of pending blocks before requesting another.
	maxPendingBlockSize = 100000000 // 100 MB
)

var (
	ErrWrongPreviousHash = errors.New("Wrong previous hash")
)

type requestedBlock struct {
	hash  bitcoin.Hash32
	time  time.Time // Time request was sent
	block wire.Block
}

func (state *State) BlockIsRequested(hash *bitcoin.Hash32) bool {
	state.lock.Lock()
	defer state.lock.Unlock()

	for _, item := range state.blocksRequested {
		if item.hash == *hash {
			return true
		}
	}
	return false
}

func (state *State) BlockIsToBeRequested(hash *bitcoin.Hash32) bool {
	state.lock.Lock()
	defer state.lock.Unlock()

	for _, item := range state.blocksToRequest {
		if item == *hash {
			return true
		}
	}
	return false
}

// AddBlockRequest adds a block request to the queue.
// Returns true if the request should be made now.
// Returns false if the request is queued for later.
func (state *State) AddBlockRequest(prevHash, hash *bitcoin.Hash32) (bool, error) {
	state.lock.Lock()
	defer state.lock.Unlock()

	h := state.lastHash()
	if !h.Equal(prevHash) {
		return false, ErrWrongPreviousHash
	}

	if len(state.blocksRequested) >= maxRequestedBlocks ||
		state.pendingBlockSize > maxPendingBlockSize {
		state.blocksToRequest = append(state.blocksToRequest, *hash)
		return false, nil
	}

	newRequest := requestedBlock{
		hash:  *hash,
		time:  time.Now(),
		block: nil,
	}

	state.blocksRequested = append(state.blocksRequested, &newRequest)
	return true, nil
}

// AddBlock adds the block message to the queued block request for later processing.
func (state *State) AddBlock(hash *bitcoin.Hash32, block wire.Block) bool {
	state.lock.Lock()
	defer state.lock.Unlock()

	for _, request := range state.blocksRequested {
		if request.hash.Equal(hash) {
			request.block = block
			state.pendingBlockSize += block.SerializeSize()
			return true
		}
	}

	return false
}

func (state *State) NextBlock() wire.Block {
	state.lock.Lock()
	defer state.lock.Unlock()

	if len(state.blocksRequested) == 0 || state.blocksRequested[0].block == nil {
		return nil
	}

	result := state.blocksRequested[0].block
	return result
}

// FinalizeBlock removes a block from block requests when it has been received and is being
// processed.
func (state *State) FinalizeBlock(hash bitcoin.Hash32) error {
	state.lock.Lock()
	defer state.lock.Unlock()

	if len(state.blocksRequested) == 0 || !state.blocksRequested[0].hash.Equal(&hash) {
		return errors.New("Not next block")
	}

	if state.blocksRequested[0].block != nil {
		state.pendingBlockSize -= state.blocksRequested[0].block.SerializeSize()
	}
	state.lastSavedHash = hash
	state.blocksRequested = state.blocksRequested[1:] // Remove first item
	return nil
}

func (state *State) GetNextBlockToRequest() (*bitcoin.Hash32, int) {
	state.lock.Lock()
	defer state.lock.Unlock()

	if len(state.blocksToRequest) == 0 || len(state.blocksRequested) >= maxRequestedBlocks ||
		state.pendingBlockSize > maxPendingBlockSize {
		return nil, -1
	}

	newRequest := requestedBlock{
		hash:  state.blocksToRequest[0],
		time:  time.Now(),
		block: nil,
	}

	state.blocksToRequest = state.blocksToRequest[1:] // Remove first item
	state.blocksRequested = append(state.blocksRequested, &newRequest)
	return &newRequest.hash, len(state.blocksRequested)
}

func (state *State) BlocksRequestedCount() int {
	state.lock.Lock()
	defer state.lock.Unlock()

	return len(state.blocksRequested)
}

func (state *State) BlocksToRequestCount() int {
	state.lock.Lock()
	defer state.lock.Unlock()

	return len(state.blocksToRequest)
}

func (state *State) TotalBlockRequestCount() int {
	state.lock.Lock()
	defer state.lock.Unlock()

	return len(state.blocksRequested) + len(state.blocksToRequest)
}

func (state *State) BlockRequestsEmpty() bool {
	state.lock.Lock()
	defer state.lock.Unlock()

	return len(state.blocksToRequest) == 0 && len(state.blocksRequested) == 0
}

func (state *State) LastHash() bitcoin.Hash32 {
	state.lock.Lock()
	defer state.lock.Unlock()

	return state.lastHash()
}

func (state *State) SetLastHash(hash bitcoin.Hash32) {
	state.lock.Lock()
	defer state.lock.Unlock()

	state.lastSavedHash = hash
}

// lastHash returns the hash of the last block in the state.
func (state *State) lastHash() bitcoin.Hash32 {
	if len(state.blocksToRequest) > 0 {
		return state.blocksToRequest[len(state.blocksToRequest)-1]
	}

	if len(state.blocksRequested) > 0 {
		return state.blocksRequested[len(state.blocksRequested)-1].hash
	}

	return state.lastSavedHash
}
