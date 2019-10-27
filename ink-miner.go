package main

import (
	"os"
	"crypto/ecdsa"
	"net"
	"log"
	"net/rpc"
	"crypto/elliptic"
	"fmt"
	"crypto/rand"
	mathrand "math/rand"
	"errors"
	"time"
	"encoding/gob"
	"strings"
	"sync"
	"sync/atomic"
	"math/big"
	"crypto/md5"
	"encoding/hex"
//	"bufio"
	"encoding/json"

	"math"
	"strconv"

	//"path"
	"crypto/x509"

	"net/http"
)

const (
	TRANSPARENT string  = "transparent"
	ADD          string  = "ADD"
)
var(
	globalMiner *Miner
)

type GetBlockChainReply struct {
	BlockChain    map[string]*BlockInfo
	LastBlockHash string
}

type MinerInfo struct {
	Address net.Addr
	Key     ecdsa.PublicKey
}

type CanvasSettings struct {
	CanvasXMax uint32
	CanvasYMax uint32
}

type MinerNetSettings struct {
	GenesisBlockHash       string
	MinNumMinerConnections uint8
	InkPerOpBlock          uint32
	InkPerNoOpBlock        uint32
	HeartBeat              uint32
	PoWDifficultyOpBlock   uint8
	PoWDifficultyNoOpBlock uint8
	CanvasSettings         CanvasSettings
}

type Miner struct {
	privKey                 *ecdsa.PrivateKey
	publicKey               ecdsa.PublicKey
	peers                   sync.Map
	peersCount              int64
	laddr                   string
	currentMsgSeqNum        uint64
	setting                 *MinerNetSettings
	sconn                   *rpc.Client
	othersMsgSeqNum         map[string]map[uint64]bool
	blockChain              map[string]*BlockInfo
	unprocessedBlocks 		map[string]*Block
	lastBlockOnLongestChain *BlockInfo
	//map signature to transaction
	curTxns       map[string]Transaction
	hasBlockChain int32
	//true = 1 (not work)
	availableToArtNode int32
	artNodeTxn         string
	artNodeTxnBlockHashToCheck []string
	mutex              *sync.Mutex
}
type BlockInfo struct {
	Hash  string
	Block *Block
	Depth uint64
	//hashes of childBlock
	ChildBlocks []string
	//map public key to string
	//state map[string]State
	InkRemainingMap map[string]uint64
	//sig to Shape
	//shapehash = Op sign to string
	ShapesMap map[string]Path
	//SignaturesMap seen so far
	//sig to bool
	SignaturesMap map[string]bool
}

//actual Block sent in the network
type Block struct {
	PrevHash    string
	Txns        []Transaction
	MinerPubKey ecdsa.PublicKey
	Nonce       uint32
}

type Op struct {
	//ADD for add ShapeHash for delete
	TxnStr string
	Lines  []Line
	Stroke string
	Fill   string
	ShapeSvgString string
}

type Transaction struct {
	Op Op
	//Op Sig
	Sig    ecdsaSignature
	PubKey ecdsa.PublicKey
}

type ecdsaSignature struct {
	R, S *big.Int
}

// Returns true if two Signatures are equal
func (sig ecdsaSignature) equal(other ecdsaSignature) bool {
	return sig.S.Cmp(other.S) == 0 && sig.R.Cmp(other.R) == 0
}

// convert Sig to string
func (sig ecdsaSignature) toString() string {
	return sig.S.String() + sig.R.String()
}

type Peer struct {
	pconn *rpc.Client
}

//include sender information
type BlockMessage struct {
	Block        Block
	SeqNum       uint64
	OriginSender string
	Sender       string
	MinerName    string
}
type TransactionMessage struct {
	Transaction  Transaction
	SeqNum       uint64
	OriginSender string
	Sender       string
}

func handleError(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func (m *Miner) AddPeer(Addr string, Reply *bool) error {
	fmt.Println("add peer", Addr)
	_, ok := m.peers.Load(Addr)
	if !ok {
		pconn, err := rpc.Dial("tcp", Addr)
		fmt.Println("after dial", err)
		if err != nil {
			*Reply = false
			return nil
		}
		m.peers.Store(Addr, &Peer{pconn})
		atomic.AddInt64(&m.peersCount, 1)
	}
	*Reply = true
	return nil
}


type ReturnCanvasReply struct{
	MinerNetSettings MinerNetSettings
	Err bool
}

type ReturnCanvasRequest struct{
	R,S *big.Int
	Hash []byte
}

func (m *Miner) ReturnCanvas(Request ReturnCanvasRequest, Reply *ReturnCanvasReply) error {
	if ecdsa.Verify(&m.publicKey,Request.Hash,Request.R,Request.S){
		Reply.MinerNetSettings = *m.setting
	}else{
		fmt.Println("invalid key")
		Reply.Err = true
	}
	return nil
}

type AddShapeRequest struct {
	ValidateNum    uint8
	ShapeSvgString string
	Fill           string
	Stroke         string
}

type AddShapeReply struct {
	ShapeHash    string
	BlockHash    string
	InkRemaining uint32
	Err          int
	ErrStr       string
}




// - noError [0]
// - DisconnectedError [1]
// - InsufficientInkError [2]
// - InvalidShapeSvgStringError [3]
// - ShapeSvgStringTooLongError [4]
// - ShapeOverlapError [5]
// - OutOfBoundsError[6]
//InvalidBlockHashError[7]
func (m *Miner) AddShape(Arg AddShapeRequest, Reply *AddShapeReply) error {
	canEnter := atomic.CompareAndSwapInt32(&m.availableToArtNode, 1, 0)
	if canEnter == false {
		for canEnter == false {
			time.Sleep(100 * time.Millisecond)
			canEnter = atomic.CompareAndSwapInt32(&m.availableToArtNode, 1, 0)
		}
	}
	defer atomic.AddInt32(&m.availableToArtNode, 1)

	if Arg.Fill == TRANSPARENT && Arg.Stroke == TRANSPARENT {
		Reply.Err = 3
		Reply.ErrStr = Arg.ShapeSvgString
		return nil
	}
	if Arg.Fill == "" || Arg.Stroke == ""{
		Reply.Err = 3
		Reply.ErrStr = Arg.ShapeSvgString
		return nil
	}


	if len(Arg.ShapeSvgString)>128{
		Reply.Err = 4
		Reply.ErrStr = Arg.ShapeSvgString
		return nil
	}
	lines,valid := parseSVG(pubKeyToString(m.publicKey),Arg.ShapeSvgString,0,Point{0,0},make([]Line,0))
	if !valid {
		Reply.Err = 3
		Reply.ErrStr = Arg.ShapeSvgString
		return nil
	}

	if !isPathInBound(lines,m.setting.CanvasSettings){
		Reply.Err = 6
		Reply.ErrStr = Arg.ShapeSvgString
		return nil
	}


	// if the shape is not closed but try to fill return error
	if !isClosed(lines) && Arg.Fill != TRANSPARENT{
		Reply.Err = 3
		Reply.ErrStr = Arg.ShapeSvgString
		return nil
	}

	// check overlap for fill
	if isselfOverlap_filled(lines, Arg.Fill){
		Reply.Err = 3
		Reply.ErrStr = Arg.ShapeSvgString
		return nil
	}

	valid=true
	m.mutex.Lock() 
	for _,v:= range m.lastBlockOnLongestChain.ShapesMap {
		for i :=0; i<= len(lines)-1;i++ {
		
			intercepted := isLineIntersectWithLines(lines[i], v.Lines)
			if intercepted == true {
				valid = false
				break;
			}
		}

		if valid == false{
			break;
		}
		// if the checked shape is not transparent and the path have a point inside , then return error
		overlapped := v.Fill != TRANSPARENT && PointInPoly(lines[0].Point1,pubKeyToString(m.publicKey),v.Lines)
		if overlapped == true {
			valid = false
			break;
		}

		// if the path is not transparent , the shape have have a point inside , then return error
		overlapped = Arg.Fill != TRANSPARENT && PointInPoly(v.Lines[0].Point1,v.Owner,lines)
		if overlapped == true {
            valid = false
			break;
		}
	}
	if valid == false{
		Reply.Err = 5
		Reply.ErrStr = Arg.ShapeSvgString
		m.mutex.Unlock()
		return nil
	}

	useInk :=calculateInk(lines,Arg.Fill,Arg.Stroke)
	myInk:= m.lastBlockOnLongestChain.InkRemainingMap[pubKeyToString(m.publicKey)]
	if useInk > myInk{
		Reply.Err = 2
		Reply.ErrStr = strconv.Itoa(int(myInk))
		m.mutex.Unlock()
		return nil
	}
	for _,v :=range m.curTxns{
		for i :=0; i<= len(lines)-1;i++ {
			intersected:= isLineIntersectWithLines(lines[i], v.Op.Lines)
			if intersected == true {
				Reply.Err = 5
				Reply.ErrStr = Arg.ShapeSvgString
				m.mutex.Unlock()
				return nil
			}
		}

		// if the checked shape is not transparent and the path have a point inside , then return error
		overlapped:= v.Op.Fill != TRANSPARENT && PointInPoly(lines[0].Point1,lines[0].Owner,v.Op.Lines)
		if overlapped == true {
			Reply.Err = 5
			Reply.ErrStr = Arg.ShapeSvgString
			m.mutex.Unlock()
			return nil
		}

		// if the path is not transparent , the shape have have a point inside , then return error
		overlapped= Arg.Fill != TRANSPARENT && PointInPoly(v.Op.Lines[0].Point1,pubKeyToString(v.PubKey),lines)
		if overlapped == true {
			Reply.Err = 5
			Reply.ErrStr = Arg.ShapeSvgString
			m.mutex.Unlock()
			return nil
		}

	}


	///////////////////////////////////////////////////////////////////////
	op := Op{ADD, lines, Arg.Stroke, Arg.Fill,Arg.ShapeSvgString}
	// sign
	opHash := computeOpHash(op)
	r, s, err := ecdsa.Sign(rand.Reader, m.privKey, []byte(opHash))
	if err != nil {
		fmt.Println("sign op error", err)
	}
	// sign
	sig := ecdsaSignature{r, s}
	txn := Transaction{op, sig, m.publicKey}
	lastBlock := m.lastBlockOnLongestChain
	curBlockChainLen := lastBlock.Depth
	depthToStop := curBlockChainLen + 4*uint64(Arg.ValidateNum)

	m.artNodeTxn = sig.toString()
	fmt.Println("start tracking txn",sig.toString())
	fmt.Println("current block hash to check",m.artNodeTxnBlockHashToCheck)
	m.curTxns[sig.toString()] = txn
	m.mutex.Unlock() ///////////////////////////


	m.sendTransaction(txn)

	//send transaction

	//call keep track blockchain -> respond art node
	success,blockHash,inkRemaining:= trackTransaction(depthToStop,m,Arg.ValidateNum)
	if success == true{
		Reply.BlockHash    =blockHash
		Reply.ShapeHash= sig.toString()
		Reply.InkRemaining = uint32(inkRemaining)
	}else{
		Reply.Err = 5
		Reply.ErrStr = Arg.ShapeSvgString
	}
	return nil
}





type GetSvgStringReply struct {
	SvgString string
	Err       int
	ErrStr    string
}

func (m *Miner) GetSvgString(ShapeHash string, Reply *GetSvgStringReply) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	shape ,ok:= m.lastBlockOnLongestChain.ShapesMap[ShapeHash]
	if !ok{
		Reply.Err = 9
		Reply.ErrStr = ShapeHash
	}else{
		Reply.SvgString = constructSVGPath(shape.ShapeSvgString,shape.Stroke,shape.Fill)
	}
	return nil
}

