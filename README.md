# BlockArt
A fault-tolerant POW blockchain system for collaborative computer art projects written in Go.  Please see [Description.pdf](Description.pdf) for more information about the project.

## Run Server
`go run server.go -c config.json`

## Run Ink Miner
`go run ink-miner.go [server ip:port][pubKey][privKey]`
 * [server ip:port] : the IP:port address of the server in the BlockArt network.
 * [pubKey] [privKey]: the public and private key pair (represented as two
string command line arguments) that this miner should use to validate
connecting art nodes and towards which it should credit mined ink.  

*Note*: Ink Miners must start after Server. 

## Run example client
`go run art-app.go`

## View Canvas
Open `frontend/index.html`

