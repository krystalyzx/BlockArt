/*

This package specifies the application's interface to the the BlockArt
library (blockartlib) to be used in project 1 of UBC CS 416 2017W2.

*/

package blockartlib

import "crypto/ecdsa"
import (
	"fmt"
	"net/rpc"
	"strconv"
	"crypto/rand"
	"math/big"
)

// Represents a type of shape in the BlockArt system.
type ShapeType int

const (
	// Path shape.
	PATH ShapeType = iota

	// Circle shape (extra credit).
	// CIRCLE
)

// Settings for a canvas in BlockArt.
type CanvasSettings struct {
	// Canvas dimensions
	CanvasXMax uint32
	CanvasYMax uint32
}

// Settings for an instance of the BlockArt project/network.
type MinerNetSettings struct {
	// Hash of the very first (empty) block in the chain.
	GenesisBlockHash string

	// The minimum number of ink miners that an ink miner should be
	// connected to. If the ink miner dips below this number, then
	// they have to retrieve more nodes from the server using
	// GetNodes().
	MinNumMinerConnections uint8

	// Mining ink reward per op and no-op blocks (>= 1)
	InkPerOpBlock   uint32
	InkPerNoOpBlock uint32

	// Number of milliseconds between heartbeat messages to the server.
	HeartBeat uint32

	// Proof of work difficulty: number of zeroes in prefix (>=0)
	PoWDifficultyOpBlock   uint8
	PoWDifficultyNoOpBlock uint8

	// Canvas settings
	CanvasSettings CanvasSettings
}

////////////////////////////////////////////////////////////////////////////////////////////
// <ERROR DEFINITIONS>

// These type definitions allow the application to explicitly check
// for the kind of error that occurred. Each API call below lists the
// errors that it is allowed to raise.
//
// Also see:
// https://blog.golang.org/error-handling-and-go
// https://blog.golang.org/errors-are-values

// Contains address IP:port that art node cannot connect to.
type DisconnectedError string

func (e DisconnectedError) Error() string {
	return fmt.Sprintf("BlockArt: cannot connect to [%s]", string(e))
}

// Contains amount of ink remaining.
type InsufficientInkError uint32

func (e InsufficientInkError) Error() string {
	return fmt.Sprintf("BlockArt: Not enough ink to addShape [%d]", uint32(e))
}

// Contains the offending svg string.
type InvalidShapeSvgStringError string

func (e InvalidShapeSvgStringError) Error() string {
	return fmt.Sprintf("BlockArt: Bad shape svg string [%s]", string(e))
}

// Contains the offending svg string.
type ShapeSvgStringTooLongError string

func (e ShapeSvgStringTooLongError) Error() string {
	return fmt.Sprintf("BlockArt: Shape svg string too long [%s]", string(e))
}

// Contains the bad shape hash string.
type InvalidShapeHashError string

func (e InvalidShapeHashError) Error() string {
	return fmt.Sprintf("BlockArt: Invalid shape hash [%s]", string(e))
}

// Contains the bad shape hash string.
type ShapeOwnerError string

func (e ShapeOwnerError) Error() string {
	return fmt.Sprintf("BlockArt: Shape owned by someone else [%s]", string(e))
}

// Empty
type OutOfBoundsError struct{}

func (e OutOfBoundsError) Error() string {
	return fmt.Sprintf("BlockArt: Shape is outside the bounds of the canvas")
}

// Contains the hash of the shape that this shape overlaps with.
type ShapeOverlapError string

func (e ShapeOverlapError) Error() string {
	return fmt.Sprintf("BlockArt: Shape overlaps with a previously added shape [%s]", string(e))
}

// Contains the invalid block hash.
type InvalidBlockHashError string

func (e InvalidBlockHashError) Error() string {
	return fmt.Sprintf("BlockArt: Invalid block hash [%s]", string(e))
}

// </ERROR DEFINITIONS>
////////////////////////////////////////////////////////////////////////////////////////////