type GetInkReply struct {
	InkAmount uint32
	Err       int
	ErrStr    string
}

func (m *Miner) GetInk(Arg string, Reply *GetInkReply) error {
	m.mutex.Lock()
	inkRemain, _ := m.lastBlockOnLongestChain.InkRemainingMap[pubKeyToString(m.publicKey)]
	Reply.InkAmount = uint32(inkRemain)
	m.mutex.Unlock()
	return nil
}

type DeleteShapeRequest struct {
	ValidateNum uint8
	ShapeHash   string
}

type DeleteShapeReply struct {
	InkAmount uint32
	Err       int
	ErrStr    string
}

// Removes a shape from the canvas.
// Can return the following errors:
// - DisconnectedError
// - ShapeOwnerError

func (m *Miner) DeleteShape(Arg DeleteShapeRequest, Reply *DeleteShapeReply) error {
	canEnter := atomic.CompareAndSwapInt32(&m.availableToArtNode, 1, 0)
	if canEnter == false {
		for canEnter == false {
			time.Sleep(100 * time.Millisecond)
			canEnter = atomic.CompareAndSwapInt32(&m.availableToArtNode, 1, 0)
		}
	}
	defer atomic.AddInt32(&m.availableToArtNode, 1)

	m.mutex.Lock()
	//lastBlock := m.lastBlockOnLongestChain
	op := Op{Arg.ShapeHash, nil, "", "",""}
	// sign
	opHash := computeOpHash(op)
	r, s, err := ecdsa.Sign(rand.Reader, m.privKey, []byte(opHash))
	if err != nil {
		fmt.Println("sign op error", err)
	}
	// sign
	sig := ecdsaSignature{r, s}
	txn := Transaction{op, sig, m.publicKey}
	if !validateTransaction(m.lastBlockOnLongestChain.InkRemainingMap,m.lastBlockOnLongestChain.ShapesMap,m.lastBlockOnLongestChain.SignaturesMap, txn,m,true) {
		fmt.Println("receive invalid transaction from art node")
		Reply.Err = 8
		Reply.ErrStr = Arg.ShapeHash
		m.mutex.Unlock()
		return nil
	}
	curBlockChainLen := m.lastBlockOnLongestChain.Depth
	depthToStop := curBlockChainLen + 4*uint64(Arg.ValidateNum)
	m.artNodeTxn = sig.toString()
	fmt.Println("start tracking txn",sig.toString())
	fmt.Println("current block hash to check",m.artNodeTxnBlockHashToCheck)
	m.curTxns[sig.toString()] = txn
	m.mutex.Unlock()
	m.sendTransaction(txn)

	//send transaction

	//call keep track blockchain -> respond art node

	success,_,inkRemaining := trackTransaction(depthToStop,m,Arg.ValidateNum)
	if success == true{
		Reply.InkAmount = uint32(inkRemaining)
	}else{
		Reply.Err = 8
		Reply.ErrStr = Arg.ShapeHash
	}
	return nil
}
func isPathInBound(lines []Line ,canvas CanvasSettings) bool {
	for i:=0;i<=len(lines)-1;i++{
		if lines[i].Point1.X <0 || lines[i].Point1.Y <0 || lines[i].Point2.X <0 || lines[i].Point2.Y < 0 {
			return false
		}

		if uint32(lines[i].Point1.X)> canvas.CanvasXMax ||
			uint32(lines[i].Point1.Y)> canvas.CanvasYMax ||
			uint32(lines[i].Point2.X)> canvas.CanvasXMax ||
			uint32(lines[i].Point2.Y)> canvas.CanvasYMax {
			return false
		}
	}
	return true

}
func trackTransaction(depthToStop uint64, m *Miner, validationNum uint8 ) (bool,string,uint64) {

	for {
		m.mutex.Lock()
		blockHashToCheck := m.artNodeTxnBlockHashToCheck
		blockChain := m.blockChain
		max:=uint8(0)
		maxHash := ""
		for _, b := range blockHashToCheck {
			fmt.Println("find block contains target transaction",b)
			bInfo := m.blockChain[b]
			a:= findNodeDepth(bInfo,blockChain)
			if a> max{
				max = a
				maxHash = b
			}
		}

		stop:= m.lastBlockOnLongestChain.Depth >= depthToStop
		if max>= validationNum {
			m.artNodeTxn = ""
			m.artNodeTxnBlockHashToCheck = nil
			inkRemaining := m.lastBlockOnLongestChain.InkRemainingMap[pubKeyToString(m.publicKey)]
			m.mutex.Unlock()
			fmt.Println("transaction validated, number block following :",max)
			return true,maxHash,inkRemaining
		}else if stop{
			m.artNodeTxn = ""
			m.artNodeTxnBlockHashToCheck = nil
			m.mutex.Unlock()
			fmt.Println("transaction fail, reach ",m.lastBlockOnLongestChain.Depth)
			return false,"",0
		}else{
			m.mutex.Unlock()
			time.Sleep(500*time.Millisecond)
		}
	}
}

func findNodeDepth(node *BlockInfo, blockChain map[string]*BlockInfo) uint8 {

	if len(node.ChildBlocks) == 0 {
		return 0
	} else {
		max := uint8(0)
		for _,chash:= range node.ChildBlocks{
			block:= blockChain[chash]
			height:= findNodeDepth(block,blockChain)
			if height > max {
				max = height
			}

		}
		return max+1
	}

}

type GetShapesReply struct {
	ShapeHashes []string
	Err         int
	ErrStr      string
}

