package main

type Empty struct{}

type User struct {
	ID                 string
	Name               string
	Balance            int
	VerificationStatus bool
}

type UserRequest struct {
	Name string
}

type ResponseBody struct {
	Status string
	Error  string
	Data   interface{}
}

type TransactionReq struct {
	SenderId   string
	ReceiverId string
	Amount     int
}
