package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

func main() {
	url := "https://tlcfjcuudswmgqfpcmzr.supabase.co/auth/v1/.well-known/jwks.json"
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("apikey", "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpc3MiOiJzdXBhYmFzZSIsInJlZiI6InRsY2ZqY3V1ZHN3bWdxZnBjbXpyIiwicm9sZSI6ImFub24iLCJpYXQiOjE3NzQzNjIxNTcsImV4cCI6MjA4OTkzODE1N30.TLxkUebisW04DPBUT9x4MYheKIEdEaXoPtAgNcm6Xbo")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	
	var data map[string]interface{}
	json.Unmarshal(body, &data)
	
	keys := data["keys"].([]interface{})
	firstKey := keys[0].(map[string]interface{})
	
	fmt.Printf("ALG: %v\n", firstKey["alg"])
	fmt.Printf("KID: %v\n", firstKey["kid"])
	fmt.Printf("X: %v\n", firstKey["x"])
	fmt.Printf("Y: %v\n", firstKey["y"])
}