func (m *Miner) GetShapes(BlockHash string, Reply *GetShapesReply) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	blockInfo,ok:=m.blockChain[BlockHash]
	if !ok{
		Reply.Err = 7
		Reply.ErrStr = BlockHash
	}else{
		hashes := make([]string, 0)
		for _,txn:=range blockInfo.Block.Txns{
			txnHash:= txn.Sig.toString()
			hashes =append(hashes, txnHash)
		}
		Reply.ShapeHashes = hashes
	}
	return nil
}

type GetChildrenReply struct {
	BlockHashes []string
	Err         int
	ErrStr      string
}

func (m *Miner) GetChildren(BlockHash string, Reply *GetChildrenReply) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	blockInfo, ok := m.blockChain[BlockHash]
	if !ok {
		Reply.Err = 7
		Reply.ErrStr = BlockHash
		return nil
	}
	Reply.BlockHashes = blockInfo.ChildBlocks
	return nil

}

type CloseCanvasReply struct {
	InkRemaining uint32
	Err          int
	ErrStr       string
}

func (m *Miner) CloseCanvas(Arg string, Reply *CloseCanvasReply) error {
	m.mutex.Lock()
	inkRemain, _ := m.lastBlockOnLongestChain.InkRemainingMap[pubKeyToString(m.publicKey)]
	Reply.InkRemaining = uint32(inkRemain)
	m.mutex.Unlock()
	return nil

}


func validateBlockAndCreateBlockInfo(block Block,  m* Miner)(bool,BlockInfo,bool){
	containArtNodeTxn := false
	difficulty := m.setting.PoWDifficultyOpBlock
	if len(block.Txns) == 0 {
		difficulty = m.setting.PoWDifficultyNoOpBlock
	}
	expectedStr := strings.Repeat("0", int(difficulty))
	hash := computeBlockHash(block)
	tail := hash[32-difficulty: 32]
	if tail != expectedStr {
		fmt.Println("Invalid Block: Invalid nonce",block)
		return false,BlockInfo{},false
	}



	if v, exist := m.blockChain[block.PrevHash]; v == nil || !exist {
		fmt.Println("Prev-hash does not point to a valid Block in Block chain. Add to unprocessed blocks", m.unprocessedBlocks)
		fmt.Println(block.PrevHash)
		m.unprocessedBlocks[hash] = &block
		return false,BlockInfo{},false
	}
	prevBlockInfo := m.blockChain[block.PrevHash]

	newInkRemaining := copyInkMap(prevBlockInfo.InkRemainingMap)
	newShapes := copyShapeMap(prevBlockInfo.ShapesMap)
	newSignatures := copySignatureMap(prevBlockInfo.SignaturesMap)
	// 3. Validate every operation
	//Check that each operation in the Block has a valid signature (this signature should be generated using the private key and the operation).
	// 3.3 Op with identical signature was not added to the current chain
	//prevBlockInfo := m.blockChain[Msg.Block.PrevHash]
	for _, txn := range block.Txns {
		fmt.Println(txn.Sig.toString())
		pubkeyStr := pubKeyToString(txn.PubKey)
		if validateTransaction(newInkRemaining, newShapes, newSignatures, txn,m,false) {
			if txn.Op.TxnStr == "ADD" {
				inkUsed := calculateInk(txn.Op.Lines, txn.Op.Fill, txn.Op.Stroke)
				newInkRemaining[pubkeyStr] = newInkRemaining[pubkeyStr] - inkUsed
				shapeHash := txn.Sig.toString()
				newShapes[shapeHash] = Path{txn.Op.Lines, txn.Op.Fill, txn.Op.Stroke, txn.Op.ShapeSvgString,pubkeyStr,inkUsed}
			} else {
				fmt.Println(" add Ink in 592")
				newInkRemaining[pubkeyStr] = newInkRemaining[pubkeyStr] + newShapes[txn.Op.TxnStr].Ink
				delete(newShapes, txn.Op.TxnStr)
			}
			newSignatures[txn.Sig.toString()] = true
		} else {
			fmt.Println("invalid transaction")
			return false, BlockInfo{},false
		}
		if txn.Sig.toString() == m.artNodeTxn{
			fmt.Println("in validate block, find target transaction")
			containArtNodeTxn = true
		}

	}

	// 4.2 Grant miner Ink
	var inkEarned uint32
	if len(block.Txns)==0 {
		inkEarned = m.setting.InkPerNoOpBlock
	} else {
		inkEarned = m.setting.InkPerOpBlock
	}
	minerPubKey := pubKeyToString(block.MinerPubKey)
	fmt.Println(" add Ink in 616")
	fmt.Print("add ink",inkEarned," to ",minerPubKey)
	newInkRemaining[minerPubKey] = newInkRemaining[minerPubKey] + uint64(inkEarned)

	// 4.3 Make a new Block info
	newDepth := prevBlockInfo.Depth + 1
	newBlockInfo := BlockInfo{hash, &block, newDepth, make([]string, 0, 0), newInkRemaining, newShapes, newSignatures}
	// 4.4 Add blockinfo to blockChain, and add to children of prev-hash
	return true,newBlockInfo,containArtNodeTxn

}


func validateTransaction(newInkRemaining map[string]uint64, newShapes map[string]Path, newSignatures map[string]bool, txn Transaction,m *Miner, checkCurTxn bool) bool{
	pubkeyStr := pubKeyToString(txn.PubKey)
	// 3.1 Check signature
	txnOpHash := computeOpHash(txn.Op)
	if !ecdsa.Verify(&txn.PubKey, []byte(txnOpHash), txn.Sig.R, txn.Sig.S) {
		fmt.Println("Invalid Operation: Bad signature", txn)
		return false
	}
	_, ok := newSignatures[txn.Sig.toString()]
	if ok {
		fmt.Println("Attempting to add operation that existed in the Block chain", txn.Sig.toString())
		return false
	}
	if txn.Op.TxnStr == "ADD" {
		if txn.Op.Fill == TRANSPARENT && txn.Op.Stroke == TRANSPARENT {
			fmt.Println("Both fill and stroke are transparent.")
			return false
		}
		if txn.Op.Fill == "" || txn.Op.Stroke == ""{
			fmt.Println("One of fill and stroke is empty.")
			return false
		}

		lines := txn.Op.Lines
		if !isPathInBound(lines,m.setting.CanvasSettings){
			fmt.Println("It is outbound")
			return false
		}
		// if the shape is not closed but try to fill return error
		if !isClosed(lines) && txn.Op.Fill != TRANSPARENT{
			fmt.Println("The shape is not closed but try to fill")
			return false
		}
		// check overlap for fill
		if isselfOverlap_filled(lines,txn.Op.Fill){
			fmt.Println("check overlap for fill")
			return false
		}

		for _,v:= range newShapes {
			for i :=0; i<= len(lines)-1;i++ {
				intercepted := isLineIntersectWithLines(lines[i], v.Lines)
				if intercepted == true {
					fmt.Println(" Line", lines[i]," are intercepted with, " ,v.Lines)
					return false
				}
			}

			///////////////////////////////////////////////////////////contain
			// if the checked shape is not transparent and the path have a point inside , then return error
			overlapped := v.Fill != TRANSPARENT && PointInPoly(lines[0].Point1,lines[0].Owner,v.Lines)
			if overlapped == true {
				fmt.Println(lines," inside " ,v)
				return false
			}

			// if the path is not transparent , the shape have have a point inside , then return error
			overlapped = txn.Op.Fill != TRANSPARENT && PointInPoly(v.Lines[0].Point1,v.Owner,lines)
			if overlapped == true {
				fmt.Println(v," inside " ,lines)
				return false
			}

		}

		useInk :=calculateInk(lines, txn.Op.Fill, txn.Op.Stroke)
		myInk:= newInkRemaining[pubkeyStr]
		if useInk > myInk{
			fmt.Println("does not have enough ink, need",useInk,"has",myInk)
			return false
		}

		if checkCurTxn{
			for k,v :=range m.curTxns{
				if txn.Sig.toString()== k{
					return false
				}
				for i :=0; i<= len(lines)-1;i++ {
					intersected:= isLineIntersectWithLines(lines[i], v.Op.Lines)
					if intersected == true {
						return false
					}
				}

				// if the checked shape is not transparent and the path have a point inside , then return error
				overlapped:= v.Op.Fill != TRANSPARENT && PointInPoly(lines[0].Point1,lines[0].Owner,v.Op.Lines)
				if overlapped == true {
					return false
				}

				// if the path is not transparent , the shape have have a point inside , then return error
				overlapped= txn.Op.Fill != TRANSPARENT && PointInPoly(v.Op.Lines[0].Point1,pubKeyToString(v.PubKey),lines)
				if overlapped == true {
					return false
				}

			}

		}


	} else {
		_, ok := m.curTxns[txn.Op.TxnStr]
		if checkCurTxn && ok {
			fmt.Println("Delete shape error. Op in current transactions")
			return false
		}
		//txn.Op for delete is the shapehash
		shape, ok :=newShapes[txn.Op.TxnStr]
		if !ok {
			fmt.Println("Delete Shape error. Shape doest not exist!")
			return false
		}
		if shape.Owner != pubkeyStr {
			fmt.Println("Delete Shape error. Shape doest not belong to public key ", pubkeyStr)
			return false
		}

	}
	return true
}