// Represents a canvas in the system.
type Canvas interface {
	// Adds a new shape to the canvas.
	// Can return the following errors:
	// - DisconnectedError
	// - InsufficientInkError
	// - InvalidShapeSvgStringError
	// - ShapeSvgStringTooLongError
	// - ShapeOverlapError
	// - OutOfBoundsError
	AddShape(validateNum uint8, shapeType ShapeType, shapeSvgString string, fill string, stroke string) (shapeHash string, blockHash string, inkRemaining uint32, err error)

	// Returns the encoding of the shape as an svg string.
	// Can return the following errors:
	// - DisconnectedError
	// - InvalidShapeHashError
	GetSvgString(shapeHash string) (svgString string, err error)

	// Returns the amount of ink currently available.
	// Can return the following errors:
	// - DisconnectedError
	GetInk() (inkRemaining uint32, err error)

	// Removes a shape from the canvas.
	// Can return the following errors:
	// - DisconnectedError
	// - ShapeOwnerError
	DeleteShape(validateNum uint8, shapeHash string) (inkRemaining uint32, err error)

	// Retrieves hashes contained by a specific block.
	// Can return the following errors:
	// - DisconnectedError
	// - InvalidBlockHashError
	GetShapes(blockHash string) (shapeHashes []string, err error)

	// Returns the block hash of the genesis block.
	// Can return the following errors:
	// - DisconnectedError
	GetGenesisBlock() (blockHash string, err error)

	// Retrieves the children blocks of the block identified by blockHash.
	// Can return the following errors:
	// - DisconnectedError
	// - InvalidBlockHashError
	GetChildren(blockHash string) (blockHashes []string, err error)

	// Closes the canvas/connection to the BlockArt network.
	// - DisconnectedError
	CloseCanvas() (inkRemaining uint32, err error)
}

type ReturnCanvasReply struct{
	MinerNetSettings MinerNetSettings
	Err bool
}

type ReturnCanvasRequest struct{
	R,S *big.Int
	Hash []byte
}

// The constructor for a new Canvas object instance. Takes the miner's
// IP:port address string and a public-private key pair (ecdsa private
// key type contains the public key). Returns a Canvas instance that
// can be used for all future interactions with blockartlib.
//
// The returned Canvas instance is a singleton: an application is
// expected to interact with just one Canvas instance at a time.
//
// Can return the following errors:
// - DisconnectedError
func OpenCanvas(minerAddr string, privKey ecdsa.PrivateKey) (canvas Canvas, setting CanvasSettings, err error) {
	// For now return DisconnectedError
	hash := []byte("test")
	r,s,err:=ecdsa.Sign(rand.Reader,&privKey,hash)
	request := ReturnCanvasRequest{r,s,hash}
		conn, err := rpc.Dial("tcp", minerAddr)
	if err != nil{
		fmt.Println("Dial error",minerAddr)
		return nil, CanvasSettings{}, DisconnectedError(minerAddr)
	}
	reply := new(ReturnCanvasReply)
	err = conn.Call("Miner.ReturnCanvas", request, reply)
	if err != nil{
		fmt.Println("Call ReturnCanvas error",minerAddr)
		return nil, CanvasSettings{}, DisconnectedError(minerAddr)
	}
	if reply.Err == true {

		return nil, CanvasSettings{}, DisconnectedError(minerAddr)

	}

    newCanvas := CanvasObj{reply.MinerNetSettings,conn,minerAddr}
	return newCanvas, reply.MinerNetSettings.CanvasSettings,nil
}
type CanvasObj struct{
	MinerNetSettings MinerNetSettings
	conn *rpc.Client
	minerAddr string
}

type AddShapeRequest struct{
	ValidateNum uint8
	ShapeSvgString string
	Fill string
	Stroke string
}

type AddShapeReply struct{
	ShapeHash string
	BlockHash string
	InkRemaining uint32
	Err int
	ErrStr string
}

// - noError [0]
// - DisconnectedError [1]
// - InsufficientInkError [2]
// - InvalidShapeSvgStringError [3]
// - ShapeSvgStringTooLongError [4]
// - ShapeOverlapError [5]
// - OutOfBoundsError[6]
//InvalidBlockHashError[7]
//ShapeOwnerError[8]
//InvalidShapeHashError[9]

type test struct{

}
func convertError(errCode int,errorStr string)error{
	switch errCode {
	case 1:
		return DisconnectedError(errorStr)

	case 2:
		i, _ := strconv.Atoi(errorStr)
		return InsufficientInkError(uint32(i))
	case 3:
		return InvalidShapeSvgStringError(errorStr)
	case 4:
		return ShapeSvgStringTooLongError(errorStr)
	case 5:
		return ShapeOverlapError(errorStr)
	case 6:
		return OutOfBoundsError(test{})
	case 7:
		return InvalidBlockHashError(errorStr)
	case 8:
		return ShapeOwnerError(errorStr)
	case 9:
		return InvalidShapeHashError(errorStr)
	default:
		fmt.Println("wrong error")
		return nil
	}

}


