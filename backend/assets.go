package backend

var deployScript []byte
var serverBinary []byte

func Init(deploy, server []byte) {
	deployScript = deploy
	serverBinary = server
}