func findUnprocessedBlocks(prevHash string, m *Miner) []*Block {
	blocksToAdd := make([]*Block,0, 0)
	for unprocessedBlockHash, unprocessedBlock := range m.unprocessedBlocks {
		if unprocessedBlock.PrevHash == prevHash {
			blocksToAdd = append(blocksToAdd, unprocessedBlock)
			blocksToAdd = append(blocksToAdd, findUnprocessedBlocks(unprocessedBlockHash, m)...)
		}
	}
	return blocksToAdd
}

func (m *Miner) addToBlockChain(block Block) error {
	valid, newBlockInfo,containArtNodeTxn := validateBlockAndCreateBlockInfo(block,m)
	if !valid{
		fmt.Println("Receive invalid block ,and drop it")
		return errors.New("Receive invalid block ,and drop it")
	}
	if containArtNodeTxn{
		fmt.Println("broadcast find target transaction")
		m.artNodeTxnBlockHashToCheck = append(m.artNodeTxnBlockHashToCheck, newBlockInfo.Hash)
	}
	// 2. prev-hash points to a valid Block in the Block chain
	//Check that the previous Block hash points to a legal, previously generated, Block.


	m.blockChain[newBlockInfo.Hash] = &newBlockInfo
	prevBlockInfo := m.blockChain[newBlockInfo.Block.PrevHash]
	if len(prevBlockInfo.ChildBlocks) > 0 {
		fmt.Println("fork, append child to ", prevBlockInfo.Hash)
	}
	prevBlockInfo.ChildBlocks = append(prevBlockInfo.ChildBlocks,newBlockInfo.Hash)
	//printMinerBlockChain(m, "Receive")
	// 4.5 If this is the longest Block chain, update lastBlockOnLongestChain, and stop working on txns addressed by this Block

	randNum := mathrand.Int() % 2

	//switch last Block on longest chain if newDepth is larger
	///////////////////////////////////////////////////////////////////////////////////////////////////////////////////
	if newBlockInfo.Depth > m.lastBlockOnLongestChain.Depth {
		for k,txn:=range m.curTxns {
			if !validateTransaction(newBlockInfo.InkRemainingMap, newBlockInfo.ShapesMap, newBlockInfo.SignaturesMap,txn,m,false ){
				fmt.Println("delete txn",txn,"from current txn")
				delete(m.curTxns, k)
			}
		}
		m.lastBlockOnLongestChain = &newBlockInfo
		fmt.Println("broadcast change lastblock to ",newBlockInfo.Hash)
	} else if newBlockInfo.Depth == m.lastBlockOnLongestChain.Depth && randNum == 0 {
		fmt.Println("switch")
		//if newDepth equal current Depth, switch on random
		newCurTxns := updateCurTxnOnSwitch(&newBlockInfo, m.lastBlockOnLongestChain.Hash, m)
		for k,txn:=range newCurTxns {
			if !validateTransaction(newBlockInfo.InkRemainingMap, newBlockInfo.ShapesMap, newBlockInfo.SignaturesMap,txn,m,false ){
				fmt.Println("on swithch delete txn",txn,"from current txn")
				delete(newCurTxns, k)
			}
		}
		m.curTxns = newCurTxns
		fmt.Println("new cur txn")
		fmt.Println(newCurTxns)
		m.lastBlockOnLongestChain = &newBlockInfo
		fmt.Println("broadcast change lastblock to ",newBlockInfo.Hash)
	}
	return nil
}

func (m *Miner) BroadcastBlock(Msg BlockMessage, Reply *bool) error {
	//time.Sleep(5 * time.Second)
	canEnter := atomic.LoadInt32(&m.hasBlockChain)
	if canEnter == 0 {
		for canEnter == 0 {
			time.Sleep(100 * time.Millisecond)
			canEnter = atomic.LoadInt32(&m.hasBlockChain)
		}
	}
	fmt.Println(m.laddr, "receive block", Msg)
	_, ok := m.peers.Load(Msg.Sender)
	if !ok {
		fmt.Println("peer does not exist")
		return errors.New("peer does not exist")
	}


	m.mutex.Lock()
	seqNumMap, ok := m.othersMsgSeqNum[Msg.OriginSender]
	if ok && seqNumMap[Msg.SeqNum]{
		//fmt.Println("receive message seq number :",Msg.SeqNum)
		//fmt.Println("current message seq number :",m.othersMsgSeqNum[Msg.OriginSender])
		fmt.Println("receive this message before")
		m.mutex.Unlock()
		return nil
	}
	hash := computeBlockHash(Msg.Block)

	fmt.Println("receive block hash",hash)
	_,ok = m.blockChain[hash]
	_, isUnprocessedBlock := m.unprocessedBlocks[hash]
	if ok || isUnprocessedBlock {
		fmt.Println("receive this block before, ignore")
		m.mutex.Unlock()
		return nil
	}

	err := m.addToBlockChain(Msg.Block)
	fmt.Println("add to block chain err",err)
	if err == nil {
		blocksToAdd := findUnprocessedBlocks(hash, m)
		for _, newBlock := range blocksToAdd {
			m.addToBlockChain(*newBlock)
			delete(m.unprocessedBlocks, computeBlockHash(*newBlock))
		}
	}

	fmt.Println("update ",Msg.OriginSender,"seq number to ",Msg.SeqNum)
	_,ok =m.othersMsgSeqNum[Msg.OriginSender]
	if !ok{
		m.othersMsgSeqNum[Msg.OriginSender] = make(map[uint64]bool)
	}
	m.othersMsgSeqNum[Msg.OriginSender][Msg.SeqNum] = true

	m.mutex.Unlock()

	m.peers.Range(func(paddr, value interface{}) bool {
		peer, ok := value.(*Peer)
		if !ok {
			fmt.Println("can not retrive peer")
		}
		if paddr != Msg.OriginSender && paddr != Msg.Sender {
			newMsg := BlockMessage{Msg.Block, Msg.SeqNum, Msg.OriginSender, m.laddr,"1"}
			reply := false
			fmt.Println("forward message to ", paddr)
			err := peer.pconn.Call("Miner.BroadcastBlock", newMsg, &reply)
			fmt.Println("after forward", err)
			if err != nil {
				peer.pconn.Close()
				m.peers.Delete(paddr)
				atomic.AddInt64(&m.peersCount, -1)

			}
		}
		return true
	})

	return nil
}

func (m *Miner) GetBlockchain(minerAddr string, reply *GetBlockChainReply) error {
	fmt.Println(m.laddr, "GetBlockChain", minerAddr)

	_, ok := m.peers.Load(minerAddr)

	if !ok {
		fmt.Println("peer does not exist")
		return errors.New("peer does not exist")
	}

	m.mutex.Lock()
	reply.BlockChain = m.blockChain
	reply.LastBlockHash = m.lastBlockOnLongestChain.Hash
	m.mutex.Unlock()
	fmt.Println("get blockchain return ")
	return nil
}