func (c CanvasObj) AddShape(validateNum uint8, shapeType ShapeType, shapeSvgString string, fill string, stroke string) (shapeHash string, blockHash string, inkRemaining uint32, err error){
	request := AddShapeRequest{validateNum,shapeSvgString,fill,stroke}
	reply := new(AddShapeReply)
	err = c.conn.Call("Miner.AddShape",request,reply)
	fmt.Println(reply)
	if err != nil{

		return "","",0,DisconnectedError(c.minerAddr)
	}
	if reply.Err !=0 {
		return "","",0,convertError(reply.Err,reply.ErrStr)
	}
	return reply.ShapeHash,reply.BlockHash,reply.InkRemaining,nil
}

type GetSvgStringReply struct{
    SvgString string
	Err int
	ErrStr string
}

func (c CanvasObj) GetSvgString(shapeHash string) (svgString string, err error){
	reply:=new (GetSvgStringReply)
	err = c.conn.Call("Miner.GetSvgString",shapeHash,reply)
	if err != nil{
		return "",DisconnectedError(c.minerAddr)
	}
	if reply.Err !=0 {
		return "",convertError(reply.Err,reply.ErrStr)
	}

	return reply.SvgString,nil
}

type GetInkReply struct{
	InkAmount uint32
	Err int
	ErrStr string
}

func (c CanvasObj)GetInk() (inkRemaining uint32, err error){
	reply:=new ( GetInkReply)
	err = c.conn.Call("Miner.GetInk","",reply)
	if err != nil{
		return 0,DisconnectedError(c.minerAddr)
	}
	if reply.Err !=0 {
		return 0,convertError(reply.Err,reply.ErrStr)
	}
	return reply.InkAmount,nil
}


type DeleteShapeRequest struct{
	ValidateNum uint8
	ShapeHash string
}

type DeleteShapeReply struct{
	InkAmount uint32
	Err int
	ErrStr string
}


func (c CanvasObj)DeleteShape(validateNum uint8, shapeHash string) (inkRemaining uint32, err error){
	request := DeleteShapeRequest {validateNum,shapeHash}
	reply := new(DeleteShapeReply)
	err = c.conn.Call("Miner.DeleteShape",request,reply)
	if err != nil{
		return 0,DisconnectedError(c.minerAddr)
	}
	if reply.Err !=0 {
		return 0,convertError(reply.Err,reply.ErrStr)
	}

	return reply.InkAmount,nil
}

type GetShapesReply struct{
	ShapeHashes []string
	Err int
	ErrStr string
}


func (c CanvasObj) GetShapes(blockHash string) (shapeHashes []string, err error){
	request := blockHash
	reply := new(GetShapesReply)
	reply.ShapeHashes = make([]string,0)
	err = c.conn.Call("Miner.GetShapes",request,reply)
	if err != nil{
		return nil,DisconnectedError(c.minerAddr)
	}
	if reply.Err !=0 {
		return nil,convertError(reply.Err,reply.ErrStr)
	}

	return reply.ShapeHashes,nil
}

func (c CanvasObj) GetGenesisBlock() (blockHash string, err error){
	reply:=new ( GetInkReply)
	err = c.conn.Call("Miner.GetInk","",reply)
	if err != nil || reply.Err !=0 {
		return "",DisconnectedError(c.minerAddr)
	}
	return c.MinerNetSettings.GenesisBlockHash,nil
}

type GetChildrenReply struct{
	BlockHashes []string
	Err int
	ErrStr string
}

func (c CanvasObj) GetChildren(blockHash string) (blockHashes []string, err error){
	request := blockHash
	reply := new(GetChildrenReply)
	reply.BlockHashes = make([]string,50)
	err = c.conn.Call("Miner.GetChildren",request,reply)
	if err != nil{
		return nil,DisconnectedError(c.minerAddr)
	}
	if reply.Err !=0 {
		return nil,convertError(reply.Err,reply.ErrStr)
	}
	return reply.BlockHashes,nil

}

type CloseCanvasReply struct{
	InkRemaining uint32
	Err int
	ErrStr string
}

func (c CanvasObj) CloseCanvas() (inkRemaining uint32, err error){
	request := ""
	reply := new(CloseCanvasReply)
	err = c.conn.Call("Miner.CloseCanvas",request,reply)
	if err != nil{
		return 0,DisconnectedError(c.minerAddr)
	}
	if reply.Err !=0 {
		return 0,convertError(reply.Err,reply.ErrStr)
	}
	return reply.InkRemaining,nil
}