//
package main

import (
	"fmt"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"os"
)


func pubKeyToString(key ecdsa.PublicKey) string {
	fmt.Println("pub key to string ",string(elliptic.Marshal(key.Curve, key.X, key.Y)))
	return string(elliptic.Marshal(key.Curve, key.X, key.Y))
}
func main(){

	for i:= 1;i<21;i++{
		istring := fmt.Sprintf("%v",i)
		priv, err := ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
		fmt.Println(err)
		privBytes,err := x509.MarshalECPrivateKey(priv)
		fmt.Println(err)
		fmt.Println(pubKeyToString(priv.PublicKey))
		file, err := os.OpenFile("priv" + istring, os.O_RDWR|os.O_CREATE, os.ModePerm)
		if err != nil {
			fmt.Println(err)
		}
		defer file.Close()
		file.Seek(0, 0)
		file.Write(privBytes)
		fmt.Println("key byte len:",len(privBytes))
	}



}