func (m *Miner) BroadcastTransaction(Msg TransactionMessage, Reply *bool) error {
	canEnter := atomic.LoadInt32(&m.hasBlockChain)
	if canEnter == 0 {
		for canEnter == 0 {
			time.Sleep(100 * time.Millisecond)
			canEnter = atomic.LoadInt32(&m.hasBlockChain)
		}
	}

	fmt.Println(m.laddr, "receive transaction", Msg)
	_, ok := m.peers.Load(Msg.Sender)
	if !ok {
		fmt.Println("peer does not exist")
		return errors.New("peer does not exist")
	}


	m.mutex.Lock()
	seqNumMap, ok := m.othersMsgSeqNum[Msg.OriginSender]
	if ok && seqNumMap[Msg.SeqNum]{
		fmt.Println("receive message seq number :",Msg.SeqNum)
		fmt.Println("current message seq number :",m.othersMsgSeqNum[Msg.OriginSender])
		fmt.Println("receive this message before")
		m.mutex.Unlock()
		return nil
	}

	//validate transaction
	txn := Msg.Transaction
	_,ok = m.curTxns[txn.Sig.toString()]
	fmt.Println("validating transaction")
	if ok{
		fmt.Println("receive this transaction before")
		m.mutex.Unlock()
		return nil
	}
	_, ok = m.lastBlockOnLongestChain.SignaturesMap[txn.Sig.toString()]
	if ok {
		fmt.Println("Attempting to add operation that existed in the Block chain", txn.Sig.toString())
		m.mutex.Unlock()
		return nil
	}
	if validateTransaction(m.lastBlockOnLongestChain.InkRemainingMap,m.lastBlockOnLongestChain.ShapesMap,m.lastBlockOnLongestChain.SignaturesMap, txn,m,true) {
		m.curTxns[txn.Sig.toString()] = txn
	}else{
		fmt.Println("receive invalid transaction, do not add to cur txn")
	}


	fmt.Println("update ",Msg.OriginSender,"seq number to ",Msg.SeqNum)
	_,ok =m.othersMsgSeqNum[Msg.OriginSender]
	if !ok{
		m.othersMsgSeqNum[Msg.OriginSender] = make(map[uint64]bool)
	}
	m.othersMsgSeqNum[Msg.OriginSender][Msg.SeqNum] = true
	m.mutex.Unlock()

	m.peers.Range(func(paddr, value interface{}) bool {
		peer, ok := value.(*Peer)
		if !ok {
			fmt.Println("can not retrive peer")
		}
		if paddr != Msg.OriginSender && paddr != Msg.Sender {
			newMsg := TransactionMessage{Msg.Transaction, Msg.SeqNum, Msg.OriginSender, m.laddr}
			reply := false
			fmt.Println("forward message to ", paddr)
			err := peer.pconn.Call("Miner.BroadcastTransaction", newMsg, &reply)
			fmt.Println("after forward", err)
			if err != nil {
				peer.pconn.Close()
				m.peers.Delete(paddr)
				atomic.AddInt64(&m.peersCount, -1)
			}
		}
		return true
	})

	return nil
}



func (m *Miner) sendTransaction(msg Transaction) {

	m.mutex.Lock()
	m.currentMsgSeqNum = m.currentMsgSeqNum + 1
	newMsg := TransactionMessage{msg, m.currentMsgSeqNum, m.laddr, m.laddr}
	m.mutex.Unlock()

	m.peers.Range(func(paddr, value interface{}) bool {
		peer, ok := value.(*Peer)
		if !ok {
			fmt.Println("can not retrive peer")
		}
		reply := false
		fmt.Println("send transaction to", paddr)
		err := peer.pconn.Call("Miner.BroadcastTransaction", newMsg, &reply)
		fmt.Println("after sending transaction", err)
		if err != nil {
			peer.pconn.Close()
			m.peers.Delete(paddr)
			atomic.AddInt64(&m.peersCount, -1)
		}
		return true
	})


}

func (m *Miner) sendBlock(msg Block) {
	fmt.Println("send Block")
	fmt.Println(msg)
	m.mutex.Lock()
	m.currentMsgSeqNum = m.currentMsgSeqNum + 1
	newMsg := BlockMessage{msg, m.currentMsgSeqNum, m.laddr, m.laddr,"Miner1"}
	m.mutex.Unlock()
	m.peers.Range(func(paddr, value interface{}) bool {
		peer, ok := value.(*Peer)
		if !ok {
			fmt.Println("can not retrive peer")
		}
		reply := false
		fmt.Println("send to", paddr)
		err := peer.pconn.Call("Miner.BroadcastBlock", newMsg, &reply)
		fmt.Println("after send", err)
		if err != nil {
			peer.pconn.Close()
			m.peers.Delete(paddr)
			atomic.AddInt64(&m.peersCount, -1)
		}
		return true
	})

}

func heartbeat(m *Miner) {
	heartbeat := time.Duration(m.setting.HeartBeat/2) * time.Millisecond
	_ignored := false
	for {
		select {
		case <-time.After(heartbeat):

			m.sconn.Call("RServer.HeartBeat", m.publicKey, &_ignored)
		}
	}
}

func maintainPeers(m *Miner) {
	sleepInterval := 5 * time.Second

	for {
		numPeers := atomic.LoadInt64(&m.peersCount)
		fmt.Println("numPeers: ", numPeers)
		fmt.Println("m.setting.MinNumMinerConnections", m.setting.MinNumMinerConnections)
		if numPeers < int64(m.setting.MinNumMinerConnections) {
			fmt.Println("Call server to get peers")
			GRequest := m.publicKey
			GReply := make([]net.Addr, 0, 255)
			err := m.sconn.Call("RServer.GetNodes", GRequest, &GReply)
			handleError(err)
			for _, paddr := range GReply {
				_, ok := m.peers.Load(paddr.String())
				if !ok {
					fmt.Println("dial", paddr.String())
					pconn, err := rpc.Dial("tcp", paddr.String())
					fmt.Println(err)
					if err != nil {
						continue
					}
					reply := false
					err = pconn.Call("Miner.AddPeer", m.laddr, &reply)
					if err != nil || reply != true {
						continue
					}
					m.peers.Store(paddr.String(), &Peer{pconn})
					atomic.AddInt64(&m.peersCount, 1)
				}
			}

		}
		time.Sleep(sleepInterval)
	}
}

func computeBlockHash(b Block) string {
	bytes, err := json.Marshal(b)
	if err != nil {
		fmt.Println("marshal error", err)
	}
	h := md5.New()
	h.Write(bytes)
	str := hex.EncodeToString(h.Sum(nil))
	return str
}

func computeOpHash(b Op) string {
	bytes, err := json.Marshal(b)
	if err != nil {
		fmt.Println("marshal error", err)
	}
	h := md5.New()
	h.Write(bytes)
	str := hex.EncodeToString(h.Sum(nil))
	return str
}
func generateBlock(m *Miner) {
	newNonce := uint32(0)

	for {

		var difficulty uint8
		var expectedStr string

		m.mutex.Lock()
		if len(m.curTxns) == 0 {
			difficulty = m.setting.PoWDifficultyNoOpBlock
		} else {
			difficulty = m.setting.PoWDifficultyOpBlock
		}
		expectedStr = strings.Repeat("0", int(difficulty))
		txnsInBlock := copyTransactionMap(m.curTxns)
		newBlock := Block{m.lastBlockOnLongestChain.Hash, txnsInBlock, m.publicKey, newNonce}

		/*\if newNonce % 500 == 0{
			fmt.Println("prevhash",m.lastBlockOnLongestChain.Hash)
		}*/
		hash := computeBlockHash(newBlock)
		tail := hash[32-difficulty: 32]
		if tail == expectedStr {
			fmt.Println("I found a Block!!")
			fmt.Println(newBlock)
			fmt.Println(hash)

			newInkRemaining := copyInkMap(m.lastBlockOnLongestChain.InkRemainingMap)
			newShapeMap := copyShapeMap(m.lastBlockOnLongestChain.ShapesMap)
			newSignatureMap := copySignatureMap(m.lastBlockOnLongestChain.SignaturesMap)
			grantInk := m.setting.InkPerNoOpBlock
			if len(newBlock.Txns) > 0 {
				grantInk = m.setting.InkPerOpBlock
			}
			pubKeyStr := pubKeyToString(m.publicKey)
			newInkRemaining[pubKeyStr] = newInkRemaining[pubKeyStr] + uint64(grantInk)
			containArtNodeTxn := false
			for _, v := range newBlock.Txns {
				vpubKeyStr := pubKeyToString(v.PubKey)
				if v.Sig.toString() == m.artNodeTxn{
					fmt.Println("I generate a block that contains my artnode txn")
					containArtNodeTxn = true
				}
				if v.Op.TxnStr == "ADD" {
					inkUsed := calculateInk(v.Op.Lines, v.Op.Fill, v.Op.Stroke)
					fmt.Println("inkUsed", inkUsed)
					newInkRemaining[vpubKeyStr ] = newInkRemaining[vpubKeyStr] - inkUsed
					newShapeMap[v.Sig.toString()] = Path{v.Op.Lines, v.Op.Fill, v.Op.Stroke, v.Op.ShapeSvgString,vpubKeyStr,inkUsed}
					newSignatureMap[v.Sig.toString()] = true
					delete(m.curTxns,v.Sig.toString())
				} else {
					getBackInk := m.lastBlockOnLongestChain.ShapesMap[v.Op.TxnStr].Ink
					newInkRemaining[vpubKeyStr] = newInkRemaining[vpubKeyStr] + getBackInk
					delete(newShapeMap, v.Op.TxnStr)
					newSignatureMap[v.Sig.toString()] = true
					delete(m.curTxns,v.Sig.toString())
				}
			}
			//m.curTxns = make(map[string]Transaction)
			if containArtNodeTxn{
				m.artNodeTxnBlockHashToCheck = append(m.artNodeTxnBlockHashToCheck,hash)
			}

			newBlockInfo := &BlockInfo{hash, &newBlock, m.lastBlockOnLongestChain.Depth + 1, nil, newInkRemaining, newShapeMap, newSignatureMap}

			if len(m.lastBlockOnLongestChain.ChildBlocks) != 0{
				fmt.Println("generate block, fork")
			}
			m.lastBlockOnLongestChain.ChildBlocks = append(m.lastBlockOnLongestChain.ChildBlocks, hash)

			m.blockChain[hash] = newBlockInfo
			m.lastBlockOnLongestChain = newBlockInfo
			//printMinerBlockChain(m, "generate Block 3 ")
			fmt.Println("generate block",hash)
			m.mutex.Unlock()
			//time.Sleep(time.Second)
			m.sendBlock(newBlock)
		} else {
			m.mutex.Unlock()
			newNonce = newNonce + 1
		}
		time.Sleep(time.Millisecond)
	}
}

