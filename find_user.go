package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
)

func main() {
	godotenv.Load(".env")
	dbURL := os.Getenv("SUPABASE_DB_URL")
	pool, err := pgxpool.New(context.Background(), dbURL)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()

	var userID string
	err = pool.QueryRow(context.Background(), "SELECT id FROM auth.users LIMIT 1").Scan(&userID)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(userID)
}
