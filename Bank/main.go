package main

import (
	"fmt"
	"nomad/Bank/accounts"
)

func main() {
	account := accounts.NewAccount("nico")
	account.Deposit(10)
	//err := account.Withdraw(20)
	//if err != nil {
	//	fmt.Println(err)
	//}
	//fmt.Println(account.Balance(), account.Owner())
	fmt.Println(account)
}