func pubKeyToString(key ecdsa.PublicKey) string {
	fmt.Println("pub key to string ", string(elliptic.Marshal(key.Curve, key.X, key.Y)))
	return string(elliptic.Marshal(key.Curve, key.X, key.Y))
}

func validateBlockChainHelper(blockInfo *BlockInfo, blockChain map[string]*BlockInfo, m *Miner) bool {
	// 1. Validate nonce
	//Check that the nonce for the Block is valid: PoW is correct and has the right difficulty.
	difficulty := m.setting.PoWDifficultyOpBlock

	fmt.Println("Block", blockInfo.Block)
	if len(blockInfo.Block.Txns) == 0 {
		difficulty = m.setting.PoWDifficultyNoOpBlock
	}
	expectedStr := strings.Repeat("0", int(difficulty))
	hash := computeBlockHash(*blockInfo.Block)
	tail := hash[32-difficulty: 32]
	if tail != expectedStr {
		fmt.Println("Invalid Block: Invalid nonce", blockInfo.Block)
		return false
	}
	// 2. prev-hash points to a valid Block in the Block chain
	//Check that the previous Block hash points to a legal, previously generated, Block.
	// 3. Validate every operation
	//Check that each operation in the Block has a valid signature (this signature should be generated using the private key and the operation).
	// 3.3 Op with identical signature was not added to the current chain
	//prevBlockInfo := m.blockChain[Msg.Block.PrevHash]
	valid,_,_ := validateBlockAndCreateBlockInfo(*blockInfo.Block,m)
	return valid

}

func validateBlockChain(block *BlockInfo, blockChain map[string]*BlockInfo, m *Miner) bool {
	if block.Hash != m.setting.GenesisBlockHash {
		fmt.Println("not a genesis Block")
		if !validateBlockChainHelper(block, blockChain, m) {
			fmt.Println("Block ", block.Hash, "is invalid")
			return false
		}
	}

	for _, blockHash := range block.ChildBlocks {
		fmt.Println("blockHash", blockHash)
		childBlock, ok := blockChain[blockHash];
		if !ok {
			fmt.Println("cannot find child Block")
			return false
		}
		if !validateBlockChain(childBlock, blockChain, m) {
			return false
		}
	}
	return true
}

// send current canvas to the frontend
func handler(rw http.ResponseWriter, req *http.Request) {
	if origin := req.Header.Get("Origin"); origin != "" {
		rw.Header().Set("Access-Control-Allow-Origin", origin)
		rw.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
		rw.Header().Set("Access-Control-Allow-Headers",
			"Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")
	}
	// Stop here if its Preflighted OPTIONS request
	if req.Method == "OPTIONS" {
		return
	}
	globalMiner.mutex.Lock()

	defer globalMiner.mutex.Unlock()
	svg := constructSVG()
	fmt.Fprintf(rw, svg)

}

func main() {
	args := os.Args[1:]
	if len(args)!=3{
		fmt.Println("1st arg: serverAddr, 2nd arg keyfile number, 3rd myport")
		os.Exit(0)
	}
	serverAddr := args[0]
	keyfile := args[1]
	port:= args[2]
	gob.Register(&net.TCPAddr{})
	gob.Register(&elliptic.CurveParams{})
	mathrand.Seed(time.Now().UnixNano())

	file, err := os.OpenFile("priv" + keyfile, os.O_RDWR|os.O_CREATE, os.ModePerm)
	if err != nil {
		fmt.Println(err)
	}

	file.Seek(0, 0)
	buf := make([]byte, 200)
	size,err := file.Read(buf)
	fmt.Println(err)
	buf = buf[:size]
	file.Close()
	priv,err := x509.ParseECPrivateKey(buf)
	fmt.Println(err)
	handleError(err)

	//Get localIp
	conn, err := net.Dial("udp", serverAddr)
	localAddr := conn.LocalAddr().String()
	conn.Close()
	idx := strings.LastIndex(localAddr, ":")
	lIP := localAddr[0:idx]
	l, err := net.Listen("tcp", lIP+":"+port)
	handleError(err)

	miner := new(Miner)
	globalMiner = miner


	miner.privKey = priv
	miner.publicKey = priv.PublicKey
	miner.laddr = fmt.Sprintf("%v:%v", lIP, l.Addr().(*net.TCPAddr).Port)
	fmt.Println("my address", miner.laddr)
	rpc.Register(miner)
	go rpc.Accept(l)
	laddr, err := net.ResolveTCPAddr("tcp", miner.laddr)
	handleError(err)
	fmt.Println(serverAddr)
	c, err := rpc.Dial("tcp", serverAddr)
	handleError(err)
	RRequest := MinerInfo{Address: laddr, Key: miner.publicKey}
	RReply := MinerNetSettings{}
	err = c.Call("RServer.Register", RRequest, &RReply)
	handleError(err)
	miner.setting = &RReply
	miner.sconn = c
	miner.othersMsgSeqNum = make(map[string]map[uint64]bool)
	miner.blockChain = make(map[string]*BlockInfo)
	genesisBlock := &BlockInfo{Hash: miner.setting.GenesisBlockHash, InkRemainingMap: make(map[string]uint64), ShapesMap: make(map[string]Path), SignaturesMap: make(map[string]bool)}
	miner.blockChain[miner.setting.GenesisBlockHash] = genesisBlock
	miner.unprocessedBlocks =  make(map[string]*Block)
	miner.lastBlockOnLongestChain = miner.blockChain[miner.setting.GenesisBlockHash]
	miner.curTxns = make(map[string]Transaction)
	miner.mutex = &sync.Mutex{}
	go heartbeat(miner)

	for {
		GRequest := miner.publicKey
		GReply := make([]net.Addr, 0, 255)
		err = c.Call("RServer.GetNodes", GRequest, &GReply)
		handleError(err)

		for _, paddr := range GReply {
			fmt.Println("dial", paddr.String())
			_, ok := miner.peers.Load(paddr.String())
			fmt.Println("load", ok)
			if !ok {
				pconn, err := rpc.Dial("tcp", paddr.String())
				fmt.Println(err)
				if err != nil {
					continue
				}
				reply := false
				err = pconn.Call("Miner.AddPeer", miner.laddr, &reply)
				if err != nil || reply != true {
					continue
				}
				miner.peers.Store(paddr.String(), &Peer{pconn})
				fmt.Println("miner.peersCount ", miner.peersCount)
				atomic.AddInt64(&miner.peersCount, 1)
				fmt.Println("miner.peersCount + 1", miner.peersCount)
			}
		}

		numPeers := atomic.LoadInt64(&miner.peersCount)
		fmt.Println("miner.peersCount", miner.peersCount)
		fmt.Println("miner.setting.MinNumMinerConnections", miner.setting.MinNumMinerConnections)
		if int64(miner.setting.MinNumMinerConnections) <= numPeers {
			break
		} else {
			time.Sleep(10 * time.Second)
		}
	}

	 go maintainPeers(miner)

	miner.peers.Range(func(paddr, value interface{}) bool {
		peer, ok := value.(*Peer)
		if !ok {
			fmt.Println("can not retrive peer")
		}
		fmt.Println("Calling peer to retrieve Block chain", paddr)
		reply := GetBlockChainReply{make(map[string]*BlockInfo), ""}
		err := peer.pconn.Call("Miner.GetBlockchain", miner.laddr, &reply)
		fmt.Println("return from calling getblockchain")
		if err != nil {
			fmt.Println("Failed to get blockchain from peer. err", err)
			return true
		}
	//	printBlockChain(reply.BlockChain, paddr.(string),miner)

		genesisBlock, ok := reply.BlockChain[miner.setting.GenesisBlockHash]
		if !ok {
			fmt.Println("Blockchain invalid. Gensis Block does not exist")
			return true
		}
		lastBlock, ok := reply.BlockChain[reply.LastBlockHash]
		if !ok {
			fmt.Println("Blockchain invalid. Last Block does not exist")
			return true
		}
		miner.mutex.Lock()
		miner.blockChain = reply.BlockChain
		blockChainValid := validateBlockChain(genesisBlock, reply.BlockChain, miner)
		if blockChainValid {
			fmt.Println("Got a valid Block chain")
			miner.lastBlockOnLongestChain = lastBlock
			miner.mutex.Unlock()
			atomic.AddInt32(&miner.hasBlockChain, 1)
			atomic.AddInt32(&miner.availableToArtNode, 1)
			return false
		}
		miner.blockChain = make(map[string]*BlockInfo)
		miner.blockChain[miner.setting.GenesisBlockHash] = genesisBlock
		miner.mutex.Unlock()
		return true
	})

	fmt.Println("start generate block")
	go generateBlock(miner)

    // Start server for the frontend
	http.HandleFunc("/", handler)
	http.ListenAndServe(":8080", nil)

	channel := make(chan string)
	<-channel
}

