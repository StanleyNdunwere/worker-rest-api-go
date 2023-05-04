package main

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/savsgio/gotils/uuid"
)

// Create a REST endpoint that creates users with IDs as counts (i.e  user1 :1  user2 :2 user3 :3).
// Users should all have a balance of 1000 once created
// Users should go through verification & should be put in a verification queue
// Verification should be processed periodically by X amount of workers (goroutines) that are spun up in X amount of time e.g every 30s
// Create an endpoint that creates transactions taking in sender userID, receiver UserID & amount to send
// The transactions should be put in a queue that will be processed
// Transaction should be processed periodically by X amount of workers (goroutines) that are spun up in X amount of time e.g every 30s
// Transactions should only be processed for verified users - If user is unverified, user should be pushed to the verification queue and verified
// Create an endpoint that returns all users and their balances and verification status
// BONUS: Figure out a way that user will not be verified even if pushed to the verification queue.
//
// User
// ID
// Name
// Balance
// VerificationStatus [true / false]

var userStore map[string]*User
var verificationStore map[string]Empty
var transactionStore map[string]*TransactionReq

func main() {
	app := fiber.New()

	userStore := make(map[string]*User)
	verificationStore = make(map[string]Empty)
	transactionStore = make(map[string]*TransactionReq)

	go func() {
		SpinWorkerForProcessingVerification(1, &verificationStore, &userStore)
	}()

	go func() {
		SpinWorkersForProcessingTransactions(1, &transactionStore, &verificationStore, &userStore)
	}()

	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendString("Hello, World!")
	})

	app.Post("/user/create", func(c *fiber.Ctx) error {
		userDestruct := UserRequest{}
		err := json.Unmarshal(c.Body(), &userDestruct)

		if err != nil {
			return c.JSON(ResponseBody{
				Status: "error",
				Error:  "failed to create user",
				Data:   User{},
			})
		}
		newUser := User{
			ID:                 uuid.V4(),
			Name:               userDestruct.Name,
			Balance:            1000,
			VerificationStatus: false,
		}
		userStore[newUser.ID] = &newUser
		verificationStore[newUser.ID] = Empty{}
		fmt.Println(verificationStore)
		for _, val := range userStore {
			fmt.Println("the user verification value: ", *val)
		}
		return c.JSON(newUser)
	})

	app.Post("/send", func(c *fiber.Ctx) error {
		transactionRequest := TransactionReq{}
		err := json.Unmarshal(c.Body(), &transactionRequest)
		if err != nil {
			c.JSON(ResponseBody{
				Status: "error",
				Error:  "transaction failed",
				Data:   nil,
			})
		}

		transactionStore[generateTransactionId(transactionRequest.SenderId, transactionRequest.ReceiverId)] = &transactionRequest

		for _, val := range transactionStore {
			fmt.Println("the transaction value: ", val)
		}

		return c.JSON(ResponseBody{
			Status: "success",
			Data:   "Transaction queued for processing",
		})
	})

	app.Get("/users", func(c *fiber.Ctx) error {
		return c.JSON(ResponseBody{
			Status: "success",
			Data:   userStore,
		})
	})

	app.Listen(":3500")

}

func generateTransactionId(senderId, receiverId string) string {
	return senderId + "-" + receiverId
}

func SpinWorkerForProcessingVerification(workersCount int, verificationStore *map[string]Empty, userStore *map[string]*User) {
	fmt.Println("started")
	spun := 0
	ticker := time.NewTicker(10 * time.Second)
	for t := range ticker.C {
		verificationChannel := make(chan string)
		createJobs := make(chan bool)
		go func(createJobs chan bool) {
			for key := range *verificationStore {
				fmt.Println("the key ", key)
				verificationChannel <- key
			}
			createJobs <- true
			fmt.Println("called crated jobs")
		}(createJobs)

		fmt.Println("Timer recreated at ", t)
		spun = 0
		for spun < workersCount {
			go func(spun int, channel chan string) {
				fmt.Printf("Go rountine started for verification: %v\n\n", spun+1)
				select {
				case userId, present := <-channel:
					if present {
						if user, ok := (*userStore)[userId]; ok {
							user.VerificationStatus = true
							fmt.Println(user)
							delete(*verificationStore, user.ID)
							return
						}
					} else {
						fmt.Println("Channel is terminated")
						return
					}
				default:
					fmt.Println("Channel is empty")
					<-createJobs
				}
			}(spun, verificationChannel)
			spun += 1
		}
	}
}

func SpinWorkersForProcessingTransactions(
	workersCount int,
	transactionStore *map[string]*TransactionReq,
	verificationStore *map[string]Empty,
	userStore *map[string]*User,
) {
	fmt.Println("transaction processing workers")
	spun := 0
	ticker1 := time.NewTicker(10 * time.Second)
	for t := range ticker1.C {
		transactionChannel := make(chan TransactionReq)
		createTransJob := make(chan bool)

		go func(createJobs chan bool) {
			for _, val := range *transactionStore {
				transactionChannel <- *val
			}
			createJobs <- true
		}(createTransJob)

		fmt.Println("Timer for transaction processing recreated at ", t)
		spun = 0
		for spun < workersCount {
			go func(spun int, channel chan TransactionReq) {
				fmt.Printf("Go rountine started for transaction processing: %v\n\n", spun+1)

				request := <-channel
				delete(*transactionStore, generateTransactionId(request.SenderId, request.ReceiverId))
				sender, senderOk := (*userStore)[request.SenderId]
				receiver, receiverOk := (*userStore)[request.ReceiverId]

				if !senderOk || !receiverOk {
					fmt.Println("sender or receiver does not exist in records... deleting")
					delete(*transactionStore, generateTransactionId(request.SenderId, request.ReceiverId))
					return
				}
				if !receiver.VerificationStatus {
					(*verificationStore)[receiver.ID] = Empty{}
					fmt.Println("receiver verification status is false... deleting")
					delete(*transactionStore, generateTransactionId(request.SenderId, request.ReceiverId))
					return
				}
				if !sender.VerificationStatus {
					(*verificationStore)[sender.ID] = Empty{}
					fmt.Println("sender verification status is false... deleting")
					delete(*transactionStore, generateTransactionId(request.SenderId, request.ReceiverId))
					return
				}

				if (sender.Balance - request.Amount) >= 0 {
					sender.Balance -= request.Amount
					receiver.Balance += request.Amount
				}
				fmt.Println("processing done ... deleting")
				delete(*transactionStore, generateTransactionId(request.SenderId, request.ReceiverId))
				return

			}(spun, transactionChannel)
			spun += 1
		}
	}
}
