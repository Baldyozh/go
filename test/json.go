package test

import (
	"encoding/json"
	"log-processor/config"
)

func FindAndModifyJson(rawJson chan []byte, cryptedJson chan []byte, config config.CryptoConfig) {
	jsonData := <-rawJson
	jsonMap := make(map[string]any)
	err := json.Unmarshal([]byte(jsonData), &jsonMap)
	if err != nil {
		panic(err)
	}
	for _, fieldToEncrypt := range config.FieldsToEncrypt {
		jsonMap[fieldToEncrypt] = "UpdatedField"
	}
	jsonData, err = json.Marshal(&jsonMap)
	if err != nil {
		panic(err)
	}
	cryptedJson <- jsonData

}

/*

msg1 := []byte(`{
  "request_id": "1",
  "timestamp": "2024-03-15T14::30:00Z",
  "partner_id": "bank_api_v1",
  "http_method": "POST",
  "endpoint": "/api/v1/verify",
  "request_body": {
    "user_id": 12345,
    "passport": "4510 123456 ",
    "snils": " 123-456 -789 00"
},
  "response_body":{
    "status": "success",
    "verification_id": "ver_789"
  },
  "http_status": 200,
  "processing_time_ms": 142
}`)
msg2 := []byte(`{
  "request_id": "1",
  "timestamp": "2024-03-15T14::30:00Z",
  "partner_id": "bank_api_v1",
  "http_method": "POST",
  "endpoint": "/api/v1/verify",
  "request_body": {
    "user_id": 12345,
    "passport": "4510 123456 ",
    "snils": " 123-456 -789 00"
},
  "response_body":{
    "status": "success",
    "verification_id": "ver_789"
  },
  "http_status": 200,
  "processing_time_ms": 142
}`)
wg := sync.WaitGroup{}
wg.Add(1)
raw := make(chan []byte, 10)
encryptedJsonChan := make(chan []byte, 10)

msgs := [][]byte{msg1, msg2}
go func() {
	for _, msg := range msgs {
		raw <- msg
		test.FindAndModifyJson(raw, encryptedJsonChan, cfg.Crypto)
	}
	wg.Done()
}()

select {
case message := <-encryptedJsonChan:
fmt.Println(message)
default:
fmt.Println("waiting for message")
}
fmt.Scanln()
wg.Wait()
*/