// stubs

func copyInkMap(oldMap map[string]uint64) map[string]uint64 {
	newMap := make(map[string]uint64)
	for key, num := range oldMap {
		newMap[key] = num
	}
	return newMap
}

func copyShapeMap(oldShapMap map[string]Path) map[string]Path {
	newShapeMap := make(map[string]Path)
	for key, shape := range oldShapMap {
		newShapeMap[key] = shape
	}
	return newShapeMap
}

func copySignatureMap(oldMap map[string]bool) map[string]bool {
	newMap := make(map[string]bool)
	for key, value := range oldMap {
		newMap[key] = value
	}
	return newMap
}

func copyTransactionMap(oldMap map[string]Transaction) []Transaction {
	var transactions []Transaction
	for _, value := range oldMap {
		transactions = append(transactions, value)
	}
	return transactions
}

func updateCurTxnOnSwitch(me *BlockInfo, siblingHash string, m *Miner) map[string]Transaction {
	parentHash := me.Block.PrevHash
	parentBlock := m.blockChain[parentHash]
	skipHash := me.Hash

	//added//
	for _, child := range parentBlock.ChildBlocks {
		if child == skipHash {
			continue
		}
		childBlock := m.blockChain[child]
		tempMap := findSibling(childBlock, siblingHash, m)
		if tempMap != nil {
			for _, t := range me.Block.Txns {
				_, ok := tempMap[t.Sig.toString()]
				if ok {
					delete(tempMap, t.Sig.toString())
				}
			}
			return tempMap
		}
	}

	if parentHash == m.setting.GenesisBlockHash {
		fmt.Println("find Common parent, reach genesis Block, blockchain disconnect")
		return nil
	}

	tempMap := updateCurTxnOnSwitch(parentBlock, siblingHash, m)
	if tempMap != nil {
		for _, t := range me.Block.Txns {
			_, ok := tempMap[t.Sig.toString()]
			if ok {
				delete(tempMap, t.Sig.toString())
			}
		}
		return tempMap
	}

	return nil

}

func findSibling(block *BlockInfo, siblingHash string, m *Miner) map[string]Transaction {
	if block.Hash == siblingHash {
		tempMap := make(map[string]Transaction)

		for _, t := range block.Block.Txns {
			tempMap[t.Sig.toString()] = t
		}
		for k, t := range m.curTxns {
			tempMap[k] = t
		}
		return tempMap
	}
	for _, child := range block.ChildBlocks {

		childBlock := m.blockChain[child]
		tempMap := findSibling(childBlock, siblingHash, m)
		if tempMap != nil {
			for _, t := range block.Block.Txns {
				tempMap[t.Sig.toString()] = t
			}
			return tempMap
		}
	}
	return nil
}

func printMinerBlockChain(m *Miner, s string) {
	for k, v := range m.blockChain {
		fmt.Println("k: ", k)
		fmt.Println("hash: ", v.Hash)
		fmt.Println("Depth: ", v.Depth)
		fmt.Println("ChildBlocks: ", v.ChildBlocks)
		fmt.Println("Block", v.Block)
		for k1, v1 := range v.InkRemainingMap {
			fmt.Print("\n inkMapKey Public Key: ")
			fmt.Printf("%v\n ", k1)
			fmt.Println("inkMapValue ink Remaining: ", v1)
		}
		for k1, v1 := range v.ShapesMap {
			fmt.Println("shapesMap Key signature: ", k1)
			fmt.Println("shapesMap Value : ", v1)
		}
		for k1, v1 := range v.SignaturesMap {
			fmt.Println("signature Key : ", k1)
			fmt.Println("signature Value bool=1 : ", v1)
		}

	}

}

func printBlockChain(blockChain map[string]*BlockInfo, s string,m* Miner) {
	fmt.Println("receive Block chain from ", s)
	for k, v := range blockChain {
		fmt.Println("k: ", k)
		fmt.Println("hash: ", v.Hash)
		fmt.Println("Depth: ", v.Depth)
		fmt.Println("ChildBlocks: ", v.ChildBlocks)
		fmt.Println("Block", v.Block)
		if (v.Block != nil){
			if (pubKeyToString(v.Block.MinerPubKey)==(pubKeyToString(m.publicKey))){
				fmt.Println("Miner1 block: ", v.Hash)
			}
		}

		for k1, v1 := range v.InkRemainingMap {
			fmt.Print("\ninkMapKey Public Key: ")
			fmt.Printf("%v\n", k1)
			fmt.Println("inkMapValue ink Remaining: ", v1)
		}
		for k1, v1 := range v.ShapesMap {
			fmt.Println("shapesMap Key signature: ", k1)
			fmt.Println("shapesMap Value : ", v1)
		}
		for k1, v1 := range v.SignaturesMap {
			fmt.Println("signature Key : ", k1)
			fmt.Println("signature Value bool=1 : ", v1)
		}

	}

}

type Point struct {
	X int
	Y int
}
type Line struct {
	Owner  string
	Point1 Point
	Point2 Point
}
type Path struct {
	Lines          []Line
	Fill           string
	Stroke         string
	ShapeSvgString string
	Owner          string
	Ink            uint64
}


func parseSVG(key string, path string, i int, p Point, l []Line) (LineSet []Line, valid bool) {
	path_list := strings.Split(path, " ")
	total_lenth := len(path_list)
	if i >= total_lenth {
		fmt.Println(l)
		return l,true
	}
	temp := path_list[i]
	var tempPoint Point
	if string(temp) == "M" {
		tempx, err := strconv.Atoi(path_list[i+1])
		if err != nil {
			return nil,false
		}
		tempPoint.X = tempx
		tempy, err := strconv.Atoi(path_list[i+2])
		if err != nil {
			return nil,false
		}
		tempPoint.Y = tempy
		return parseSVG(key, path, i+3, tempPoint, l)
	}
	if string(temp) == "m" {
		tempx, err := strconv.Atoi(path_list[i+1])
		if err != nil {
			return l,false
		}
		tempPoint.X = tempx + p.X
		tempy, err := strconv.Atoi(path_list[i+2])
		if err != nil {
			return l,false
		}
		tempPoint.Y = tempy + p.Y
		return parseSVG(key, path, i+3, tempPoint, l)

	}
	if string(temp) == "L" {
		tempx, err := strconv.Atoi(path_list[i+1])
		if err != nil {
			return l,false
		}
		tempPoint.X = tempx
		tempy, err := strconv.Atoi(path_list[i+2])
		if err != nil {
			return l,false
		}
		tempPoint.Y = tempy
		tempLine := makeLine(key, p, tempPoint)
		LineSet = append(l, tempLine)

		return parseSVG(key, path, i+3, tempPoint, LineSet)

	}
	if string(temp) == "l" {
		tempx, err := strconv.Atoi(path_list[i+1])
		if err != nil {
			return l,false
		}
		tempPoint.X = tempx + p.X
		tempy, err := strconv.Atoi(path_list[i+2])
		if err != nil {
			return l,false
		}
		tempPoint.Y = tempy + p.Y
		tempLine := makeLine(key, p, tempPoint)
		LineSet = append(l, tempLine)

		return parseSVG(key, path, i+3, tempPoint, LineSet)

	}
	if string(temp) == "v" {
		tempPoint.X = p.X
		v, err := strconv.Atoi(path_list[i+1])
		if err != nil {
			return l,false
		}
		tempPoint.Y = v + p.Y
		tempLine := makeLine(key, p, tempPoint)
		LineSet = append(l, tempLine)

		return parseSVG(key, path, i+2, tempPoint, LineSet)

	}
	if string(temp) == "V" {
		tempPoint.X = p.X
		v, err := strconv.Atoi(path_list[i+1])
		if err != nil {
			return l,false
		}
		tempPoint.Y = v
		tempLine := makeLine(key, p, tempPoint)
		LineSet = append(l, tempLine)

		return parseSVG(key, path, i+2, tempPoint, LineSet)

	}
	if string(temp) == "h" {
		v, err := strconv.Atoi(path_list[i+1])
		if err != nil {
			return l,false
		}
		tempPoint.X = v + p.X
		tempPoint.Y = p.Y
		tempLine := makeLine(key, p, tempPoint)
		LineSet = append(l, tempLine)

		return parseSVG(key, path, i+2, tempPoint, LineSet)

	}
	if string(temp) == "H" {
		v, err := strconv.Atoi(path_list[i+1])
		if err != nil {
			return l,false
		}
		tempPoint.X = v
		tempPoint.Y = p.Y
		tempLine := makeLine(key, p, tempPoint)
		LineSet = append(l, tempLine)

		return parseSVG(key, path, i+2, tempPoint, LineSet)

	}
	if string(temp) == "z" || string(temp) == "Z" {
		tempx, err := strconv.Atoi(path_list[1])
		if err != nil {
			return l,false
		}
		tempPoint.X = tempx
		tempy, err := strconv.Atoi(path_list[2])
		if err != nil {
			return l,false
		}
		tempPoint.Y = tempy
		tempLine := makeLine(key, p, tempPoint)
		LineSet = append(l, tempLine)
		return parseSVG(key, path, i+1, tempPoint, LineSet)

	}
	return nil,false
}
func makeLine(tkey string, p1 Point, p2 Point) Line {
	var l Line
	l.Owner = tkey
	l.Point1 = p1
	l.Point2 = p2
	return l
}


