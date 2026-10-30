package main

import (
	"bytes"
	"fmt"
	"os"
)

func main() {
	acct := &Account{
		Nonce:   1,
		Balance: 100,
		Name:    "ab",
		Code:    []byte{0x60, 0x00},
		note:    "hidden",
	}

	var generated Writer
	acct.EncodeGen(&generated)

	var reflected Writer
	if err := EncodeReflect(acct, &reflected); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	fmt.Printf("EncodeGen     %s\n", generated.Hex())
	fmt.Printf("EncodeReflect %s\n", reflected.Hex())
	fmt.Printf("字节相同       %v\n", bytes.Equal(generated.Dump(), reflected.Dump()))

	fmt.Println()
	var viaCache Writer
	if err := EncodeCached(acct, &viaCache); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	var viaCacheAgain Writer
	if err := EncodeCached(acct, &viaCacheAgain); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("Account 经缓存 %s\n", viaCache.Hex())
	fmt.Printf("与 EncodeGen 相同 %v\n", bytes.Equal(viaCache.Dump(), generated.Dump()))

	fmt.Println()
	tr := &Transfer{Amount: 7, Note: "go"}
	var transfer Writer
	if err := EncodeCached(tr, &transfer); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	var transferAgain Writer
	if err := EncodeCached(tr, &transferAgain); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("Transfer 经缓存 %s\n", transfer.Hex())
	fmt.Printf("两次字节相同     %v\n", bytes.Equal(transfer.Dump(), transferAgain.Dump()))
}
