package visor

import (
	"fmt"

	"github.com/skycoin/skycoin/src/coin"
	"github.com/skycoin/skycoin/src/visor/historydb"
)

// ParserOption option type which will be used when creating parser instance
type ParserOption func(*BlockchainParser)

// BlockchainParser parses the blockchain and stores the data into historydb.
type BlockchainParser struct {
	historyDB  *historydb.HistoryDB
	parsedFunc func(height uint64) // notify caller the parse process
	blkC       chan coin.Block
	closing    chan chan struct{}
	bc         *Blockchain

	startC  chan struct{}
	isStart bool
}

// NewBlockchainParser create and init the parser instance.
func NewBlockchainParser(hisDB *historydb.HistoryDB, bc *Blockchain, ops ...ParserOption) *BlockchainParser {
	bp := &BlockchainParser{
		bc:        bc,
		historyDB: hisDB,
		closing:   make(chan chan struct{}),
		blkC:      make(chan coin.Block, 10),
		startC:    make(chan struct{}),
	}

	for _, op := range ops {
		op(bp)
	}

	return bp
}

// BlockListener when new block appended to blockchain, this method will b invoked
func (bcp *BlockchainParser) BlockListener(b coin.Block) {
	bcp.blkC <- b
}

// Run starts blockchain parser, the q channel will be
// closed to notify the invoker that the running process
// is going to shutdown.
func (bcp *BlockchainParser) Run(q chan struct{}) {
	logger.Info("Blockchain parser start")

	// parse to the blockchain head
	headSeq := bcp.bc.Head().Head.BkSeq
	if err := bcp.parseTo(headSeq); err != nil {
		logger.Error("%v", err)
		close(q)
		return
	}

	for {
		select {
		case cc := <-bcp.closing:
			cc <- struct{}{}
			return
		case b := <-bcp.blkC:
			if err := bcp.parseTo(b.Head.BkSeq); err != nil {
				logger.Error("%v", err)
				close(q)
				return
			}
		}
	}
}

// Stop close the block parsing process.
func (bcp *BlockchainParser) Stop() {
	cc := make(chan struct{})
	bcp.closing <- cc
	<-cc
	logger.Info("blockchain parser stopped")
}

func (bcp *BlockchainParser) parseTo(bcHeight uint64) error {
	parsedHeight := bcp.historyDB.ParsedHeight()

	for i := int64(0); i < int64(bcHeight)-parsedHeight; i++ {
		b := bcp.bc.GetBlockInDepth(uint64(parsedHeight + i + 1))
		if b == nil {
			return fmt.Errorf("no block exist in depth:%d", parsedHeight+i+1)
		}

		if err := bcp.historyDB.ProcessBlock(b); err != nil {
			return err
		}
	}

	if parsedHeight < int64(bcHeight) {
		logger.Info("parse block from %d to %d", parsedHeight+1, bcHeight)
	}

	return bcp.historyDB.SetParsedHeight(bcHeight)
}

// ParseNotifier sets the callback function that will be invoked
// when block is parsed.
func ParseNotifier(f func(height uint64)) ParserOption {
	return func(p *BlockchainParser) {
		p.parsedFunc = f
	}
}