func PointInPoly(pt Point,ptOwner string,lines []Line) bool {
	if ptOwner == lines[0].Owner{
		return false
	}
	i := len(lines) - 1
	j := len(lines) - 1
	oddNodes := false
	corners := getPoints(lines)
	x := pt.X
	y := pt.Y
	for i = 0; i < len(corners)-1; i++ {
		polyXi := corners[i].X
		polyYi := corners[i].Y
		polyXj := corners[j].X
		polyYj := corners[j].Y

		if (polyYi < y && polyYj >= y || polyYj < y && polyYi >= y) && (polyXi <= x || polyXj <= x) {
			oddNodes = (oddNodes || (polyXi+(y-polyYi)/(polyYj-polyYi)*(polyXj-polyXi) < x)) && !(oddNodes && (polyXi+(y-polyYi)/(polyYj-polyYi)*(polyXj-polyXi) < x))
		}
		j = i;
	}
	fmt.Println("在里面吗", oddNodes)
	return oddNodes
}
func isselfOverlap_filled(lines []Line, fill string)bool{
	if fill == TRANSPARENT{
		return false
	}
	for i:=0;i<=len(lines)-1;i++{
		for j := i+1; j <= len(lines)-1; j++ {
			isIntersected, interceptPoint:= isInter(lines[i], lines[j])
			if isIntersected{
				if !(lines[i].Point1.X == interceptPoint.X && lines[i].Point1.Y == interceptPoint.Y ||
					lines[i].Point2.X == interceptPoint.X && lines[i].Point2.Y == interceptPoint.Y ||
					lines[j].Point1.X == interceptPoint.X && lines[j].Point1.Y == interceptPoint.Y ||
					lines[j].Point2.X == interceptPoint.X && lines[j].Point2.Y == interceptPoint.Y) {
					return true
				}
			}
		}
	}
	return false

}


func isLineIntersectWithLines(l Line, ls []Line) (bool) {
	if l.Owner == ls[0].Owner {
		return false
	}
	for i := 0; i <= len(ls)-1; i++ {
		isIntersected,_:= isInter(l, ls[i])
		if isIntersected{
			return true
		}
	}
	return false
}

//return false if two lines do not intersact or they are same writer and no color
func isInter(l1 Line, l2 Line) (bool,Point) {
	point1_x := l1.Point1.X
	point1_y := l1.Point1.Y
	point2_x := l1.Point2.X
	point2_y := l1.Point2.Y

	linePoint1_x := l2.Point1.X
	linePoint1_y := l2.Point1.Y
	linePoint2_x := l2.Point2.X
	linePoint2_y := l2.Point2.Y

	var denominator = (point1_y-point2_y)*(linePoint1_x-linePoint2_x) - (point2_x-point1_x)*(linePoint2_y-linePoint1_y)
	if denominator == 0 {
		return false,Point{}
	}
	var x = ((point1_x-point2_x)*(linePoint1_x-linePoint2_x)*(linePoint2_y-point2_y) +
		(point1_y-point2_y)*(linePoint1_x-linePoint2_x)*point2_x - (linePoint1_y - linePoint2_y) *
		(point1_x - point2_x) * linePoint2_x) / denominator
	var y = -((point1_y-point2_y)*(linePoint1_y-linePoint2_y)*(linePoint2_x-point2_x) +
		(point1_x-point2_x)*(linePoint1_y-linePoint2_y)*point2_y - (linePoint1_x - linePoint2_x) *
		(point1_y - point2_y) * linePoint2_y) / denominator
	if (x-point2_x)*(x-point1_x) <= 0 && (y-point2_y)*(y-point1_y) <= 0 && (x-linePoint2_x)*(x-linePoint1_x) <= 0 &&
		(y-linePoint2_y)*(y-linePoint1_y) <= 0 {
		return true,Point{x,y}
	}

	return false,Point{}
}

func (p Point)equal(p1 Point)bool{
	return p.X == p1.X && p.Y == p1.Y
}

func isClosed(lines []Line) bool {
	lastp := len(lines) - 1
	if !lines[0].Point1.equal(lines[lastp].Point2) {
		return false
	}
	for i:=0;i<=len(lines)-2;i++{
		if !lines[i].Point2.equal(lines[i+1].Point1) {
			return false
		}
	}
	return true
}
func calculatelength(ls []Line) uint64 {
	var tlength float64
	tlength = 0
	for i := 0; i <= len(ls)-1; i++ {
		cline := ls[i]
		p1 := cline.Point1
		p2 := cline.Point2
		d := math.Sqrt(float64((p1.X-p2.X)*(p1.X-p2.X) + (p1.Y-p2.Y)*(p1.Y-p2.Y)))
		tlength = tlength + d
	}
	return uint64(tlength)
}

func calculateInk(lines []Line, filled string,stroke string) (uint64) {
	inkUsed := uint64(0)
	if filled != TRANSPARENT{
		inkUsed = inkUsed + calculateArea(lines)
	}
	if stroke != TRANSPARENT{
		inkUsed = inkUsed + calculatelength(lines)
	}
	return inkUsed
}

func det(p0 Point, p1 Point) (res int) {
	res += p0.X * p1.Y;
	res -= p0.Y * p1.X;
	return res
}

func getPoints(linelist []Line) (points []Point) {
	points = append(points, linelist[0].Point1)
	for i := 0; i < len(linelist); i++ {
		points = append(points, linelist[i].Point2)
	}
	return points
}

func calculateArea(linelist []Line) (uint64) {
	pointlist := getPoints(linelist)
	totalPoints := len(pointlist)
	area := 0
	for i := 1; i < totalPoints; i++ {
		p1 := pointlist[i-1]
		p2 := pointlist[i]
		area += det(p1, p2)
	}
	temp := float64(area)
	return uint64(math.Abs(temp / 2))
}


func constructSVG() string{
	ps := globalMiner.lastBlockOnLongestChain.ShapesMap
	width := fmt.Sprintf("%v",globalMiner.setting.CanvasSettings.CanvasXMax)
	height := fmt.Sprintf("%v",globalMiner.setting.CanvasSettings.CanvasYMax)
	a := "<svg width=\"" +width + "\" height=\"" + height + "\">"
	g:=	"</svg>"
	final := a

	for _,v:=range ps {
		final = final + constructSVGPath(v.ShapeSvgString,v.Stroke,v.Fill)
	}
	final = final + g
	fmt.Println(final)
	return final
}

func constructSVGPath(dstring string,stroke string,fill string)string{
	b := "<path d=\""
	c := "\" stroke=\""
	e := "\" fill=\""
	f := "\" />"
	return b + dstring + c + stroke + e +fill + f

}